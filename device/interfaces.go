/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package device

import "context"

// Controllable represents a device that supports interactive remote control
// operations such as tapping, swiping, and taking screenshots.
//
// Only iOS and Android devices implement this interface. Tizen and WebOS TV
// devices do NOT — use a type assertion to check before calling.
type Controllable interface {
	// Tap performs a single tap at the given screen coordinates.
	Tap(x, y float64) error

	// DoubleTap performs a double tap at the given screen coordinates.
	DoubleTap(x, y float64) error

	// Swipe performs a swipe gesture from (x, y) to (endX, endY).
	Swipe(x, y, endX, endY float64) error

	// TouchAndHold performs a long-press at the given coordinates for the
	// specified duration in milliseconds.
	TouchAndHold(x, y, duration float64) error

	// Pinch performs a pinch gesture centered at (x, y) with the given scale
	// factor. A scale < 1 zooms out; scale > 1 zooms in.
	Pinch(x, y, scale float64) error

	// Home navigates the device to the home screen.
	Home() error

	// Lock locks the device screen.
	Lock() error

	// Unlock unlocks the device screen.
	Unlock() error

	// TypeText inputs the given text on the device. On Android this uses the
	// GADS IME server; on iOS it uses a WDA type action.
	TypeText(text string) error

	// Screenshot captures the current screen and returns the raw PNG bytes.
	Screenshot() ([]byte, error)

	// GetClipboard returns the current contents of the device clipboard.
	GetClipboard() (string, error)
}

// AppManager represents a device that supports app lifecycle operations.
// All platforms (Android, iOS, Tizen, WebOS) implement this interface.
type AppManager interface {
	// GetInstalledApps returns the list of installed app identifiers
	// (package names on Android, bundle IDs on iOS, app IDs on TV platforms).
	GetInstalledApps() ([]string, error)

	// InstallApp installs the application located at appPath on the device.
	// appPath is a local filesystem path to an APK (Android) or IPA (iOS).
	InstallApp(appPath string) error

	// UninstallApp removes the application identified by appID from the device.
	UninstallApp(appID string) error
}

// Provisionable represents the setup and teardown lifecycle of a managed device.
// All platform device types implement this interface.
type Provisionable interface {
	// Setup runs the full provisioning sequence for the device. This includes
	// port allocation, app installation, service startup (WDA, Appium, streaming),
	// and transitioning the device to the "live" state.
	//
	// ctx is used to cancel an in-progress setup when the device disconnects.
	Setup(ctx context.Context) error

	// Reset cancels any in-progress setup or running services, frees allocated
	// ports, and returns the device to the "init" state so it can be set up again.
	//
	// reason is a short human-readable string logged to explain why the reset
	// occurred (e.g. "device disconnected", "WDA failed to start").
	Reset(reason string)

	// ProviderState returns the current lifecycle state of the device as seen
	// by the provider: "init", "preparing", or "live".
	ProviderState() string
}

// ManagedDevice is the composite interface the DeviceManager works with.
// It combines identity (Info), lifecycle (Provisionable), and app management
// (AppManager) into a single interface.
//
// For control operations, use a type assertion to Controllable:
//
//	ctrl, ok := dev.(device.Controllable)
//	if !ok {
//	    // device does not support remote control (e.g. Tizen/WebOS)
//	}
type ManagedDevice interface {
	// Info returns a pointer to the DeviceInfo for this device. The returned
	// pointer is stable for the device's lifetime and may be read concurrently,
	// but writes must be coordinated by the device's own mutex.
	Info() *DeviceInfo

	Provisionable
	AppManager
}
