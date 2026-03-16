/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

// Package android implements the GADS Android device type. It provides full
// ManagedDevice and Controllable implementations backed by ADB and the
// GADS-Settings companion app running on the device.
package android

import (
	"sync"

	"GADS/common/models"
	"GADS/device"
	"github.com/Masterminds/semver"
)

// AndroidDevice manages a single Android device connected via ADB.
// It implements device.ManagedDevice and device.Controllable.
//
// All platform-specific state (ports, semver, context) is held as private
// fields — nothing here is serialised to JSON or BSON directly.
type AndroidDevice struct {
	// info is the shared serialisable state sent to the hub and stored in
	// MongoDB. Callers hold a pointer; writes are guarded by mu.
	info *device.DeviceInfo

	// Runtime port assignments, filled in during Setup.
	streamPort       string // host port forwarded from device port 1991 (GADS-stream)
	imePort          string // host port forwarded from device port 1993 (GADS IME)
	remoteServerPort string // host port forwarded from device port 1994 (remote control)
	appiumPort       string // host port for the Appium server process

	// semVer is the parsed semantic version of the device OS. Used to gate
	// behaviour that differs across Android versions (e.g. POST_NOTIFICATIONS
	// permission is only required on Android 15+).
	semVer *semver.Version

	// mu protects ProviderState and IsResetting during concurrent resets.
	mu sync.Mutex
	// setupMu serialises concurrent Setup calls for the same device.
	setupMu sync.Mutex

	// log is the per-device structured logger.
	log models.CustomLogger

	// cfg is the provider configuration (folder paths, feature flags, etc.).
	cfg *models.Provider

	// Injected dependencies — replaceable with mocks in tests.
	cmd   device.CommandRunner
	ports device.PortAllocator
	store device.DeviceStore
	http  device.HTTPClient
}

// New constructs an AndroidDevice with the given shared DeviceInfo and injected
// dependencies. info must not be nil. The device is initially in the "init"
// state; call Setup to provision it.
func New(
	info *device.DeviceInfo,
	cmd device.CommandRunner,
	ports device.PortAllocator,
	store device.DeviceStore,
	httpClient device.HTTPClient,
	log models.CustomLogger,
	cfg *models.Provider,
) *AndroidDevice {
	info.ProviderState = "init"
	info.SupportedStreamTypes = device.StreamTypesForOS("android")
	return &AndroidDevice{
		info:  info,
		cmd:   cmd,
		ports: ports,
		store: store,
		http:  httpClient,
		log:   log,
		cfg:   cfg,
	}
}

// Info returns the shared DeviceInfo pointer for this device. The pointer is
// stable for the device's lifetime; reads are generally safe without a lock,
// but writes to the struct fields must be guarded by the caller.
func (d *AndroidDevice) Info() *device.DeviceInfo {
	return d.info
}

// ProviderState returns the current lifecycle state of the device as seen by
// the provider: "init", "preparing", or "live".
func (d *AndroidDevice) ProviderState() string {
	return d.info.ProviderState
}
