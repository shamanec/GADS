/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package tizen

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"GADS/common/models"
	"GADS/device"
)

// Auto-connection constants — mirror the values in the legacy provider/devices/tizen.go.
const (
	maxRetries    = 5
	retryInterval = 30 * time.Second
	pauseAfterMax = 5 * time.Minute
)

// retryState tracks connection attempt history for a single Tizen device.
type retryState struct {
	deviceID    string
	retryCount  int
	lastAttempt time.Time
	isPaused    bool
	pauseUntil  time.Time
}

// RetryTracker is a shared, concurrency-safe registry of per-device retry state.
// A single instance should be created by the caller (e.g. device manager) and
// passed to AttemptConnection / ShouldAttemptConnection.
type RetryTracker struct {
	mu      sync.RWMutex
	entries map[string]*retryState
}

// NewRetryTracker allocates an empty RetryTracker.
func NewRetryTracker() *RetryTracker {
	return &RetryTracker{entries: make(map[string]*retryState)}
}

func (t *RetryTracker) get(id string) *retryState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.entries[id]
}

func (t *RetryTracker) set(s *retryState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[s.deviceID] = s
}

func (t *RetryTracker) resetEntry(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.entries[id]; ok {
		t.entries[id] = &retryState{deviceID: id}
	}
}

// ShouldAttemptConnection returns true when a connection attempt for deviceID
// should be made, taking retry count, interval, and pause state into account.
// On first call for a deviceID it initialises the entry and returns true.
func (t *RetryTracker) ShouldAttemptConnection(deviceID string) bool {
	state := t.get(deviceID)
	now := time.Now()

	if state == nil {
		t.set(&retryState{deviceID: deviceID})
		return true
	}

	if state.isPaused {
		if now.Before(state.pauseUntil) {
			return false
		}
		// Pause expired — reset and allow.
		t.set(&retryState{deviceID: deviceID})
		return true
	}

	if state.retryCount >= maxRetries {
		pauseUntil := now.Add(pauseAfterMax)
		t.set(&retryState{
			deviceID:    deviceID,
			retryCount:  state.retryCount,
			lastAttempt: state.lastAttempt,
			isPaused:    true,
			pauseUntil:  pauseUntil,
		})
		return false
	}

	if !state.lastAttempt.IsZero() && now.Sub(state.lastAttempt) < retryInterval {
		return false
	}

	return true
}

// recordAttempt updates the tracker after a connection attempt. pass is true
// on success.
func (t *RetryTracker) recordAttempt(deviceID string, pass bool, now time.Time) {
	if pass {
		t.resetEntry(deviceID)
		return
	}
	state := t.get(deviceID)
	newCount := 1
	if state != nil {
		newCount = state.retryCount + 1
	}
	s := &retryState{
		deviceID:    deviceID,
		retryCount:  newCount,
		lastAttempt: now,
	}
	if newCount >= maxRetries {
		s.isPaused = true
		s.pauseUntil = now.Add(pauseAfterMax)
	}
	t.set(s)
}

// GetConnectedDevices runs `sdb devices` and returns the list of device IDs
// currently reported as connected.
func GetConnectedDevices(cmd device.CommandRunner) []string {
	out, err := cmd.Run(context.Background(), "sdb", "devices")
	if err != nil {
		return nil
	}

	var ids []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "List of devices attached") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "device" {
			ids = append(ids, fields[0])
		}
	}
	return ids
}

// IsConnected checks whether deviceUDID appears in the sdb device list.
func IsConnected(cmd device.CommandRunner, deviceUDID string) bool {
	return slices.Contains(GetConnectedDevices(cmd), deviceUDID)
}

// AttemptConnection tries to connect to deviceUDID via `sdb connect` and
// records the result in tracker.
func AttemptConnection(
	tracker *RetryTracker,
	cmd device.CommandRunner,
	deviceUDID string,
	log models.CustomLogger,
) {
	state := tracker.get(deviceUDID)
	count := 1
	if state != nil {
		count = state.retryCount + 1
	}
	log.LogInfo("tizen_auto_connect",
		fmt.Sprintf("Attempting to connect to Tizen device %s — attempt %d/%d", deviceUDID, count, maxRetries))

	ip, err := tvHost(deviceUDID)
	if err != nil {
		tracker.recordAttempt(deviceUDID, false, time.Now())
		log.LogWarn("tizen_auto_connect",
			fmt.Sprintf("Invalid UDID format for %s: %v", deviceUDID, err))
		return
	}

	now := time.Now()
	_, runErr := cmd.Run(context.Background(), "sdb", "connect", ip)
	tracker.recordAttempt(deviceUDID, runErr == nil, now)

	if runErr != nil {
		log.LogWarn("tizen_auto_connect",
			fmt.Sprintf("Failed to connect to %s attempt %d/%d: %v", deviceUDID, count, maxRetries, runErr))
	} else {
		log.LogInfo("tizen_auto_connect",
			fmt.Sprintf("Successfully connected to Tizen device %s", deviceUDID))
	}
}

// tvHost extracts the host IP from a deviceUDID in IP:PORT format.
func tvHost(udid string) (string, error) {
	if matched, _ := regexp.MatchString(`^([0-9]{1,3}\.){3}[0-9]{1,3}:\d+$`, udid); matched {
		return strings.Split(udid, ":")[0], nil
	}
	return "", fmt.Errorf("tvHost: invalid format %q (expected IP:PORT)", udid)
}
