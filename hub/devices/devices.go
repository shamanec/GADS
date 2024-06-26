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

// Get the latest devices information from MongoDB each second
func GetLatestDBDevices() {
	var latestDBDevices []models.Device

	for {
		latestDBDevices = db.GetDevices()
		for _, dbDevice := range latestDBDevices {
			hubDevice, ok := HubDevicesMap[dbDevice.UDID]
			if ok {
				hubDevice.Device = dbDevice
				if hubDevice.Device.Connected && hubDevice.Device.LastUpdatedTimestamp >= (time.Now().UnixMilli()-3000) {
					hubDevice.Device.Available = true

					if hubDevice.InUseTS >= (time.Now().UnixMilli() - 3000) {
						hubDevice.InUse = true
					} else {
						hubDevice.InUse = false
					}
				} else {
					hubDevice.InUse = false
					hubDevice.Device.Available = false
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
