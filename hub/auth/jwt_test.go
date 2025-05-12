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

func (m *MockSecretStore) AddSecretKey(secretKey *SecretKey) error {
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

func (m *MockSecretStore) UpdateSecretKey(secretKey *SecretKey) error {
	m.keys[secretKey.Origin] = secretKey
	return nil
}

func (m *MockSecretStore) DisableSecretKey(id primitive.ObjectID) error {
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
	})

	// Add web key
	store.AddSecretKey(&SecretKey{
		Origin:    "https://web.example.com",
		Key:       "web_secret_key",
		IsDefault: false,
	})

	// Add mobile key
	store.AddSecretKey(&SecretKey{
		Origin:    "mobile-app",
		Key:       "mobile_secret_key",
		IsDefault: false,
	})

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
	// Create a mock store with a default key
	store := NewMockSecretStore()
	store.AddSecretKey(&SecretKey{
		Origin:    "default",
		Key:       "default_secret_key",
		IsDefault: true,
	})

	// Create a secret cache
	cache := NewSecretCache(store, time.Minute)

	// Test GetKey for default key
	key := cache.GetKey("unknown-origin")
	assert.Equal(t, []byte("default_secret_key"), key)

	// Test GetDefaultKey
	defaultKey := cache.GetDefaultKey()
	assert.Equal(t, []byte("default_secret_key"), defaultKey)

	// Add a new key and test GetKey
	store.AddSecretKey(&SecretKey{
		Origin:    "test-origin",
		Key:       "test_secret_key",
		IsDefault: false,
	})

	// Refresh cache
	err := cache.Refresh()
	assert.NoError(t, err)

	// Test GetKey for specific origin
	key = cache.GetKey("test-origin")
	assert.Equal(t, []byte("test_secret_key"), key)
}
