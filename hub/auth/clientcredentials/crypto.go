/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package clientcredentials

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// GenerateClientID generates a unique client identifier with default "gads" prefix
// Format: gads_<timestamp>_<random_suffix>
func GenerateClientID() string {
	return GenerateClientIDWithPrefix("gads")
}

// GenerateClientIDWithPrefix generates a unique client identifier with custom prefix
// Format: <prefix>_<timestamp>_<random_suffix>
func GenerateClientIDWithPrefix(prefix string) string {
	if prefix == "" {
		prefix = "gads" // Fallback to default if empty
	}

	// Use timestamp for uniqueness + random suffix for security
	timestamp := time.Now().Unix()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes) // Ignore error for simplicity, crypto/rand is reliable

	suffix := base64.URLEncoding.EncodeToString(randomBytes)[:8] // 8 chars, no padding
	return fmt.Sprintf("%s_%d_%s", prefix, timestamp, suffix)
}

// GenerateClientSecret generates a cryptographically secure client secret
func GenerateClientSecret() (string, error) {
	// 32 bytes = 256 bits of entropy, encoded as base64
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Use URL-safe base64 encoding (no padding issues)
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// HashSecret hashes a client secret for secure storage
func HashSecret(secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret cannot be empty")
	}

	// Use bcrypt with reasonable cost (10 = ~100ms on modern hardware)
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), 10)
	if err != nil {
		return "", fmt.Errorf("failed to hash secret: %w", err)
	}

	return string(hash), nil
}

// ValidateSecret validates a client secret against its hash
// Uses constant-time comparison to prevent timing attacks
func ValidateSecret(secret, hash string) bool {
	if secret == "" || hash == "" {
		return false
	}

	// bcrypt.CompareHashAndPassword already uses constant-time comparison
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret))
	return err == nil
}

// GenerateSecureToken generates a secure random token of specified length
// Useful for additional tokens or nonces
func GenerateSecureToken(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be positive")
	}

	// Calculate bytes needed for desired base64 length
	// base64 encoding adds ~33% overhead, so adjust accordingly
	byteLength := (length * 3) / 4
	if byteLength == 0 {
		byteLength = 1
	}

	bytes := make([]byte, byteLength)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(bytes)

	// Trim to exact length requested
	if len(token) > length {
		token = token[:length]
	}

	return token, nil
}

// CompareTokens performs constant-time comparison of two tokens
// Prevents timing attacks when comparing sensitive values
func CompareTokens(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
