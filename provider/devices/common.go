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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/imagemounter"

	"github.com/pelletier/go-toml/v2"

	"GADS/common/cli"
	"GADS/common/constants"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
	"GADS/provider/providerutil"

	"GADS/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}
var DBDeviceMap = make(map[string]*models.Device)

func Listener() {
	DBDeviceMap = getDBProviderDevices()
	setupDevices()

	// Start updating devices each 10 seconds in a goroutine
	go updateDevices()
	// Start updating the local devices data to the hub in a goroutine
	go updateProviderHub()
}

func updateProviderHub() {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	var updateFailureCounter = 1
	var mu sync.Mutex

	for {
		if updateFailureCounter >= 30 {
			log.Fatalf("Unsuccessfully attempted to update device data in hub 30 times, killing provider")
		}
		time.Sleep(1 * time.Second)

		mu.Lock()

		updatedDevices := getDBProviderDevices()

		var properJson models.ProviderData
		for _, dbDevice := range DBDeviceMap {

			// Update the WorkspaceID in dbDevice from the updatedDevices map
			if updatedDevice, ok := updatedDevices[dbDevice.UDID]; ok {
				dbDevice.WorkspaceID = updatedDevice.WorkspaceID
			}

			properJson.DeviceData = append(properJson.DeviceData, *dbDevice)
			properJson.ProviderData = *config.ProviderConfig
		}
		mu.Unlock()
		jsonData, err := json.Marshal(properJson)
		if err != nil {
			updateFailureCounter++
			logger.ProviderLogger.LogError("update_provider_hub", "Failed marshaling provider data to json - "+err.Error())
			continue
		}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/provider-update", config.ProviderConfig.HubAddress), bytes.NewBuffer(jsonData))
		if err != nil {
			updateFailureCounter++
			logger.ProviderLogger.LogError("update_provider_hub", "Failed to create request to update provider data in hub - "+err.Error())
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			updateFailureCounter++
			logger.ProviderLogger.LogError("update_provider_hub", fmt.Sprintf("Failed to execute request to update provider data in hub, hub is probably down, current retry counter is `%v` - %s", updateFailureCounter, err))
			continue
		}

		if resp.StatusCode != 200 {
			updateFailureCounter++
			logger.ProviderLogger.LogError("update_provider_hub", fmt.Sprintf("Executed request to update provider data in hub but it was not successful, current retry counter is `%v` - %s", updateFailureCounter, err))
			continue
		}
		// Reset the counter if update went well
		updateFailureCounter = 1
	}
}

// When provider is started and respective devices are taken from the DB, we do the initial device data setup here
func setupDevices() {
	for _, dbDevice := range DBDeviceMap {
		dbDevice.ProviderState = "init"
		dbDevice.Connected = false
		dbDevice.LastUpdatedTimestamp = 0
		dbDevice.IsResetting = false
		dbDevice.InitialSetupDone = false

		dbDevice.Host = fmt.Sprintf("%s:%v", config.ProviderConfig.HostAddress, config.ProviderConfig.Port)

		semver, err := semver.NewVersion(dbDevice.OSVersion)
		if err != nil {
			logger.ProviderLogger.Errorf("updateDevices: Failed to get semver for device `%s` - %s", dbDevice, err)
			continue
		}
		dbDevice.SemVer = semver

		// Check if a capped Appium logs collection already exists for the current device
		exists, err := db.GlobalMongoStore.CheckCollectionExistsWithDB("appium_logs", dbDevice.UDID)
		if err != nil {
			logger.ProviderLogger.Warnf("Could not check if device collection exists in `appium_logs` db, will attempt to create it either way - %s", err)
		}

		// If it doesn't exist - attempt to create it
		if !exists {
			err = db.GlobalMongoStore.CreateCappedCollectionWithDB("appium_logs", dbDevice.UDID, 30000, 30)
			if err != nil {
				logger.ProviderLogger.Errorf("updateDevices: Failed to create capped collection for device `%s` - %s", dbDevice, err)
				continue
			}
		}

		// Create an index model and add it to the respective device Appium log collection
		appiumCollectionIndexModel := mongo.IndexModel{
			Keys: bson.D{
				{
					Key: "ts", Value: constants.SortAscending},
				{
					Key: "session_id", Value: constants.SortAscending,
				},
			},
		}
		db.GlobalMongoStore.AddCollectionIndexWithDB("appium_logs", dbDevice.UDID, appiumCollectionIndexModel)

		// Create logs directory for the device if it doesn't already exist
		if _, err := os.Stat(fmt.Sprintf("%s/device_%s", config.ProviderConfig.ProviderFolder, dbDevice.UDID)); os.IsNotExist(err) {
			err = os.Mkdir(fmt.Sprintf("%s/device_%s", config.ProviderConfig.ProviderFolder, dbDevice.UDID), os.ModePerm)
			if err != nil {
				logger.ProviderLogger.Errorf("updateDevices: Could not create logs folder for device `%s` - %s\n", dbDevice.UDID, err)
				continue
			}
		}

		// Create a custom logger and attach it to the local device
		deviceLogger, err := logger.CreateCustomLogger(fmt.Sprintf("%s/device_%s/device.log", config.ProviderConfig.ProviderFolder, dbDevice.UDID), dbDevice.UDID)
		if err != nil {
			logger.ProviderLogger.Errorf("updateDevices: Could not create custom logger for device `%s` - %s\n", dbDevice.UDID, err)
			continue
		}
		dbDevice.Logger = *deviceLogger

		appiumLogger, err := logger.NewAppiumLogger(fmt.Sprintf("%s/device_%s/appium.log", config.ProviderConfig.ProviderFolder, dbDevice.UDID), dbDevice.UDID)
		if err != nil {
			logger.ProviderLogger.Errorf("updateDevices: Could not create Appium logger for device `%s` - %s\n", dbDevice.UDID, err)
			continue
		}
		dbDevice.AppiumLogger = appiumLogger
		dbDevice.InitialSetupDone = true
	}
}

