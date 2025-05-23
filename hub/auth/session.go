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
	"GADS/common/models"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	sessionsMap = make(map[string]*Session)
	mapMutex    = &sync.Mutex{}
)

type Session struct {
	User      models.User
	SessionID string
	ExpireAt  time.Time
}

func GetSession(sessionID string) (*Session, bool) {
	mapMutex.Lock()
	defer mapMutex.Unlock()
	session, exists := sessionsMap[sessionID]
	return session, exists
}

func CreateSession(user models.User, sessionID uuid.UUID) {

	session := &Session{
		User:      user,
		SessionID: sessionID.String(),
		ExpireAt:  time.Now().Add(time.Hour),
	}

	mapMutex.Lock()
	sessionsMap[sessionID.String()] = session
	mapMutex.Unlock()
}

func DeleteSession(sessionID string) {
	mapMutex.Lock()
	defer mapMutex.Unlock()
	delete(sessionsMap, sessionID)
}
