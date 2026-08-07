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
	"time"
)

func CalculateCanvasDimensions(device *models.DBDevice) (canvasWidth string, canvasHeight string) {
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

func InitHubDevicesData() {
	HubDeviceStore = NewDeviceStore()
}

// ephemeralDeviceExpiryMs is how long an ephemeral device stays in the store after
// its provider stopped reporting it. Long enough to absorb a briefly hiccuping
// provider update cycle, short enough that a killed emulator disappears promptly.
const ephemeralDeviceExpiryMs = 15_000

// RegisterEphemeralDevice creates a store entry for a device that exists only in
// provider memory (no DB record), built entirely from the provider update payload.
// Mirrors the store entry creation for DB devices in GetLatestDBDevices.
func RegisterEphemeralDevice(providerDevice *models.ProviderDeviceSync, providerRunsAppium bool) *LocalHubDevice {
	hubDevice := &LocalHubDevice{
		Device:                   *providerDevice.EphemeralDevice,
		IsRunningAutomation:      false,
		IsAvailableForAutomation: true,
		LastAutomationActionTS:   0,
		AppiumEnabled:            providerRunsAppium,
		Ephemeral:                true,
		LastEphemeralReportTS:    time.Now().UnixMilli(),
	}
	HubDeviceStore.Set(hubDevice.Device.UDID, hubDevice)
	return hubDevice
}

// RefreshEphemeralDevice updates an ephemeral device's descriptor from the latest
// provider update and stamps its report timestamp so it does not expire.
func RefreshEphemeralDevice(hubDevice *LocalHubDevice, providerDevice *models.ProviderDeviceSync, providerRunsAppium bool) {
	hubDevice.Mu.Lock()
	defer hubDevice.Mu.Unlock()
	if providerDevice.EphemeralDevice != nil {
		hubDevice.Device = *providerDevice.EphemeralDevice
	}
	hubDevice.AppiumEnabled = providerRunsAppium
	hubDevice.LastEphemeralReportTS = time.Now().UnixMilli()
}

// Get the latest devices information from MongoDB each second
func GetLatestDBDevices() {
	var latestDBDevices []models.DBDevice

	for {
		latestDBDevices, _ = db.GlobalMongoStore.GetDevices()

		// Whether each provider runs Appium servers, by nickname. When the lookup
		// fails the previous per-device values are kept rather than flipped
		appiumByProvider := map[string]bool{}
		providersKnown := false
		if dbProviders, err := db.GlobalMongoStore.GetAllProviders(); err == nil {
			providersKnown = true
			for _, dbProvider := range dbProviders {
				appiumByProvider[dbProvider.Nickname] = dbProvider.SetupAppiumServers
			}
		}

		// Remove devices from the store that are no longer in the DB. Ephemeral
		// devices have no DB record - they expire instead when their provider
		// stopped reporting them (killed emulator, dead provider)
		var toDelete []string
		for _, hubDevice := range HubDeviceStore.All() {
			if hubDevice.Ephemeral {
				hubDevice.Mu.RLock()
				lastReportTS := hubDevice.LastEphemeralReportTS
				hubDevice.Mu.RUnlock()
				if time.Now().UnixMilli()-lastReportTS > ephemeralDeviceExpiryMs {
					toDelete = append(toDelete, hubDevice.Device.UDID)
				}
				continue
			}
			found := false
			for _, dbDevice := range latestDBDevices {
				if dbDevice.UDID == hubDevice.Device.UDID {
					found = true
					break
				}
			}
			if !found {
				toDelete = append(toDelete, hubDevice.Device.UDID)
			}
		}
		for _, udid := range toDelete {
			HubDeviceStore.Delete(udid)
		}

		for i := range latestDBDevices {
			dbDevice := &latestDBDevices[i]
			hubDevice, ok := HubDeviceStore.Get(dbDevice.UDID)
			if ok {
				hubDevice.Mu.Lock()
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
				if providersKnown {
					hubDevice.AppiumEnabled = providerAppiumEnabled(appiumByProvider, dbDevice.Provider)
				}
				hubDevice.Mu.Unlock()
			} else {
				HubDeviceStore.Set(dbDevice.UDID, &LocalHubDevice{
					Device:                   *dbDevice,
					IsRunningAutomation:      false,
					IsAvailableForAutomation: true,
					LastAutomationActionTS:   0,
					AppiumEnabled:            providerAppiumEnabled(appiumByProvider, dbDevice.Provider),
				})
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// providerAppiumEnabled resolves whether a device's provider runs Appium servers.
// An unknown nickname (deleted provider, failed lookup) defaults to true so a
// transient gap never hides working devices from the grid or the UI
func providerAppiumEnabled(appiumByProvider map[string]bool, nickname string) bool {
	enabled, ok := appiumByProvider[nickname]
	if !ok {
		return true
	}
	return enabled
}

func GetHubDeviceByUDID(udid string) *LocalHubDevice {
	device, ok := HubDeviceStore.Get(udid)
	if ok {
		return device
	}
	return nil
}
