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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestParseSessionRequest(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantErrCode    string
		wantErrStatus  int
		wantCandidates int
		check          func(t *testing.T, req *GridSessionRequest)
	}{
		{
			name:           "firstMatch absent - alwaysMatch only",
			body:           `{"capabilities":{"alwaysMatch":{"platformName":"Android"}}}`,
			wantCandidates: 1,
			check: func(t *testing.T, req *GridSessionRequest) {
				assert.Equal(t, "Android", req.Candidates[0].PlatformName)
			},
		},
		{
			name:          "firstMatch present but empty is an error (used to panic)",
			body:          `{"capabilities":{"alwaysMatch":{"platformName":"Android"},"firstMatch":[]}}`,
			wantErrCode:   "invalid argument",
			wantErrStatus: 400,
		},
		{
			name:           "firstMatch single empty object",
			body:           `{"capabilities":{"alwaysMatch":{"platformName":"iOS"},"firstMatch":[{}]}}`,
			wantCandidates: 1,
			check: func(t *testing.T, req *GridSessionRequest) {
				assert.Equal(t, "iOS", req.Candidates[0].PlatformName)
			},
		},
		{
			name:           "split caps - platformName in alwaysMatch and appium caps in firstMatch merge",
			body:           `{"capabilities":{"alwaysMatch":{"platformName":"Android"},"firstMatch":[{"appium:automationName":"UiAutomator2","appium:udid":"some-udid"}]}}`,
			wantCandidates: 1,
			check: func(t *testing.T, req *GridSessionRequest) {
				assert.Equal(t, "Android", req.Candidates[0].PlatformName)
				assert.Equal(t, "UiAutomator2", req.Candidates[0].AutomationName)
				assert.Equal(t, "some-udid", req.Candidates[0].DeviceUDID)
			},
		},
		{
			name:          "duplicate key in alwaysMatch and firstMatch is an error",
			body:          `{"capabilities":{"alwaysMatch":{"platformName":"Android"},"firstMatch":[{"platformName":"Android"}]}}`,
			wantErrCode:   "invalid argument",
			wantErrStatus: 400,
		},
		{
			name:           "candidate order preserved for multiple firstMatch entries",
			body:           `{"capabilities":{"firstMatch":[{"platformName":"iOS"},{"platformName":"Android"}]}}`,
			wantCandidates: 2,
			check: func(t *testing.T, req *GridSessionRequest) {
				assert.Equal(t, "iOS", req.Candidates[0].PlatformName)
				assert.Equal(t, "Android", req.Candidates[1].PlatformName)
			},
		},
		{
			name:          "legacy desiredCapabilities-only body is rejected",
			body:          `{"desiredCapabilities":{"platformName":"Android","appium:automationName":"UiAutomator2"}}`,
			wantErrCode:   "invalid argument",
			wantErrStatus: 400,
		},
		{
			name:           "desiredCapabilities alongside a W3C capabilities object is ignored",
			body:           `{"capabilities":{"alwaysMatch":{"platformName":"Android"}},"desiredCapabilities":{"platformName":"iOS"}}`,
			wantCandidates: 1,
			check: func(t *testing.T, req *GridSessionRequest) {
				assert.Equal(t, "Android", req.Candidates[0].PlatformName)
			},
		},
		{
			name:           "newCommandTimeout as number",
			body:           `{"capabilities":{"alwaysMatch":{"platformName":"Android","appium:newCommandTimeout":120}}}`,
			wantCandidates: 1,
			check: func(t *testing.T, req *GridSessionRequest) {
				if assert.NotNil(t, req.Candidates[0].NewCommandTimeout) {
					assert.Equal(t, int64(120), *req.Candidates[0].NewCommandTimeout)
				}
			},
		},
		{
			name:           "newCommandTimeout as numeric string",
			body:           `{"capabilities":{"alwaysMatch":{"platformName":"Android","appium:newCommandTimeout":"45"}}}`,
			wantCandidates: 1,
			check: func(t *testing.T, req *GridSessionRequest) {
				if assert.NotNil(t, req.Candidates[0].NewCommandTimeout) {
					assert.Equal(t, int64(45), *req.Candidates[0].NewCommandTimeout)
				}
			},
		},
		{
			name:           "newCommandTimeout explicit zero is distinguishable from absent",
			body:           `{"capabilities":{"alwaysMatch":{"platformName":"Android","appium:newCommandTimeout":0}}}`,
			wantCandidates: 1,
			check: func(t *testing.T, req *GridSessionRequest) {
				if assert.NotNil(t, req.Candidates[0].NewCommandTimeout) {
					assert.Equal(t, int64(0), *req.Candidates[0].NewCommandTimeout)
				}
			},
		},
		{
			name:           "newCommandTimeout absent",
			body:           `{"capabilities":{"alwaysMatch":{"platformName":"Android"}}}`,
			wantCandidates: 1,
			check: func(t *testing.T, req *GridSessionRequest) {
				assert.Nil(t, req.Candidates[0].NewCommandTimeout)
			},
		},
		{
			name:          "newCommandTimeout as non-numeric string is an error",
			body:          `{"capabilities":{"alwaysMatch":{"platformName":"Android","appium:newCommandTimeout":"a lot"}}}`,
			wantErrCode:   "invalid argument",
			wantErrStatus: 400,
		},
		{
			name:           "gads queueTimeout parsed",
			body:           `{"capabilities":{"alwaysMatch":{"platformName":"Android","gads:queueTimeout":90}}}`,
			wantCandidates: 1,
			check: func(t *testing.T, req *GridSessionRequest) {
				if assert.NotNil(t, req.Candidates[0].QueueTimeout) {
					assert.Equal(t, int64(90), *req.Candidates[0].QueueTimeout)
				}
			},
		},
		{
			name:          "capabilities not an object is an error",
			body:          `{"capabilities":"nope"}`,
			wantErrCode:   "invalid argument",
			wantErrStatus: 400,
		},
		{
			name:          "firstMatch not a list is an error",
			body:          `{"capabilities":{"firstMatch":{"platformName":"Android"}}}`,
			wantErrCode:   "invalid argument",
			wantErrStatus: 400,
		},
		{
			name:          "firstMatch entry not an object is an error",
			body:          `{"capabilities":{"firstMatch":["Android"]}}`,
			wantErrCode:   "invalid argument",
			wantErrStatus: 400,
		},
		{
			name:          "no capabilities at all is an error",
			body:          `{"something":"else"}`,
			wantErrCode:   "invalid argument",
			wantErrStatus: 400,
		},
		{
			name:          "invalid JSON is an error",
			body:          `{not json`,
			wantErrCode:   "invalid argument",
			wantErrStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, w3cErr := parseSessionRequest([]byte(tt.body), "gads")
			if tt.wantErrCode != "" {
				if assert.NotNil(t, w3cErr) {
					assert.Equal(t, tt.wantErrCode, w3cErr.Code)
					assert.Equal(t, tt.wantErrStatus, w3cErr.HTTPStatus)
				}
				return
			}
			assert.Nil(t, w3cErr)
			assert.Len(t, req.Candidates, tt.wantCandidates)
			if tt.check != nil {
				tt.check(t, req)
			}
		})
	}
}

