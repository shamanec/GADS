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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"GADS/common/models"
	"GADS/hub/devices"
)

func postProviderUpdate(t *testing.T, payload models.ProviderData) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/provider-update", ProviderUpdate)

	body, err := json.Marshal(payload)
	assert.NoError(t, err)

	req, _ := http.NewRequest("POST", "/provider-update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProviderUpdateEphemeralDevices(t *testing.T) {
	emulatorUdid := "emu_test-provider_Pixel_8_API_35"

	ephemeralSync := func() models.ProviderDeviceSync {
		return models.ProviderDeviceSync{
			UDID:          emulatorUdid,
			Host:          "192.168.1.5:10001",
			Connected:     true,
			ProviderState: "live",
			Ephemeral:     true,
			EphemeralDevice: &models.DBDevice{
				UDID:         emulatorUdid,
				OS:           "android",
				Name:         "Pixel_8_API_35",
				OSVersion:    "15",
				Provider:     "test-provider",
				Usage:        "enabled",
				DeviceType:   "emulator",
				ScreenWidth:  "1080",
				ScreenHeight: "2400",
				WorkspaceID:  "default-workspace-id",
			},
		}
	}

	t.Run("unknown ephemeral device is upserted into the store", func(t *testing.T) {
		defer devices.HubDeviceStore.Delete(emulatorUdid)

		postProviderUpdate(t, models.ProviderData{
			ProviderData: models.Provider{Nickname: "test-provider", SetupAppiumServers: true},
			DeviceData:   []models.ProviderDeviceSync{ephemeralSync()},
		})

		hubDevice, ok := devices.HubDeviceStore.Get(emulatorUdid)
		assert.True(t, ok, "expected ephemeral device to be created in the store")
		assert.True(t, hubDevice.Ephemeral)
		assert.True(t, hubDevice.Connected)
		assert.Equal(t, "live", hubDevice.ProviderState)
		assert.Equal(t, "192.168.1.5:10001", hubDevice.Host)
		assert.Equal(t, "Pixel_8_API_35", hubDevice.Device.Name)
		assert.Equal(t, "emulator", hubDevice.Device.DeviceType)
		assert.Equal(t, "default-workspace-id", hubDevice.Device.WorkspaceID)
		assert.True(t, hubDevice.AppiumEnabled)
		assert.NotZero(t, hubDevice.LastEphemeralReportTS)
	})

	t.Run("subsequent update refreshes descriptor and report timestamp", func(t *testing.T) {
		defer devices.HubDeviceStore.Delete(emulatorUdid)

		postProviderUpdate(t, models.ProviderData{
			ProviderData: models.Provider{Nickname: "test-provider", SetupAppiumServers: true},
			DeviceData:   []models.ProviderDeviceSync{ephemeralSync()},
		})
		hubDevice, ok := devices.HubDeviceStore.Get(emulatorUdid)
		assert.True(t, ok)
		firstReportTS := hubDevice.LastEphemeralReportTS

		// Screen size is detected during device setup on the provider, so a later
		// update can carry dimensions the initial one did not have yet
		updated := ephemeralSync()
		updated.EphemeralDevice.ScreenWidth = "1440"
		updated.EphemeralDevice.ScreenHeight = "3120"
		postProviderUpdate(t, models.ProviderData{
			ProviderData: models.Provider{Nickname: "test-provider", SetupAppiumServers: true},
			DeviceData:   []models.ProviderDeviceSync{updated},
		})

		hubDevice, ok = devices.HubDeviceStore.Get(emulatorUdid)
		assert.True(t, ok)
		assert.Equal(t, "1440", hubDevice.Device.ScreenWidth)
		assert.GreaterOrEqual(t, hubDevice.LastEphemeralReportTS, firstReportTS)
	})

	t.Run("provider without Appium servers yields AppiumEnabled false", func(t *testing.T) {
		defer devices.HubDeviceStore.Delete(emulatorUdid)

		postProviderUpdate(t, models.ProviderData{
			ProviderData: models.Provider{Nickname: "test-provider", SetupAppiumServers: false},
			DeviceData:   []models.ProviderDeviceSync{ephemeralSync()},
		})

		hubDevice, ok := devices.HubDeviceStore.Get(emulatorUdid)
		assert.True(t, ok)
		assert.False(t, hubDevice.AppiumEnabled)
	})

	t.Run("unknown non-ephemeral device is still skipped", func(t *testing.T) {
		unknownUdid := "unknown-real-device"
		postProviderUpdate(t, models.ProviderData{
			ProviderData: models.Provider{Nickname: "test-provider", SetupAppiumServers: true},
			DeviceData: []models.ProviderDeviceSync{{
				UDID:          unknownUdid,
				Connected:     true,
				ProviderState: "live",
			}},
		})

		_, ok := devices.HubDeviceStore.Get(unknownUdid)
		assert.False(t, ok, "non-ephemeral unknown device must not be upserted")
	})

	t.Run("ephemeral flag without descriptor does not create an entry", func(t *testing.T) {
		malformed := ephemeralSync()
		malformed.EphemeralDevice = nil
		postProviderUpdate(t, models.ProviderData{
			ProviderData: models.Provider{Nickname: "test-provider", SetupAppiumServers: true},
			DeviceData:   []models.ProviderDeviceSync{malformed},
		})

		_, ok := devices.HubDeviceStore.Get(emulatorUdid)
		assert.False(t, ok, "ephemeral device without a descriptor must be skipped")
	})
}
