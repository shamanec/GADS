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

	"github.com/danielpaulus/go-ios/ios"
	"github.com/pelletier/go-toml/v2"

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
var DBDeviceMap = make(map[string]*models.Device)

func Listener() {
	Setup()
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
		if updateFailureCounter >= 10 {
			log.Fatalf("Unsuccessfully attempted to update device data in hub for 10 times, killing provider")
		}
		time.Sleep(1 * time.Second)

		mu.Lock()

		var properJson models.ProviderData
		for _, dbDevice := range DBDeviceMap {
			properJson.DeviceData = append(properJson.DeviceData, *dbDevice)
			properJson.ProviderData = *config.ProviderConfig
		}
		mu.Unlock()
		jsonData, err := json.Marshal(properJson)
		if err != nil {
			logger.ProviderLogger.LogError("update_provider_hub", "Failed marshaling provider data to json - "+err.Error())
			continue
		}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/provider-update", config.ProviderConfig.HubAddress), bytes.NewBuffer(jsonData))
		if err != nil {
			logger.ProviderLogger.LogError("update_provider_hub", "Failed to create request to update provider data in hub - "+err.Error())
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

func setupDevices() {
	for _, dbDevice := range DBDeviceMap {
		dbDevice.ProviderState = "init"
		dbDevice.Connected = false
		dbDevice.LastUpdatedTimestamp = 0
		dbDevice.IsResetting = false

		dbDevice.Host = fmt.Sprintf("%s:%v", config.ProviderConfig.HostAddress, config.ProviderConfig.Port)

		// Check if a capped Appium logs collection already exists for the current device
		exists, err := db.CollectionExists("appium_logs", dbDevice.UDID)
		if err != nil {
			logger.ProviderLogger.Warnf("Could not check if device collection exists in `appium_logs` db, will attempt to create it either way - %s", err)
		}

		// If it doesn't exist - attempt to create it
		if !exists {
			err = db.CreateCappedCollection("appium_logs", dbDevice.UDID, 30000, 30)
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
		db.AddCollectionIndex("appium_logs", dbDevice.UDID, appiumCollectionIndexModel)

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
	}
}

func updateDevices() {
	ticker := time.NewTicker(5 * time.Second)
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
					if dbDevice.OS == "ios" {
						dbDevice.WdaReadyChan = make(chan bool, 1)
						go setupIOSDevice(dbDevice)
					}

					if dbDevice.OS == "android" {
						go setupAndroidDevice(dbDevice)
					}
				}
			} else {
				dbDevice.ProviderState = "init"
				dbDevice.IsResetting = false
				dbDevice.Connected = false
			}
		}
	}
}

func Setup() {
	if config.ProviderConfig.ProvideAndroid {
		err := providerutil.CheckGadsStreamAndDownload()
		if err != nil {
			log.Fatalf("Setup: Could not check availability of and download GADS-stream latest release - %s", err)
		}
	}
}

func setupAndroidDevice(device *models.Device) {
	device.ProviderState = "preparing"

	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Running setup for device `%v`", device.UDID))

	// If Selenium Grid is used attempt to create a TOML file for the grid connection
	if config.ProviderConfig.UseSeleniumGrid {
		err := createGridTOML(device)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Selenium Grid use is enabled but couldn't create TOML for device `%s` - %s", device.UDID, err))
			resetLocalDevice(device)
			return
		}
	}
	getAndroidDeviceHardwareModel(device)
	if device.OSVersion == "" {
		getAndroidOSVersion(device)
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
	if config.ProviderConfig.UseSeleniumGrid {
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
	if device.OSVersion == "" {
		device.OSVersion = plistValues["ProductVersion"].(string)
	}

	isAboveIOS17 := isAboveIOS17(device)
	if err != nil {
		device.Logger.LogError("ios_device_setup", fmt.Sprintf("Could not determine if device `%v` is above iOS 17 - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if isAboveIOS17 && config.ProviderConfig.OS != "darwin" {
		logger.ProviderLogger.LogInfo("ios_device_setup", "Device `%s` is iOS 17+ which is not supported on Windows/Linux, setup will be skipped")
		device.ProviderState = "init"
		return
	}

	// If Selenium Grid is used attempt to create a TOML file for the grid connection
	if config.ProviderConfig.UseSeleniumGrid {
		err := createGridTOML(device)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Selenium Grid use is enabled but couldn't create TOML for device `%s` - %s", device.UDID, err))
			resetLocalDevice(device)
			return
		}
	}

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

	// If on Linux or Windows use the prebuilt and provided WebDriverAgent.ipa/app file
	if config.ProviderConfig.OS != "darwin" {
		wdaPath := fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, config.ProviderConfig.WebDriverBinary)
		err = installAppIOS(device, wdaPath)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not install WebDriverAgent on device `%s` - %s", device.UDID, err))
			resetLocalDevice(device)
			return
		}
		go startXCTestWithGoIOS(device, config.ProviderConfig.WdaBundleID, "WebDriverAgentRunner.xctest")
	} else {
		if !isAboveIOS17 {
			wdaRepoPath := strings.TrimSuffix(config.ProviderConfig.WdaRepoPath, "/")
			wdaPath := fmt.Sprintf("%s/build/Build/Products/Debug-iphoneos/WebDriverAgentRunner-Runner.app", wdaRepoPath)
			err = installAppIOS(device, wdaPath)
			if err != nil {
				logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not install WebDriverAgent on device `%s` - %s", device.UDID, err))
				resetLocalDevice(device)
				return
			}
			go startXCTestWithGoIOS(device, config.ProviderConfig.WdaBundleID, "WebDriverAgentRunner.xctest")
		} else {
			go startWdaWithXcodebuild(device)
		}
	}

	go checkWebDriverAgentUp(device)

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
	if config.ProviderConfig.UseSeleniumGrid {
		go startGridNode(device)
	}

	device.InstalledApps = getInstalledAppsIOS(device)

	// Mark the device as 'live'
	device.ProviderState = "live"
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
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("getConnectedDevicesIOS: Could not get connected devices with `go-ios` library, returning empty slice - %s", err))
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

func getAndroidOSVersion(device *models.Device) {
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
