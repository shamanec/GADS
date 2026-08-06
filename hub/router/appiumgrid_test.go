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
	"GADS/hub/auth"
	"GADS/hub/devices"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// newGridSessionDevice returns a connected, live device fixture with an active
// Appium session, registered in the hub device store and the session registry.
// Callers must defer the returned cleanup func
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
		AppiumEnabled:            true,
		LastAutomationActionTS:   time.Now().UnixMilli(),
		AppiumNewCommandTimeout:  60000,
	}
	devices.HubDeviceStore.Set(udid, device)
	devices.RegisterSession(sessionID, device)
	return device, func() {
		devices.UnregisterSession(sessionID)
		devices.HubDeviceStore.Delete(udid)
	}
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
		name string
		// Usage-based rejections carry the reason (static config, middleware fails
		// fast on them); runtime rejections use the generic non-leaking message
		wantErrContains string
		mutate          func(d *devices.LocalHubDevice)
	}{
		{"disconnected device", "not available", func(d *devices.LocalHubDevice) { d.Connected = false }},
		{"provider state not live", "not available", func(d *devices.LocalHubDevice) { d.ProviderState = "init" }},
		{"stale provider update", "not available", func(d *devices.LocalHubDevice) { d.LastUpdatedTimestamp = time.Now().UnixMilli() - 10000 }},
		{"usage control", "is not enabled for automation", func(d *devices.LocalHubDevice) { d.Device.Usage = "control" }},
		{"usage disabled", "is not enabled for automation", func(d *devices.LocalHubDevice) { d.Device.Usage = "disabled" }},
		{"provider without Appium servers", "is not enabled for automation", func(d *devices.LocalHubDevice) { d.AppiumEnabled = false }},
		{"locked by another user", "not available", func(d *devices.LocalHubDevice) {
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
				assert.Contains(t, err.Error(), tt.wantErrContains)
			}
		})
	}

	t.Run("usage misconfiguration fails the session request fast with the reason", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		prevGridDB := gridDB
		defer func() { gridDB = prevGridDB }()
		gridDB = &fakeGridStore{
			credential: models.ClientCredentials{UserID: "test-user", Tenant: "tenant1", IsActive: true},
			workspaces: []models.Workspace{{ID: "ws1", Tenant: "tenant1"}},
		}

		udid := "udid-control-usage-device"
		device, cleanup := newGridSessionDevice(udid, "", "fake-host")
		defer cleanup()
		device.SessionID = ""
		device.IsRunningAutomation = false
		device.IsAvailableForAutomation = true
		device.Device.Usage = "control"

		sessionBody := `{"capabilities":{"alwaysMatch":{"platformName":"Android","gads:clientSecret":"test-secret","appium:udid":"` + udid + `"}}}`
		router := newGridTestRouter()
		req, _ := http.NewRequest("POST", "/grid/session", bytes.NewBufferString(sessionBody))
		start := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Must not sit in the 10-second no-device retry loop - usage cannot change by waiting
		assert.Less(t, time.Since(start), 2*time.Second)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "is not enabled for automation")
		assert.Contains(t, w.Body.String(), "session not created")
	})

	t.Run("generic search skips devices whose provider has no Appium servers", func(t *testing.T) {
		udid := "generic-no-appium-device"
		device, cleanup := newGridSessionDevice(udid, "", "fake-host")
		defer cleanup()
		device.SessionID = ""
		device.IsRunningAutomation = false
		device.IsAvailableForAutomation = true
		device.AppiumEnabled = false

		found, err := findAvailableDevice(gridCandidate{PlatformName: "Android"}, []string{"ws1"}, "test-user", "tenant1")
		assert.Nil(t, found)
		assert.Error(t, err)
	})

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

		_, stillRegistered := devices.DeviceBySession("orphan-session")
		assert.False(t, stillRegistered)
	})

	t.Run("provider-reported command activity keeps a hub-quiet session alive", func(t *testing.T) {
		device, cleanup := newGridSessionDevice("janitor-provider-active-device", "provider-active-session", "fake-host")
		defer cleanup()
		// Idle-expired by hub-side traffic alone, but the plugin saw a command just now
		device.AppiumNewCommandTimeout = 100
		device.LastAutomationActionTS = time.Now().UnixMilli() - 5000
		device.ProviderReportsSessionState = true
		device.ProviderHasSession = true
		device.ProviderLastCommandTS = time.Now().UnixMilli()

		sweepExpiredGridSessions()

		device.Mu.RLock()
		defer device.Mu.RUnlock()
		assert.True(t, device.IsRunningAutomation)
		assert.Equal(t, "provider-active-session", device.SessionID)
	})

	t.Run("provider command activity is ignored for old providers without session truth", func(t *testing.T) {
		device, cleanup := newGridSessionDevice("janitor-old-provider-device", "old-provider-session", "fake-host")
		defer cleanup()
		device.AppiumNewCommandTimeout = 100
		device.LastAutomationActionTS = time.Now().UnixMilli() - 5000
		device.ProviderReportsSessionState = false
		device.ProviderLastCommandTS = time.Now().UnixMilli()

		sweepExpiredGridSessions()

		device.Mu.RLock()
		defer device.Mu.RUnlock()
		assert.False(t, device.IsRunningAutomation)
		assert.True(t, device.IsAvailableForAutomation)
	})

	t.Run("session the provider has reported gone for over 10s is released without a provider DELETE", func(t *testing.T) {
		providerRequests := make(chan string, 1)
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				providerRequests <- r.URL.Path
			}
			w.Write([]byte(`{"value":null}`))
		}))
		defer fakeProvider.Close()

		device, cleanup := newGridSessionDevice("janitor-gone-session-device", "gone-session", strings.TrimPrefix(fakeProvider.URL, "http://"))
		defer cleanup()
		device.ProviderReportsSessionState = true
		device.ProviderHasSession = false
		device.ProviderSessionMissingSinceTS = time.Now().UnixMilli() - 11000

		sweepExpiredGridSessions()

		device.Mu.RLock()
		assert.False(t, device.IsRunningAutomation)
		assert.True(t, device.IsAvailableForAutomation)
		assert.Equal(t, "", device.SessionID)
		device.Mu.RUnlock()

		// The provider already reported the session gone - deleting it there is pointless
		select {
		case path := <-providerRequests:
			t.Fatalf("the janitor sent an unnecessary session DELETE to the provider - %s", path)
		case <-time.After(500 * time.Millisecond):
		}
	})

	t.Run("session missing on the provider for under 10s survives the grace period", func(t *testing.T) {
		device, cleanup := newGridSessionDevice("janitor-grace-device", "grace-session", "fake-host")
		defer cleanup()
		device.ProviderReportsSessionState = true
		device.ProviderHasSession = false
		device.ProviderSessionMissingSinceTS = time.Now().UnixMilli() - 3000

		sweepExpiredGridSessions()

		device.Mu.RLock()
		defer device.Mu.RUnlock()
		assert.True(t, device.IsRunningAutomation)
		assert.Equal(t, "grace-session", device.SessionID)
	})
}

