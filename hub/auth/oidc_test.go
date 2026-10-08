/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package auth

import (
	"GADS/common/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

// withOIDCClient makes a client with the given configuration current, without
// discovering a provider
func withOIDCClient(t *testing.T, config models.OIDCConfig) {
	t.Helper()

	oidcMu.Lock()
	previous := oidcCurrent
	oidcCurrent = &oidcClient{
		config: config,
		oauth2: &oauth2.Config{
			ClientID:    config.ClientID,
			RedirectURL: config.RedirectURI,
			Endpoint:    oauth2.Endpoint{AuthURL: config.IssuerURL + "/auth", TokenURL: config.IssuerURL + "/token"},
		},
	}
	oidcMu.Unlock()

	t.Cleanup(func() {
		oidcMu.Lock()
		oidcCurrent = previous
		oidcMu.Unlock()
	})
}

// oidcProvider serves the discovery document of a provider at a test server
func oidcProvider(t *testing.T) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 server.URL,
			"authorization_endpoint": server.URL + "/auth",
			"token_endpoint":         server.URL + "/token",
			"jwks_uri":               server.URL + "/certs",
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func waitForOIDC(t *testing.T, enabled bool) bool {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if IsOIDCEnabled() == enabled {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestConfigureOIDCDiscoversTheProviderInTheBackground(t *testing.T) {
	provider := oidcProvider(t)
	t.Cleanup(func() { ConfigureOIDC(models.OIDCConfig{}) })

	ConfigureOIDC(models.OIDCConfig{Enabled: true, IssuerURL: provider.URL, ClientID: "gads"})

	assert.True(t, waitForOIDC(t, true), "OIDC sign-in did not come up")
}

func TestConfigureOIDCTurnsSignInOffAtOnce(t *testing.T) {
	withOIDCClient(t, models.OIDCConfig{IssuerURL: "https://sso.example.com"})

	ConfigureOIDC(models.OIDCConfig{Enabled: false})

	assert.False(t, IsOIDCEnabled())
}

func TestConfigureOIDCDropsDiscoveryOfAReplacedConfiguration(t *testing.T) {
	provider := oidcProvider(t)
	t.Cleanup(func() { ConfigureOIDC(models.OIDCConfig{}) })

	ConfigureOIDC(models.OIDCConfig{Enabled: true, IssuerURL: provider.URL, ClientID: "gads"})
	ConfigureOIDC(models.OIDCConfig{Enabled: false})

	assert.False(t, waitForOIDC(t, true), "a replaced configuration enabled OIDC sign-in")
}

func TestIdentityFromClaims(t *testing.T) {
	config := models.OIDCConfig{AdminGroup: "gads-admins", GroupsClaim: "groups"}

	testCases := []struct {
		name     string
		subject  string
		claims   map[string]any
		config   models.OIDCConfig
		expected oidcIdentity
		fails    bool
	}{
		{
			name:     "member of the admin group",
			subject:  "sub-1",
			claims:   map[string]any{"preferred_username": "alice", "groups": []any{"qa", "gads-admins"}},
			config:   config,
			expected: oidcIdentity{Subject: "sub-1", Username: "alice", Role: "admin"},
		},
		{
			name:     "no admin group membership",
			subject:  "sub-2",
			claims:   map[string]any{"preferred_username": "bob", "groups": []any{"qa"}},
			config:   config,
			expected: oidcIdentity{Subject: "sub-2", Username: "bob", Role: "user"},
		},
		{
			name:     "groups in a custom claim",
			subject:  "sub-1",
			claims:   map[string]any{"preferred_username": "alice", "roles": []any{"gads-admins"}},
			config:   models.OIDCConfig{AdminGroup: "gads-admins", GroupsClaim: "roles"},
			expected: oidcIdentity{Subject: "sub-1", Username: "alice", Role: "admin"},
		},
		{
			name:     "no admin group configured",
			subject:  "sub-1",
			claims:   map[string]any{"preferred_username": "alice", "groups": []any{""}},
			config:   models.OIDCConfig{},
			expected: oidcIdentity{Subject: "sub-1", Username: "alice", Role: "user"},
		},
		{
			name:    "email does not stand in for a missing username",
			subject: "sub-3",
			claims:  map[string]any{"email": "carol@example.com"},
			config:  config,
			fails:   true,
		},
		{
			name:    "no subject",
			subject: "",
			claims:  map[string]any{"preferred_username": "alice"},
			config:  config,
			fails:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			identity, err := identityFromClaims(tc.subject, tc.claims, tc.config)
			if tc.fails {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, identity)
		})
	}
}

func TestReconcileSSOUser(t *testing.T) {
	identity := oidcIdentity{Subject: "sub-1", Username: "alice", Role: "admin"}

	testCases := []struct {
		name        string
		user        models.User
		expected    models.User
		changed     bool
		expectedErr error
	}{
		{
			name:        "local account is not taken over",
			user:        models.User{Username: "alice", Password: "secret", Role: "user"},
			expectedErr: errLocalUserExists,
		},
		{
			name:        "account of another identity",
			user:        models.User{Username: "alice", Role: "admin", AuthSource: models.AuthSourceOIDC, OIDCSubject: "sub-2"},
			expectedErr: errSubjectMismatch,
		},
		{
			name:     "account without a subject gets bound",
			user:     models.User{Username: "alice", Role: "admin", AuthSource: models.AuthSourceOIDC},
			expected: models.User{Username: "alice", Role: "admin", AuthSource: models.AuthSourceOIDC, OIDCSubject: "sub-1"},
			changed:  true,
		},
		{
			name:     "role follows the provider",
			user:     models.User{Username: "alice", Role: "user", AuthSource: models.AuthSourceOIDC, OIDCSubject: "sub-1"},
			expected: models.User{Username: "alice", Role: "admin", AuthSource: models.AuthSourceOIDC, OIDCSubject: "sub-1"},
			changed:  true,
		},
		{
			name:     "nothing to change",
			user:     models.User{Username: "alice", Role: "admin", AuthSource: models.AuthSourceOIDC, OIDCSubject: "sub-1"},
			expected: models.User{Username: "alice", Role: "admin", AuthSource: models.AuthSourceOIDC, OIDCSubject: "sub-1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user := tc.user
			changed, err := reconcileSSOUser(&user, identity)
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
				assert.Equal(t, tc.user, user, "a refused account was modified")
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.changed, changed)
			assert.Equal(t, tc.expected, user)
		})
	}
}

func TestSSOLoginRedirectsWithPKCEAndNonce(t *testing.T) {
	withOIDCClient(t, models.OIDCConfig{
		IssuerURL:   "https://sso.example.com",
		ClientID:    "gads",
		RedirectURI: "https://gads.example.com/hub/auth/sso/callback",
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/auth/sso/login", SSOLoginHandler)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/sso/login", nil))

	assert.Equal(t, http.StatusFound, recorder.Code)
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))

	location, err := url.Parse(recorder.Header().Get("Location"))
	assert.NoError(t, err)
	query := location.Query()
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.NotEmpty(t, query.Get("code_challenge"))
	assert.NotEmpty(t, query.Get("nonce"))

	cookies := recorder.Result().Cookies()
	if assert.Len(t, cookies, 1) {
		cookie := cookies[0]
		assert.Equal(t, ssoCookiePrefix+query.Get("state")[:ssoCookieStateChars], cookie.Name)
		assert.Equal(t, "/hub/auth/sso/callback", cookie.Path)
		assert.True(t, cookie.HttpOnly)
		assert.True(t, cookie.Secure, "the redirect URI is https")
		assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	}
}

