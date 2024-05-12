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

type Device struct {
	Connected            bool               `json:"connected" bson:"connected"`
	UDID                 string             `json:"udid" bson:"udid"`
	OS                   string             `json:"os" bson:"os"`
	Name                 string             `json:"name" bson:"name"`
	OSVersion            string             `json:"os_version" bson:"os_version"`
	Model                string             `json:"model" bson:"model"`
	Host                 string             `json:"host" bson:"host"`
	Provider             string             `json:"provider" bson:"provider"`
	ScreenWidth          string             `json:"screen_width" bson:"screen_width"`
	ScreenHeight         string             `json:"screen_height" bson:"screen_height"`
	HardwareModel        string             `json:"hardware_model,omitempty" bson:"hardware_model,omitempty"`
	InstalledApps        []string           `json:"installed_apps" bson:"-"`
	IOSProductType       string             `json:"ios_product_type,omitempty" bson:"ios_product_type,omitempty"`
	LastUpdatedTimestamp int64              `json:"last_updated_timestamp" bson:"last_updated_timestamp"`
	ProviderState        string             `json:"provider_state" bson:"provider_state"`
	WdaReadyChan         chan bool          `json:"-" bson:"-"`
	Context              context.Context    `json:"-" bson:"-"`
	CtxCancel            context.CancelFunc `json:"-" bson:"-"`
	GoIOSDeviceEntry     ios.DeviceEntry    `json:"-" bson:"-"`
	IsResetting          bool               `json:"is_resetting" bson:"is_resetting"`
	Logger               CustomLogger       `json:"-" bson:"-"`
	InstallableApps      []string           `json:"installable_apps" bson:"-"`
	AppiumSessionID      string             `json:"appiumSessionID" bson:"-"`
	WDASessionID         string             `json:"wdaSessionID" bson:"-"`
	AppiumPort           string             `json:"appium_port" bson:"-"`
	StreamPort           string             `json:"stream_port" bson:"-"`
	WDAStreamPort        string             `json:"wda_stream_port" bson:"-"`
	WDAPort              string             `json:"wda_port" bson:"-"`
	AppiumLogger         AppiumLogger       `json:"-" bson:"-"`
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

type ConnectedDevice struct {
	OS   string
	UDID string
}
