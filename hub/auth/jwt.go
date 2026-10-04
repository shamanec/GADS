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
	"crypto/rsa"
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// RSA key pair structure for RS256 signature
var (
	signKey   *rsa.PrivateKey
	verifyKey *rsa.PublicKey
)

// Global cache instance with mutex for initialization
var (
	secretCache     *SecretCache
	secretCacheMu   sync.Mutex
	secretCacheInit bool
)

// Error definitions
var (
	ErrSecretCacheNotInitialized = errors.New("secret cache is not initialized")
	ErrDefaultKeyRequired        = errors.New("default secret key is required but not available")
)

// TokenIssuer is the `iss` claim stamped on every token the hub issues itself.
// Only those tokens are backed by a session - tokens from another issuer (e.g.
// an external identity provider validated with an origin secret key) are taken
// at face value for as long as their own expiry says.
const TokenIssuer = "gads"

// JWTClaims defines the structure of claims in the JWT token
type JWTClaims struct {
	jwt.RegisteredClaims
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Scope    []string `json:"scope"`
	Tenant   string   `json:"tenant"`
	Origin   string   `json:"origin,omitempty"` // Added origin claim
	// SessionID points at the session this token was issued for. The session is
	// what decides whether the token still works - see session.go
	SessionID string `json:"sid,omitempty"`
}

// InitSecretCache initializes the secret cache with the database store
func InitSecretCache(store *SecretStore, refreshInterval time.Duration) error {
	secretCacheMu.Lock()
	defer secretCacheMu.Unlock()

	if !secretCacheInit {
		secretCache = NewSecretCache(store, refreshInterval)

		// Make sure we have a default key (the cache will handle creating one if needed)
		err := secretCache.Refresh()
		if err != nil {
			return err
		}

		// Verify that we have a default key
		if secretCache.GetDefaultKey() == nil {
			return ErrDefaultKeyRequired
		}

		secretCacheInit = true
	}

	return nil
}

// GetSecretCache returns the global secret cache instance
func GetSecretCache() *SecretCache {
	secretCacheMu.Lock()
	defer secretCacheMu.Unlock()

	return secretCache
}

// getSecretKeyForOrigin returns the secret key for the given origin
func getSecretKeyForOrigin(origin string) ([]byte, error) {
	if secretCache == nil {
		return nil, ErrSecretCacheNotInitialized
	}

	key := secretCache.GetKey(origin)
	if key == nil {
		return nil, errors.New("no key found for origin: " + origin)
	}

	return key, nil
}

// getDefaultSecretKey returns the default secret key
func getDefaultSecretKey() ([]byte, error) {
	if secretCache == nil {
		return nil, ErrSecretCacheNotInitialized
	}

	key := secretCache.GetDefaultKey()
	if key == nil {
		return nil, ErrDefaultKeyRequired
	}

	return key, nil
}

// GenerateJWT starts a user session and returns a token for it. The token stops
// working as soon as that session is gone, so its own expiry is only a backstop
// for the absolute session age
func GenerateJWT(username, role, tenant string, scope []string, origin ...string) (string, error) {
	originValue := ""
	if len(origin) > 0 && origin[0] != "" {
		originValue = origin[0]
	}

	return generateSessionJWT(CreateSession(username, ""), role, tenant, scope, originValue)
}

// GenerateClientCredentialsJWT starts a session for a machine client authenticated
// through the OAuth2 client credentials flow and returns a token for it. Those
// sessions have no absolute age limit - the client holds a secret and could
// authenticate again at any time anyway
func GenerateClientCredentialsJWT(clientID, username, role, tenant string, scope []string, origin string) (string, error) {
	return generateSessionJWT(CreateSession(username, clientID), role, tenant, scope, origin)
}

// generateSessionJWT signs a token bound to a session
func generateSessionJWT(session *Session, role, tenant string, scope []string, origin string) (string, error) {
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   session.Username,
			ExpiresAt: sessionTokenExpiry(session),
			IssuedAt:  jwt.NewNumericDate(session.AuthTime),
			Issuer:    TokenIssuer,
		},
		Username:  session.Username,
		Role:      role,
		Scope:     scope,
		Tenant:    tenant,
		Origin:    origin,
		SessionID: session.ID,
	}

	return signClaims(claims)
}

