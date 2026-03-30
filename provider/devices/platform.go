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
	"context"

	"GADS/common/models"
)

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
	GetHost() string
	SetHost(host string)

	// Hub sync - builds the lightweight update sent to the hub each second
	ToSyncUpdate() models.ProviderDeviceSync

	// Appium - returns platform-specific Appium server capabilities
	AppiumCapabilities() models.AppiumServerCapabilities

	// Infrastructure accessors
	GetLogger() models.CustomLogger
	GetContext() context.Context
	GetAppiumPort() string
	SetAppiumPort(port string)
	SetNewContext(ctx context.Context, cancel context.CancelFunc)

	// Port accessor — platform types return their stream port; TV types return ""
	GetStreamPort() string

	// Appium state accessors
	GetAppiumSessionID() string
	SetAppiumSessionID(id string)
	SetAppiumUp(up bool)
	SetAppiumLastPingTS(ts int64)
	SetHasAppiumSession(has bool)
	GetIsAppiumUp() bool

	// Runtime state accessors (provider-only fields on RuntimeState)
	GetIsResetting() bool
	SetIsResetting(v bool)
	GetHardwareModelValue() string
	SetHardwareModel(model string)
	GetStreamTargetFPS() int
	SetStreamTargetFPS(fps int)
	GetStreamJpegQuality() int
	SetStreamJpegQuality(q int)
	GetStreamScalingFactor() int
	SetStreamScalingFactor(f int)
	GetCurrentRotationValue() string
	SetCurrentRotation(rotation string)
	GetSupportedStreamTypes() []models.StreamType
	SetSupportedStreamTypes(types []models.StreamType)
	GetInstalledAppIDs() []string
	SetInstalledAppIDs(apps []string)
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
