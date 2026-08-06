/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package devices

import (
	"GADS/common/models"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	LockSourceUI  = "ui"
	LockSourceAPI = "api" // REST API lock — can be used by CI, scripts, or any non-UI client
)

// LocalHubDevice represents a device as tracked by the hub at runtime.
// All field access must be protected by Mu.
type LocalHubDevice struct {
	Mu     sync.RWMutex    `json:"-" bson:"-"` // protects this device's fields
	Device models.DBDevice `json:"info"`
	// Runtime fields synced from provider
	Host                     string `json:"host"`
	Connected                bool   `json:"connected"`
	ProviderState            string `json:"provider_state"`
	LastUpdatedTimestamp     int64  `json:"last_updated_timestamp"`
	SessionID                string `json:"-"`
	IsRunningAutomation      bool   `json:"is_running_automation"`
	LastAutomationActionTS   int64  `json:"last_automation_action_ts"`
	InUse                    bool   `json:"in_use"`
	InUseBy                  string `json:"in_use_by"`
	InUseByTenant            string `json:"in_use_by_tenant"`
	InUseTS                  int64  `json:"in_use_ts"`
	LockSource               string `json:"lock_source" bson:"-"` // "ui", "api", or ""
	LeaseExpiresAt           int64  `json:"-" bson:"-"`           // Unix ms, 0 = no active lease
	AppiumNewCommandTimeout  int64  `json:"appium_new_command_timeout"`
	AutomationSessionStartTS int64  `json:"-" bson:"-"` // Unix ms when the current Appium grid session was created, 0 = no session
	// Appium session truth reported by the provider (via the appium-plugin) - only
	// meaningful while ProviderReportsSessionState is true (older providers do not
	// send these fields)
	ProviderReportsSessionState   bool   `json:"-" bson:"-"`
	ProviderHasSession            bool   `json:"provider_has_session" bson:"-"`
	ProviderSessionID             string `json:"-" bson:"-"`
	ProviderLastCommandTS         int64  `json:"provider_last_command_ts" bson:"-"`
	ProviderSessionMissingSinceTS int64  `json:"-" bson:"-"` // Unix ms since the provider stopped reporting a session the hub still tracks, 0 = not missing
	IsAvailableForAutomation      bool   `json:"is_available_for_automation"`
	AppiumEnabled                 bool   `json:"appium_enabled" bson:"-"` // whether the device's provider runs Appium servers - devices without one can never serve automation
	// Ephemeral marks a device with no DB record (e.g. an Android emulator) - its
	// store entry is created and refreshed purely from provider updates and expires
	// when the provider stops reporting it
	Ephemeral             bool     `json:"ephemeral" bson:"-"`
	LastEphemeralReportTS int64    `json:"-" bson:"-"`         // Unix ms of the last provider update that included this ephemeral device
	Available             bool     `json:"available" bson:"-"` // if device is currently available - not only connected, but setup completed
	InUseWSConnection     net.Conn `json:"-" bson:"-"`         // stores the ws connection made when device is in use to send data from different sources
	LastActionTS          int64    `json:"-" bson:"-"`         // Timestamp of when was the last time an action was performed via the UI through the proxy to the provider
}

// All methods below assume the caller holds device.Mu.

// AcquireLock reserves the device for a user. Returns an error if already locked by another user.
func (d *LocalHubDevice) AcquireLock(user, tenant, source string) error {
	if d.IsLockedByOther(user, tenant) {
		return fmt.Errorf("device is already locked by another user")
	}
	d.InUseBy = user
	d.InUseByTenant = tenant
	d.InUseTS = time.Now().UnixMilli()
	d.LockSource = source
	d.LastActionTS = time.Now().UnixMilli()
	return nil
}

// ReleaseLock clears all lock fields unconditionally and closes the WebSocket connection if present.
func (d *LocalHubDevice) ReleaseLock() {
	if d.InUseWSConnection != nil {
		d.InUseWSConnection.Close()
		d.InUseWSConnection = nil
	}
	d.InUseBy = ""
	d.InUseByTenant = ""
	d.InUseTS = 0
	d.LockSource = ""
	d.LeaseExpiresAt = 0
}

// ReleaseLockIfNotHeld clears the lock only when no UI WebSocket and no active API lease are present.
// Used by automation cleanup to avoid releasing a manually-held device.
func (d *LocalHubDevice) ReleaseLockIfNotHeld() {
	if d.HasUISession() || d.HasActiveLease() {
		return
	}
	d.ReleaseLock()
}

// IsLocked reports whether the device is currently locked by any means:
// an active UI WebSocket, an active API lease, or a recent InUseTS timestamp.
func (d *LocalHubDevice) IsLocked() bool {
	if d.InUseWSConnection != nil {
		return true
	}
	if d.LockSource == LockSourceAPI && d.LeaseExpiresAt > time.Now().UnixMilli() {
		return true
	}
	if d.InUseTS > 0 && (time.Now().UnixMilli()-d.InUseTS) < 3000 {
		return true
	}
	return false
}

// IsLockedByOther reports whether the device is locked by a different user/tenant combination.
func (d *LocalHubDevice) IsLockedByOther(user, tenant string) bool {
	if d.InUseBy == "" {
		return false
	}
	if d.InUseBy == user && d.InUseByTenant == tenant {
		return false
	}
	return d.IsLocked()
}

// HasUISession reports whether a UI WebSocket connection is active on the device.
func (d *LocalHubDevice) HasUISession() bool {
	return d.InUseWSConnection != nil
}

// HasActiveLease reports whether an API-sourced lease is currently valid.
func (d *LocalHubDevice) HasActiveLease() bool {
	return d.LockSource == LockSourceAPI && d.LeaseExpiresAt > time.Now().UnixMilli()
}

// ClaimForAutomation marks the device as running an automation session, before the
// Appium session request is even forwarded, so no other automation request can grab it.
// The action timestamp is stamped so the janitor does not reset the device while the
// session is still being created. A newCommandTimeoutMS of 0 means the client
// explicitly disabled the idle expiry for this session.
func (d *LocalHubDevice) ClaimForAutomation(newCommandTimeoutMS int64) {
	d.IsRunningAutomation = true
	d.IsAvailableForAutomation = false
	d.LastAutomationActionTS = time.Now().UnixMilli()
	d.AppiumNewCommandTimeout = newCommandTimeoutMS
}

// ReleaseFromAutomation clears the device's automation session state, including its
// session registry entry, and releases the lock unless a UI or API session holds it.
// The grid session queue is poked so a queued session request can grab the device.
func (d *LocalHubDevice) ReleaseFromAutomation() {
	if d.SessionID != "" {
		UnregisterSession(d.SessionID)
	}
	d.SessionID = ""
	d.AutomationSessionStartTS = 0
	d.ProviderSessionMissingSinceTS = 0
	d.IsRunningAutomation = false
	d.IsAvailableForAutomation = true
	d.ReleaseLockIfNotHeld()
	NotifyDeviceFreed()
}

// RefreshLock updates InUseTS to now, keeping the lock alive.
func (d *LocalHubDevice) RefreshLock() {
	d.InUseTS = time.Now().UnixMilli()
}

// SetWSConnection stores the WebSocket connection on the device.
func (d *LocalHubDevice) SetWSConnection(conn net.Conn) {
	d.InUseWSConnection = conn
}

// ClearWSConnection nils the WebSocket connection field without closing it.
func (d *LocalHubDevice) ClearWSConnection() {
	d.InUseWSConnection = nil
}
