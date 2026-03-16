/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

// Package ios implements the GADS iOS device type. It provides full
// ManagedDevice and Controllable implementations backed by the go-ios library
// and the WebDriverAgent (WDA) HTTP server running on the device.
package ios

import (
	"sync"

	"GADS/common/models"
	"GADS/device"

	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/tunnel"
)

// IOSDevice manages a single iOS device connected via USB/usbmuxd.
// It implements device.ManagedDevice and device.Controllable.
//
// go-ios types (ios.DeviceEntry, tunnel.Tunnel) are held directly — we do not
// try to abstract the go-ios library; the goal is to contain its usage within
// this package.
type IOSDevice struct {
	// info is the shared serialisable device state. Callers hold a pointer;
	// writes are guarded by mu.
	info *device.DeviceInfo

	// Runtime port assignments, filled in during Setup.
	wdaPort       string // host port forwarded from WDA on device port 8100
	wdaStreamPort string // host port forwarded from WDA MJPEG on device port 9100
	streamPort    string // host port forwarded from GADS stream on device port 8765
	appiumPort    string // host port for the Appium server process

	// semVer is the parsed semantic version used to gate version-specific
	// behaviour (developer mode check on iOS 16+, userspace tunnel on iOS 17.4+).
	semVer *semver.Version

	// goIOSEntry is the go-ios device entry used for all go-ios library calls.
	// It is populated during Setup and updated when a userspace tunnel is established.
	goIOSEntry ios.DeviceEntry

	// goIOSTunnel holds the userspace tunnel for iOS 17.4+. A zero-value
	// Tunnel (Address == "") means no tunnel is active.
	goIOSTunnel tunnel.Tunnel

	// wdaReadyChan is closed (or receives true) when WDA responds to /status,
	// signalling that Setup can proceed past the WDA startup wait.
	wdaReadyChan chan bool

	// mu protects ProviderState and IsResetting during concurrent resets.
	mu sync.Mutex
	// setupMu serialises concurrent Setup calls for the same device.
	setupMu sync.Mutex

	// log is the per-device structured logger.
	log models.CustomLogger

	// cfg is the provider configuration (WDA bundle ID, folder paths, flags).
	cfg *models.Provider

	// Injected dependencies — replaceable with mocks in tests.
	cmd   device.CommandRunner // used only for the Appium process
	ports device.PortAllocator
	store device.DeviceStore
	http  device.HTTPClient
}

// New constructs an IOSDevice with the given shared DeviceInfo and injected
// dependencies. info must not be nil. The device starts in the "init" state;
// call Setup to provision it.
func New(
	info *device.DeviceInfo,
	cmd device.CommandRunner,
	ports device.PortAllocator,
	store device.DeviceStore,
	httpClient device.HTTPClient,
	log models.CustomLogger,
	cfg *models.Provider,
) *IOSDevice {
	info.ProviderState = "init"
	info.SupportedStreamTypes = device.StreamTypesForOS("ios")
	return &IOSDevice{
		info:         info,
		cmd:          cmd,
		ports:        ports,
		store:        store,
		http:         httpClient,
		log:          log,
		cfg:          cfg,
		wdaReadyChan: make(chan bool, 1),
	}
}

// Info returns the shared DeviceInfo pointer for this device.
func (d *IOSDevice) Info() *device.DeviceInfo {
	return d.info
}

// ProviderState returns the current lifecycle state of the device on the
// provider: "init", "preparing", or "live".
func (d *IOSDevice) ProviderState() string {
	return d.info.ProviderState
}