func updateDevices() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		connectedDevices := GetConnectedDevicesCommon()

	DEVICE_MAP_LOOP:
		for dbDeviceUDID, dbDevice := range DBDeviceMap {
			if dbDevice.Usage == "disabled" {
				continue DEVICE_MAP_LOOP
			}
			if slices.Contains(connectedDevices, dbDeviceUDID) {
				dbDevice.Connected = true
				if dbDevice.ProviderState != "preparing" && dbDevice.ProviderState != "live" {
					setContext(dbDevice)
					dbDevice.AppiumReadyChan = make(chan bool, 1)
					if dbDevice.OS == "ios" {
						dbDevice.WdaReadyChan = make(chan bool, 1)
						go setupIOSDevice(dbDevice)
					}

					if dbDevice.OS == "android" {
						go setupAndroidDevice(dbDevice)
					}
				}
			} else {
				ResetLocalDevice(dbDevice, "Device is no longer connected.")
				dbDevice.Connected = false
			}
		}
	}
}

func setupAndroidDevice(device *models.Device) {
	device.SetupMutex.Lock()
	defer device.SetupMutex.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)

	device.ProviderState = "preparing"

	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Running setup for device `%v`", device.UDID))

	// Log before killing existing Appium processes
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Attempting to kill existing Appium processes for device `%s`", device.UDID))
	err := cli.KillDeviceAppiumProcess(device.UDID)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Failed attempt to kill existing Appium processes for device `%s` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to kill existing Appium processes.")
		return
	}
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully killed existing Appium processes for device `%s`", device.UDID))

	// If Selenium Grid is used attempt to create a TOML file for the grid connection
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Checking if Selenium Grid is used for device `%s`", device.UDID))
	if config.ProviderConfig.UseSeleniumGrid {
		err := createGridTOML(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Selenium Grid use is enabled but couldn't create TOML for device `%s` - %s", device.UDID, err))
			ResetLocalDevice(device, "Failed to create TOML for device.")
			return
		}
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully created TOML for Selenium Grid for device `%s`", device.UDID))
	}

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Retrieving hardware model for device `%s`", device.UDID))
	getAndroidDeviceHardwareModel(device)

	if device.ScreenHeight == "" || device.ScreenWidth == "" {
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Updating screen dimensions for device `%v`", device.UDID))
		err := updateAndroidScreenSizeADB(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Failed to update screen dimensions with adb for device `%v` - %v", device.UDID, err))
			ResetLocalDevice(device, "Failed to update screen dimensions.")
			return
		}
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully updated screen dimensions for device `%v`", device.UDID))
	}

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Allocating free port for GADS-stream for device `%v`", device.UDID))
	streamPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not allocate free host port for GADS-stream for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free host port for GADS-stream.")
		return
	}
	device.StreamPort = streamPort
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully allocated free port `%v` for GADS-stream for device `%v`", device.StreamPort, device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Allocating free port for GADS Android IME for device `%v`", device.UDID))
	imePort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not allocate free host port for GADS Android IME for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free host port for GADS Android IME.")
		return
	}
	device.AndroidIMEPort = imePort
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully allocated free port `%v` for GADS Android IME for device `%v`", device.StreamPort, device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Allocating free port for GADS Android remote control server for device `%v`", device.UDID))
	remoteServerPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not allocate free host port for GADS Android remote control server for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free host port for GADS Android remote control server.")
		return
	}
	device.AndroidRemoteServerPort = remoteServerPort
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully allocated free port `%v` for GADS Android remote control server server for device `%v`", device.StreamPort, device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Allocating free port for Appium for device `%v`", device.UDID))
	appiumPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not allocate free host port for Appium for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free host port for Appium.")
		return
	}
	device.AppiumPort = appiumPort
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully allocated free port `%v` for Appium for device `%v`", device.AppiumPort, device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Checking for existing GADS-stream app on device `%v`", device.UDID))
	device.InstalledApps = GetInstalledAppsAndroid(device)
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Updated installed apps for Android device `%v`", device.UDID))
	if slices.Contains(device.InstalledApps, "com.shamanec.stream") {
		err = UninstallApp(device, "com.shamanec.stream")
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not uninstall GADS mjpeg stream app from Android device - %v:\n %v", device.UDID, err))
			ResetLocalDevice(device, "Failed to uninstall GADS mjpeg stream app from Android device.")
		}
		time.Sleep(3 * time.Second)
	}

	if slices.Contains(device.InstalledApps, "com.gads.webrtc") {
		err = UninstallApp(device, "com.gads.webrtc")
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not uninstall GADS WebRTC stream app from Android device - %v:\n %v", device.UDID, err))
			ResetLocalDevice(device, "Failed to uninstall GADS WebRTC stream app from Android device.")
		}
		time.Sleep(3 * time.Second)
	}

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Checked for and uninstalled existing GADS stream apps on device `%v` if they were present", device.UDID))

	if !device.UseWebRTCVideo {
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Installing GADS-stream on device `%v`", device.UDID))
		err = installGadsStream(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not install GADS-stream on Android device - %v:\n %v", device.UDID, err))
			ResetLocalDevice(device, "Failed to install GADS-stream on Android device.")
			return
		}
		time.Sleep(2 * time.Second)
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully installed GADS-stream on device `%v`", device.UDID))
	} else {
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Installing GADS WebRTC stream on device `%v`", device.UDID))
		err = installGadsWebRTCStream(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not install GADS WebRTC stream on Android device - %v:\n %v", device.UDID, err))
			ResetLocalDevice(device, "Failed to install GADS WebRTC stream on Android device.")
			return
		}
		time.Sleep(2 * time.Second)
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully installed GADS WebRTC stream on device `%v`", device.UDID))
	}

	err = installGadsSettingsApp(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not install GADS Settings on Android device - %v:\n %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to install GADS Settings on Android device.")
		return
	}

	err = pushGadsSettingsInTmpLocal(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not push GADS Settings on Android device - %v:\n %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to push GADS Settings on Android device.")
		return
	}
	time.Sleep(2 * time.Second)

	go startGadsRemoteControlServer(device)
	time.Sleep(2 * time.Second)

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Setting GADS stream recording permissions on Android device `%v`", device.UDID))
	err = addGadsStreamRecordingPermissions(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not set GADS stream recording permissions on Android device - %v:\n %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to set GADS-stream recording permissions on Android device.")
		return
	}
	time.Sleep(2 * time.Second)
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully set GADS stream recording permissions on Android device `%v`", device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Starting GADS stream app on Android device `%v`", device.UDID))
	err = startGadsStreamApp(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not start GADS stream app on Android device - %v:\n %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to start GADS-stream app on Android device.")
		return
	}
	time.Sleep(2 * time.Second)
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully started GADS stream app on Android device `%v`", device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Pressing home button on Android device `%v`", device.UDID))
	pressHomeButton(device)
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully pressed home button on Android device `%v`", device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Forwarding GADS stream port to host port for Android device `%v`", device.UDID))
	err = forwardGadsStream(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS stream port to host port %v for Android device - %v:\n %v", device.StreamPort, device.UDID, err))
		ResetLocalDevice(device, "Failed to forward GADS-stream port to host port.")
		return
	}
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully forwarded GADS stream port to host port for Android device `%v`", device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Setup GADS Android IME for Android device `%v`", device.UDID))
	err = setupGadsAndroidIME(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Failed to setup GADS Android IME for Android device `%v` - \n %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to setup GADS Android IME.")
	}
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully setup GADS Android IME for Android device `%v`", device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Forwarding GADS Android IME port to host port for Android device `%v`", device.UDID))
	err = forwardGadsAndroidIME(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS Android IME port to host port %v for Android device - %v:\n %v", device.StreamPort, device.UDID, err))
		ResetLocalDevice(device, "Failed to forward GADS Android IME port to host port.")
		return
	}
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully forwarded GADS Android IME port to host port for Android device `%v`", device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Forwarding GADS Android Settings port to host port for Android device `%v`", device.UDID))
	err = forwardGadsRemoteServer(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS Android Settings port to host port %v for Android device - %v:\n %v", device.AndroidRemoteServerPort, device.UDID, err))
		ResetLocalDevice(device, "Failed to forward GADS Android Settings port to host port.")
		return
	}
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully forwarded GADS Android IME port to host port for Android device `%v`", device.UDID))

	if slices.Contains(device.InstalledApps, "io.appium.settings") {
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Uninstalling Appium settings on device `%s`", device.UDID))
		err = UninstallApp(device, "io.appium.settings")
		if err != nil {
			logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to uninstall Appium settings on device %s - %s", device.UDID, err))
		}
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully uninstalled Appium settings on device `%s`", device.UDID))
	}

	if slices.Contains(device.InstalledApps, "io.appium.uiautomator2.server") {
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Uninstalling Appium uiautomator2 server on device `%s`", device.UDID))
		err = UninstallApp(device, "io.appium.uiautomator2.server")
		if err != nil {
			logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to uninstall Appium uiautomator2 server on device %s - %s", device.UDID, err))
		}
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully uninstalled Appium uiautomator2 server on device `%s`", device.UDID))
	}

	if slices.Contains(device.InstalledApps, "io.appium.uiautomator2.server.test") {
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Uninstalling Appium uiautomator2 server test on device `%s`", device.UDID))
		err = UninstallApp(device, "io.appium.uiautomator2.server.test")
		if err != nil {
			logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to uninstall Appium uiautomator2 server test on device %s - %s", device.UDID, err))
		}
		logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully uninstalled Appium uiautomator2 server test on device `%s`", device.UDID))
	}

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Applying device stream settings to device `%v`", device.UDID))
	err = applyDeviceStreamSettings(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Did not successfully apply the device stream settings to device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to apply device stream settings.")
		return
	}
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully applied device stream settings to device `%v`", device.UDID))

	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Updating GADS stream settings for device `%s`", device.UDID))
	err = UpdateGadsStreamSettings(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Failed to update GADS stream settings for device `%s` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to update GADS stream settings.")
		return
	}
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully updated GADS stream settings for device `%s`", device.UDID))

	go startAppium(device, &wg)
	go checkAppiumUp(device)

	select {
	case <-device.AppiumReadyChan:
		logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Successfully started Appium for device `%v` on port %v", device.UDID, device.AppiumPort))
		break
	case <-time.After(30 * time.Second):
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Did not successfully start Appium for device `%v` in 60 seconds", device.UDID))
		ResetLocalDevice(device, "Failed to start Appium for device.")
		return
	}

	if config.ProviderConfig.UseSeleniumGrid {
		go startGridNode(device)
	}

	// Mark the device as 'live'
	device.ProviderState = "live"
	wg.Wait()
}

