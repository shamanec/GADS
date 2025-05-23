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
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Mock implementation of SecretStore for testing
type MockSecretStore struct {
	keys map[string]*SecretKey
}

func NewMockSecretStore() *MockSecretStore {
	return &MockSecretStore{
		keys: make(map[string]*SecretKey),
	}
}

func (m *MockSecretStore) AddSecretKey(secretKey *SecretKey, username, justification string) error {
	secretKey.ID = primitive.NewObjectID()
	m.keys[secretKey.Origin] = secretKey
	return nil
}

func (m *MockSecretStore) GetSecretKeyByOrigin(origin string) (*SecretKey, error) {
	if key, exists := m.keys[origin]; exists {
		return key, nil
	}
	return nil, ErrSecretKeyNotFound
}

func (m *MockSecretStore) GetDefaultSecretKey() (*SecretKey, error) {
	for _, key := range m.keys {
		if key.IsDefault {
			return key, nil
		}
	}
	return nil, ErrDefaultKeyNotFound
}

func (m *MockSecretStore) GetAllSecretKeys() ([]*SecretKey, error) {
	var keys []*SecretKey
	for _, key := range m.keys {
		keys = append(keys, key)
	}
	return keys, nil
}

func (m *MockSecretStore) GetSecretKeyByID(id primitive.ObjectID) (*SecretKey, error) {
	for _, key := range m.keys {
		if key.ID == id {
			return key, nil
		}
	}
	return nil, ErrSecretKeyNotFound
}

func (m *MockSecretStore) UpdateSecretKey(secretKey *SecretKey, username, justification string) error {
	m.keys[secretKey.Origin] = secretKey
	return nil
}

func (m *MockSecretStore) DisableSecretKey(id primitive.ObjectID, username, justification string) error {
	for _, key := range m.keys {
		if key.ID == id {
			if key.IsDefault {
				return errors.New("cannot disable the default secret key")
			}
			key.Disabled = true
			now := time.Now()
			key.DisabledAt = &now
			return nil
		}
	}
	return ErrSecretKeyNotFound
}

func (m *MockSecretStore) ensureNoOtherDefaultKeys(exceptID ...primitive.ObjectID) error {
	// No need to implement for tests
	return nil
}

func setupTestSecretCache() *SecretCache {
	store := NewMockSecretStore()

	// Add default key
	store.AddSecretKey(&SecretKey{
		Origin:    "default",
		Key:       "default_secret_key",
		IsDefault: true,
	}, "system", "Test setup")

	// Add web key
	store.AddSecretKey(&SecretKey{
		Origin:    "https://web.example.com",
		Key:       "web_secret_key",
		IsDefault: false,
	}, "system", "Test setup")

	// Add mobile key
	store.AddSecretKey(&SecretKey{
		Origin:    "mobile-app",
		Key:       "mobile_secret_key",
		IsDefault: false,
	}, "system", "Test setup")

	// Create and initialize cache
	cache := NewSecretCache(store, time.Minute)
	return cache
}

func TestGenerateAndValidateJWTWithMultipleKeys(t *testing.T) {
	// Setup test environment
	originalCache := secretCache
	defer func() {
		secretCache = originalCache
	}()

	secretCache = setupTestSecretCache()

	// Test cases
	testCases := []struct {
		name          string
		origin        string
		expectSuccess bool
	}{
		{
			name:          "Default Origin",
			origin:        "default",
			expectSuccess: true,
		},
		{
			name:          "Web Origin",
			origin:        "https://web.example.com",
			expectSuccess: true,
		},
		{
			name:          "Mobile Origin",
			origin:        "mobile-app",
			expectSuccess: true,
		},
		{
			name:          "Unknown Origin",
			origin:        "unknown-origin",
			expectSuccess: true, // Should fallback to default key
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate token with specific origin
			token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"}, time.Hour, tc.origin)
			assert.NoError(t, err)
			assert.NotEmpty(t, token)

			// Validate token with same origin
			claims, err := ValidateJWT(token, tc.origin)
			if tc.expectSuccess {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, "testuser", claims.Username)
				assert.Equal(t, "user", claims.Role)
				assert.Equal(t, "tenant1", claims.Tenant)
				assert.Equal(t, tc.origin, claims.Origin)
			} else {
				assert.Error(t, err)
			}

			// Validate token without specifying origin (should extract from claims)
			claims, err = ValidateJWT(token)
			if tc.expectSuccess {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, "testuser", claims.Username)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestOriginClaimInToken(t *testing.T) {
	// Setup test environment
	originalCache := secretCache
	defer func() {
		secretCache = originalCache
	}()

	secretCache = setupTestSecretCache()

	// Generate token with origin
	origin := "https://web.example.com"
	token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"}, time.Hour, origin)
	assert.NoError(t, err)

	// Parse token to verify origin is included in claims
	parsedToken, err := jwt.ParseWithClaims(token, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte("web_secret_key"), nil
	})
	assert.NoError(t, err)

	// Check that origin claim exists
	if claims, ok := parsedToken.Claims.(*JWTClaims); ok {
		assert.Equal(t, origin, claims.Origin)
	} else {
		t.Fatal("Failed to parse claims")
	}
}

func TestFallbackToDefaultKey(t *testing.T) {
	// Setup test environment
	originalCache := secretCache
	defer func() {
		secretCache = originalCache
	}()

	secretCache = setupTestSecretCache()

	// Generate token with unknown origin
	origin := "unknown-origin"
	token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"}, time.Hour, origin)
	assert.NoError(t, err)

	// Validate token with unknown origin
	claims, err := ValidateJWT(token, origin)
	assert.NoError(t, err)
	assert.NotNil(t, claims)

	// This works because for unknown origins, we fallback to the default key
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, origin, claims.Origin)
}

