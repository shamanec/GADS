/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

// Package tizen implements the GADS Tizen TV device type. It provides a
// ManagedDevice implementation backed by the sdb and tizen CLI tools.
// Tizen TVs do not support screen control, so Controllable is not implemented.
package tizen

import (
	"sync"

	"GADS/common/models"
	"GADS/provider/devices"
)

// TizenDevice manages a single Tizen TV connected via sdb.
// It implements devices.ManagedDevice but NOT devices.Controllable — Tizen TVs
// are automation targets only (Appium-based testing).
type TizenDevice struct {
	// info is the shared serialisable state sent to the hub and stored in
	// MongoDB. Callers hold a pointer; writes are guarded by mu.
	info *models.DeviceInfo

	// AppiumPort is stored in d.info (single source of truth).

	// mu protects ProviderState and IsResetting during concurrent resets.
	mu sync.Mutex
	// setupMu serialises concurrent Setup calls for the same device.
	setupMu sync.Mutex

	// log is the per-device structured logger.
	log models.CustomLogger

	// cfg is the provider configuration (folder paths, feature flags, etc.).
	cfg *models.Provider

	// Injected dependencies — replaceable with mocks in tests.
	cmd   devices.CommandRunner
	ports devices.PortAllocator
	store devices.DeviceStore
}

// New constructs a TizenDevice with the given shared DeviceInfo and injected
// dependencies. info must not be nil. The device is initially in the "init"
// state; call Setup to provision it.
func New(
	info *models.DeviceInfo,
	cmd devices.CommandRunner,
	ports devices.PortAllocator,
	store devices.DeviceStore,
	log models.CustomLogger,
	cfg *models.Provider,
) *TizenDevice {
	info.ProviderState = "init"
	return &TizenDevice{
		info:  info,
		cmd:   cmd,
		ports: ports,
		store: store,
		log:   log,
		cfg:   cfg,
	}
}

// Info returns the shared DeviceInfo pointer for this device.
func (d *TizenDevice) Info() *models.DeviceInfo {
	return d.info
}

// ProviderState returns the current lifecycle state: "init", "preparing", or "live".
func (d *TizenDevice) ProviderState() string {
	return d.info.ProviderState
}
