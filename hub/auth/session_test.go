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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// withTestLifetimes applies session lifetimes for the duration of a test and
// restores the previous ones afterwards
func withTestLifetimes(t *testing.T, ttl, maxAge time.Duration) {
	t.Helper()

	previousTTL, previousMaxAge := TokenTTL(), MaxSessionAge()
	SetTokenLifetimes(ttl, maxAge)
	t.Cleanup(func() {
		SetTokenLifetimes(previousTTL, previousMaxAge)
	})
}

// withEmptySessions runs a test against a session store of its own
func withEmptySessions(t *testing.T) {
	t.Helper()

	sessionsMu.Lock()
	previousSessions := sessions
	sessions = make(map[string]*Session)
	sessionsMu.Unlock()

	t.Cleanup(func() {
		sessionsMu.Lock()
		sessions = previousSessions
		sessionsMu.Unlock()
	})
}

// ageSession rewinds a session's timestamps to make it look older than it is
func ageSession(t *testing.T, id string, sinceAuth, sinceLastUsed time.Duration) {
	t.Helper()

	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	session, exists := sessions[id]
	assert.True(t, exists, "session should exist")

	now := time.Now()
	session.AuthTime = now.Add(-sinceAuth)
	session.LastUsed = now.Add(-sinceLastUsed)
}

func TestTouchSessionSlidesTheIdleWindow(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)

	session := CreateSession("testuser", "")
	// Used 59 minutes ago - just inside the hour
	ageSession(t, session.ID, 59*time.Minute, 59*time.Minute)

	touched, err := TouchSession(session.ID)
	assert.NoError(t, err)
	assert.WithinDuration(t, time.Now(), touched.LastUsed, time.Second, "using a session should slide it forward")

	// Which means it is good for another full hour from now
	ageSession(t, session.ID, 2*time.Hour, 59*time.Minute)
	_, err = TouchSession(session.ID)
	assert.NoError(t, err)
}

func TestTouchSessionExpiresWhenIdle(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)

	session := CreateSession("testuser", "")
	ageSession(t, session.ID, time.Hour+time.Minute, time.Hour+time.Minute)

	_, err := TouchSession(session.ID)
	assert.ErrorIs(t, err, ErrSessionExpired)
	assert.Equal(t, 0, ActiveSessionCount(), "an expired session is dropped, not left to be found again")
}

func TestTouchSessionExpiresAtTheAbsoluteAge(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)

	// In constant use, but started over a day ago
	session := CreateSession("testuser", "")
	ageSession(t, session.ID, 24*time.Hour+time.Minute, time.Minute)

	_, err := TouchSession(session.ID)
	assert.ErrorIs(t, err, ErrSessionExpired)
	assert.Equal(t, 0, ActiveSessionCount())
}

func TestTouchSessionIgnoresAbsoluteAgeForClientCredentials(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)

	session := CreateSession("testuser", "gads_client_id")
	ageSession(t, session.ID, 240*time.Hour, time.Minute)

	_, err := TouchSession(session.ID)
	assert.NoError(t, err, "machine clients are not bound by the absolute session age")

	// They do still go away when nobody uses them
	ageSession(t, session.ID, 240*time.Hour, time.Hour+time.Minute)
	_, err = TouchSession(session.ID)
	assert.ErrorIs(t, err, ErrSessionExpired)
}

func TestTouchSessionWithoutAbsoluteAgeLimit(t *testing.T) {
	withTestLifetimes(t, time.Hour, 0)
	withEmptySessions(t)

	session := CreateSession("testuser", "")
	ageSession(t, session.ID, 240*time.Hour, time.Minute)

	_, err := TouchSession(session.ID)
	assert.NoError(t, err)
}

func TestTouchUnknownSession(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)

	_, err := TouchSession("no-such-session")
	assert.ErrorIs(t, err, ErrSessionExpired)

	_, err = TouchSession("")
	assert.ErrorIs(t, err, ErrSessionExpired)
}

func TestDeleteSession(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)

	session := CreateSession("testuser", "")
	DeleteSession(session.ID)

	_, err := TouchSession(session.ID)
	assert.ErrorIs(t, err, ErrSessionExpired, "a deleted session never comes back")
}

func TestSweepExpiredSessions(t *testing.T) {
	withTestLifetimes(t, time.Hour, 24*time.Hour)
	withEmptySessions(t)

	live := CreateSession("live-user", "")
	idle := CreateSession("idle-user", "")
	old := CreateSession("old-user", "")
	ageSession(t, idle.ID, 2*time.Hour, time.Hour+time.Minute)
	ageSession(t, old.ID, 24*time.Hour+time.Minute, time.Minute)

	sweepExpiredSessions(time.Now())

	assert.Equal(t, 1, ActiveSessionCount())
	_, err := TouchSession(live.ID)
	assert.NoError(t, err)
}