func TestCandidateTargetOS(t *testing.T) {
	tests := []struct {
		name           string
		platformName   string
		automationName string
		want           string
	}{
		{"platformName only", "Android", "", "android"},
		{"automationName only", "", "XCUITest", "ios"},
		{"case insensitive", "ANDROID", "", "android"},
		{"tizen", "TizenTV", "", "tizen"},
		{"webos", "lgtv", "", "webos"},
		{"roku", "", "roku", "roku"},
		{"unknown", "Windows", "", ""},
		{"empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := gridCandidate{PlatformName: tt.platformName, AutomationName: tt.automationName}
			assert.Equal(t, tt.want, candidateTargetOS(candidate))
		})
	}
}

func TestSelectGridCandidate(t *testing.T) {
	t.Run("first matching candidate wins in order", func(t *testing.T) {
		candidates := []gridCandidate{
			{PlatformName: "Windows"},
			{PlatformName: "iOS"},
			{PlatformName: "Android"},
		}
		selected, ok := selectGridCandidate(candidates)
		assert.True(t, ok)
		assert.Equal(t, "iOS", selected.PlatformName)
	})

	t.Run("udid alone is enough to match", func(t *testing.T) {
		candidates := []gridCandidate{{DeviceUDID: "some-udid"}}
		selected, ok := selectGridCandidate(candidates)
		assert.True(t, ok)
		assert.Equal(t, "some-udid", selected.DeviceUDID)
	})

	t.Run("no candidate matches", func(t *testing.T) {
		candidates := []gridCandidate{{PlatformName: "Windows"}, {}}
		_, ok := selectGridCandidate(candidates)
		assert.False(t, ok)
	})
}

