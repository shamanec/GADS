/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package models

type ActionData struct {
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	EndX       float64 `json:"endX,omitempty"`
	EndY       float64 `json:"endY,omitempty"`
	TextToType string  `json:"text,omitempty"`
	Duration   float64 `json:"duration,omitempty"`
}

type DeviceAction struct {
	Type     string  `json:"type"`
	Duration int     `json:"duration"`
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
	Button   int     `json:"button"`
	Origin   string  `json:"origin,omitempty"`
}

type DeviceActionParameters struct {
	PointerType string `json:"pointerType"`
}

type DevicePointerAction struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Parameters DeviceActionParameters `json:"parameters"`
	Actions    []DeviceAction         `json:"actions"`
}

type DevicePointerActions struct {
	Actions []DevicePointerAction `json:"actions"`
}

type ActiveElementData struct {
	Value struct {
		Element string `json:"ELEMENT"`
	} `json:"value"`
}

type AppiumTypeText struct {
	Text string `json:"text"`
}

type AndroidKeycodePayload struct {
	Keycode int `json:"keycode"`
}

type AppiumServerCapabilities struct {
	UDID                   string `json:"appium:udid"`
	WdaMjpegPort           string `json:"appium:mjpegServerPort,omitempty"`
	ClearSystemFiles       string `json:"appium:clearSystemFiles,omitempty"`
	WdaURL                 string `json:"appium:webDriverAgentUrl,omitempty"`
	PreventWdaAttachments  string `json:"appium:preventWDAAttachments,omitempty"`
	SimpleIsVisibleCheck   string `json:"appium:simpleIsVisibleCheck,omitempty"`
	WdaLocalPort           string `json:"appium:wdaLocalPort,omitempty"`
	PlatformVersion        string `json:"appium:platformVersion,omitempty"`
	AutomationName         string `json:"appium:automationName"`
	PlatformName           string `json:"platformName"`
	DeviceName             string `json:"appium:deviceName"`
	WdaLaunchTimeout       string `json:"appium:wdaLaunchTimeout,omitempty"`
	WdaConnectionTimeout   string `json:"appium:wdaConnectionTimeout,omitempty"`
	DeviceAddress          string `json:"appium:deviceAddress,omitempty"`
	DeviceHost             string `json:"appium:deviceHost,omitempty"`
	ChromeDriverExecutable string `json:"appium:chromedriverExecutable,omitempty"`
}

type AppiumTomlNode struct {
	DetectDrivers bool `toml:"detect-drivers"`
}

type AppiumTomlServer struct {
	Port int `toml:"port"`
}

type AppiumTomlRelay struct {
	URL            string   `toml:"url"`
	StatusEndpoint string   `toml:"status-endpoint"`
	Configs        []string `toml:"configs"`
}

type AppiumTomlConfig struct {
	Server AppiumTomlServer `toml:"server"`
	Node   AppiumTomlNode   `toml:"node"`
	Relay  AppiumTomlRelay  `toml:"relay"`
}

type WDAMjpegSettings struct {
	Settings WDAMjpegProperties `json:"settings"`
}

type WDAMjpegProperties struct {
	MjpegServerFramerate         int `json:"mjpegServerFramerate,omitempty"`
	MjpegServerScreenshotQuality int `json:"mjpegServerScreenshotQuality,omitempty"`
	MjpegServerScalingFactor     int `json:"mjpegScalingFactor,omitempty"`
}

type CommonCapabilities struct {
	AutomationName    string `json:"appium:automationName"`
	BundleID          string `json:"appium:bundleId"`
	PlatformVersion   string `json:"appium:platformVersion"`
	PlatformName      string `json:"platformName,omitempty"`
	DeviceUDID        string `json:"appium:udid"`
	NewCommandTimeout int64  `json:"appium:newCommandTimeout"`
	SessionTimeout    int64  `json:"appium:sessionTimeout"`
	GadsTenant        string `json:"gads:tenant,omitempty"`   // Tenant name for test report filtering
	GadsBuildId       string `json:"gads:buildId,omitempty"`  // Custom build identifier for test reports
	GadsTestName      string `json:"gads:testName,omitempty"` // Custom test name for test reports
}

type Capabilities struct {
	FirstMatch  []CommonCapabilities `json:"firstMatch"`
	AlwaysMatch CommonCapabilities   `json:"alwaysMatch"`
}

type AppiumSession struct {
	Capabilities        Capabilities       `json:"capabilities"`
	DesiredCapabilities CommonCapabilities `json:"desiredCapabilities"`
}

