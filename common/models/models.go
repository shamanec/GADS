package models

import (
	"context"

	"github.com/danielpaulus/go-ios/ios"
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

type IOSModelData struct {
	Width  string
	Height string
	Model  string
}

type User struct {
	Username string `json:"username" bson:"username"`
	Password string `json:"password" bson:"password"`
	Role     string `json:"role,omitempty" bson:"role"`
	ID       string `json:"_id" bson:"_id,omitempty"`
}

type Device struct {
	Connected            bool               `json:"connected" bson:"connected"`                                   // common value - if device is currently connected
	UDID                 string             `json:"udid" bson:"udid"`                                             // common value - device UDID
	OS                   string             `json:"os" bson:"os"`                                                 // common value - device OS
	Name                 string             `json:"name" bson:"name"`                                             // common value - name of the device
	OSVersion            string             `json:"os_version" bson:"os_version"`                                 // common value - OS version of the device
	Host                 string             `json:"host" bson:"host"`                                             // common value - IP address of the device host(provider)
	Provider             string             `json:"provider" bson:"provider"`                                     // common value - nickname of the device host(provider)
	ScreenWidth          string             `json:"screen_width" bson:"screen_width"`                             // common value - screen width of device
	ScreenHeight         string             `json:"screen_height" bson:"screen_height"`                           // common value - screen height of device
	HardwareModel        string             `json:"hardware_model,omitempty" bson:"hardware_model,omitempty"`     // common value - hardware model of device
	InstalledApps        []string           `json:"installed_apps" bson:"-"`                                      // provider value - list of installed apps on device
	IOSProductType       string             `json:"ios_product_type,omitempty" bson:"ios_product_type,omitempty"` // provider value - product type of iOS devices
	LastUpdatedTimestamp int64              `json:"last_updated_timestamp" bson:"last_updated_timestamp"`         // common value - last time the device data was updated
	WdaReadyChan         chan bool          `json:"-" bson:"-"`                                                   // provider value - channel for checking that WebDriverAgent is up after start
	Context              context.Context    `json:"-" bson:"-"`                                                   // provider value - context used to control the device set up since we have multiple goroutines
	CtxCancel            context.CancelFunc `json:"-" bson:"-"`                                                   // provider value - cancel func for the context above, can be used to stop all running device goroutines
	GoIOSDeviceEntry     ios.DeviceEntry    `json:"-" bson:"-"`                                                   // provider value - `go-ios` device entry object used for `go-ios` library interactions
	IsResetting          bool               `json:"is_resetting" bson:"is_resetting"`                             // common value - if device setup is currently being reset
	Logger               CustomLogger       `json:"-" bson:"-"`                                                   // provider value - CustomLogger object for the device
	AppiumSessionID      string             `json:"appiumSessionID" bson:"-"`                                     // provider value - current Appium session ID
	WDASessionID         string             `json:"wdaSessionID" bson:"-"`                                        // provider value - current WebDriverAgent session ID
	AppiumPort           string             `json:"appium_port" bson:"-"`                                         // provider value - port assigned to the device for the Appium server
	StreamPort           string             `json:"stream_port" bson:"-"`                                         // provider value - port assigned to the device for the video stream
	WDAStreamPort        string             `json:"wda_stream_port" bson:"-"`                                     // provider value - port assigned to iOS devices for the WebDriverAgent stream
	WDAPort              string             `json:"wda_port" bson:"-"`                                            // provider value - port assigned to iOS devices for the WebDriverAgent instance
	AppiumLogger         AppiumLogger       `json:"-" bson:"-"`                                                   // provider value - AppiumLogger object for logging appium actions
	Available            bool               `json:"available" bson:"-"`                                           // provider value - if device is currently available - not only connected, but setup completed
	ProviderState        string             `json:"provider_state" bson:"provider_state"`                         // common value - current state of the device on the provider - init, preparing, live
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
}
