/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

// Package models provides the core types, interfaces, and dependency abstractions
// for the GADS device management system. It is the foundation of the refactored
// device package hierarchy.
package models

// StreamingType is the custom type used to identify a video streaming mode in
// DB persistence and wire-format JSON. Values are defined as constants below.
type StreamingType string

const (
	// MJPEGStreamTypeID identifies the MJPEG streaming mode, supported by all
	// mobile platforms (iOS and Android).
	MJPEGStreamTypeID StreamingType = "mjpeg"

	// IOSWebRTCFFMpegStreamTypeID identifies the iOS WebRTC streaming mode that
	// transcodes WDA MJPEG output through FFmpeg to produce an H264 RTP stream.
	IOSWebRTCFFMpegStreamTypeID StreamingType = "ios_webrtc_ffmpeg"

	// AndroidWebRTCGetStreamStreamTypeID identifies the Android WebRTC streaming
	// mode that uses the GADS-Settings GetStream API.
	AndroidWebRTCGetStreamStreamTypeID StreamingType = "android_webrtc_getstream"

	// AndroidWebRTCGadsH264StreamTypeID identifies the Android WebRTC streaming
	// mode that receives H264 directly from the GADS-Settings H264Server.
	AndroidWebRTCGadsH264StreamTypeID StreamingType = "android_webrtc_gads_h264"

	// IOSWebRTCBroadcastExtensionID identifies the iOS WebRTC streaming mode
	// that uses a broadcast extension (ReplayKit) for screen capture.
	IOSWebRTCBroadcastExtensionID StreamingType = "ios_webrtc_broadcast"
)

// IsWebRTCStreamType reports whether st is any WebRTC-based streaming mode.
// This is used to decide whether TURN/ICE configuration must be sent to the device.
func IsWebRTCStreamType(st StreamingType) bool {
	return st == AndroidWebRTCGetStreamStreamTypeID ||
		st == AndroidWebRTCGadsH264StreamTypeID ||
		st == IOSWebRTCFFMpegStreamTypeID ||
		st == IOSWebRTCBroadcastExtensionID
}

// Description returns a human-readable label for the streaming type, suitable
// for display in the UI or logs.
func (st StreamingType) Description() string {
	switch st {
	case MJPEGStreamTypeID:
		return "MJPEG"
	case IOSWebRTCFFMpegStreamTypeID:
		return "WebRTC - FFMpeg"
	case AndroidWebRTCGetStreamStreamTypeID:
		return "Android WebRTC GetStream"
	case AndroidWebRTCGadsH264StreamTypeID:
		return "Android WebRTC GADS H264"
	case IOSWebRTCBroadcastExtensionID:
		return "WebRTC - Broadcast Extension"
	default:
		return "Unknown"
	}
}

// StreamType describes a single streaming option that can be presented to the
// user or communicated over the API.
type StreamType struct {
	// Name is the human-readable label (e.g. "MJPEG", "WebRTC - FFMpeg").
	Name string `json:"name" bson:"-"`
	// ID is the machine-readable streaming type identifier stored in the DB.
	ID StreamingType `json:"id" bson:"-"`
	// DeviceOS is the platform that supports this streaming mode: "ios",
	// "android", or "both".
	DeviceOS string `json:"device_os" bson:"-"`
}

// Pre-built StreamType descriptors for each supported streaming mode.
var (
	MJPEGStreamType = StreamType{
		Name:     MJPEGStreamTypeID.Description(),
		ID:       MJPEGStreamTypeID,
		DeviceOS: "both",
	}

	IOSWebRTCFFMpegStreamType = StreamType{
		Name:     IOSWebRTCFFMpegStreamTypeID.Description(),
		ID:       IOSWebRTCFFMpegStreamTypeID,
		DeviceOS: "ios",
	}

	AndroidWebRTCGetStreamStreamType = StreamType{
		Name:     AndroidWebRTCGetStreamStreamTypeID.Description(),
		ID:       AndroidWebRTCGetStreamStreamTypeID,
		DeviceOS: "android",
	}

	AndroidWebRTCGadsH264StreamType = StreamType{
		Name:     AndroidWebRTCGadsH264StreamTypeID.Description(),
		ID:       AndroidWebRTCGadsH264StreamTypeID,
		DeviceOS: "android",
	}

	IOSWebRTCBroadcastExtensionStreamType = StreamType{
		Name:     IOSWebRTCBroadcastExtensionID.Description(),
		ID:       IOSWebRTCBroadcastExtensionID,
		DeviceOS: "ios",
	}

	// IOSStreamTypes lists all streaming modes available for iOS devices.
	IOSStreamTypes = []StreamType{
		MJPEGStreamType,
		IOSWebRTCFFMpegStreamType,
		IOSWebRTCBroadcastExtensionStreamType,
	}

	// AndroidStreamTypes lists all streaming modes available for Android devices.
	AndroidStreamTypes = []StreamType{
		MJPEGStreamType,
		AndroidWebRTCGetStreamStreamType,
		AndroidWebRTCGadsH264StreamType,
	}
)

// StreamTypesForOS returns the list of supported StreamType values for the
// given OS identifier ("ios" or "android"). An empty slice is returned for
// platforms that do not support streaming (e.g. Tizen, WebOS).
func StreamTypesForOS(os string) []StreamType {
	switch os {
	case "ios":
		return IOSStreamTypes
	case "android":
		return AndroidStreamTypes
	default:
		return []StreamType{}
	}
}