func setupIOSDevice(device *models.Device) {
	device.SetupMutex.Lock()
	defer device.SetupMutex.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)

	device.ProviderState = "preparing"
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", device.UDID))

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Attempting to kill existing Appium processes for device `%s`", device.UDID))
	err := cli.KillDeviceAppiumProcess(device.UDID)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed attempt to kill existing Appium processes for device `%s` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to kill existing Appium processes.")
		return
	}
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully killed existing Appium processes for device `%s`", device.UDID))

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Getting `go-ios` DeviceEntry for device `%s`", device.UDID))
	goIosDeviceEntry, err := ios.GetDevice(device.UDID)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not get `go-ios` DeviceEntry for device - %v, err - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to get `go-ios` DeviceEntry for device.")
		return
	}

	device.GoIOSDeviceEntry = goIosDeviceEntry
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully retrieved `go-ios` DeviceEntry for device `%s`", device.UDID))

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Pairing device `%s` with go-ios", device.UDID))
	err = pairIOS(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to pair device `%s` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to pair device.")
		return
	}
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully paired device `%s` with go-ios", device.UDID))

	// Check if developer mode is enabled on the device
	if device.SemVer.Major() >= 16 {
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Checking if developer mode is enabled on device `%s`", device.UDID))
		devModeEnabled, err := imagemounter.IsDevModeEnabled(device.GoIOSDeviceEntry)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not check developer mode status on device `%s` - %s", device.UDID, err))
			ResetLocalDevice(device, "Failed to check developer mode status on device.")
			return
		}
		if !devModeEnabled {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Device `%s` is iOS 16+ but developer mode is not enabled!", device.UDID))
			ResetLocalDevice(device, "Device is iOS 16+ but developer mode is not enabled.")
			return
		}
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Developer mode is enabled on device `%s`", device.UDID))
	}

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Mounting the DDI on device `%s`", device.UDID))
	mountDeveloperImageIOS(device)

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Getting device info with go-ios for device `%s`", device.UDID))
	plistValues, err := ios.GetValuesPlist(device.GoIOSDeviceEntry)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not get info plist values with go-ios `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to get info plist values with go-ios.")
		return
	}
	// Update hardware model got from plist
	device.HardwareModel = plistValues["HardwareModel"].(string)
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully retrieved hardware model for device `%s`: %s", device.UDID, device.HardwareModel))

	if device.ScreenHeight == "" || device.ScreenWidth == "" {
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Updating screen dimensions for device `%s`", device.UDID))
		err = updateIOSScreenSize(device, plistValues["ProductType"].(string))
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to update screen dimensions for device `%s` - %s", device.UDID, err))
			ResetLocalDevice(device, "Failed to update screen dimensions for device.")
			return
		}
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully updated screen dimensions for device `%s`", device.UDID))
	}

	// If Selenium Grid is used attempt to create a TOML file for the grid connection
	if config.ProviderConfig.UseSeleniumGrid {
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Creating TOML file for Selenium Grid for device `%s`", device.UDID))
		err := createGridTOML(device)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Selenium Grid use is enabled but couldn't create TOML for device `%s` - %s", device.UDID, err))
			ResetLocalDevice(device, "Failed to create TOML for device.")
			return
		}
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully created TOML for Selenium Grid for device `%s`", device.UDID))
	}

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Allocating free port for WebDriverAgent for device `%s`", device.UDID))
	tunnelPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free WebDriverAgent port for device.")
		return
	}
	intTunnelPort, _ := strconv.Atoi(tunnelPort)
	device.GoIOSDeviceEntry.UserspaceTUNPort = intTunnelPort
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully allocated free WebDriverAgent port `%s` for device `%s`", tunnelPort, device.UDID))

	// Create userspace tunnel for devices iOS 17.4+
	if device.SemVer.Compare(semver.MustParse("17.4.0")) >= 0 {
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Creating userspace tunnel for device `%s`", device.UDID))
		deviceTunnel, err := createGoIOSTunnel(device.Context, device)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to create userspace tunnel for device `%s` - %v", device.UDID, err))
			ResetLocalDevice(device, "Failed to create userspace tunnel for device.")
			return
		}
		device.GoIOSTunnel = deviceTunnel
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully created userspace tunnel for device `%s`", device.UDID))

		// Set the ports from the tunnel on the GoIOSDeviceEntry
		device.GoIOSDeviceEntry.UserspaceTUNPort = device.GoIOSTunnel.UserspaceTUNPort
		device.GoIOSDeviceEntry.UserspaceTUN = device.GoIOSTunnel.UserspaceTUN

		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Creating go-ios device entry with rsd provider for device `%s`", device.UDID))
		err = goIosDeviceWithRsdProvider(device)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to create go-ios device entry with rsd provider for device `%s` - %v", device.UDID, err))
			ResetLocalDevice(device, "Failed to create go-ios device entry with rsd provider for device.")
			return
		}
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully created go-ios device entry with rsd provider for device `%s`", device.UDID))
	}

	time.Sleep(1 * time.Second)

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Allocating free WebDriverAgent port for device `%s`", device.UDID))
	wdaPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free WebDriverAgent port for device.")
		return
	}
	device.WDAPort = wdaPort
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully allocated free WebDriverAgent port `%s` for device `%s`", wdaPort, device.UDID))

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Allocating free stream port for device `%s`", device.UDID))
	streamPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free iOS stream port for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free iOS stream port for device.")
		return
	}
	device.StreamPort = streamPort
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully allocated free iOS stream port `%s` for device `%s`", streamPort, device.UDID))

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Allocating free WebDriverAgent stream port for device `%s`", device.UDID))
	wdaStreamPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent stream port for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free WebDriverAgent stream port for device.")
		return
	}
	device.WDAStreamPort = wdaStreamPort
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully allocated free WebDriverAgent stream port `%s` for device `%s`", wdaStreamPort, device.UDID))

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Allocating free Appium port for device `%s`", device.UDID))
	appiumPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free Appium port for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free Appium port for device.")
		return
	}
	device.AppiumPort = appiumPort
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully allocated free Appium port `%s` for device `%s`", appiumPort, device.UDID))

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Forwarding WebDriverAgent server and stream to the host for device `%s`", device.UDID))
	go goIosForward(device, device.WDAPort, "8100")
	go goIosForward(device, device.StreamPort, "9500")
	go goIosForward(device, device.WDAStreamPort, "9100")
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully forwarded WebDriverAgent server and stream for device `%s`", device.UDID))

	if device.SemVer.Major() < 17 || device.SemVer.Compare(semver.MustParse("17.4.0")) >= 0 {
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Installing WebDriverAgent on device `%s`", device.UDID))
		err = installAppIOS(device, fmt.Sprintf("%s/WebDriverAgent.ipa", config.ProviderConfig.ProviderFolder))
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not install WebDriverAgent on device `%s` - %s", device.UDID, err))
			ResetLocalDevice(device, "Failed to install WebDriverAgent on device.")
			return
		}
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully installed WebDriverAgent on device `%s`", device.UDID))
		go runWDAGoIOS(device)
	} else {
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Launching WebDriverAgent on device `%s`", device.UDID))
		err = launchAppIOS(device, config.ProviderConfig.WdaBundleID, true)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not launch WebDriverAgent on device `%s` - %s", device.UDID, err))
			ResetLocalDevice(device, "Failed to launch WebDriverAgent on device.")
			return
		}
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully launched WebDriverAgent on device `%s`", device.UDID))
	}

	go checkWebDriverAgentUp(device)

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Waiting until WebDriverAgent successfully starts for device `%s`", device.UDID))
	select {
	case <-device.WdaReadyChan:
		logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Successfully started WebDriverAgent for device `%v` forwarded on port %v", device.UDID, device.WDAPort))
		break
	case <-time.After(60 * time.Second):
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully start WebDriverAgent on device `%v` in 60 seconds", device.UDID))
		ResetLocalDevice(device, "Failed to start WebDriverAgent on device.")
		return
	}

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Applying device stream settings to device `%v`", device.UDID))
	err = applyDeviceStreamSettings(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully apply the device stream settings to device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to apply device stream settings.")
		return
	}
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully applied device stream settings to device `%v`", device.UDID))

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Creating WebDriverAgent session and updating MJPEG stream settings for device `%v`", device.UDID))
	err = updateWebDriverAgent(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully create WebDriverAgent session or update its stream settings for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to create WebDriverAgent session or update its stream settings.")
		return
	}
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Successfully created WebDriverAgent session and updated stream settings for device `%v`", device.UDID))

	go startAppium(device, &wg)
	go checkAppiumUp(device)

	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Waiting until Appium successfully starts for device `%s`", device.UDID))
	select {
	case <-device.AppiumReadyChan:
		logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Successfully started Appium for device `%v` on port %v", device.UDID, device.AppiumPort))
		break
	case <-time.After(30 * time.Second):
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully start Appium for device `%v` in 60 seconds", device.UDID))
		ResetLocalDevice(device, "Failed to start Appium for device.")
		return
	}

	if config.ProviderConfig.UseSeleniumGrid {
		logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Starting Selenium grid node for device `%s`", device.UDID))
		go startGridNode(device)
	}

	device.InstalledApps = GetInstalledAppsIOS(device)
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("Updated installed apps for device `%v`", device.UDID))

	// Mark the device as 'live'
	device.ProviderState = "live"
	wg.Wait()
}

// Gets all connected iOS and Android devices to the host
func GetConnectedDevicesCommon() []string {
	var connectedDevices []string

	var androidDevices []string
	var iosDevices []string

	if config.ProviderConfig.ProvideAndroid {
		androidDevices = getConnectedDevicesAndroid()
	}

	if config.ProviderConfig.ProvideIOS {
		iosDevices = getConnectedDevicesIOS()
	}

	connectedDevices = append(connectedDevices, iosDevices...)
	connectedDevices = append(connectedDevices, androidDevices...)

	return connectedDevices
}

// Gets the connected iOS devices using the `go-ios` library
func getConnectedDevicesIOS() []string {
	var connectedDevices []string

	deviceList, err := ios.ListDevices()
	if err != nil {
		return connectedDevices
	}

	for _, connDevice := range deviceList.DeviceList {
		connectedDevices = append(connectedDevices, connDevice.Properties.SerialNumber)
	}
	return connectedDevices
}

// Gets the connected android devices using `adb`
func getConnectedDevicesAndroid() []string {
	var connectedDevices []string

	cmd := exec.Command("adb", "devices")
	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("getConnectedDevicesAndroid: Creating exec cmd StdoutPipe failed, returning empty slice - %s", err))
		return connectedDevices
	}

	err = cmd.Start()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("getConnectedDevicesAndroid: Error executing `%s` , returning empty slice - %s", cmd.Args, err))
		return connectedDevices
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "List of devices") && line != "" && strings.Contains(line, "device") && !strings.Contains(line, "emulator") {
			connectedDevices = append(connectedDevices, strings.Fields(line)[0])
		}
	}

	err = cmd.Wait()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("getConnectedDevicesAndroid: Waiting for `%s` command to finish failed, returning empty slice - %s", cmd.Args, err))
		return []string{}
	}

	return connectedDevices
}

func ResetLocalDevice(device *models.Device, reason string) {
	device.Mutex.Lock()
	defer device.Mutex.Unlock()
	if !device.IsResetting && device.ProviderState != "init" {
		logger.ProviderLogger.LogInfo("provider", fmt.Sprintf("Resetting LocalDevice for device `%v` with reason: %s. Cancelling context, setting ProviderState to `init`, Healthy to `false` and updating the DB", device.UDID, reason))

		device.IsResetting = true
		device.CtxCancel()
		device.ProviderState = "init"
		device.IsResetting = false
		if device.GoIOSTunnel.Address != "" {
			device.GoIOSTunnel.Close()
		}

		// Free any used ports from the map where we keep them
		common.MutexManager.LocalDevicePorts.Lock()
		delete(providerutil.UsedPorts, device.WDAPort)
		delete(providerutil.UsedPorts, device.StreamPort)
		delete(providerutil.UsedPorts, device.AppiumPort)
		delete(providerutil.UsedPorts, device.WDAStreamPort)
		common.MutexManager.LocalDevicePorts.Unlock()
	}
}

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func setContext(device *models.Device) {
	device.SetupMutex.Lock()
	defer device.SetupMutex.Unlock()
	ctx, cancelFunc := context.WithCancel(context.Background())
	device.CtxCancel = cancelFunc
	device.Context = ctx
}

func startAppium(device *models.Device, deviceSetupWg *sync.WaitGroup) {
	var capabilities models.AppiumServerCapabilities

	if device.OS == "ios" {
		capabilities = models.AppiumServerCapabilities{
			UDID:                  device.UDID,
			WdaURL:                "http://localhost:" + device.WDAPort,
			WdaLocalPort:          device.WDAPort,
			WdaLaunchTimeout:      "120000",
			WdaConnectionTimeout:  "240000",
			ClearSystemFiles:      "false",
			PreventWdaAttachments: "true",
			SimpleIsVisibleCheck:  "false",
			AutomationName:        "XCUITest",
			PlatformName:          "iOS",
			DeviceName:            device.Name,
		}
	} else if device.OS == "android" {
		capabilities = models.AppiumServerCapabilities{
			UDID:           device.UDID,
			AutomationName: "UiAutomator2",
			PlatformName:   "Android",
			DeviceName:     device.Name,
		}
	}

	capabilitiesJson, _ := json.Marshal(capabilities)
	cmd := exec.CommandContext(
		device.Context,
		"appium",
		"-p",
		device.AppiumPort,
		"--log-timestamp",
		"--session-override",
		"--log-no-colors",
		"--relaxed-security",
		"--default-capabilities", string(capabilitiesJson))

	logger.ProviderLogger.LogDebug("device_setup", fmt.Sprintf("Starting Appium on device `%s` with command `%s`", device.UDID, cmd.Args))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startAppium: Error creating stdoutpipe on `%s` for device `%v` - %v", cmd.Args, device.UDID, err))
		ResetLocalDevice(device, "Failed to create stdoutpipe on Appium command.")
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startAppium: Error creating stderrpipe on `%s` for device `%v` - %v", cmd.Args, device.UDID, err))
		ResetLocalDevice(device, "Failed to create stderrpipe on Appium command.")
		return
	}

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error executing `%s` for device `%v` - %v", cmd.Args, device.UDID, err))
		ResetLocalDevice(device, "Failed to execute Appium command.")
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Process stdout
	go func() {
		defer wg.Done()
		processStream(bufio.NewScanner(stdout), device, false)
	}()

	// Process stderr
	go func() {
		defer wg.Done()
		processStream(bufio.NewScanner(stderr), device, true)
	}()

	// Wait for stdout and stderr processing to finish
	wg.Wait()

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf(
			"startAppium: Error waiting for `%s` command to finish, it errored out or device `%v` was disconnected - %v",
			cmd.Args, device.UDID, err))

		ResetLocalDevice(device, "Appium command errored out or device was disconnected.")
		deviceSetupWg.Done()
	}
}

func processStream(scanner *bufio.Scanner, device *models.Device, isErrorStream bool) {
	linesChan := make(chan string)

	// Goroutine responsible for reading from the scanner and sending lines to the channel
	go func() {
		defer close(linesChan)
		for scanner.Scan() {
			linesChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("processStream: %v scanner error - %v", device.UDID, err))
			return
		}
	}()

	// Main loop that listens to the lines channel and the device context
	for {
		select {
		case <-device.Context.Done():
			return // Exit if the device context is canceled
		case line, ok := <-linesChan:
			if !ok {
				return // Exit if the channel is closed (EOF reached)
			}
			// If it's an error stream, split the line if it contains multiple lines and log each separately
			if isErrorStream {
				lines := strings.Split(line, "\n")
				for _, l := range lines {
					logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startAppium: %v Appium error - %v", device.UDID, l))
				}
			} else {
				device.AppiumLogger.Log(device, line)
			}
		case <-time.After(500 * time.Millisecond):
			// On timeout, continue the loop waiting for new lines
			continue
		}
	}
}

func createGridTOML(device *models.Device) error {
	automationName := ""
	if device.OS == "ios" {
		automationName = "XCUITest"
	} else {
		automationName = "UiAutomator2"
	}

	url := fmt.Sprintf("http://%s:%v/device/%s/appium", config.ProviderConfig.HostAddress, config.ProviderConfig.Port, device.UDID)
	configs := fmt.Sprintf(`{"appium:deviceName": "%s", "platformName": "%s", "appium:platformVersion": "%s", "appium:automationName": "%s", "appium:udid": "%s"}`, device.Name, device.OS, device.OSVersion, automationName, device.UDID)

	port, _ := providerutil.GetFreePort()
	portInt, _ := strconv.Atoi(port)
	conf := models.AppiumTomlConfig{
		Server: models.AppiumTomlServer{
			Port: portInt,
		},
		Node: models.AppiumTomlNode{
			DetectDrivers: false,
		},
		Relay: models.AppiumTomlRelay{
			URL:            url,
			StatusEndpoint: "/status",
			Configs: []string{
				"1",
				configs,
			},
		},
	}

	res, err := toml.Marshal(conf)
	if err != nil {
		return fmt.Errorf("Failed marshalling TOML Appium config - %s", err)
	}

	file, err := os.Create(fmt.Sprintf("%s/%s.toml", config.ProviderConfig.ProviderFolder, device.UDID))
	if err != nil {
		return fmt.Errorf("Failed creating TOML Appium config file - %s", err)
	}
	defer file.Close()

	_, err = io.WriteString(file, string(res))
	if err != nil {
		return fmt.Errorf("Failed writing to TOML Appium config file - %s", err)
	}

	return nil
}

func startGridNode(device *models.Device) {
	time.Sleep(5 * time.Second)
	cmd := exec.CommandContext(device.Context,
		"java",
		"-jar",
		fmt.Sprintf("%s/selenium.jar", config.ProviderConfig.ProviderFolder),
		"node",
		"--host",
		config.ProviderConfig.HostAddress,
		"--config",
		fmt.Sprintf("%s/%s.toml", config.ProviderConfig.ProviderFolder, device.UDID),
		"--grid-url",
		config.ProviderConfig.SeleniumGrid,
	)

	logger.ProviderLogger.LogInfo("device_setup", fmt.Sprintf("Starting Selenium grid node for device `%s` with command `%s`", device.UDID, cmd.Args))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while starting Selenium Grid node for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to create stdoutpipe while starting Selenium Grid node.")
		return
	}

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start Selenium Grid node for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to start Selenium Grid node.")
		return
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		device.Logger.LogDebug("grid-node", strings.TrimSpace(line))
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Selenium Grid node command to finish, it errored out or device `%v` was disconnected - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to wait for Selenium Grid node command to finish.")
	}
}

func UpdateInstalledApps(device *models.Device) {
	if device.OS == "ios" {
		device.InstalledApps = GetInstalledAppsIOS(device)
	} else {
		device.InstalledApps = GetInstalledAppsAndroid(device)
	}
}

func UninstallApp(device *models.Device, app string) error {
	if device.OS == "ios" {
		err := uninstallAppIOS(device, app)
		if err != nil {
			return err
		}
	} else {
		err := uninstallAppAndroid(device, app)
		if err != nil {
			return err
		}
	}

	return nil
}

func InstallApp(device *models.Device, app string) error {
	if device.OS == "ios" {
		err := installAppDefaultPath(device, app)
		if err != nil {
			device.Logger.LogError("install_app_ios", fmt.Sprintf("Failed installing app on device `%s` - %s", device.UDID, err))
			return err
		}
	} else {
		err := installAppAndroid(device, app)
		if err != nil {
			device.Logger.LogError("install_app_android", fmt.Sprintf("Failed installing app on device `%s` - %s", device.UDID, err))
			return err
		}
	}

	return nil
}

func getAndroidDeviceHardwareModel(device *models.Device) {
	brandCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "getprop", "ro.product.brand")
	var outBuffer bytes.Buffer
	brandCmd.Stdout = &outBuffer
	if err := brandCmd.Run(); err != nil {
		device.HardwareModel = "Unknown"
	}
	brand := outBuffer.String()
	outBuffer.Reset()

	modelCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "getprop", "ro.product.model")
	modelCmd.Stdout = &outBuffer
	if err := modelCmd.Run(); err != nil {
		device.HardwareModel = "Unknown"
		return
	}
	model := outBuffer.String()

	device.HardwareModel = fmt.Sprintf("%s %s", strings.TrimSpace(brand), strings.TrimSpace(model))
}

func checkAppiumUp(device *models.Device) {
	var netClient = &http.Client{
		Timeout: time.Second * 30,
	}

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%v/status", device.AppiumPort), nil)

	loops := 0
	for {
		if loops >= 30 {
			return
		}
		resp, err := netClient.Do(req)
		if err != nil {
			time.Sleep(1 * time.Second)
		} else {
			if resp.StatusCode == http.StatusOK {
				device.AppiumReadyChan <- true
				return
			}
		}
		loops++
	}
}

func updateDeviceWithGlobalSettings(dbDevice *models.Device) error {
	globalSettings, err := db.GlobalMongoStore.GetGlobalStreamSettings()
	if err != nil {
		return fmt.Errorf("failed to get global stream settings: %v", err)
	}

	dbDevice.StreamTargetFPS = globalSettings.TargetFPS
	dbDevice.StreamJpegQuality = globalSettings.JpegQuality

	// Check the device OS before assigning the scaling factor
	if dbDevice.OS == "android" {
		dbDevice.StreamScalingFactor = globalSettings.ScalingFactorAndroid
	} else if dbDevice.OS == "ios" {
		dbDevice.StreamScalingFactor = globalSettings.ScalingFactoriOS
	}

	return nil
}

func applyDeviceStreamSettings(device *models.Device) error {
	common.MutexManager.StreamSettings.Lock()
	defer common.MutexManager.StreamSettings.Unlock()
	// Get the DeviceStreamSettings for the current device
	deviceStreamSettings, err := db.GlobalMongoStore.GetDeviceStreamSettings(device.UDID)

	if err != nil {
		// If there's an error (including not found), update the device with global settings
		err = updateDeviceWithGlobalSettings(device)
		if err != nil {
			logger.ProviderLogger.LogError("setupDevices", fmt.Sprintf("Failed to update device `%s` with global settings: %v", device.UDID, err))
			return err
		}
	} else {
		// Apply the retrieved stream settings
		device.StreamTargetFPS = deviceStreamSettings.StreamTargetFPS
		device.StreamJpegQuality = deviceStreamSettings.StreamJpegQuality
		device.StreamScalingFactor = deviceStreamSettings.StreamScalingFactor
	}

	return nil
}
