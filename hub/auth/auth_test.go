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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// withTestSecretCache swaps in a secret cache with test keys so tokens can be signed
func withTestSecretCache(t *testing.T) {
	t.Helper()

	previousCache := secretCache
	secretCache = setupTestSecretCache()
	t.Cleanup(func() {
		secretCache = previousCache
	})
}

// authedRequest runs a request carrying the given token through a router with
// the authentication middleware in front of a trivial handler
func authedRequest(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

// sessionIDOf returns the session a token was issued for
func sessionIDOf(t *testing.T, token string) string {
	t.Helper()

	claims, err := ValidateJWT(token)
	assert.NoError(t, err)

	return claims.SessionID
}

func TestAuthMiddlewareAcceptsLiveSession(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)
	withTestSecretCache(t)

	token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"})
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, authedRequest(t, token).Code)
}

func TestAuthMiddlewareKeepsSessionAliveWhileUsed(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)
	withTestSecretCache(t)

	token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"})
	assert.NoError(t, err)
	sessionID := sessionIDOf(t, token)

	// A client working away for well over the TTL keeps the same token
	for hour := 1; hour <= 5; hour++ {
		ageSession(t, sessionID, time.Duration(hour)*time.Hour, 59*time.Minute)
		assert.Equal(t, http.StatusOK, authedRequest(t, token).Code, "a session in use should not expire")
	}
}

func TestAuthMiddlewareRejectsIdleSession(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)
	withTestSecretCache(t)

	token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"})
	assert.NoError(t, err)
	ageSession(t, sessionIDOf(t, token), time.Hour+time.Minute, time.Hour+time.Minute)

	assert.Equal(t, http.StatusUnauthorized, authedRequest(t, token).Code)
	assert.Equal(t, 0, ActiveSessionCount())
}

func TestAuthMiddlewareRejectsSessionPastAbsoluteAge(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)
	withTestSecretCache(t)

	token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"})
	assert.NoError(t, err)
	ageSession(t, sessionIDOf(t, token), 24*time.Hour+time.Minute, time.Minute)

	assert.Equal(t, http.StatusUnauthorized, authedRequest(t, token).Code)
}

func TestAuthMiddlewareRejectsTokenOfDeletedSession(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)
	withTestSecretCache(t)

	token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, authedRequest(t, token).Code)

	// Logging out drops the session, which is what stops the token working -
	// the token itself is still perfectly valid on its own
	DeleteSession(sessionIDOf(t, token))

	assert.Equal(t, http.StatusUnauthorized, authedRequest(t, token).Code)
}

func TestAuthMiddlewareRejectsTokenWithoutSession(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)
	withTestSecretCache(t)

	// A hub-issued token from before this hub started, e.g. one that survived a
	// restart in someone's browser
	token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"})
	assert.NoError(t, err)
	sessionsMu.Lock()
	sessions = make(map[string]*Session)
	sessionsMu.Unlock()

	assert.Equal(t, http.StatusUnauthorized, authedRequest(t, token).Code)
}

func TestAuthMiddlewareKeepsClientCredentialsSessionPastAbsoluteAge(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)
	withTestSecretCache(t)

	token, err := GenerateClientCredentialsJWT("gads_client_id", "testuser", "user", "tenant1", []string{"user"}, "")
	assert.NoError(t, err)
	ageSession(t, sessionIDOf(t, token), 240*time.Hour, time.Minute)

	assert.Equal(t, http.StatusOK, authedRequest(t, token).Code)
}

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)
	withTestSecretCache(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestSessionTokenExpiryBacksTheAbsoluteAge(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)
	withTestSecretCache(t)

	// A user token cannot outlive the session cap even if the store is bypassed
	userToken, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"})
	assert.NoError(t, err)
	userClaims, err := ValidateJWT(userToken)
	assert.NoError(t, err)
	assert.NotNil(t, userClaims.ExpiresAt)
	assert.InDelta(t, time.Now().Add(24*time.Hour).Unix(), userClaims.ExpiresAt.Time.Unix(), 5)
	assert.NotEmpty(t, userClaims.SessionID)

	// Sessions without an absolute age have nothing to put in `exp` - the
	// session store is what ends them
	clientToken, err := GenerateClientCredentialsJWT("gads_client_id", "testuser", "user", "tenant1", []string{"user"}, "")
	assert.NoError(t, err)
	clientClaims, err := ValidateJWT(clientToken)
	assert.NoError(t, err)
	assert.Nil(t, clientClaims.ExpiresAt)
	assert.NotEmpty(t, clientClaims.SessionID)
}
