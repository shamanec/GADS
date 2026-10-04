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
	"sync"
	"time"
)

const (
	// DefaultTokenTTL is how long a session survives without being used. Every
	// request slides it forward, so it is an inactivity timeout rather than a
	// limit on how long a client can keep working
	DefaultTokenTTL = time.Hour
	// DefaultMaxSessionAge is how long a user session can live in total, no
	// matter how much it is used. 0 means no absolute limit
	DefaultMaxSessionAge = 24 * time.Hour
)

var (
	lifetimeMu    sync.RWMutex
	tokenTTL      = DefaultTokenTTL
	maxSessionAge = DefaultMaxSessionAge
)

// SetTokenLifetimes configures the session lifetimes for the hub. A non-positive
// ttl keeps the current value; a negative maxAge keeps the current value, while
// 0 removes the absolute session limit entirely
func SetTokenLifetimes(ttl, maxAge time.Duration) {
	lifetimeMu.Lock()
	defer lifetimeMu.Unlock()

	if ttl > 0 {
		tokenTTL = ttl
	}
	if maxAge >= 0 {
		maxSessionAge = maxAge
	}
}

// TokenTTL returns how long a session survives without being used
func TokenTTL() time.Duration {
	lifetimeMu.RLock()
	defer lifetimeMu.RUnlock()

	return tokenTTL
}

// MaxSessionAge returns the absolute lifetime of a user session, 0 if unlimited
func MaxSessionAge() time.Duration {
	lifetimeMu.RLock()
	defer lifetimeMu.RUnlock()

	return maxSessionAge
}