func TestStripGadsCaps(t *testing.T) {
	t.Run("strips prefixed caps from all three locations", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"alwaysMatch": map[string]interface{}{
					"platformName":      "Android",
					"gads:clientSecret": "secret",
				},
				"firstMatch": []interface{}{
					map[string]interface{}{
						"appium:automationName": "UiAutomator2",
						"gads:queueTimeout":     float64(30),
					},
				},
			},
			"desiredCapabilities": map[string]interface{}{
				"platformName":      "Android",
				"gads:clientSecret": "secret",
			},
		}

		stripGadsCaps(sessionReq, "gads")

		marshaled, _ := json.Marshal(sessionReq)
		assert.NotContains(t, string(marshaled), "gads:")
		alwaysMatch := sessionReq["capabilities"].(map[string]interface{})["alwaysMatch"].(map[string]interface{})
		assert.Equal(t, "Android", alwaysMatch["platformName"])
		firstMatch := sessionReq["capabilities"].(map[string]interface{})["firstMatch"].([]interface{})
		assert.Equal(t, "UiAutomator2", firstMatch[0].(map[string]interface{})["appium:automationName"])
	})

	t.Run("custom prefix", func(t *testing.T) {
		sessionReq := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"alwaysMatch": map[string]interface{}{
					"custom:clientSecret": "secret",
					"gads:kept":           "yes",
				},
			},
		}

		stripGadsCaps(sessionReq, "custom")

		alwaysMatch := sessionReq["capabilities"].(map[string]interface{})["alwaysMatch"].(map[string]interface{})
		assert.NotContains(t, alwaysMatch, "custom:clientSecret")
		assert.Contains(t, alwaysMatch, "gads:kept")
	})
}

type fakeGridStore struct {
	credential    models.ClientCredentials
	credentialErr error
	user          models.User
	workspaces    []models.Workspace
}

func (f *fakeGridStore) GetClientCredentialBySecret(clientSecret string) (models.ClientCredentials, error) {
	return f.credential, f.credentialErr
}

func (f *fakeGridStore) GetUser(username string) (models.User, error) {
	return f.user, nil
}

func (f *fakeGridStore) GetUserWorkspaces(username string) []models.Workspace {
	return f.workspaces
}

func (f *fakeGridStore) GetWorkspaces() ([]models.Workspace, error) {
	return f.workspaces, nil
}

func (f *fakeGridStore) GetOrCreateDefaultTenant() (string, error) {
	return "default", nil
}

func newGridTestRouter() *gin.Engine {
	router := gin.New()
	registerGridRoutes(router.Group("/grid"))
	return router
}

func TestGridCreateSessionHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevGridDB := gridDB
	defer func() { gridDB = prevGridDB }()
	gridDB = &fakeGridStore{
		credential: models.ClientCredentials{UserID: "test-user", Tenant: "tenant1", IsActive: true},
		workspaces: []models.Workspace{{ID: "ws1", Tenant: "tenant1"}},
	}

	t.Run("session created with split caps and secret stripped before forwarding", func(t *testing.T) {
		var forwardedBody []byte
		fakeProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			forwardedBody, _ = readBody(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"value":{"sessionId":"fake-session-id","capabilities":{"platformName":"Android"}}}`))
		}))
		defer fakeProvider.Close()

		udid := "grid-test-device"
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
		}
		devices.HubDeviceStore.Set(udid, device)
		defer devices.HubDeviceStore.Delete(udid)

		// platformName only in alwaysMatch, appium:* only in firstMatch - must match after merging
		sessionBody := `{"capabilities":{"alwaysMatch":{"platformName":"Android","gads:clientSecret":"test-secret"},"firstMatch":[{"appium:automationName":"UiAutomator2"}]}}`

		router := newGridTestRouter()
		req, _ := http.NewRequest("POST", "/grid/session", bytes.NewBufferString(sessionBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "fake-session-id")

		assert.NotContains(t, string(forwardedBody), "test-secret")
		assert.NotContains(t, string(forwardedBody), "gads:")
		assert.Contains(t, string(forwardedBody), "UiAutomator2")
		assert.Contains(t, string(forwardedBody), "Android")

		device.Mu.RLock()
		defer device.Mu.RUnlock()
		assert.Equal(t, "fake-session-id", device.SessionID)
		assert.True(t, device.IsRunningAutomation)
		assert.False(t, device.IsAvailableForAutomation)
	})

	t.Run("malformed JSON body returns 400 invalid argument", func(t *testing.T) {
		router := newGridTestRouter()
		req, _ := http.NewRequest("POST", "/grid/session", bytes.NewBufferString(`{not json`))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response SeleniumSessionErrorResponse
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, "invalid argument", response.Value.Error)
	})

	t.Run("firstMatch empty list returns 400 without panicking", func(t *testing.T) {
		router := newGridTestRouter()
		body := `{"capabilities":{"alwaysMatch":{"gads:clientSecret":"test-secret"},"firstMatch":[]}}`
		req, _ := http.NewRequest("POST", "/grid/session", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response SeleniumSessionErrorResponse
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, "invalid argument", response.Value.Error)
	})

	t.Run("unknown session ID returns 404 invalid session id", func(t *testing.T) {
		router := newGridTestRouter()
		req, _ := http.NewRequest("POST", "/grid/session/does-not-exist-session/url", bytes.NewBufferString(`{"url":"https://example.com"}`))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var response SeleniumSessionErrorResponse
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, "invalid session id", response.Value.Error)
		assert.Equal(t, fmt.Sprintf("No session ID `%s` is available to GADS, it timed out or something unexpected occurred", "does-not-exist-session"), response.Value.Message)
	})
}
