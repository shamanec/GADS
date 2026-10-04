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
	"sync"
	"time"

	"github.com/google/uuid"
)

// Sessions are the authority on whether a token still works. A token the hub
// issued is only as alive as the session it was issued for - using it slides the
// session's idle window forward, and dropping the session makes the token stop
// working immediately, even though it is still correctly signed.
//
// They live in memory only. A hub restart ends every session and everyone
// authenticates again, which is also what keeps logout honest - there is no
// place a deleted session could come back from.

// sessionSweepInterval is how often the janitor drops sessions nobody uses
// anymore. They are dropped on use as well, this only keeps the map from
// growing with sessions that are never touched again
const sessionSweepInterval = time.Minute

var ErrSessionExpired = errors.New("session expired")

type Session struct {
	ID       string
	Username string
	// ClientID is set for sessions started through the OAuth2 client credentials
	// flow. Those have no absolute age limit - the client holds a secret and can
	// authenticate again at any time anyway
	ClientID string
	AuthTime time.Time
	LastUsed time.Time
}

var (
	sessionsMu sync.Mutex
	sessions   = make(map[string]*Session)
)

// CreateSession starts a session for a user, or for a machine client when
// clientID is not empty, and returns it
func CreateSession(username, clientID string) *Session {
	now := time.Now()
	session := &Session{
		ID:       uuid.NewString(),
		Username: username,
		ClientID: clientID,
		AuthTime: now,
		LastUsed: now,
	}

	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	sessions[session.ID] = session

	return session
}

// TouchSession marks a session as used and returns it. An unknown session, one
// idle for longer than the token TTL or one past the absolute session age is
// dropped on the spot and reported as expired, so its token can never come back
// to life on a later request
func TouchSession(id string) (*Session, error) {
	now := time.Now()

	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	session, exists := sessions[id]
	if !exists {
		return nil, ErrSessionExpired
	}

	if sessionExpired(session, now) {
		delete(sessions, id)
		return nil, ErrSessionExpired
	}

	session.LastUsed = now

	return session, nil
}

// DeleteSession ends a session, making every token issued for it stop working
func DeleteSession(id string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	delete(sessions, id)
}

// sessionExpired reports whether a session went idle for longer than the token
// TTL, or whether a user session outlived the absolute session age
func sessionExpired(session *Session, now time.Time) bool {
	if now.Sub(session.LastUsed) > TokenTTL() {
		return true
	}

	maxAge := MaxSessionAge()
	if session.ClientID != "" || maxAge <= 0 {
		return false
	}

	return now.Sub(session.AuthTime) > maxAge
}

// SweepExpiredSessions drops expired sessions until the hub stops
func SweepExpiredSessions() {
	for {
		time.Sleep(sessionSweepInterval)
		sweepExpiredSessions(time.Now())
	}
}

func sweepExpiredSessions(now time.Time) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	for id, session := range sessions {
		if sessionExpired(session, now) {
			delete(sessions, id)
		}
	}
}

// ActiveSessionCount returns how many sessions are currently held
func ActiveSessionCount() int {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	return len(sessions)
}