func TestSSOLoginFromRequest(t *testing.T) {
	login, err := newSSOLogin()
	assert.NoError(t, err)

	requestWithCookie := func(name, value string) *http.Request {
		request := httptest.NewRequest(http.MethodGet, "/auth/sso/callback", nil)
		request.AddCookie(&http.Cookie{Name: name, Value: value})
		return request
	}

	found, err := ssoLoginFromRequest(requestWithCookie(login.cookieName(), login.cookieValue()), login.state)
	assert.NoError(t, err)
	assert.Equal(t, login, found)

	// Same cookie name, another state - a callback crafted for someone else's sign-in
	forged := login.state[:ssoCookieStateChars] + "forged-rest-of-the-state"
	_, err = ssoLoginFromRequest(requestWithCookie(login.cookieName(), login.cookieValue()), forged)
	assert.Error(t, err)

	_, err = ssoLoginFromRequest(httptest.NewRequest(http.MethodGet, "/auth/sso/callback", nil), login.state)
	assert.Error(t, err, "no sign-in was started in this browser")

	_, err = ssoLoginFromRequest(requestWithCookie(login.cookieName(), login.cookieValue()), "short")
	assert.Error(t, err)
}

func TestSSOCallbackRefusesAStateFromAnotherBrowser(t *testing.T) {
	withOIDCClient(t, models.OIDCConfig{IssuerURL: "https://sso.example.com", RedirectURI: "https://gads.example.com/auth/sso/callback"})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/auth/sso/callback", SSOCallbackHandler)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/sso/callback?state=0123456789abcdefghij&code=abc", nil))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}
