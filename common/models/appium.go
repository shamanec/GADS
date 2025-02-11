package models

type ActionData struct {
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	EndX       float64 `json:"endX,omitempty"`
	EndY       float64 `json:"endY,omitempty"`
	TextToType string  `json:"text,omitempty"`
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

type AppiumLog struct {
	SystemTS  int64  `json:"ts" bson:"ts"`
	Message   string `json:"msg" bson:"msg"`
	AppiumTS  string `json:"appium_ts" bson:"appium_ts"`
	Type      string `json:"log_type" bson:"log_type"`
	SessionID string `json:"session_id" bson:"session_id"`
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
	RCToken                string `json:"appium:rcToken,omitempty"`
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
