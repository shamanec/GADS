/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package devices

import "sync"

// Index of active Appium grid sessions (session ID -> device), replacing the old
// linear scan over all hub devices on every proxied session command.
//
// The device's `SessionID` field stays the source of truth - this registry is an
// index over it and both are updated together (create session, ReleaseFromAutomation).
//
// Lock ordering: the registry mutex is separate from each device's Mu. Code that
// needs both must take the device's Mu FIRST and the registry mutex second (this
// is what happens when ReleaseFromAutomation, called under d.Mu, unregisters the
// session). Never acquire a device's Mu while holding the registry mutex.

var gridSessionsMu sync.RWMutex
var gridSessions = map[string]*LocalHubDevice{}

// RegisterSession indexes an Appium session ID to the device serving it
func RegisterSession(sessionID string, device *LocalHubDevice) {
	if sessionID == "" {
		return
	}
	gridSessionsMu.Lock()
	gridSessions[sessionID] = device
	gridSessionsMu.Unlock()
}

// UnregisterSession removes a session ID from the index
func UnregisterSession(sessionID string) {
	if sessionID == "" {
		return
	}
	gridSessionsMu.Lock()
	delete(gridSessions, sessionID)
	gridSessionsMu.Unlock()
}

// DeviceBySession returns the device serving the given Appium session ID
func DeviceBySession(sessionID string) (*LocalHubDevice, bool) {
	gridSessionsMu.RLock()
	device, ok := gridSessions[sessionID]
	gridSessionsMu.RUnlock()
	return device, ok
}

// AllSessions returns a snapshot of the active session index
func AllSessions() map[string]*LocalHubDevice {
	gridSessionsMu.RLock()
	defer gridSessionsMu.RUnlock()
	snapshot := make(map[string]*LocalHubDevice, len(gridSessions))
	for sessionID, device := range gridSessions {
		snapshot[sessionID] = device
	}
	return snapshot
}
