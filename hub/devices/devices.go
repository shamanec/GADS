/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package devices

import (
	"GADS/common/db"
	"GADS/common/models"
	"fmt"
	"strconv"
	"sync"
	"time"
)

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

type HubDevices struct {
	Mu      sync.Mutex
	Devices map[string]*models.LocalHubDevice
}

var HubDevicesData HubDevices

func InitHubDevicesData() {
	HubDevicesData = HubDevices{
		Devices: make(map[string]*models.LocalHubDevice),
	}
}

// Get the latest devices information from MongoDB each second
func GetLatestDBDevices() {
	var latestDBDevices []models.Device

	for {
		latestDBDevices, _ = db.GlobalMongoStore.GetDevices()

		HubDevicesData.Mu.Lock()
		for udid, _ := range HubDevicesData.Devices {
			found := false
			for _, dbDevice := range latestDBDevices {
				if dbDevice.UDID == udid {
					found = true
					break
				}
			}
			if !found {
				delete(HubDevicesData.Devices, udid)
			}
		}
		HubDevicesData.Mu.Unlock()

		for _, dbDevice := range latestDBDevices {
			HubDevicesData.Mu.Lock()
			hubDevice, ok := HubDevicesData.Devices[dbDevice.UDID]
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
				if hubDevice.Device.WorkspaceID != dbDevice.WorkspaceID {
					hubDevice.Device.WorkspaceID = dbDevice.WorkspaceID
				}
			} else {
				HubDevicesData.Devices[dbDevice.UDID] = &models.LocalHubDevice{
					Device:                   dbDevice,
					IsRunningAutomation:      false,
					IsAvailableForAutomation: true,
					LastAutomationActionTS:   0,
				}
			}
			HubDevicesData.Mu.Unlock()
		}
		time.Sleep(1 * time.Second)
	}
}

var getDeviceMu sync.RWMutex

func GetHubDeviceByUDID(udid string) *models.LocalHubDevice {
	getDeviceMu.Lock()
	defer getDeviceMu.Unlock()
	for _, hubDevice := range HubDevicesData.Devices {
		if hubDevice.Device.UDID == udid {
			return hubDevice
		}
	}

	return nil
}
