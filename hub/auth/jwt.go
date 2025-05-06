package auth

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Secret key for HMAC signature (HS256)
// In production, this should be loaded from an environment variable or secure configuration
var hmacSecret = []byte("gads_secret_key_replace_in_production")

// RSA key pair structure for RS256 signature
var (
	signKey   *rsa.PrivateKey
	verifyKey *rsa.PublicKey
)

// JWTClaims defines the structure of claims in the JWT token
type JWTClaims struct {
	jwt.RegisteredClaims
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Scope    []string `json:"scope"`
	Tenant   string   `json:"tenant"`
}

// GenerateJWT generates a JWT token using HS256
func GenerateJWT(username, role, tenant string, scope []string, duration time.Duration) (string, error) {
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gads",
		},
		Username: username,
		Role:     role,
		Scope:    scope,
		Tenant:   tenant,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(hmacSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateJWT validates a JWT token and returns its claims
func ValidateJWT(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify if the signing method is the expected one
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return hmacSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	if claims, ok := token.Claims.(*JWTClaims); ok {
		return claims, nil
	}

	return nil, errors.New("invalid claims")
}

// ExtractTokenFromBearer extracts the JWT token from an Authorization Bearer header
func ExtractTokenFromBearer(authHeader string) (string, error) {
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return "", errors.New("invalid Authorization header format")
	}
	return authHeader[7:], nil
}

// GetClaimsFromToken is a utility function that extracts the claims from a JWT token
func GetClaimsFromToken(token string) (*JWTClaims, error) {
	if token == "" {
		return nil, errors.New("empty token provided")
	}

	// Validate the token and get the claims
	claims, err := ValidateJWT(token)
	if err != nil {
		return nil, errors.New("failed to validate JWT token: " + err.Error())
	}

	// Verificar se os campos obrigatórios estão presentes
	if claims.Username == "" {
		return nil, errors.New("token has no username claim")
	}

	return claims, nil
}
