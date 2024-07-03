package devices

import (
	"GADS/common/db"
	"GADS/common/models"
	"fmt"
	"strconv"
	"sync"
	"time"
)

var ConfigData *models.HubConfig

func CalculateCanvasDimensions(device *models.Device) (canvasWidth string, canvasHeight string) {
	// Get the width and height provided
	widthString := device.ScreenWidth
	heightString := device.ScreenHeight

	// Convert them to ints
	width, _ := strconv.Atoi(widthString)
	height, _ := strconv.Atoi(heightString)

	screen_ratio := float64(width) / float64(height)

	canvasHeight = "850"
	canvasWidth = fmt.Sprintf("%f", 850*screen_ratio)

	return
}

var HubDevicesMap = make(map[string]*models.LocalHubDevice)
var mapMu sync.Mutex

// Get the latest devices information from MongoDB each second
func GetLatestDBDevices() {
	var latestDBDevices []models.Device

	for {
		latestDBDevices = db.GetDevices()

		mapMu.Lock()
		for udid, _ := range HubDevicesMap {
			found := false
			for _, dbDevice := range latestDBDevices {
				if dbDevice.UDID == udid {
					found = true
					break
				}
			}
			if !found {
				delete(HubDevicesMap, udid)
			}
		}

		for _, dbDevice := range latestDBDevices {
			hubDevice, ok := HubDevicesMap[dbDevice.UDID]
			if ok {
				// Update data only if needed
				if hubDevice.Device.OSVersion != dbDevice.OSVersion {
					hubDevice.Device.OSVersion = dbDevice.OSVersion
				}
				if hubDevice.Device.Name != dbDevice.Name {
					hubDevice.Device.Name = dbDevice.Name
				}
				if hubDevice.Device.ScreenWidth != dbDevice.ScreenWidth {
					hubDevice.Device.ScreenWidth = dbDevice.ScreenWidth
				}
				if hubDevice.Device.ScreenHeight != dbDevice.ScreenHeight {
					hubDevice.Device.ScreenHeight = dbDevice.ScreenHeight
				}
				if hubDevice.Device.Usage != dbDevice.Usage {
					hubDevice.Device.Usage = dbDevice.Usage
				}
				if hubDevice.Device.Provider != dbDevice.Provider {
					hubDevice.Device.Provider = dbDevice.Provider
				}
			} else {
				HubDevicesMap[dbDevice.UDID] = &models.LocalHubDevice{
					Device:                   dbDevice,
					IsRunningAutomation:      false,
					IsAvailableForAutomation: true,
					LastAutomationActionTS:   0,
				}
			}
		}
		mapMu.Unlock()
		time.Sleep(1 * time.Second)
	}
}

var getDeviceMu sync.Mutex

func GetHubDeviceByUDID(udid string) *models.LocalHubDevice {
	getDeviceMu.Lock()
	defer getDeviceMu.Unlock()
	for _, hubDevice := range HubDevicesMap {
		if hubDevice.Device.UDID == udid {
			return hubDevice
		}
	}

	return nil
}
