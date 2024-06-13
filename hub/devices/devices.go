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

var LatestDevices []*models.Device

// Get the latest devices information from MongoDB each second
func GetLatestDBDevices() {
	LatestDevices = []*models.Device{}

	for {
		LatestDevices = db.GetDevices()
		time.Sleep(1 * time.Second)
	}
}

var getDeviceMu sync.Mutex

func GetDeviceByUDID(udid string) *models.Device {
	getDeviceMu.Lock()
	for _, device := range LatestDevices {
		if device.UDID == udid {
			return device
		}
	}
	getDeviceMu.Unlock()

	return nil
}