// sessionTokenExpiry returns the `exp` claim for a session token. The session is
// what actually ends a login, so the only expiry worth signing into the token is
// the absolute session age - sessions without one get a token without an expiry
func sessionTokenExpiry(session *Session) *jwt.NumericDate {
	maxAge := MaxSessionAge()
	if session.ClientID != "" || maxAge <= 0 {
		return nil
	}

	return jwt.NewNumericDate(session.AuthTime.Add(maxAge))
}

// signClaims signs the claims with the secret key of their origin, falling back
// to the default key when the origin has none
func signClaims(claims JWTClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	var secretKey []byte
	var err error

	if claims.Origin != "" {
		secretKey, err = getSecretKeyForOrigin(claims.Origin)
		if err != nil {
			// If we can't get a key for the specific origin, try the default
			secretKey, err = getDefaultSecretKey()
			if err != nil {
				return "", err
			}
		}
	} else {
		secretKey, err = getDefaultSecretKey()
		if err != nil {
			return "", err
		}
	}

	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateJWT validates a JWT token using the appropriate secret key and returns its claims
func ValidateJWT(tokenString string, origin ...string) (*JWTClaims, error) {
	var keyFunc jwt.Keyfunc
	var usedOrigin string

	// Set up the key function based on whether we're using origin-specific keys
	if len(origin) > 0 && origin[0] != "" {
		usedOrigin = origin[0]
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}

			secretKey, err := getSecretKeyForOrigin(usedOrigin)
			if err != nil {
				// Try default key if origin-specific key not found
				secretKey, err = getDefaultSecretKey()
				if err != nil {
					return nil, err
				}
			}
			return secretKey, nil
		}
	} else {
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}

			// Try to get the origin from claims
			if claims, ok := token.Claims.(*JWTClaims); ok && claims.Origin != "" {
				usedOrigin = claims.Origin
				secretKey, err := getSecretKeyForOrigin(claims.Origin)
				if err == nil {
					return secretKey, nil
				}
				// If key for origin not found, continue to use default
			}

			// Fallback to default key
			secretKey, err := getDefaultSecretKey()
			if err != nil {
				return nil, err
			}
			return secretKey, nil
		}
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, keyFunc)
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}

	// Parse token as MapClaims to access custom claims
	parsedToken, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	var mapClaims jwt.MapClaims
	if err == nil {
		if mc, ok := parsedToken.Claims.(jwt.MapClaims); ok {
			mapClaims = mc
		}
	}

	// Get the secret key from the origin to know which claim to use
	var userIdentifierClaim string
	var tenantIdentifierClaim string
	store := GetSecretCache().store
	if store != nil {
		secretKey, err := store.GetSecretKeyByOrigin(usedOrigin)
		if err != nil {
			// fallback to default
			secretKey, _ = store.GetDefaultSecretKey()
		}
		if secretKey != nil {
			if secretKey.UserIdentifierClaim != "" {
				userIdentifierClaim = secretKey.UserIdentifierClaim
			}
			if secretKey.TenantIdentifierClaim != "" {
				tenantIdentifierClaim = secretKey.TenantIdentifierClaim
			}
		}
	}

	// Get the correct claim value for username (dynamic)
	if userIdentifierClaim != "" && mapClaims != nil {
		if val, ok := mapClaims[userIdentifierClaim]; ok {
			if s, ok := val.(string); ok {
				claims.Username = s
			} else {
				claims.Username = ""
			}
		} else {
			switch userIdentifierClaim {
			case "sub":
				claims.Username = claims.Subject
			}
		}
	}

	// Get the correct claim value for tenant (dynamic)
	if tenantIdentifierClaim != "" && mapClaims != nil {
		if val, ok := mapClaims[tenantIdentifierClaim]; ok {
			if s, ok := val.(string); ok {
				claims.Tenant = s
			} else {
				claims.Tenant = ""
			}
		}
	}

	return claims, nil
}

// ExtractTokenFromBearer extracts the JWT token from an Authorization Bearer header
func ExtractTokenFromBearer(authHeader string) (string, error) {
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return "", errors.New("invalid Authorization header format")
	}
	return authHeader[7:], nil
}

// GetClaimsFromToken is a utility function that extracts the claims from a JWT token
func GetClaimsFromToken(token string, origin ...string) (*JWTClaims, error) {
	if token == "" {
		return nil, errors.New("empty token provided")
	}

	// Validate the token and get the claims
	return ValidateJWT(token, origin...)
}