func TestExtractTokenFromBearer(t *testing.T) {
	testCases := []struct {
		name        string
		authHeader  string
		expectToken string
		expectError bool
	}{
		{
			name:        "Valid Bearer Token",
			authHeader:  "Bearer abcdef123456",
			expectToken: "abcdef123456",
			expectError: false,
		},
		{
			name:        "Invalid Format - No Bearer Prefix",
			authHeader:  "abcdef123456",
			expectToken: "",
			expectError: true,
		},
		{
			name:        "Invalid Format - Empty",
			authHeader:  "",
			expectToken: "",
			expectError: true,
		},
		{
			name:        "Invalid Format - Just Bearer",
			authHeader:  "Bearer",
			expectToken: "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := ExtractTokenFromBearer(tc.authHeader)

			if tc.expectError {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectToken, token)
			}
		})
	}
}

func TestGetClaimsFromToken(t *testing.T) {
	// Setup test environment
	originalCache := secretCache
	defer func() {
		secretCache = originalCache
	}()

	secretCache = setupTestSecretCache()

	// Generate token
	origin := "https://web.example.com"
	token, err := GenerateJWT("testuser", "user", "tenant1", []string{"user"}, time.Hour, origin)
	assert.NoError(t, err)

	// Test cases
	testCases := []struct {
		name        string
		token       string
		origin      string
		expectError bool
	}{
		{
			name:        "Valid Token With Origin",
			token:       token,
			origin:      origin,
			expectError: false,
		},
		{
			name:        "Valid Token Without Origin",
			token:       token,
			origin:      "",
			expectError: false,
		},
		{
			name:        "Empty Token",
			token:       "",
			origin:      origin,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var claims *JWTClaims
			var err error

			if tc.origin != "" {
				claims, err = GetClaimsFromToken(tc.token, tc.origin)
			} else {
				claims, err = GetClaimsFromToken(tc.token)
			}

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, "testuser", claims.Username)
			}
		})
	}
}

func TestSecretCache(t *testing.T) {
	// Create mock store
	store := NewMockSecretStore()

	// Add default key
	store.AddSecretKey(&SecretKey{
		Origin:    "default",
		Key:       "default_secret_key",
		IsDefault: true,
	}, "system", "Test setup")

	// Create a secret cache
	cache := NewSecretCache(store, time.Minute)

	// Test GetKey for default key
	key := cache.GetKey("unknown-origin")
	assert.Equal(t, []byte("default_secret_key"), key)

	// Test GetDefaultKey
	key = cache.GetDefaultKey()
	assert.Equal(t, []byte("default_secret_key"), key)

	// Add a new key and test GetKey
	store.AddSecretKey(&SecretKey{
		Origin:    "test-origin",
		Key:       "test_secret_key",
		IsDefault: false,
	}, "system", "Test setup")

	// Refresh cache
	cache.Refresh()

	// Test GetKey for specific origin
	key = cache.GetKey("test-origin")
	assert.Equal(t, []byte("test_secret_key"), key)
}

func TestDynamicIdentifierClaims(t *testing.T) {
	// Setup test environment
	originalCache := secretCache
	defer func() {
		secretCache = originalCache
	}()

	store := NewMockSecretStore()

	// Add a key with custom identifier claims
	customKey := &SecretKey{
		Origin:                "custom-origin",
		Key:                   "custom_secret_key",
		IsDefault:             false,
		UserIdentifierClaim:   "custom_user",
		TenantIdentifierClaim: "custom_tenant",
	}
	store.AddSecretKey(customKey, "system", "Test setup")

	// Add default key
	store.AddSecretKey(&SecretKey{
		Origin:    "default",
		Key:       "default_secret_key",
		IsDefault: true,
	}, "system", "Test setup")

	// Initialize cache with our test store
	secretCache = NewSecretCache(store, time.Minute)

	// Create a token with custom claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":           "standard-subject",
		"custom_user":   "user-from-custom-claim",
		"custom_tenant": "tenant-from-custom-claim",
		"username":      "standard-username",
		"tenant":        "standard-tenant",
		"role":          "user",
		"scope":         []string{"user"},
		"origin":        "custom-origin",
		"exp":           time.Now().Add(time.Hour).Unix(),
	})

	// Sign the token with our key
	tokenString, err := token.SignedString([]byte("custom_secret_key"))
	assert.NoError(t, err)

	// Validate and verify that custom claims are used
	claims, err := ValidateJWT(tokenString, "custom-origin")
	assert.NoError(t, err)
	assert.NotNil(t, claims)

	// Should use the custom identifier claims
	assert.Equal(t, "user-from-custom-claim", claims.Username)
	assert.Equal(t, "tenant-from-custom-claim", claims.Tenant)
}
