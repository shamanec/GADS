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
	"encoding/json"
	"fmt"
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
		assert.Equal(t, "Device `test-device-unavailable` is not available", response["error"])

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

	t.Run("Missing Client Credentials - Should Return W3C Error Format", func(t *testing.T) {
		// Setup a device
		udid := "test-device-no-credentials"
		devices.HubDevicesData.Mu.Lock()
		devices.HubDevicesData.Devices[udid] = &models.LocalHubDevice{
			Device: models.Device{
				UDID: udid,
				Host: "localhost:8080",
			},
			Available: true,
		}
		devices.HubDevicesData.Mu.Unlock()

		// Create request WITHOUT credentials
		router := gin.New()
		router.POST("/device/:udid/*path", DeviceProxyHandler)

		sessionReq := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"alwaysMatch": map[string]interface{}{
					"platformName": "iOS",
					// Note: NO client credentials provided
				},
			},
		}
		jsonData, _ := json.Marshal(sessionReq)

		req, _ := http.NewRequest("POST", "/device/"+udid+"/session", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(w, req)

		// Verify status code
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// Verify W3C error format
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		// Check W3C structure
		assert.Contains(t, response, "value")
		value, ok := response["value"].(map[string]interface{})
		assert.True(t, ok, "value should be a map")

		assert.Equal(t, "invalid argument", value["error"])
		expectedMsg := fmt.Sprintf("Client credentials are required. Provide %[1]s:clientId, %[1]s:clientSecret and %[1]s:tenant in the capabilities.", capabilityPrefix)
		assert.Equal(t, expectedMsg, value["message"])
		assert.Equal(t, "", value["stacktrace"])

		// Cleanup
		devices.HubDevicesData.Mu.Lock()
		delete(devices.HubDevicesData.Devices, udid)
		devices.HubDevicesData.Mu.Unlock()
	})

	t.Run("Partial Client Credentials - Should Return W3C Error Format", func(t *testing.T) {
		// Setup a device
		udid := "test-device-partial-credentials"
		devices.HubDevicesData.Mu.Lock()
		devices.HubDevicesData.Devices[udid] = &models.LocalHubDevice{
			Device: models.Device{
				UDID: udid,
				Host: "localhost:8080",
			},
			Available: true,
		}
		devices.HubDevicesData.Mu.Unlock()

		// Test cases for partial credentials
		testCases := []struct {
			name       string
			sessionReq map[string]interface{}
		}{
			{
				name: "Only clientId",
				sessionReq: map[string]interface{}{
					"capabilities": map[string]interface{}{
						"alwaysMatch": map[string]interface{}{
							"gads:clientId": "test-client-id",
							"platformName":  "iOS",
						},
					},
				},
			},
			{
				name: "Only clientSecret",
				sessionReq: map[string]interface{}{
					"capabilities": map[string]interface{}{
						"alwaysMatch": map[string]interface{}{
							"gads:clientSecret": "test-secret",
							"platformName":      "iOS",
						},
					},
				},
			},
			{
				name: "Missing tenant",
				sessionReq: map[string]interface{}{
					"capabilities": map[string]interface{}{
						"alwaysMatch": map[string]interface{}{
							"gads:clientId":     "test-client-id",
							"gads:clientSecret": "test-secret",
							"platformName":      "iOS",
						},
					},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create request
				router := gin.New()
				router.POST("/device/:udid/*path", DeviceProxyHandler)

				jsonData, _ := json.Marshal(tc.sessionReq)

				req, _ := http.NewRequest("POST", "/device/"+udid+"/session", bytes.NewBuffer(jsonData))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				// Execute request
				router.ServeHTTP(w, req)

				// Verify status code
				assert.Equal(t, http.StatusUnauthorized, w.Code)

				// Verify W3C error format
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				// Check error message
				value, _ := response["value"].(map[string]interface{})
				expectedMsg := fmt.Sprintf("Client credentials are required. Provide %[1]s:clientId, %[1]s:clientSecret and %[1]s:tenant in the capabilities.", capabilityPrefix)
				assert.Equal(t, expectedMsg, value["message"])
			})
		}

		// Cleanup
		devices.HubDevicesData.Mu.Lock()
		delete(devices.HubDevicesData.Devices, udid)
		devices.HubDevicesData.Mu.Unlock()
	})
}

func TestExtractGADSCredentials(t *testing.T) {
	t.Run("Extract from capabilities.alwaysMatch", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"alwaysMatch": map[string]interface{}{
					"gads:clientId":     "test-client-id",
					"gads:clientSecret": "test-secret",
					"gads:tenant":       "test-tenant",
				},
			},
		}

		clientID, clientSecret, tenant := extractGADSCredentials(sessionReq, "gads")
		assert.Equal(t, "test-client-id", clientID)
		assert.Equal(t, "test-secret", clientSecret)
		assert.Equal(t, "test-tenant", tenant)
	})

	t.Run("Extract from desiredCapabilities", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"desiredCapabilities": map[string]interface{}{
				"gads:clientId":     "test-client-id",
				"gads:clientSecret": "test-secret",
				"gads:tenant":       "test-tenant",
			},
		}

		clientID, clientSecret, tenant := extractGADSCredentials(sessionReq, "gads")
		assert.Equal(t, "test-client-id", clientID)
		assert.Equal(t, "test-secret", clientSecret)
		assert.Equal(t, "test-tenant", tenant)
	})

	t.Run("Custom prefix extraction", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"alwaysMatch": map[string]interface{}{
					"custom:clientId":     "test-client-id",
					"custom:clientSecret": "test-secret",
					"custom:tenant":       "test-tenant",
				},
			},
		}

		clientID, clientSecret, tenant := extractGADSCredentials(sessionReq, "custom")
		assert.Equal(t, "test-client-id", clientID)
		assert.Equal(t, "test-secret", clientSecret)
		assert.Equal(t, "test-tenant", tenant)
	})

	t.Run("Missing capabilities structure", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"someOtherField": "value",
		}

		clientID, clientSecret, tenant := extractGADSCredentials(sessionReq, "gads")
		assert.Empty(t, clientID)
		assert.Empty(t, clientSecret)
		assert.Empty(t, tenant)
	})

	t.Run("Invalid type for capabilities", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": "not a map",
		}

		clientID, clientSecret, tenant := extractGADSCredentials(sessionReq, "gads")
		assert.Empty(t, clientID)
		assert.Empty(t, clientSecret)
		assert.Empty(t, tenant)
	})

	t.Run("Capabilities.alwaysMatch takes precedence over desiredCapabilities", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"alwaysMatch": map[string]interface{}{
					"gads:clientId":     "from-alwaysMatch",
					"gads:clientSecret": "secret-alwaysMatch",
					"gads:tenant":       "tenant-alwaysMatch",
				},
			},
			"desiredCapabilities": map[string]interface{}{
				"gads:clientId":     "from-desired",
				"gads:clientSecret": "secret-desired",
				"gads:tenant":       "tenant-desired",
			},
		}

		clientID, clientSecret, tenant := extractGADSCredentials(sessionReq, "gads")
		assert.Equal(t, "from-alwaysMatch", clientID)
		assert.Equal(t, "secret-alwaysMatch", clientSecret)
		assert.Equal(t, "tenant-alwaysMatch", tenant)
	})
}
