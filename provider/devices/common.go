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
	"log"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"

	"GADS/common"
	"GADS/common/constants"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
	"GADS/provider/providerutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

// DevManager is the primary device store holding PlatformDevice instances.
var DevManager = NewDeviceStore()

// dbDevices holds the raw DB device map for initial setup. Not exported or used by the router.
var dbDevices map[string]*models.Device

func Listener() {
	dbDevices = getDBProviderDevices()
	setupDevices()

	Setup()

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

	for {
		if updateFailureCounter >= 30 {
			log.Fatalf("Unsuccessfully attempted to update device data in hub 30 times, killing provider")
		}
		time.Sleep(1 * time.Second)

		updatedDevices := getDBProviderDevices()

		// Track devices to remove or reset
		var devicesToRemove []string
		var devicesToReset []string

		var properJson models.ProviderData
		properJson.ProviderData = *config.ProviderConfig

		// Iterate current devices in DevManager
		allDevs := DevManager.All()
		for _, platDev := range allDevs {
			udid := platDev.GetUDID()
			dbDevice := platDev.GetDBDevice()
			// Check if device still exists in DB
			if updatedDevice, ok := updatedDevices[udid]; ok {
				// Update configuration fields from DB
				if dbDevice.ScreenWidth != updatedDevice.ScreenWidth {
					dbDevice.ScreenWidth = updatedDevice.ScreenWidth
				}
				if dbDevice.ScreenHeight != updatedDevice.ScreenHeight {
					dbDevice.ScreenHeight = updatedDevice.ScreenHeight
				}
				if dbDevice.Name != updatedDevice.Name {
					dbDevice.Name = updatedDevice.Name
				}
				if dbDevice.OSVersion != updatedDevice.OSVersion {
					dbDevice.OSVersion = updatedDevice.OSVersion
				}
				if dbDevice.Usage != updatedDevice.Usage {
					dbDevice.Usage = updatedDevice.Usage
				}
				if dbDevice.WorkspaceID != updatedDevice.WorkspaceID {
					dbDevice.WorkspaceID = updatedDevice.WorkspaceID
				}
				if dbDevice.StreamType != updatedDevice.StreamType {
					dbDevice.StreamType = updatedDevice.StreamType
					devicesToReset = append(devicesToReset, udid)
				}

				// If the provider does not set up Appium servers
				// Always return device usage as `control`
				if !config.ProviderConfig.SetupAppiumServers {
					if dbDevice.Usage != "disabled" {
						dbDevice.Usage = "control"
					}
				}

				properJson.DeviceData = append(properJson.DeviceData, platDev.ToHubDevice())
			} else {
				// Device no longer exists in DB, mark for removal
				devicesToRemove = append(devicesToRemove, udid)
			}
		}

		// Process resets and removals
		for _, udid := range devicesToReset {
			if platDev, ok := DevManager.Get(udid); ok {
				platDev.Reset("WebRTC configuration changed, reprovisioning device")
			}
		}
		for _, udid := range devicesToRemove {
			if platDev, ok := DevManager.Get(udid); ok {
				platDev.Reset("Device removed from DB")
				DevManager.Delete(udid)
			}
		}

		// Add new devices from DB
		for udid, updatedDevice := range updatedDevices {
			if _, exists := DevManager.Get(udid); !exists {
				logger.ProviderLogger.LogInfo("update_provider_hub", fmt.Sprintf("New device `%s` detected in DB, adding to provider", udid))
				if err := initializeDevice(updatedDevice); err != nil {
					logger.ProviderLogger.LogError("update_provider_hub", fmt.Sprintf("Failed to initialize new device `%s` - %s", udid, err))
					continue
				}
				if platDev, ok := DevManager.Get(udid); ok {
					properJson.DeviceData = append(properJson.DeviceData, platDev.ToHubDevice())
				}
			}
		}

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

// initializeDevice initializes a single device: sets up DB-level fields, creates a
// PlatformDevice with Logger/SemVer on RuntimeState, and stores it in DevManager.
func initializeDevice(dbDevice *models.Device) error {
	dbDevice.ProviderState = "init"
	dbDevice.Connected = false
	dbDevice.LastUpdatedTimestamp = 0
	dbDevice.IsResetting = false

	dbDevice.Host = fmt.Sprintf("%s:%v", config.ProviderConfig.HostAddress, config.ProviderConfig.Port)

	sv, err := semver.NewVersion(dbDevice.OSVersion)
	if err != nil {
		return fmt.Errorf("failed to get semver for device `%s` - %s", dbDevice.UDID, err)
	}

	if config.ProviderConfig.SetupAppiumServers {
		// Check if a capped Appium logs collection already exists for the current device
		exists, err := db.GlobalMongoStore.CheckCollectionExistsWithDB("appium_logs_new", dbDevice.UDID)
		if err != nil {
			logger.ProviderLogger.LogWarn("device_setup", fmt.Sprintf("Could not check if device collection exists in `appium_logs_new` db, will attempt to create it either way - %s", err))
		}

		// If it doesn't exist - attempt to create it
		if !exists {
			err = db.GlobalMongoStore.CreateCappedCollectionWithDB("appium_logs_new", dbDevice.UDID, 30000, 30)
			if err != nil {
				return fmt.Errorf("failed to create capped collection for device `%s` - %s", dbDevice.UDID, err)
			}
		}

		// Create an index model and add it to the respective device Appium log collection
		appiumCollectionIndexModel := mongo.IndexModel{
			Keys: bson.D{
				{
					Key: "timestamp", Value: constants.SortAscending,
				},
				{
					Key: "session_id", Value: constants.SortAscending,
				},
				{
					Key: "sequenceNumber", Value: constants.SortAscending,
				},
			},
		}
		db.GlobalMongoStore.AddCollectionIndexWithDB("appium_logs_new", dbDevice.UDID, appiumCollectionIndexModel)
	}

	// Create logs directory for the device if it doesn't already exist
	if _, err := os.Stat(fmt.Sprintf("%s/device_%s", config.ProviderConfig.ProviderFolder, dbDevice.UDID)); os.IsNotExist(err) {
		err = os.Mkdir(fmt.Sprintf("%s/device_%s", config.ProviderConfig.ProviderFolder, dbDevice.UDID), os.ModePerm)
		if err != nil {
			return fmt.Errorf("could not create logs folder for device `%s` - %s", dbDevice.UDID, err)
		}
	}

	// Create a custom logger
	deviceLogger, err := logger.CreateCustomLogger(fmt.Sprintf("%s/device_%s/device.log", config.ProviderConfig.ProviderFolder, dbDevice.UDID), dbDevice.UDID)
	if err != nil {
		return fmt.Errorf("could not create custom logger for device `%s` - %s", dbDevice.UDID, err)
	}

	// Create PlatformDevice with runtime fields on RuntimeState
	platDev := newPlatformDevice(dbDevice, *deviceLogger, sv)
	if platDev == nil {
		return fmt.Errorf("unsupported OS `%s` for device `%s`", dbDevice.OS, dbDevice.UDID)
	}
	DevManager.Set(dbDevice.UDID, platDev)

	return nil
}

// When provider is started and respective devices are taken from the DB, we do the initial device data setup here
func setupDevices() {
	for _, dbDevice := range dbDevices {
		if err := initializeDevice(dbDevice); err != nil {
			logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("setupDevices: %s", err))
		}
	}
	// dbDevices is no longer needed after initial setup
	dbDevices = nil
}

// newPlatformDevice creates a PlatformDevice wrapping the given *models.Device
// with Logger and SemVer set on RuntimeState.
func newPlatformDevice(dbDevice *models.Device, deviceLogger models.CustomLogger, sv *semver.Version) PlatformDevice {
	// Each case builds RuntimeState inline to avoid copying sync.Mutex via struct assignment.
	switch dbDevice.OS {
	case "ios":
		d := &IOSDevice{WdaReadyChan: make(chan bool, 1)}
		d.DBDevice = dbDevice
		d.Logger = deviceLogger
		d.SemVer = sv
		d.InitialSetupDone = true
		return d
	case "android":
		d := &AndroidDevice{}
		d.DBDevice = dbDevice
		d.Logger = deviceLogger
		d.SemVer = sv
		d.InitialSetupDone = true
		return d
	case "tizen":
		d := &TizenDevice{}
		d.DBDevice = dbDevice
		d.Logger = deviceLogger
		d.SemVer = sv
		d.InitialSetupDone = true
		return d
	case "webos":
		d := &WebOSDevice{}
		d.DBDevice = dbDevice
		d.Logger = deviceLogger
		d.SemVer = sv
		d.InitialSetupDone = true
		return d
	default:
		return nil
	}
}

func updateDevices() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var tizenTicker *time.Ticker
	var tizenChan <-chan time.Time

	if config.ProviderConfig.ProvideTizen {
		tizenTicker = time.NewTicker(30 * time.Second)
		tizenChan = tizenTicker.C
		defer tizenTicker.Stop()
	}

	for {
		select {
		case <-ticker.C:
			connectedDevices := GetConnectedDevicesCommon()

			// Create a snapshot of devices to iterate over
			allDevices := DevManager.All()

			for _, platDev := range allDevices {
				dbDevice := platDev.GetDBDevice()
				udid := platDev.GetUDID()
				if dbDevice.Usage == "disabled" {
					continue
				}
				if slices.Contains(connectedDevices, udid) {
					dbDevice.Connected = true
					if dbDevice.ProviderState != "preparing" && dbDevice.ProviderState != "live" {
						// Validate device configuration before setup
						err := models.ValidateDeviceUsageForOS(dbDevice.OS, dbDevice.Usage)
						if err != nil {
							logger.ProviderLogger.LogWarn("device_setup_validation", fmt.Sprintf("Device %s has invalid configuration: %s. Skipping setup.", udid, err.Error()))
							continue
						}

						setContext(platDev)
						go platDev.Setup()
					}
				} else {
					platDev.Reset("Device is no longer connected.")
					dbDevice.Connected = false
				}
			}

		case <-tizenChan:
			if tizenChan != nil {
				handleTizenAutoConnection(GetConnectedDevicesCommon())
			}
		}
	}
}

