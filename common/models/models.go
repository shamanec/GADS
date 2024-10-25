package models

import (
	"context"
	"sync"

	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/tunnel"
)

type CustomLogger interface {
	LogDebug(eventName string, message string)
	LogInfo(eventName string, message string)
	LogError(eventName string, message string)
	LogWarn(eventName string, message string)
	LogFatal(eventName string, message string)
	LogPanic(eventName string, message string)
}

type AppiumLogger interface {
	Log(device *Device, logLine string)
}

type ByUDID []Device

func (a ByUDID) Len() int           { return len(a) }
func (a ByUDID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByUDID) Less(i, j int) bool { return a[i].UDID < a[j].UDID }

type User struct {
	Username string `json:"username" bson:"username"`
	Password string `json:"password" bson:"password"`
	Role     string `json:"role,omitempty" bson:"role"`
	ID       string `json:"_id" bson:"_id,omitempty"`
}

type Device struct {
	// DB DATA
	UDID         string `json:"udid" bson:"udid"`                   // device UDID
	OS           string `json:"os" bson:"os"`                       // device OS
	Name         string `json:"name" bson:"name"`                   // name of the device
	OSVersion    string `json:"os_version" bson:"os_version"`       // OS version of the device
	Provider     string `json:"provider" bson:"provider"`           // nickname of the device host(provider)
	Usage        string `json:"usage" bson:"usage"`                 // what is the device used for: enabled(automation and remote control), automation(only Appium testing), remote(only remote control), disabled
	ScreenWidth  string `json:"screen_width" bson:"screen_width"`   // screen width of device
	ScreenHeight string `json:"screen_height" bson:"screen_height"` // screen height of device
	DeviceType   string `json:"device_type" bson:"device_type"`     // The type of device - `real` or `emulator`
	// NON-DB DATA
	/// COMMON VALUES
	Host                 string `json:"host" bson:"-"`                            // IP address of the device host(provider)
	HardwareModel        string `json:"hardware_model" bson:"-"`                  // hardware model of device
	LastUpdatedTimestamp int64  `json:"last_updated_timestamp" bson:"-"`          // last time the device data was updated
	Connected            bool   `json:"connected" bson:"-"`                       // if device is currently connected
	IsResetting          bool   `json:"is_resetting" bson:"-"`                    // if device setup is currently being reset
	ProviderState        string `json:"provider_state" bson:"-"`                  // current state of the device on the provider - init, preparing, live
	StreamTargetFPS      int    `json:"stream_target_fps,omitempty" bson:"-"`     // The target FPS for the MJPEG video streams
	StreamJpegQuality    int    `json:"stream_jpeg_quality,omitempty" bson:"-"`   // The target JPEG quality for the MJPEG video streams
	StreamScalingFactor  int    `json:"stream_scaling_factor,omitempty" bson:"-"` // The target scaling factor for the MJPEG video streams
	/// PROVIDER ONLY VALUES
	//// RETURNABLE VALUES
	InstalledApps []string `json:"installed_apps" bson:"-"`  // list of installed apps on device
	UsesCustomWDA bool     `json:"uses_custom_wda" bson:"-"` // Flag for iOS device if provider sets up custom WDA
	///// NON-RETURNABLE VALUES
	AppiumSessionID  string             `json:"-" bson:"-"` // current Appium session ID
	WDASessionID     string             `json:"-" bson:"-"` // current WebDriverAgent session ID
	AppiumPort       string             `json:"-" bson:"-"` // port assigned to the device for the Appium server
	StreamPort       string             `json:"-" bson:"-"` // port assigned to the device for the video stream
	WDAStreamPort    string             `json:"-" bson:"-"` // port assigned to iOS devices for the WebDriverAgent stream
	WDAPort          string             `json:"-" bson:"-"` // port assigned to iOS devices for the WebDriverAgent instance
	WdaReadyChan     chan bool          `json:"-" bson:"-"` // channel for checking that WebDriverAgent is up after start
	AppiumReadyChan  chan bool          `json:"-" bson:"-"` // channel for checking that Appium is up after start
	Context          context.Context    `json:"-" bson:"-"` // context used to control the device set up since we have multiple goroutines
	CtxCancel        context.CancelFunc `json:"-" bson:"-"` // cancel func for the context above, can be used to stop all running device goroutines
	GoIOSDeviceEntry ios.DeviceEntry    `json:"-" bson:"-"` // `go-ios` device entry object used for `go-ios` library interactions
	Logger           CustomLogger       `json:"-" bson:"-"` // CustomLogger object for the device
	AppiumLogger     AppiumLogger       `json:"-" bson:"-"` // AppiumLogger object for logging appium actions
	Mutex            sync.Mutex         `json:"-" bson:"-"` // Mutex to lock resources - especially on device reset
	GoIOSTunnel      tunnel.Tunnel      `json:"-" bson:"-"` // Tunnel obj for go-ios handling of iOS 17.4+
	SemVer           *semver.Version    `json:"-" bson:"-"` // Semantic version of device for checks around the provider
	InitialSetupDone bool               `json:"-" bson:"-"` // On provider startup some data is prepared for devices like logger, Mongo collection, etc. This is true if all is done
}

type LocalHubDevice struct {
	Device                   Device `json:"info"`
	SessionID                string `json:"-"`
	IsRunningAutomation      bool   `json:"is_running_automation"`
	LastAutomationActionTS   int64  `json:"last_automation_action_ts"`
	InUse                    bool   `json:"in_use"`
	InUseBy                  string `json:"in_use_by"`
	InUseTS                  int64  `json:"in_use_ts"`
	AppiumNewCommandTimeout  int64  `json:"appium_new_command_timeout"`
	IsAvailableForAutomation bool   `json:"is_available_for_automation"`
	Available                bool   `json:"available" bson:"-"` // if device is currently available - not only connected, but setup completed
}

type IOSModelData struct {
	Width  string
	Height string
	Model  string
}

type UpdateStreamSettings struct {
	TargetFPS     int `json:"target_fps,omitempty"`
	JpegQuality   int `json:"jpeg_quality,omitempty"`
	ScalingFactor int `json:"scaling_factor,omitempty"`
}
