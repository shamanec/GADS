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
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"time"
)

// getTURNUsernameSuffix retrieves the customizable username suffix from environment.
// Following the GADS pattern (like GADS_CAPABILITY_PREFIX, GADS_CLIENT_ID_PREFIX).
//
// This allows multi-tenant deployments and environment-specific identifiers.
// Examples:
//   - export GADS_TURN_USERNAME_SUFFIX="myorg" → generates "timestamp:myorg"
//   - export GADS_TURN_USERNAME_SUFFIX="gads-dev" → generates "timestamp:gads-dev"
//   - (not set) → generates "timestamp:gads" (default)
func getTURNUsernameSuffix() string {
	if suffix := os.Getenv("GADS_TURN_USERNAME_SUFFIX"); suffix != "" {
		return suffix
	}
	return "gads" // Default fallback
}

// GenerateTURNCredentials generates time-limited TURN credentials using HMAC-SHA1
// following the TURN REST API specification (draft-uberti-behave-turn-rest).
//
// The credentials are self-validating and stateless:
// - Username format: "<timestamp>:<suffix>" where:
//   - timestamp: Unix time of expiration
//   - suffix: Customizable via GADS_TURN_USERNAME_SUFFIX env var (default: "gads")
//
// - Password: base64(HMAC-SHA1(shared_secret, username))
//
// The username suffix is customizable for multi-tenant deployments and environment isolation.
//
// Returns:
// - username: Time-limited username containing expiration timestamp and suffix
// - password: HMAC-SHA1 signature for authentication
// - expiresAt: Unix timestamp when credentials expire
func GenerateTURNCredentials(sharedSecret string, ttl int) (string, string, int64) {
	expiresAt := time.Now().Unix() + int64(ttl)
	suffix := getTURNUsernameSuffix()
	username := fmt.Sprintf("%d:%s", expiresAt, suffix)

	mac := hmac.New(sha1.New, []byte(sharedSecret))
	mac.Write([]byte(username))
	password := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return username, password, expiresAt
}