func TestSyncDeviceFieldsProviderTruth(t *testing.T) {
	t.Run("session truth fields are copied and missing-session tracking starts", func(t *testing.T) {
		device := &devices.LocalHubDevice{
			SessionID:           "hub-session",
			IsRunningAutomation: true,
		}
		syncDeviceFields(device, &models.ProviderDeviceSync{
			Connected:                 true,
			ProviderState:             "live",
			ReportsAppiumSessionState: true,
			HasAppiumSession:          false,
			AppiumLastCommandTS:       1234,
		})

		assert.True(t, device.ProviderReportsSessionState)
		assert.False(t, device.ProviderHasSession)
		assert.Equal(t, int64(1234), device.ProviderLastCommandTS)
		assert.NotZero(t, device.ProviderSessionMissingSinceTS)

		// The missing-since timestamp must hold its first value across pushes, not
		// restart the grace period every second
		firstMissingTS := device.ProviderSessionMissingSinceTS
		syncDeviceFields(device, &models.ProviderDeviceSync{
			Connected:                 true,
			ProviderState:             "live",
			ReportsAppiumSessionState: true,
			HasAppiumSession:          false,
		})
		assert.Equal(t, firstMissingTS, device.ProviderSessionMissingSinceTS)

		// The provider reporting the session again clears the tracking
		syncDeviceFields(device, &models.ProviderDeviceSync{
			Connected:                 true,
			ProviderState:             "live",
			ReportsAppiumSessionState: true,
			HasAppiumSession:          true,
			AppiumSessionID:           "hub-session",
		})
		assert.True(t, device.ProviderHasSession)
		assert.Equal(t, "hub-session", device.ProviderSessionID)
		assert.Zero(t, device.ProviderSessionMissingSinceTS)
	})

	t.Run("no tracking starts without a hub-side session", func(t *testing.T) {
		device := &devices.LocalHubDevice{}
		syncDeviceFields(device, &models.ProviderDeviceSync{
			Connected:                 true,
			ProviderState:             "live",
			ReportsAppiumSessionState: true,
			HasAppiumSession:          false,
		})
		assert.Zero(t, device.ProviderSessionMissingSinceTS)
	})

	t.Run("old provider without the marker clears any tracking", func(t *testing.T) {
		device := &devices.LocalHubDevice{
			SessionID:                     "hub-session",
			IsRunningAutomation:           true,
			ProviderSessionMissingSinceTS: 1234,
		}
		syncDeviceFields(device, &models.ProviderDeviceSync{Connected: true, ProviderState: "live"})

		assert.False(t, device.ProviderReportsSessionState)
		assert.Zero(t, device.ProviderSessionMissingSinceTS)
	})
}

func TestGridStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevGridDB := gridDB
	defer func() { gridDB = prevGridDB }()
	gridDB = &fakeGridStore{
		credential: models.ClientCredentials{UserID: "status-user", Tenant: "tenant1", IsActive: true},
		workspaces: []models.Workspace{{ID: "ws1", Tenant: "tenant1"}},
	}

	// statusRequest hits /grid/status, with the client secret header when given
	statusRequest := func(secret string) *httptest.ResponseRecorder {
		router := newGridTestRouter()
		req, _ := http.NewRequest("GET", "/grid/status", bytes.NewBufferString(""))
		if secret != "" {
			req.Header.Set("Authorization", "Bearer "+secret)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	t.Run("no devices means not ready", func(t *testing.T) {
		w := statusRequest("")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"ready":false`)
		assert.Contains(t, w.Body.String(), "no devices available")
	})

	t.Run("without credentials only readiness is reported, never the device list", func(t *testing.T) {
		_, cleanup := newGridSessionDevice("status-anon-device", "", "fake-host")
		defer cleanup()

		w := statusRequest("")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"ready":true`)
		assert.Contains(t, w.Body.String(), `"queued":0`)
		assert.NotContains(t, w.Body.String(), "devices")
		assert.NotContains(t, w.Body.String(), "status-anon-device")
	})

	t.Run("an invalid client secret is rejected", func(t *testing.T) {
		prevStore := gridDB
		defer func() { gridDB = prevStore }()
		gridDB = &fakeGridStore{credentialErr: fmt.Errorf("not found")}

		w := statusRequest("wrong-secret")

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid client credentials")
	})

	t.Run("live device makes the grid ready and is listed with credentials", func(t *testing.T) {
		device, cleanup := newGridSessionDevice("status-live-device", "", "fake-host")
		defer cleanup()
		device.Device.Name = "Status Phone"
		device.Device.Provider = "provider1"
		device.SessionID = ""
		device.IsRunningAutomation = false
		device.IsAvailableForAutomation = true

		w := statusRequest("status-secret")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"ready":true`)
		assert.Contains(t, w.Body.String(), "GADS grid ready")
		assert.Contains(t, w.Body.String(), `"udid":"status-live-device"`)
		assert.Contains(t, w.Body.String(), `"name":"Status Phone"`)
		assert.Contains(t, w.Body.String(), `"provider":"provider1"`)
		assert.Contains(t, w.Body.String(), `"available":true`)
		assert.Contains(t, w.Body.String(), `"in_use_by_automation":false`)
	})

	t.Run("devices outside the credential's workspaces are not listed and do not count as ready", func(t *testing.T) {
		device, cleanup := newGridSessionDevice("status-foreign-device", "", "fake-host")
		defer cleanup()
		device.SessionID = ""
		device.IsRunningAutomation = false
		device.IsAvailableForAutomation = true
		device.Device.WorkspaceID = "other-tenant-ws"

		w := statusRequest("status-secret")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "status-foreign-device")
		assert.Contains(t, w.Body.String(), `"ready":false`)
	})

	t.Run("device running automation is ready but not available", func(t *testing.T) {
		_, cleanup := newGridSessionDevice("status-busy-device", "status-busy-session", "fake-host")
		defer cleanup()

		w := statusRequest("status-secret")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"ready":true`)
		assert.Contains(t, w.Body.String(), `"available":false`)
		assert.Contains(t, w.Body.String(), `"in_use_by_automation":true`)
	})

	t.Run("device on a provider without Appium servers is not listed at all", func(t *testing.T) {
		device, cleanup := newGridSessionDevice("status-no-appium-device", "", "fake-host")
		defer cleanup()
		device.SessionID = ""
		device.IsRunningAutomation = false
		device.IsAvailableForAutomation = true
		device.AppiumEnabled = false

		w := statusRequest("status-secret")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "status-no-appium-device")
		// A grid whose only device cannot serve automation is not ready either
		assert.Contains(t, w.Body.String(), `"ready":false`)
	})
}

func TestGetAutomationSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("lists active sessions with device and user info plus availability and queue depth", func(t *testing.T) {
		device, cleanup := newGridSessionDevice("sessions-list-device", "sessions-list-session", "fake-host")
		defer cleanup()
		device.Device.Name = "Sessions Phone"
		device.InUseBy = "session-user"
		device.InUseByTenant = "tenant1"
		device.AutomationSessionStartTS = time.Now().UnixMilli()

		// A second, idle device - available counts the whole reachable pool, so
		// both this and the busy session device above are included
		freeDevice, freeCleanup := newGridSessionDevice("sessions-free-device", "", "fake-host")
		defer freeCleanup()
		freeDevice.SessionID = ""
		freeDevice.IsRunningAutomation = false
		freeDevice.IsAvailableForAutomation = true

		// A disconnected iOS device - not reachable, so its OS reports zero
		offlineIOSDevice, offlineIOSCleanup := newGridSessionDevice("sessions-offline-ios-device", "", "fake-host")
		defer offlineIOSCleanup()
		offlineIOSDevice.SessionID = ""
		offlineIOSDevice.IsRunningAutomation = false
		offlineIOSDevice.Device.OS = "ios"
		offlineIOSDevice.Connected = false

		router := gin.New()
		router.GET("/automation-sessions", GetAutomationSessions)
		req, _ := http.NewRequest("GET", "/automation-sessions", bytes.NewBufferString(""))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"session_id":"sessions-list-session"`)
		assert.Contains(t, w.Body.String(), `"device_udid":"sessions-list-device"`)
		assert.Contains(t, w.Body.String(), `"device_name":"Sessions Phone"`)
		assert.Contains(t, w.Body.String(), `"device_os":"android"`)
		assert.Contains(t, w.Body.String(), `"device_os_version":"14.0.0"`)
		assert.Contains(t, w.Body.String(), `"in_use_by":"session-user"`)
		assert.Contains(t, w.Body.String(), `"started_ts"`)
		assert.Contains(t, w.Body.String(), `"last_command_ts"`)
		// Every supported OS is present - the ones without reachable devices as
		// explicit zeroes; the busy session device still counts for android
		assert.Contains(t, w.Body.String(), `"available_devices_by_os":{"android":2,"androidtv":0,"ios":0,"roku":0,"tizen":0,"webos":0}`)
		assert.Contains(t, w.Body.String(), `"queued":0`)
	})

	t.Run("requires authentication", func(t *testing.T) {
		router := gin.New()
		router.Use(auth.AuthMiddleware())
		router.GET("/automation-sessions", GetAutomationSessions)
		req, _ := http.NewRequest("GET", "/automation-sessions", bytes.NewBufferString(""))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestEnrichSessionResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevGridDB := gridDB
	defer func() { gridDB = prevGridDB }()
	gridDB = &fakeGridStore{
		credential: models.ClientCredentials{UserID: "test-user", Tenant: "tenant1", IsActive: true},
		workspaces: []models.Workspace{{ID: "ws1", Tenant: "tenant1"}},
	}

	newEnrichmentDevice := func(udid string, providerHost string) (*devices.LocalHubDevice, func()) {
		device := &devices.LocalHubDevice{
			Device: models.DBDevice{
				UDID:        udid,
				OS:          "android",
				OSVersion:   "14.0.0",
				WorkspaceID: "ws1",
				Name:        "Enrichment Phone",
				Provider:    "provider1",
			},
			Host:                     providerHost,
			Connected:                true,
			ProviderState:            "live",
			LastUpdatedTimestamp:     time.Now().UnixMilli(),
			IsAvailableForAutomation: true,
			AppiumEnabled:            true,
		}
		devices.HubDeviceStore.Set(udid, device)
		return device, func() { devices.HubDeviceStore.Delete(udid) }
	}

	createSession := func(t *testing.T, udid string, requestMutators ...func(req *http.Request)) string {
		t.Helper()
		router := newGridTestRouter()
		sessionBody := `{"capabilities":{"alwaysMatch":{"platformName":"Android","gads:clientSecret":"test-secret","appium:udid":"` + udid + `"}}}`
		req, _ := http.NewRequest("POST", "/grid/session", bytes.NewBufferString(sessionBody))
		req.Host = "hub.example.com:10000"
		for _, mutate := range requestMutators {
			mutate(req)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		return w.Body.String()
	}

	t.Run("device info caps are injected and Appium caps survive", func(t *testing.T) {
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"value":{"sessionId":"enrich-session","capabilities":{"platformName":"Android","appium:automationName":"UiAutomator2","appium:deviceApiLevel":34}}}`))
		}))
		defer fakeProvider.Close()

		udid := "enrichment-device"
		_, cleanup := newEnrichmentDevice(udid, strings.TrimPrefix(fakeProvider.URL, "http://"))
		defer cleanup()
		defer devices.UnregisterSession("enrich-session")

		body := createSession(t, udid)

		// Everything Appium returned must survive the enrichment untouched
		assert.Contains(t, body, `"sessionId":"enrich-session"`)
		assert.Contains(t, body, `"appium:automationName":"UiAutomator2"`)
		assert.Contains(t, body, `"appium:deviceApiLevel":34`)
		// The grid's own capabilities are added
		assert.Contains(t, body, `"gads:deviceUdid":"enrichment-device"`)
		assert.Contains(t, body, `"gads:deviceName":"Enrichment Phone"`)
		assert.Contains(t, body, `"gads:provider":"provider1"`)
		// The control link is built from the address the client reached the hub on
		assert.Contains(t, body, `"gads:controlUrl":"http://hub.example.com:10000/devices/control/enrichment-device"`)
		// The client secret must never be echoed back
		assert.NotContains(t, body, "test-secret")
	})

	t.Run("control URL respects X-Forwarded-Proto from a TLS-terminating proxy", func(t *testing.T) {
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"value":{"sessionId":"enrich-url-session","capabilities":{"platformName":"Android"}}}`))
		}))
		defer fakeProvider.Close()

		udid := "enrichment-url-device"
		_, cleanup := newEnrichmentDevice(udid, strings.TrimPrefix(fakeProvider.URL, "http://"))
		defer cleanup()
		defer devices.UnregisterSession("enrich-url-session")

		body := createSession(t, udid, func(req *http.Request) {
			req.Host = "gads.example.com"
			req.Header.Set("X-Forwarded-Proto", "https")
		})

		assert.Contains(t, body, `"gads:controlUrl":"https://gads.example.com/devices/control/enrichment-url-device"`)
	})

	t.Run("webSocketUrl is stripped - BiDi is not supported", func(t *testing.T) {
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"value":{"sessionId":"enrich-bidi-session","capabilities":{"platformName":"Android","webSocketUrl":"ws://localhost:4723/bidi/enrich-bidi-session","appium:automationName":"UiAutomator2"}}}`))
		}))
		defer fakeProvider.Close()

		udid := "enrichment-bidi-device"
		_, cleanup := newEnrichmentDevice(udid, strings.TrimPrefix(fakeProvider.URL, "http://"))
		defer cleanup()
		defer devices.UnregisterSession("enrich-bidi-session")

		body := createSession(t, udid)

		// The provider-localhost WS URL must never reach the client
		assert.NotContains(t, body, "webSocketUrl")
		assert.NotContains(t, body, "ws://")
		// The session is otherwise normal - other caps and the enrichment survive
		assert.Contains(t, body, `"sessionId":"enrich-bidi-session"`)
		assert.Contains(t, body, `"appium:automationName":"UiAutomator2"`)
		assert.Contains(t, body, `"gads:deviceUdid":"enrichment-bidi-device"`)
	})
}

