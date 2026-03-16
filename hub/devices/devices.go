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
	"GADS/device"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

// LocalHubDevice wraps a DeviceInfo with hub-side state (in-use tracking,
// automation session, WebSocket connection). It is never persisted to MongoDB;
// the hub builds it from the DeviceInfo received from providers.
type LocalHubDevice struct {
	Device                   device.DeviceInfo `json:"info"`
	SessionID                string            `json:"-"`
	IsRunningAutomation      bool              `json:"is_running_automation"`
	LastAutomationActionTS   int64             `json:"last_automation_action_ts"`
	InUse                    bool              `json:"in_use"`
	InUseBy                  string            `json:"in_use_by"`
	InUseByTenant            string            `json:"in_use_by_tenant"`
	InUseTS                  int64             `json:"in_use_ts"`
	AppiumNewCommandTimeout  int64             `json:"appium_new_command_timeout"`
	IsAvailableForAutomation bool              `json:"is_available_for_automation"`
	Available                bool              `json:"available" bson:"-"`
	InUseWSConnection        net.Conn          `json:"-" bson:"-"`
	LastActionTS             int64             `json:"-" bson:"-"`
}

// CalculateCanvasDimensions computes the canvas width and height for a device
// based on its screen dimensions, scaling to a fixed 850-pixel height.
func CalculateCanvasDimensions(dev *device.DeviceInfo) (canvasWidth string, canvasHeight string) {
	width, _ := strconv.Atoi(dev.ScreenWidth)
	height, _ := strconv.Atoi(dev.ScreenHeight)

	screenRatio := float64(width) / float64(height)

	canvasHeight = "850"
	canvasWidth = fmt.Sprintf("%f", 850*screenRatio)

	return
}

// HubDevices is the in-memory registry of all devices known to the hub,
// protected by a mutex for concurrent access.
type HubDevices struct {
	Mu      sync.Mutex
	Devices map[string]*LocalHubDevice
}

// HubDevicesData is the global device registry used by the hub.
var HubDevicesData HubDevices

// InitHubDevicesData initialises the global device registry.
func InitHubDevicesData() {
	HubDevicesData = HubDevices{
		Devices: make(map[string]*LocalHubDevice),
	}
}

// GetLatestDBDevices continuously polls MongoDB every second and reconciles
// the in-memory HubDevices map with the latest device records from the DB.
// Devices removed from the DB are pruned; new devices are added; existing
// devices have their persisted fields (Name, OSVersion, etc.) refreshed.
func GetLatestDBDevices() {
	var latestDBDevices []device.DeviceInfo

	for {
		latestDBDevices, _ = device.GetDevices()

		HubDevicesData.Mu.Lock()
		for udid := range HubDevicesData.Devices {
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
				dbDevice.InstalledApps = make([]string, 0)
				dbDevice.SupportedStreamTypes = device.StreamTypesForOS(dbDevice.OS)
				HubDevicesData.Devices[dbDevice.UDID] = &LocalHubDevice{
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

// GetHubDeviceByUDID returns the LocalHubDevice for the given UDID, or nil if not found.
func GetHubDeviceByUDID(udid string) *LocalHubDevice {
	getDeviceMu.Lock()
	defer getDeviceMu.Unlock()
	for _, hubDevice := range HubDevicesData.Devices {
		if hubDevice.Device.UDID == udid {
			return hubDevice
		}
	}
	return nil
}
