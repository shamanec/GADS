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

	t.Run("Available Device - Should Proxy Normally", func(t *testing.T) {
		// Setup an available device
		udid := "test-device-available"
		devices.HubDeviceStore.Set(udid, &devices.LocalHubDevice{
			Device: models.DBDevice{
				UDID: udid,
			},
			Host:      "localhost:8080",
			Available: true,
		})

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
		devices.HubDeviceStore.Delete(udid)
	})

	t.Run("Unavailable Device - Should Return 422", func(t *testing.T) {
		// Setup an unavailable device
		udid := "test-device-unavailable"
		devices.HubDeviceStore.Set(udid, &devices.LocalHubDevice{
			Device: models.DBDevice{
				UDID: udid,
			},
			Host:      "localhost:8080",
			Available: false,
		})

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
		assert.Equal(t, "Device `test-device-unavailable` is not available", response["error"])

		// Cleanup
		devices.HubDeviceStore.Delete(udid)
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
		devices.HubDeviceStore.Set(udid, &devices.LocalHubDevice{
			Device: models.DBDevice{
				UDID: udid,
			},
			Host:      "localhost:8080",
			Available: true,
			InUseBy:   "another-user",
			InUseTS:   currentTime, // Use current time to simulate active session
		})

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
		devices.HubDeviceStore.Delete(udid)
	})

	// NOTE: client-credential enforcement for session creation lives on the /grid
	// surface (GridCreateSession) and is covered by gridcapabilities_test.go -
	// DeviceProxyHandler itself performs no credential checks
}

func TestExtractClientSecretFromSession(t *testing.T) {
	t.Run("Extract from capabilities.alwaysMatch", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"alwaysMatch": map[string]interface{}{
					"gads:clientSecret": "test-secret",
				},
			},
		}

		clientSecret := models.ExtractClientSecretFromSession(sessionReq, "gads")
		assert.Equal(t, "test-secret", clientSecret)
	})

	t.Run("Extract from desiredCapabilities", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"desiredCapabilities": map[string]interface{}{
				"gads:clientSecret": "test-secret",
			},
		}

		clientSecret := models.ExtractClientSecretFromSession(sessionReq, "gads")
		assert.Equal(t, "test-secret", clientSecret)
	})

	t.Run("Custom prefix extraction", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"alwaysMatch": map[string]interface{}{
					"custom:clientSecret": "test-secret",
				},
			},
		}

		clientSecret := models.ExtractClientSecretFromSession(sessionReq, "custom")
		assert.Equal(t, "test-secret", clientSecret)
	})

	t.Run("Missing capabilities structure", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"someOtherField": "value",
		}

		clientSecret := models.ExtractClientSecretFromSession(sessionReq, "gads")
		assert.Empty(t, clientSecret)
	})

	t.Run("Invalid type for capabilities", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": "not a map",
		}

		clientSecret := models.ExtractClientSecretFromSession(sessionReq, "gads")
		assert.Empty(t, clientSecret)
	})

	t.Run("Capabilities.alwaysMatch takes precedence over desiredCapabilities", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"alwaysMatch": map[string]interface{}{
					"gads:clientSecret": "secret-alwaysMatch",
				},
			},
			"desiredCapabilities": map[string]interface{}{
				"gads:clientSecret": "secret-desired",
			},
		}

		clientSecret := models.ExtractClientSecretFromSession(sessionReq, "gads")
		assert.Equal(t, "secret-alwaysMatch", clientSecret)
	})
}