func TestGridCreateSessionStampsIdentityAtClaimTime(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevGridDB := gridDB
	defer func() { gridDB = prevGridDB }()
	gridDB = &fakeGridStore{
		credential: models.ClientCredentials{UserID: "claim-user", Tenant: "tenant1", IsActive: true},
		workspaces: []models.Workspace{{ID: "ws1", Tenant: "tenant1"}},
	}

	newClaimDevice := func(udid string, providerHost string) (*devices.LocalHubDevice, func()) {
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
			IsAvailableForAutomation: true,
			AppiumEnabled:            true,
		}
		devices.HubDeviceStore.Set(udid, device)
		return device, func() { devices.HubDeviceStore.Delete(udid) }
	}

	createSession := func(udid string) *httptest.ResponseRecorder {
		router := newGridTestRouter()
		sessionBody := `{"capabilities":{"alwaysMatch":{"platformName":"Android","gads:clientSecret":"test-secret","appium:udid":"` + udid + `"}}}`
		req, _ := http.NewRequest("POST", "/grid/session", bytes.NewBufferString(sessionBody))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	t.Run("the automation user is visible on the device before the provider responds", func(t *testing.T) {
		udid := "claim-stamp-device"
		// Capture what the device selection UI would show while the provider is
		// still busy creating the session
		var inUseByDuringCreate, inUseByTenantDuringCreate string
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claimedDevice, _ := devices.HubDeviceStore.Get(udid)
			claimedDevice.Mu.RLock()
			inUseByDuringCreate = claimedDevice.InUseBy
			inUseByTenantDuringCreate = claimedDevice.InUseByTenant
			claimedDevice.Mu.RUnlock()
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"value":{"sessionId":"claim-stamp-session","capabilities":{"platformName":"Android"}}}`))
		}))
		defer fakeProvider.Close()

		_, cleanup := newClaimDevice(udid, strings.TrimPrefix(fakeProvider.URL, "http://"))
		defer cleanup()
		defer devices.UnregisterSession("claim-stamp-session")

		w := createSession(udid)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "claim-user", inUseByDuringCreate)
		assert.Equal(t, "tenant1", inUseByTenantDuringCreate)
	})

	t.Run("a failed session create clears the stamped identity", func(t *testing.T) {
		// A non-JSON provider response aborts the claim after the identity stamp
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`not json`))
		}))
		defer fakeProvider.Close()

		device, cleanup := newClaimDevice("claim-abort-device", strings.TrimPrefix(fakeProvider.URL, "http://"))
		defer cleanup()

		w := createSession("claim-abort-device")
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		device.Mu.RLock()
		defer device.Mu.RUnlock()
		assert.Equal(t, "", device.InUseBy)
		assert.Equal(t, "", device.InUseByTenant)
		assert.False(t, device.IsRunningAutomation)
		assert.True(t, device.IsAvailableForAutomation)
	})
}

func TestGridSessionLifecycleFlow(t *testing.T) {
	// Full create -> command -> delete flow through the real routes, asserting the
	// session registry entry appears and disappears with the session
	gin.SetMode(gin.TestMode)

	prevGridDB := gridDB
	defer func() { gridDB = prevGridDB }()
	gridDB = &fakeGridStore{
		credential: models.ClientCredentials{UserID: "test-user", Tenant: "tenant1", IsActive: true},
		workspaces: []models.Workspace{{ID: "ws1", Tenant: "tenant1"}},
	}

	var proxiedRequests []string
	fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxiedRequests = append(proxiedRequests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/appium/session") {
			w.Write([]byte(`{"value":{"sessionId":"flow-session","capabilities":{"platformName":"Android"}}}`))
			return
		}
		w.Write([]byte(`{"value":null}`))
	}))
	defer fakeProvider.Close()

	udid := "lifecycle-flow-device"
	device := &devices.LocalHubDevice{
		Device: models.DBDevice{
			UDID:        udid,
			OS:          "android",
			OSVersion:   "14.0.0",
			WorkspaceID: "ws1",
		},
		Host:                     strings.TrimPrefix(fakeProvider.URL, "http://"),
		Connected:                true,
		ProviderState:            "live",
		LastUpdatedTimestamp:     time.Now().UnixMilli(),
		IsAvailableForAutomation: true,
		AppiumEnabled:            true,
	}
	devices.HubDeviceStore.Set(udid, device)
	defer devices.HubDeviceStore.Delete(udid)
	defer devices.UnregisterSession("flow-session")

	router := newGridTestRouter()

	sessionBody := `{"capabilities":{"alwaysMatch":{"platformName":"Android","gads:clientSecret":"test-secret"}}}`
	req, _ := http.NewRequest("POST", "/grid/session", bytes.NewBufferString(sessionBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	registeredDevice, ok := devices.DeviceBySession("flow-session")
	if assert.True(t, ok, "the created session must be indexed in the session registry") {
		assert.Same(t, device, registeredDevice)
	}

	req, _ = http.NewRequest("POST", "/grid/session/flow-session/url", bytes.NewBufferString(`{"url":"https://example.com"}`))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, proxiedRequests, "POST /device/lifecycle-flow-device/appium/session/flow-session/url")

	// A bare `GET /session/{id}` is an ordinary command, not a session end
	req, _ = http.NewRequest("GET", "/grid/session/flow-session", bytes.NewBufferString(""))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, proxiedRequests, "GET /device/lifecycle-flow-device/appium/session/flow-session")
	_, ok = devices.DeviceBySession("flow-session")
	assert.True(t, ok)

	req, _ = http.NewRequest("DELETE", "/grid/session/flow-session", bytes.NewBufferString(""))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, proxiedRequests, "DELETE /device/lifecycle-flow-device/appium/session/flow-session")

	device.Mu.RLock()
	assert.True(t, device.IsAvailableForAutomation)
	device.Mu.RUnlock()

	// The full release (session cleared, registry entry removed) happens only after
	// the post-session cool-down
	assert.Eventually(t, func() bool {
		_, stillRegistered := devices.DeviceBySession("flow-session")
		device.Mu.RLock()
		defer device.Mu.RUnlock()
		return !stillRegistered && !device.IsRunningAutomation && device.SessionID == ""
	}, postSessionReleaseCooldown+3*time.Second, 100*time.Millisecond)
}
