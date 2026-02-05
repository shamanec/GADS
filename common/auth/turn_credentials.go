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
	"time"
)

// GenerateTURNCredentials generates time-limited TURN credentials using HMAC-SHA1
// following the TURN REST API specification (draft-uberti-behave-turn-rest).
//
// The credentials are self-validating and stateless:
// - Username format: "<timestamp>:<suffix>" where:
//   - timestamp: Unix time of expiration
//   - suffix: Customizable via CLI flag --turn-username-suffix (default: "gads")
//
// - Password: base64(HMAC-SHA1(shared_secret, username))
//
// The username suffix is customizable for multi-tenant deployments and environment isolation.
//
// Returns:
// - username: Time-limited username containing expiration timestamp and suffix
// - password: HMAC-SHA1 signature for authentication
// - expiresAt: Unix timestamp when credentials expire
func GenerateTURNCredentials(sharedSecret string, ttl int, suffix string) (string, string, int64) {
	if suffix == "" {
		suffix = "gads"
	}
	expiresAt := time.Now().Unix() + int64(ttl)
	username := fmt.Sprintf("%d:%s", expiresAt, suffix)

	mac := hmac.New(sha1.New, []byte(sharedSecret))
	mac.Write([]byte(username))
	password := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return username, password, expiresAt
}
