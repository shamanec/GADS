/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"GADS/common/models"
	"GADS/hub/devices"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// newGridSessionDevice returns a connected, live device fixture with an active
// Appium session, registered in the hub device store. Callers must defer the
// returned cleanup func
func newGridSessionDevice(udid string, sessionID string, providerHost string) (*devices.LocalHubDevice, func()) {
	device := &devices.LocalHubDevice{
		Device: models.DBDevice{
			UDID:        udid,
			OS:          "android",
			OSVersion:   "14.0.0",
			WorkspaceID: "ws1",
		},
		Host:                     providerHost,
		Connected:                true,
		ProviderState:            "live",
		LastUpdatedTimestamp:     time.Now().UnixMilli(),
		SessionID:                sessionID,
		IsRunningAutomation:      true,
		IsAvailableForAutomation: false,
		LastAutomationActionTS:   time.Now().UnixMilli(),
		AppiumNewCommandTimeout:  60000,
	}
	devices.HubDeviceStore.Set(udid, device)
	return device, func() { devices.HubDeviceStore.Delete(udid) }
}

func TestGridSessionDeleteRouting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DELETE on a session subpath proxies the command and does not release the device", func(t *testing.T) {
		var proxiedPath string
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			proxiedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"value":null}`))
		}))
		defer fakeProvider.Close()

		device, cleanup := newGridSessionDevice("delete-subpath-device", "subpath-session", strings.TrimPrefix(fakeProvider.URL, "http://"))
		defer cleanup()

		router := newGridTestRouter()
		req, _ := http.NewRequest("DELETE", "/grid/session/subpath-session/actions", bytes.NewBufferString(""))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/device/delete-subpath-device/appium/session/subpath-session/actions", proxiedPath)

		device.Mu.RLock()
		defer device.Mu.RUnlock()
		assert.Equal(t, "subpath-session", device.SessionID)
		assert.True(t, device.IsRunningAutomation)
		assert.False(t, device.IsAvailableForAutomation)
	})

	t.Run("exact session DELETE releases the device", func(t *testing.T) {
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"value":null}`))
		}))
		defer fakeProvider.Close()

		device, cleanup := newGridSessionDevice("delete-session-device", "ending-session", strings.TrimPrefix(fakeProvider.URL, "http://"))
		defer cleanup()

		router := newGridTestRouter()
		req, _ := http.NewRequest("DELETE", "/grid/session/ending-session", bytes.NewBufferString(""))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		device.Mu.RLock()
		defer device.Mu.RUnlock()
		assert.True(t, device.IsAvailableForAutomation)
	})
}

func TestFindAvailableDeviceByUDIDEligibility(t *testing.T) {
	// The UDID-pinned path must enforce the same eligibility checks as the generic
	// search - a pinned device that is disconnected, controlled or locked by
	// another user must not be handed out
	tests := []struct {
		name   string
		mutate func(d *devices.LocalHubDevice)
	}{
		{"disconnected device", func(d *devices.LocalHubDevice) { d.Connected = false }},
		{"provider state not live", func(d *devices.LocalHubDevice) { d.ProviderState = "init" }},
		{"stale provider update", func(d *devices.LocalHubDevice) { d.LastUpdatedTimestamp = time.Now().UnixMilli() - 10000 }},
		{"usage control", func(d *devices.LocalHubDevice) { d.Device.Usage = "control" }},
		{"usage disabled", func(d *devices.LocalHubDevice) { d.Device.Usage = "disabled" }},
		{"locked by another user", func(d *devices.LocalHubDevice) {
			d.InUseBy = "other-user"
			d.InUseByTenant = "tenant1"
			d.InUseTS = time.Now().UnixMilli()
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			udid := "udid-eligibility-device"
			device, cleanup := newGridSessionDevice(udid, "", "fake-host")
			defer cleanup()
			device.SessionID = ""
			device.IsRunningAutomation = false
			device.IsAvailableForAutomation = true
			tt.mutate(device)

			found, err := findAvailableDevice(gridCandidate{DeviceUDID: udid}, []string{"ws1"}, "test-user", "tenant1")
			assert.Nil(t, found)
			if assert.Error(t, err) {
				assert.Equal(t, "Device is currently not available for automation", err.Error())
			}
		})
	}

	t.Run("eligible pinned device is claimed", func(t *testing.T) {
		udid := "udid-eligible-device"
		device, cleanup := newGridSessionDevice(udid, "", "fake-host")
		defer cleanup()
		device.SessionID = ""
		device.IsRunningAutomation = false
		device.IsAvailableForAutomation = true

		found, err := findAvailableDevice(gridCandidate{DeviceUDID: udid}, []string{"ws1"}, "test-user", "tenant1")
		assert.NoError(t, err)
		if assert.NotNil(t, found) {
			found.Mu.RLock()
			defer found.Mu.RUnlock()
			assert.Equal(t, udid, found.Device.UDID)
			assert.False(t, found.IsAvailableForAutomation)
		}
	})
}

func TestSweepExpiredGridSessions(t *testing.T) {
	t.Run("idled-out session is reset and the provider Appium session is deleted", func(t *testing.T) {
		providerRequests := make(chan string, 1)
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				providerRequests <- r.URL.Path
			}
			w.Write([]byte(`{"value":null}`))
		}))
		defer fakeProvider.Close()

		device, cleanup := newGridSessionDevice("janitor-expired-device", "expired-session", strings.TrimPrefix(fakeProvider.URL, "http://"))
		defer cleanup()
		device.AppiumNewCommandTimeout = 100
		device.LastAutomationActionTS = time.Now().UnixMilli() - 5000

		sweepExpiredGridSessions()

		device.Mu.RLock()
		assert.False(t, device.IsRunningAutomation)
		assert.True(t, device.IsAvailableForAutomation)
		assert.Equal(t, "", device.SessionID)
		device.Mu.RUnlock()

		// The provider-side session DELETE is fired in a background goroutine
		select {
		case path := <-providerRequests:
			assert.Equal(t, "/device/janitor-expired-device/appium/session/expired-session", path)
		case <-time.After(3 * time.Second):
			t.Fatal("the janitor never sent the session DELETE to the provider")
		}
	})

	t.Run("newCommandTimeout 0 disables the idle expiry", func(t *testing.T) {
		device, cleanup := newGridSessionDevice("janitor-disabled-timeout-device", "long-lived-session", "fake-host")
		defer cleanup()
		device.AppiumNewCommandTimeout = 0
		device.LastAutomationActionTS = time.Now().UnixMilli() - 24*60*60*1000

		sweepExpiredGridSessions()

		device.Mu.RLock()
		defer device.Mu.RUnlock()
		assert.True(t, device.IsRunningAutomation)
		assert.Equal(t, "long-lived-session", device.SessionID)
	})

	t.Run("disconnected device is reset without contacting the provider", func(t *testing.T) {
		device, cleanup := newGridSessionDevice("janitor-disconnected-device", "orphan-session", "fake-host")
		defer cleanup()
		device.Connected = false

		sweepExpiredGridSessions()

		device.Mu.RLock()
		defer device.Mu.RUnlock()
		assert.False(t, device.IsRunningAutomation)
		assert.True(t, device.IsAvailableForAutomation)
		assert.Equal(t, "", device.SessionID)
	})
}
