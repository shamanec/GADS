package devices

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/pelletier/go-toml/v2"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

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
var DeviceMap = make(map[string]*models.Device)

func Listener() {
	Setup()

	// Start updating devices each 10 seconds in a goroutine
	go updateDevices()
	// Start updating the local devices data to Mongo in a goroutine
	go updateDevicesMongo()
}

func updateDevices() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		connectedDevices := GetConnectedDevicesCommon()

		// Loop through the connected devices
		for _, connectedDevice := range connectedDevices {
			// If a connected device is not already in the local devices map
			// Do the initial set up and add it
			if _, ok := DeviceMap[connectedDevice.UDID]; !ok {
				newDevice := &models.Device{}
				newDevice.UDID = connectedDevice.UDID
				newDevice.OS = connectedDevice.OS
				newDevice.ProviderState = "init"
				newDevice.IsResetting = false
				newDevice.Connected = true

				// Add default name for the device
				if connectedDevice.OS == "ios" {
					newDevice.Name = "iPhone"
				} else {
					newDevice.Name = "Android"
				}

				newDevice.Host = fmt.Sprintf("%s:%v", config.Config.EnvConfig.HostAddress, config.Config.EnvConfig.Port)
				newDevice.Provider = config.Config.EnvConfig.Nickname
				// Set N/A for model and OS version because we will set those during the device set up
				newDevice.Model = "N/A"
				newDevice.OSVersion = "N/A"

				// Check if a capped Appium logs collection already exists for the current device
				exists, err := db.CollectionExists("appium_logs", newDevice.UDID)
				if err != nil {
					logger.ProviderLogger.Warnf("Could not check if device collection exists in `appium_logs` db, will attempt to create it either way - %s", err)
				}

				// If it doesn't exist - attempt to create it
				if !exists {
					err = db.CreateCappedCollection("appium_logs", newDevice.UDID, 30000, 30)
					if err != nil {
						logger.ProviderLogger.Errorf("updateDevices: Failed to create capped collection for device `%s` - %s", connectedDevice.UDID, err)
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
				db.AddCollectionIndex("appium_logs", newDevice.UDID, appiumCollectionIndexModel)

				// Create logs directory for the device if it doesn't already exist
				if _, err := os.Stat(fmt.Sprintf("%s/device_%s", config.Config.EnvConfig.ProviderFolder, newDevice.UDID)); os.IsNotExist(err) {
					err = os.Mkdir(fmt.Sprintf("%s/device_%s", config.Config.EnvConfig.ProviderFolder, newDevice.UDID), os.ModePerm)
					if err != nil {
						logger.ProviderLogger.Errorf("updateDevices: Could not create logs folder for device `%s` - %s\n", newDevice.UDID, err)
						continue
					}
				}

				// Create a custom logger and attach it to the local device
				deviceLogger, err := logger.CreateCustomLogger(fmt.Sprintf("%s/device_%s/device.log", config.Config.EnvConfig.ProviderFolder, newDevice.UDID), newDevice.UDID)
				if err != nil {
					logger.ProviderLogger.Errorf("updateDevices: Could not create custom logger for device `%s` - %s\n", newDevice.UDID, err)
					continue
				}
				newDevice.Logger = *deviceLogger

				appiumLogger, err := logger.NewAppiumLogger(fmt.Sprintf("%s/device_%s/appium.log", config.Config.EnvConfig.ProviderFolder, newDevice.UDID), newDevice.UDID)
				if err != nil {
					logger.ProviderLogger.Errorf("updateDevices: Could not create Appium logger for device `%s` - %s\n", newDevice.UDID, err)
					continue
				}
				newDevice.AppiumLogger = appiumLogger

				// Add the new local device to the map
				DeviceMap[connectedDevice.UDID] = newDevice
			}
		}

		// Loop through the local devices map to remove any no longer connected devices
		for _, localDevice := range DeviceMap {
			isConnected := false
			for _, connectedDevice := range connectedDevices {
				if connectedDevice.UDID == localDevice.UDID {
					isConnected = true
				}
			}

			// If the device is no longer connected
			// Reset its set up in case something is lingering and delete it from the map
			if !isConnected {
				resetLocalDevice(localDevice)
				delete(DeviceMap, localDevice.UDID)
			}
		}

		// Loop through the final local device map and set up the devices if they are not already being set up or live
		for _, device := range DeviceMap {
			// If we are not already preparing the device, or it's not already prepared
			if device.ProviderState != "preparing" && device.ProviderState != "live" {
				setContext(device)
				if device.OS == "ios" {
					device.WdaReadyChan = make(chan bool, 1)
					go setupIOSDevice(device)
				}

				if device.OS == "android" {
					go setupAndroidDevice(device)
				}
			}
		}
	}
}