// ExtractClientSecretFromSession extracts client secret from Appium session request
func ExtractClientSecretFromSession(sessionReq map[string]interface{}, prefix string) string {
	// Check capabilities.alwaysMatch (W3C format)
	if caps, ok := sessionReq["capabilities"].(map[string]interface{}); ok {
		if alwaysMatch, ok := caps["alwaysMatch"].(map[string]interface{}); ok {
			if secret, ok := alwaysMatch[prefix+":clientSecret"].(string); ok {
				return secret
			}
		}
	}

	// Also check desiredCapabilities for backward compatibility
	if desired, ok := sessionReq["desiredCapabilities"].(map[string]interface{}); ok {
		if secret, ok := desired[prefix+":clientSecret"].(string); ok {
			return secret
		}
	}

	return ""
}

// ----- APPIUM PLUGIN------//
type AppiumPluginLog struct {
	Level          string `json:"level,omitempty" bson:"level"`           // The Appium log level - debug, info, etc
	Message        string `json:"message,omitempty" bson:"message"`       //  The actual Appium log message
	SessionID      string `json:"session_id,omitempty" bson:"session_id"` // The ID of the current active session (if any)
	Prefix         string `json:"prefix,omitempty" bson:"prefix"`         // The Appium log prefix - `AndroidUiautomator2Driver@60a2` or `Logcat` etc
	Timestamp      int64  `json:"timestamp" bson:"timestamp"`             // The timestamp in milliseconds when the log was sent via the plugin
	SequenceNumber int64  `json:"sequenceNumber" bson:"sequenceNumber"`   // Sequence number of the log - because plugin might send multiple logs at the same millisecond we can use this to sort logs in their actual order
}

type AppiumPluginSessionLog struct {
	Timestamp           int64    `json:"timestamp" bson:"timestamp"`
	SessionID           string   `json:"session_id" bson:"session_id"`
	DeviceUDID          string   `json:"udid" bson:"udid"`
	Action              string   `json:"action" bson:"action"`
	Command             string   `json:"command" bson:"command"`
	Source              string   `json:"source" bson:"source"`
	Duration            int      `json:"duration_ms" bson:"duration_ms"`
	Success             bool     `json:"success" bson:"success"`
	Error               string   `json:"error,omitempty" bson:"error,omitempty"`
	Args                []string `json:"args" bson:"args"`
	SequenceNumber      int64    `json:"sequence_number" bson:"sequence_number"`
	Tenant              string   `json:"tenant" bson:"tenant"`
	BuildID             string   `json:"build_id" bson:"build_id"`
	TestName            string   `json:"test_name" bson:"test_name"`
	LocatorUsing        string   `json:"locator_using" bson:"locator_using"`
	LocatorValue        string   `json:"locator_value" bson:"locator_value"`
	DeviceName          string   `json:"device_name" bson:"device_name"`
	AndroidAppPackage   string   `json:"app_package" bson:"app_package"`
	IOSBundleIdentifier string   `json:"bundle_identifier" bson:"bundle_identifier"`
	PlatformName        string   `json:"platform_name" bson:"platform_name"`
}

// SessionLogsSummary is the compact record we will show in tests report table
type SessionLogsSummary struct {
	SessionID    string `bson:"session_id" json:"session_id"`                       // The unique session id
	PlatformName string `bson:"platform_name" json:"platform_name"`                 // Android, iOS, Tizen etc
	UDID         string `bson:"udid" json:"udid"`                                   // The device UDID
	BuildID      string `bson:"build_id" json:"build_id"`                           // Build identifier provided via capabilities
	DeviceName   string `bson:"device_name,omitempty" json:"device_name,omitempty"` // GADS device name
	Count        int64  `bson:"count" json:"count"`                                 // Number of logs for the current session
}

type BuildReport struct {
	BuildID      string   `json:"build_id" bson:"build_id"`
	SessionCount int      `json:"session_count" bson:"session_count"`
	SessionIDs   []string `json:"session_ids" bson:"session_ids"`
	TestNames    []string `json:"test_names" bson:"test_names"`
	DeviceNames  []string `json:"device_names" bson:"device_names"`
	FirstAction  int64    `json:"first_action" bson:"first_action"`
	LastAction   int64    `json:"last_action" bson:"last_action"`
}

type SessionReport struct {
	SessionID     string `json:"session_id" bson:"session_id"`
	TestName      string `json:"test_name" bson:"test_name"`
	DeviceName    string `json:"device_name" bson:"device_name"`
	DeviceUDID    string `json:"device_udid" bson:"device_udid"`
	PlatformName  string `json:"platform_name" bson:"platform_name"`
	LogCount      int    `json:"log_count" bson:"log_count"`
	FailedActions int    `json:"failed_actions" bson:"failed_actions"`
	FirstAction   int64  `json:"first_action" bson:"first_action"`
	LastAction    int64  `json:"last_action" bson:"last_action"`
}
