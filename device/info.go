/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package device

import (
	"strconv"
)

// DeviceInfo is the serializable core of a device. It is the wire format for
// provider-to-hub communication and the document stored in MongoDB. Platform
// device types (AndroidDevice, IOSDevice, etc.) embed a pointer to this struct
// rather than duplicating fields.
//
// Fields tagged with bson:"-" are runtime-only and are never persisted to the
// database. They are still included in JSON payloads sent from provider to hub.
type DeviceInfo struct {
	// --- Persisted fields (stored in MongoDB) ---

	// UDID is the unique device identifier. For Android this is the ADB serial;
	// for iOS it is the device UDID returned by go-ios.
	UDID string `json:"udid" bson:"udid"`

	// OS is the operating system of the device: "android", "ios", "tizen", or "webos".
	OS string `json:"os" bson:"os"`

	// Name is the human-readable device name (e.g. "iPhone 15 Pro").
	Name string `json:"name" bson:"name"`

	// OSVersion is the OS version string as reported by the device (e.g. "17.4").
	OSVersion string `json:"os_version" bson:"os_version"`

	// IPAddress is the device's network IP address, used for TV devices (Tizen/WebOS).
	IPAddress string `json:"ip_address" bson:"ip_address"`

	// Provider is the nickname of the provider host that manages this device.
	Provider string `json:"provider" bson:"provider"`

	// Usage controls which capabilities are enabled for the device:
	// "enabled" (automation + remote control), "automation" (Appium only),
	// "remote" (remote control only), or "disabled".
	Usage string `json:"usage" bson:"usage"`

	// ScreenWidth is the device screen width in pixels, stored as a string.
	ScreenWidth string `json:"screen_width" bson:"screen_width"`

	// ScreenHeight is the device screen height in pixels, stored as a string.
	ScreenHeight string `json:"screen_height" bson:"screen_height"`

	// DeviceType is either "real" or "emulator".
	DeviceType string `json:"device_type" bson:"device_type"`

	// WorkspaceID is the ID of the workspace this device belongs to.
	WorkspaceID string `json:"workspace_id" bson:"workspace_id"`

	// StreamType identifies the video streaming mode configured for this device.
	StreamType StreamingType `json:"stream_type" bson:"stream_type"`

	// --- Runtime state (not persisted, sent to hub via JSON) ---

	// Host is the provider's IP address or hostname, set at runtime.
	Host string `json:"host" bson:"-"`

	// HardwareModel is the hardware model identifier (e.g. "iPhone15,2"),
	// populated during device setup.
	HardwareModel string `json:"hardware_model" bson:"-"`

	// DeviceAddress is the network address used for Tizen Appium capabilities.
	// For Tizen it equals UDID (IP:PORT); unused for other platforms.
	DeviceAddress string `json:"device_address,omitempty" bson:"-"`

	// Connected is true when the device is currently detected as connected
	// to the provider host.
	Connected bool `json:"connected" bson:"-"`

	// IsResetting is true while the device is undergoing a setup reset.
	IsResetting bool `json:"is_resetting" bson:"-"`

	// ProviderState is the current lifecycle state of the device on the provider:
	// "init", "preparing", or "live".
	ProviderState string `json:"provider_state" bson:"-"`

	// LastUpdatedTimestamp is the Unix millisecond timestamp of the last state update.
	LastUpdatedTimestamp int64 `json:"last_updated_timestamp" bson:"-"`

	// CurrentRotation is the current screen orientation: "portrait" or "landscape".
	CurrentRotation string `json:"current_rotation" bson:"-"`

	// --- Stream settings (populated from DB at setup time) ---

	// StreamTargetFPS is the target frames-per-second for MJPEG streaming.
	StreamTargetFPS int `json:"stream_target_fps,omitempty" bson:"-"`

	// StreamJpegQuality is the JPEG compression quality (0-100) for MJPEG streaming.
	StreamJpegQuality int `json:"stream_jpeg_quality,omitempty" bson:"-"`

	// StreamScalingFactor is the scaling percentage (e.g. 50 = half resolution)
	// applied to the video stream.
	StreamScalingFactor int `json:"stream_scaling_factor,omitempty" bson:"-"`

	// --- Appium state ---

	// AppiumLastPingTS is the Unix millisecond timestamp of the last heartbeat
	// received from the GADS Appium plugin, indicating the Appium server is up.
	AppiumLastPingTS int64 `json:"appium_last_ts" bson:"-"`

	// AppiumSessionID is the currently active Appium session ID, or empty if none.
	AppiumSessionID string `json:"appium_session_id" bson:"-"`

	// IsAppiumUp is true when the Appium server process is running and healthy.
	IsAppiumUp bool `json:"is_appium_up" bson:"-"`

	// HasAppiumSession is true when an Appium test session is currently active.
	HasAppiumSession bool `json:"has_appium_session" bson:"-"`

	// --- App / UI metadata ---

	// InstalledApps is the list of app IDs (bundle IDs / package names) currently
	// installed on the device. Populated during device setup and on demand.
	InstalledApps []string `json:"installed_apps" bson:"-"`

	// SupportedStreamTypes lists the streaming modes available for this device's OS.
	SupportedStreamTypes []StreamType `json:"supported_stream_types" bson:"-"`
}

// ScreenWidthInt returns the screen width parsed as an integer.
// Returns 0 if the value is empty or cannot be parsed.
func (d *DeviceInfo) ScreenWidthInt() int {
	v, _ := strconv.Atoi(d.ScreenWidth)
	return v
}

// ScreenHeightInt returns the screen height parsed as an integer.
// Returns 0 if the value is empty or cannot be parsed.
func (d *DeviceInfo) ScreenHeightInt() int {
	v, _ := strconv.Atoi(d.ScreenHeight)
	return v
}

// CenterCoordinates returns the X and Y coordinates of the center of the screen.
// Returns 0, 0 if the screen dimensions are not set.
func (d *DeviceInfo) CenterCoordinates() (x, y float64) {
	return float64(d.ScreenWidthInt()) / 2, float64(d.ScreenHeightInt()) / 2
}