// Create Mongo collections for all devices for logging
// Create a map of *device.LocalDevice for easier access across the code
func Setup() {
	if config.Config.EnvConfig.ProvideAndroid {
		err := providerutil.CheckGadsStreamAndDownload()
		if err != nil {
			log.Fatalf("Setup: Could not check availability of and download GADS-stream latest release - %s", err)
		}
	}
}

func setupAndroidDevice(device *models.Device) {
	device.ProviderState = "preparing"

	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Running setup for device `%v`", device.UDID))

	err := updateScreenSize(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not update screen dimensions with adb for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	getModel(device)
	getAndroidOSVersion(device)

	// If Selenium Grid is used attempt to create a TOML file for the grid connection
	if config.Config.EnvConfig.UseSeleniumGrid {
		err := createGridTOML(device)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Selenium Grid use is enabled but couldn't create TOML for device `%s` - %s", device.UDID, err))
			resetLocalDevice(device)
			return
		}
	}

	streamPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not allocate free host port for GADS-stream for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.StreamPort = streamPort

	apps := getInstalledAppsAndroid(device)
	if slices.Contains(apps, "com.shamanec.stream") {
		stopGadsStreamService(device)
		time.Sleep(3 * time.Second)
		err = uninstallGadsStream(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not uninstall GADS-stream from Android device - %v:\n %v", device.UDID, err))
			resetLocalDevice(device)
			return
		}
		time.Sleep(3 * time.Second)
	}

	err = installGadsStream(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not install GADS-stream on Android device - %v:\n %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	time.Sleep(2 * time.Second)

	err = addGadsStreamRecordingPermissions(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not set GADS-stream recording permissions on Android device - %v:\n %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	time.Sleep(2 * time.Second)

	err = startGadsStreamApp(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not start GADS-stream app on Android device - %v:\n %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	time.Sleep(2 * time.Second)

	pressHomeButton(device)

	err = forwardGadsStream(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS-stream port to host port %v for Android device - %v:\n %v", device.StreamPort, device.UDID, err))
		resetLocalDevice(device)
		return
	}

	device.InstalledApps = getInstalledAppsAndroid(device)

	if slices.Contains(device.InstalledApps, "io.appium.settings") {
		logger.ProviderLogger.LogInfo("android_device_setup", "Appium settings found on device, attempting to uninstall")
		err = UninstallApp(device, "io.appium.settings")
		if err != nil {
			logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to uninstall Appium settings on device %s - %s", device.UDID, err))
		}
	}

	if slices.Contains(device.InstalledApps, "io.appium.uiautomator2.server") {
		logger.ProviderLogger.LogInfo("android_device_setup", "Appium uiautomator2 server found on device, attempting to uninstall")
		err = UninstallApp(device, "io.appium.uiautomator2.server")
		if err != nil {
			logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to uninstall Appium uiautomator2 server on device %s - %s", device.UDID, err))
		}
	}

	if slices.Contains(device.InstalledApps, "io.appium.uiautomator2.server.test") {
		logger.ProviderLogger.LogInfo("android_device_setup", "Appium uiautomator2 server test found on device, attempting to uninstall")
		err = UninstallApp(device, "io.appium.uiautomator2.server.test")
		if err != nil {
			logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to uninstall Appium uiautomator2 server test on device %s - %s", device.UDID, err))
		}
	}

	go startAppium(device)
	if config.Config.EnvConfig.UseSeleniumGrid {
		go startGridNode(device)
	}

	// Mark the device as 'live'
	device.ProviderState = "live"
}

func setupIOSDevice(device *models.Device) {
	device.ProviderState = "preparing"
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", device.UDID))

	goIosDeviceEntry, err := ios.GetDevice(device.UDID)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not get `go-ios` DeviceEntry for device - %v, err - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	device.GoIOSDeviceEntry = goIosDeviceEntry

	// Get device info with go-ios to get the hardware model
	plistValues, err := ios.GetValuesPlist(device.GoIOSDeviceEntry)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not get info plist values with go-ios `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	// Update hardware model got from plist, os version and product type
	device.HardwareModel = plistValues["HardwareModel"].(string)
	device.OSVersion = plistValues["ProductVersion"].(string)
	device.IOSProductType = plistValues["ProductType"].(string)

	isAboveIOS17, err := isAboveIOS17(device)
	if err != nil {
		device.Logger.LogError("ios_device_setup", fmt.Sprintf("Could not determine if device `%v` is above iOS 17 - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if isAboveIOS17 && config.Config.EnvConfig.OS != "darwin" {
		logger.ProviderLogger.LogInfo("ios_device_setup", "Device `%s` is iOS 17+ which is not supported on Windows/Linux, setup will be skipped")
		device.ProviderState = "init"
		return
	}

	// If Selenium Grid is used attempt to create a TOML file for the grid connection
	if config.Config.EnvConfig.UseSeleniumGrid {
		err := createGridTOML(device)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Selenium Grid use is enabled but couldn't create TOML for device `%s` - %s", device.UDID, err))
			resetLocalDevice(device)
			return
		}
	}

	// Update the screen dimensions of the device using data from the IOSDeviceDimensions map
	err = updateScreenSize(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not update screen dimensions for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	getModel(device)

	wdaPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.WDAPort = wdaPort

	streamPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free iOS stream port for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.StreamPort = streamPort

	wdaStreamPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent stream port for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.WDAStreamPort = wdaStreamPort

	// Forward the WebDriverAgent server and stream to the host
	go goIOSForward(device, device.WDAPort, "8100")
	go goIOSForward(device, device.StreamPort, "9500")
	go goIOSForward(device, device.WDAStreamPort, "9100")

	// TODO - finalize this when we can use go-ios to start tests anywhere
	//if config.Config.EnvConfig.UseGadsIosStream {
	//	err = startGadsIosBroadcastViaXCTestGoIOS(device)
	//	if err != nil {
	//		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not start GADS broadcast with XCTest on device `%s` - %s", device.UDID, err))
	//		resetLocalDevice(device)
	//		return
	//	}
	//}

	// If on Linux or Windows use the prebuilt and provided WebDriverAgent.ipa/app file
	if config.Config.EnvConfig.OS != "darwin" {
		wdaPath := fmt.Sprintf("%s/%s", config.Config.EnvConfig.ProviderFolder, config.Config.EnvConfig.WebDriverBinary)
		err = installAppWithPathIOS(device, wdaPath)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not install WebDriverAgent on device `%s` - %s", device.UDID, err))
			resetLocalDevice(device)
			return
		}
		go startWdaWithGoIOS(device)
	} else {
		go startWdaWithXcodebuild(device)
	}
	// Wait until WebDriverAgent successfully starts
	select {
	case <-device.WdaReadyChan:
		logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Successfully started WebDriverAgent for device `%v` forwarded on port %v", device.UDID, device.WDAPort))
		break
	case <-time.After(30 * time.Second):
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully start WebDriverAgent on device `%v` in 30 seconds", device.UDID))
		resetLocalDevice(device)
		return
	}

	// Create a WebDriverAgent session and update the MJPEG stream settings
	err = updateWebDriverAgent(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully create WebDriverAgent session or update its stream settings for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	go startAppium(device)
	if config.Config.EnvConfig.UseSeleniumGrid {
		go startGridNode(device)
	}

	device.InstalledApps = getInstalledAppsIOS(device)

	// Mark the device as 'live'
	device.ProviderState = "live"
}

// Gets all connected iOS and Android devices to the host
func GetConnectedDevicesCommon() []models.ConnectedDevice {
	var connectedDevices []models.ConnectedDevice

	var androidDevices []models.ConnectedDevice
	var iosDevices []models.ConnectedDevice

	if config.Config.EnvConfig.ProvideAndroid {
		androidDevices = getConnectedDevicesAndroid()
	}

	if config.Config.EnvConfig.ProvideIOS {
		iosDevices = getConnectedDevicesIOS()
	}

	connectedDevices = append(connectedDevices, iosDevices...)
	connectedDevices = append(connectedDevices, androidDevices...)

	return connectedDevices
}

// Gets the connected iOS devices using the `go-ios` library
func getConnectedDevicesIOS() []models.ConnectedDevice {
	var connectedDevices []models.ConnectedDevice

	deviceList, err := ios.ListDevices()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("getConnectedDevicesIOS: Could not get connected devices with `go-ios` library, returning empty slice - %s", err))
		return connectedDevices
	}

	for _, connDevice := range deviceList.DeviceList {
		connectedDevices = append(connectedDevices, models.ConnectedDevice{OS: "ios", UDID: connDevice.Properties.SerialNumber})
	}
	return connectedDevices
}

// Gets the connected android devices using `adb`
func getConnectedDevicesAndroid() []models.ConnectedDevice {
	var connectedDevices []models.ConnectedDevice

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
			connectedDevices = append(connectedDevices, models.ConnectedDevice{OS: "android", UDID: strings.Fields(line)[0]})
		}
	}

	err = cmd.Wait()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("getConnectedDevicesAndroid: Waiting for `%s` command to finish failed, returning empty slice - %s", cmd.Args, err))
		return []models.ConnectedDevice{}
	}

	return connectedDevices
}

func resetLocalDevice(device *models.Device) {
	if !device.IsResetting && device.ProviderState != "init" {
		logger.ProviderLogger.LogInfo("provider", fmt.Sprintf("Resetting LocalDevice for device `%v` after error. Cancelling context, setting ProviderState to `init`, Healthy to `false` and updating the DB", device.UDID))

		device.IsResetting = true
		device.CtxCancel()
		device.ProviderState = "init"
		device.IsResetting = false

		// Free any used ports from the map where we keep them
		delete(providerutil.UsedPorts, device.WDAPort)
		delete(providerutil.UsedPorts, device.StreamPort)
		delete(providerutil.UsedPorts, device.AppiumPort)
		delete(providerutil.UsedPorts, device.WDAStreamPort)
	}
}

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func setContext(device *models.Device) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	device.CtxCancel = cancelFunc
	device.Context = ctx
}

func startAppium(device *models.Device) {
	var capabilities models.AppiumServerCapabilities

	if device.OS == "ios" {
		capabilities = models.AppiumServerCapabilities{
			UDID:                  device.UDID,
			WdaURL:                "http://localhost:" + device.WDAPort,
			WdaMjpegPort:          device.WDAStreamPort,
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

	// Get a free port on the host for Appium server
	appiumPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startAppium: Could not allocate free Appium host port for device - %v, err - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.AppiumPort = appiumPort

	cmd := exec.CommandContext(device.Context, "appium", "-p", device.AppiumPort, "--log-timestamp", "--session-override", "--log-no-colors", "--default-capabilities", string(capabilitiesJson))

	logger.ProviderLogger.LogDebug("device_setup", fmt.Sprintf("Starting Appium on device `%s` with command `%s`", device.UDID, cmd.Args))
	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startAppium: Error creating stdoutpipe on `%s` for device `%v` - %v", cmd.Args, device.UDID, err))
		resetLocalDevice(device)
		return
	}

	err = cmd.Start()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startAppium: Error executing `%s` for device `%v` - %v", cmd.Args, device.UDID, err))
		resetLocalDevice(device)
		return
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		device.AppiumLogger.Log(device, line)
	}

	err = cmd.Wait()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startAppium: Error waiting for `%s` command to finish, it errored out or device `%v` was disconnected - %v", cmd.Args, device.UDID, err))
		resetLocalDevice(device)
	}
}

func createGridTOML(device *models.Device) error {
	automationName := ""
	if device.OS == "ios" {
		automationName = "XCUITest"
	} else {
		automationName = "UiAutomator2"
	}

	url := fmt.Sprintf("http://%s:%v/device/%s/appium", config.Config.EnvConfig.HostAddress, config.Config.EnvConfig.Port, device.UDID)
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

	file, err := os.Create(fmt.Sprintf("%s/%s.toml", config.Config.EnvConfig.ProviderFolder, device.UDID))
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
		fmt.Sprintf("%s/%s", config.Config.EnvConfig.ProviderFolder, config.Config.EnvConfig.SeleniumJarFile),
		"node",
		"--host",
		config.Config.EnvConfig.HostAddress,
		"--config",
		fmt.Sprintf("%s/%s.toml", config.Config.EnvConfig.ProviderFolder, device.UDID),
		"--grid-url",
		config.Config.EnvConfig.SeleniumGrid,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while starting Selenium Grid node for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start Selenium Grid node for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		device.Logger.LogDebug("grid-node", strings.TrimSpace(line))
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Selenium Grid node command to finish, it errored out or device `%v` was disconnected - %v", device.UDID, err))
		resetLocalDevice(device)
	}
}

func updateScreenSize(device *models.Device) error {
	if device.OS == "ios" {
		if dimensions, ok := constants.IOSDeviceInfoMap[device.IOSProductType]; ok {
			device.ScreenHeight = dimensions.Height
			device.ScreenWidth = dimensions.Width
		} else {
			return fmt.Errorf("could not find `%s` hardware model in the IOSDeviceDimensions map, please update the map", device.HardwareModel)
		}
	} else {
		err := updateAndroidScreenSizeADB(device)
		if err != nil {
			return err
		}
	}

	return nil
}

func getModel(device *models.Device) {
	if device.OS == "ios" {
		if info, ok := constants.IOSDeviceInfoMap[device.IOSProductType]; ok {
			device.Model = info.Model
		} else {
			device.Model = "Unknown iOS device"
		}
	} else {
		brandCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "getprop", "ro.product.brand")
		var outBuffer bytes.Buffer
		brandCmd.Stdout = &outBuffer
		if err := brandCmd.Run(); err != nil {
			device.Model = "Unknown brand and model"
		}
		brand := outBuffer.String()
		outBuffer.Reset()

		modelCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "getprop", "ro.product.model")
		modelCmd.Stdout = &outBuffer
		if err := modelCmd.Run(); err != nil {
			device.Model = "Unknown brand/model"
			return
		}
		model := outBuffer.String()

		device.Model = fmt.Sprintf("%s %s", strings.TrimSpace(brand), strings.TrimSpace(model))
	}
}

func getAndroidOSVersion(device *models.Device) {
	if device.OS == "ios" {

	} else {
		sdkCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "getprop", "ro.build.version.sdk")
		var outBuffer bytes.Buffer
		sdkCmd.Stdout = &outBuffer
		if err := sdkCmd.Run(); err != nil {
			device.OSVersion = "N/A"
		}
		sdkVersion := strings.TrimSpace(outBuffer.String())
		if osVersion, ok := constants.AndroidVersionToSDK[sdkVersion]; ok {
			device.OSVersion = osVersion
		} else {
			device.OSVersion = "N/A"
		}
	}
}

func UpdateInstalledApps(device *models.Device) {
	if device.OS == "ios" {
		device.InstalledApps = getInstalledAppsIOS(device)
	} else {
		device.InstalledApps = getInstalledAppsAndroid(device)
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
		err := installAppIOS(device, app)
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