func Setup() {
	if config.ProviderConfig.ProvideTizen || config.ProviderConfig.ProvideWebOS {
		err := providerutil.CheckChromeDriverAndDownload()
		if err != nil {
			log.Fatalf("Setup: Failed to download and extract ChromeDriver - %s", err)
		}
	}
}

// Gets all connected iOS and Android devices to the host
func GetConnectedDevicesCommon() []string {
	var connectedDevices []string

	var androidDevices []string
	var iosDevices []string
	var tizenDevices []string
	var webosDevices []string

	if config.ProviderConfig.ProvideAndroid {
		androidDevices = getConnectedDevicesAndroid()
	}

	if config.ProviderConfig.ProvideIOS {
		iosDevices = getConnectedDevicesIOS()
	}

	if config.ProviderConfig.ProvideTizen {
		tizenDevices = getConnectedDevicesTizen()
	}

	if config.ProviderConfig.ProvideWebOS {
		webosDevices = getConnectedDevicesWebOS()
	}

	connectedDevices = append(connectedDevices, iosDevices...)
	connectedDevices = append(connectedDevices, androidDevices...)
	connectedDevices = append(connectedDevices, tizenDevices...)
	connectedDevices = append(connectedDevices, webosDevices...)

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
		if !strings.Contains(line, "List of devices") && line != "" && strings.Contains(line, "device") {
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

// setContext creates a new context for a device and stores it on the PlatformDevice's RuntimeState.
func setContext(platDev PlatformDevice) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	platDev.SetNewContext(ctx, cancelFunc)
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

func getConnectedDevicesTizen() []string {
	var devices []string
	cmd := exec.Command("sdb", "devices")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Failed to get connected Tizen devices - %s", err))
		return devices
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "List of devices attached") || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "device" {
			deviceID := fields[0]
			devices = append(devices, deviceID)
		}
	}

	return devices
}
