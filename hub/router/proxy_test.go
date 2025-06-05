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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestDeviceProxyHandler(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Initialize the global devices data structure if not already done
	if devices.HubDevicesData.Devices == nil {
		devices.HubDevicesData.Devices = make(map[string]*models.LocalHubDevice)
	}

	t.Run("Available Device - Should Proxy Normally", func(t *testing.T) {
		// Setup an available device
		udid := "test-device-available"
		devices.HubDevicesData.Mu.Lock()
		devices.HubDevicesData.Devices[udid] = &models.LocalHubDevice{
			Device: models.Device{
				UDID: udid,
				Host: "localhost:8080",
			},
			Available: true,
		}
		devices.HubDevicesData.Mu.Unlock()

		// Create request
		router := gin.New()
		router.GET("/device/:udid/*path", DeviceProxyHandler)

		req, _ := http.NewRequest("GET", "/device/"+udid+"/status", nil)
		w := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(w, req)

		// Note: This will fail because there's no actual server at localhost:8080
		// but we're testing that it doesn't return 422 (passes availability check)
		assert.NotEqual(t, http.StatusUnprocessableEntity, w.Code)

		// Cleanup
		devices.HubDevicesData.Mu.Lock()
		delete(devices.HubDevicesData.Devices, udid)
		devices.HubDevicesData.Mu.Unlock()
	})

	t.Run("Unavailable Device - Should Return 422", func(t *testing.T) {
		// Setup an unavailable device
		udid := "test-device-unavailable"
		devices.HubDevicesData.Mu.Lock()
		devices.HubDevicesData.Devices[udid] = &models.LocalHubDevice{
			Device: models.Device{
				UDID: udid,
				Host: "localhost:8080",
			},
			Available: false,
		}
		devices.HubDevicesData.Mu.Unlock()

		// Create request
		router := gin.New()
		router.GET("/device/:udid/*path", DeviceProxyHandler)

		req, _ := http.NewRequest("GET", "/device/"+udid+"/status", nil)
		w := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(w, req)

		// Verify status code
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

		// Verify response body
		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "Device with UDID `test-device-unavailable` is not available", response["error"])

		// Cleanup
		devices.HubDevicesData.Mu.Lock()
		delete(devices.HubDevicesData.Devices, udid)
		devices.HubDevicesData.Mu.Unlock()
	})

	t.Run("Non-existent Device - Should Return 400", func(t *testing.T) {
		// Create request for non-existent device
		router := gin.New()
		router.GET("/device/:udid/*path", DeviceProxyHandler)

		req, _ := http.NewRequest("GET", "/device/non-existent-udid/status", nil)
		w := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(w, req)

		// Verify status code (existing behavior should be maintained)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Verify response contains expected error message
		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response["error"], "Device with UDID `non-existent-udid` not found")
	})

	t.Run("Device In Use By Another User - Should Return 409", func(t *testing.T) {
		// Setup a device in use by another user
		udid := "test-device-in-use"
		currentTime := time.Now().UnixMilli()
		devices.HubDevicesData.Mu.Lock()
		devices.HubDevicesData.Devices[udid] = &models.LocalHubDevice{
			Device: models.Device{
				UDID: udid,
				Host: "localhost:8080",
			},
			Available: true,
			InUseBy:   "another-user",
			InUseTS:   currentTime, // Use current time to simulate active session
		}
		devices.HubDevicesData.Mu.Unlock()

		// Create request
		router := gin.New()
		router.GET("/device/:udid/*path", DeviceProxyHandler)

		req, _ := http.NewRequest("GET", "/device/"+udid+"/status", nil)
		w := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(w, req)

		// Verify status code (existing behavior should be maintained)
		assert.Equal(t, http.StatusConflict, w.Code)

		// Cleanup
		devices.HubDevicesData.Mu.Lock()
		delete(devices.HubDevicesData.Devices, udid)
		devices.HubDevicesData.Mu.Unlock()
	})
}
