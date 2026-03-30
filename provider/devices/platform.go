/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package devices

import "GADS/common/models"

// PlatformDevice is the interface that each OS-specific device type implements.
// It provides a unified API for device lifecycle and app management,
// eliminating the need for switch/case on device.OS throughout the codebase.
type PlatformDevice interface {
	// Lifecycle
	Setup() error
	Reset(reason string)

	// Apps
	InstallApp(path string) error
	UninstallApp(bundleID string) error
	GetInstalledApps() ([]models.DeviceApp, error)
	GetInstalledAppBundleIDs() []string
	LaunchApp(bundleID string) error
	KillApp(bundleID string) error

	// State accessors
	GetUDID() string
	GetOS() string
	GetDBDevice() *models.Device
	GetProviderState() string
	SetProviderState(state string)
	IsConnected() bool
	SetConnected(connected bool)

	// Hub sync - builds a models.Device with runtime fields populated for JSON serialization to hub
	ToHubDevice() models.Device

	// Appium - returns platform-specific Appium server capabilities
	AppiumCapabilities() models.AppiumServerCapabilities
}

// RemoteControllable extends PlatformDevice with capabilities for devices that support
// remote control, streaming, and screen interaction (Android, iOS).
// TV platforms (Tizen, WebOS) do not implement this interface.
type RemoteControllable interface {
	PlatformDevice

	// Device info
	GetScreenSize() (width, height string, err error)
	GetHardwareModel() (string, error)

	// Rotation
	GetCurrentRotation() (string, error)
	ChangeRotation(rotation string) error

	// Stream settings
	ApplyStreamSettings() error
	UpdateStreamSettingsOnDevice() error
}
