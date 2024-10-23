package devices

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"GADS/common/constants"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/forward"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/testmanagerd"
	"github.com/danielpaulus/go-ios/ios/tunnel"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
)

func goIosForward(device *models.Device, hostPort string, devicePort string) {
	hostPortInt, _ := strconv.Atoi(hostPort)
	devicePortInt, _ := strconv.Atoi(devicePort)

	cl, err := forward.Forward(device.GoIOSDeviceEntry, uint16(hostPortInt), uint16(devicePortInt))
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to forward device port %s to host port %s for device `%s` - %s", devicePort, hostPort, device.UDID, err))
		resetLocalDevice(device)
		return
	}

	// Close the forward connection if device context is done
	select {
	case <-device.Context.Done():
		cl.Close()
		return
	}
}

// Start the prebuilt WebDriverAgent with `xcodebuild`
func startWdaWithXcodebuild(device *models.Device) {
	cmd := exec.CommandContext(device.Context, "xcodebuild",
		"-project", "WebDriverAgent.xcodeproj",
		"-scheme", "WebDriverAgentRunner",
		"-destination", "platform=iOS,id="+device.UDID,
		"-derivedDataPath", "./build",
		"test-without-building")
	cmd.Dir = config.ProviderConfig.WdaRepoPath
	logger.ProviderLogger.LogDebug("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Starting WebDriverAgent with command `%v`", cmd.Args))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		device.Logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Error creating stdoutpipe while running WebDriverAgent with xcodebuild for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if err := cmd.Start(); err != nil {
		device.Logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Could not start WebDriverAgent with xcodebuild for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()

		//device.Logger.LogInfo("webdriveragent", strings.TrimSpace(line))

		if strings.Contains(line, "Restarting after") {
			resetLocalDevice(device)
			return
		}
	}

	if err := cmd.Wait(); err != nil {
		device.Logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Error waiting for WebDriverAgent(xcodebuild) command to finish, it errored out or device `%v` was disconnected - %v", device.UDID, err))
		resetLocalDevice(device)
	}
}

// Create a new WebDriverAgent session and update stream settings
func updateWebDriverAgent(device *models.Device) error {
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Updating WebDriverAgent session and mjpeg stream settings for device `%s`", device.UDID))

	err := createWebDriverAgentSession(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Could not create WebDriverAgent session for device %v - %v", device.UDID, err))
		return err
	}

	err = UpdateWebDriverAgentStreamSettings(device, 15, 75, 100, true)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Could not update WebDriverAgent stream settings for device %v - %v", device.UDID, err))
		return err
	}

	return nil
}

func UpdateWebDriverAgentStreamSettings(device *models.Device, targetFramerate int, screenshotQuality int, scalingFactor int, useWDA bool) error {
	var mjpegProperties models.WDAMjpegProperties
	if targetFramerate != 0 {
		mjpegProperties.MjpegServerFramerate = targetFramerate
	}
	if screenshotQuality != 0 {
		mjpegProperties.MjpegServerScreenshotQuality = screenshotQuality
	}
	if scalingFactor != 0 {
		mjpegProperties.MjpegServerScalingFactor = scalingFactor
	}
	mjpegSettings := models.WDAMjpegSettings{
		Settings: mjpegProperties,
	}

	// Marshal the struct to JSON
	requestBody, err := json.Marshal(mjpegSettings)
	if err != nil {
		return err
	}

	fmt.Println("Sending to http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/appium/settings")
	fmt.Println(bytes.NewBuffer(requestBody))

	var url string
	if useWDA {
		url = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/appium/settings"
	} else {
		url = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/appium/settings"
	}

	// Post the mjpeg server settings
	response, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}

	// TODO - potentially read the body to supply in the error
	if response.StatusCode != 200 {
		return fmt.Errorf("updateWebDriverAgentStreamSettings: Could not successfully update WDA stream settings, status code=%v", response.StatusCode)
	}

	return nil
}

// Create a new WebDriverAgent session
func createWebDriverAgentSession(device *models.Device) error {
	requestString := `{
		"capabilities": {
			"firstMatch": [{}],
			"alwaysMatch": {
				
			}
		}
	}`

	req, err := http.NewRequest(http.MethodPost, "http://localhost:"+device.WDAPort+"/session", strings.NewReader(requestString))
	if err != nil {
		return err
	}

	response, err := netClient.Do(req)
	if err != nil {
		return err
	}

	// Get the response into a byte slice
	responseBody, _ := io.ReadAll(response.Body)
	// Unmarshal response into a basic map
	var responseJson map[string]interface{}
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		return err
	}

	// Check the session ID from the map
	if responseJson["sessionId"] == "" {
		if err != nil {
			return fmt.Errorf("createWebDriverAgentSession: Could not get `sessionId` while creating a new WebDriverAgent session")
		}
	}

	device.WDASessionID = fmt.Sprintf("%v", responseJson["sessionId"])
	return nil
}

func mountDeveloperImageIOS(device *models.Device) {
	basedir := fmt.Sprintf("%s/devimages", config.ProviderConfig.ProviderFolder)

	path, err := imagemounter.DownloadImageFor(device.GoIOSDeviceEntry, basedir)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to download DDI for device `%s` to path `%s` - %s", device.UDID, basedir, err))
		resetLocalDevice(device)
		return
	}

	err = imagemounter.MountImage(device.GoIOSDeviceEntry, path)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to mount DDI on device `%s` from path `%s` - %s", device.UDID, path, err))
		resetLocalDevice(device)
	}
}

// Pair an iOS device with host with/without supervision
func pairIOS(device *models.Device) error {
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Pairing device `%s`", device.UDID))

	p12, err := os.ReadFile(fmt.Sprintf("%s/supervision.p12", config.ProviderConfig.ProviderFolder))
	if err != nil {
		logger.ProviderLogger.LogWarn("ios_device_setup", fmt.Sprintf("Could not read supervision.p12 file when pairing device with UDID: %s, falling back to unsupervised pairing - %s", device.UDID, err))
		err = ios.Pair(device.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("Could not perform unsupervised pairing successfully - %s", err)
		}
		return nil
	}

	err = ios.PairSupervised(device.GoIOSDeviceEntry, p12, config.ProviderConfig.SupervisionPassword)
	if err != nil {
		return fmt.Errorf("Could not perform supervised pairing successfully - %s", err)
	}

	return nil
}

func GetInstalledAppsIOS(device *models.Device) []string {
	var installedApps []string
	svc, err := installationproxy.New(device.GoIOSDeviceEntry)
	if err != nil {
		logger.ProviderLogger.LogError("get_installed_apps", fmt.Sprintf("Failed to create installation proxy connection for device `%s` when getting installed apps - %s", device.UDID, err))
		return installedApps
	}

	response, err := svc.BrowseUserApps()
	if err != nil {
		logger.ProviderLogger.LogError("get_installed_apps", fmt.Sprintf("Failed to get installed apsp for device `%s` - %s", device.UDID, err))
		return installedApps
	}

	for _, appInfo := range response {
		installedApps = append(installedApps, appInfo.CFBundleIdentifier)
	}

	return installedApps
}

func uninstallAppIOS(device *models.Device, bundleID string) error {
	svc, err := installationproxy.New(device.GoIOSDeviceEntry)
	if err != nil {
		device.Logger.LogError("uninstall_app", fmt.Sprintf("uninstallAppIOS: Failed creating installation proxy connection - %v", bundleID, err))
		return err
	}
	err = svc.Uninstall(bundleID)
	if err != nil {
		device.Logger.LogError("uninstall_app", fmt.Sprintf("uninstallAppIOS: Failed uninstalling app with bundleID `%s` - %v", bundleID, err))
		return err
	}

	return nil
}

func installAppDefaultPath(device *models.Device, appName string) error {
	appPath := fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName)

	return installAppIOS(device, appPath)
}

func installAppIOS(device *models.Device, appPath string) error {
	if config.ProviderConfig.OS == "windows" {
		appPath = strings.TrimPrefix(appPath, "./")
	}

	logger.ProviderLogger.LogInfo("install_app_ios", fmt.Sprintf("Attempting to install app `%s` on device `%s`", appPath, device.UDID))
	conn, err := zipconduit.New(device.GoIOSDeviceEntry)
	if err != nil {
		logger.ProviderLogger.LogInfo("install_app_ios", fmt.Sprintf("Failed to create zipconduit connection when installing app `%s` on device `%s`", appPath, device.UDID))
		return err
	}
	err = conn.SendFile(appPath)

	return nil
}

func checkWebDriverAgentUp(device *models.Device) {
	var netClient = &http.Client{
		Timeout: time.Second * 30,
	}

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%v/status", device.WDAPort), nil)

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
				device.WdaReadyChan <- true
				return
			}
		}
		loops++
	}
}

// Only for iOS 17.4+
func createGoIOSTunnel(ctx context.Context, device *models.Device) (tunnel.Tunnel, error) {
	tun, err := tunnel.ConnectUserSpaceTunnelLockdown(device.GoIOSDeviceEntry, device.GoIOSDeviceEntry.UserspaceTUNPort)
	tun.UserspaceTUN = true

	tun.UserspaceTUNPort = device.GoIOSDeviceEntry.UserspaceTUNPort
	return tun, err
}

func goIosDeviceWithRsdProvider(device *models.Device) error {
	var err error
	rsdService, err := ios.NewWithAddrPort(device.GoIOSTunnel.Address, device.GoIOSTunnel.RsdPort, device.GoIOSDeviceEntry)
	if err != nil {
		return err
	}
	defer rsdService.Close()
	rsdProvider, err := rsdService.Handshake()
	if err != nil {
		return err
	}
	newEntry, err := ios.GetDeviceWithAddress(device.UDID, device.GoIOSTunnel.Address, rsdProvider)
	newEntry.UserspaceTUN = device.GoIOSDeviceEntry.UserspaceTUN
	newEntry.UserspaceTUNPort = device.GoIOSDeviceEntry.UserspaceTUNPort
	device.GoIOSDeviceEntry = newEntry
	if err != nil {
		return err
	}

	return nil
}

func runWDAGoIOS(device *models.Device) {
	_, err := testmanagerd.RunXCUITest(
		config.ProviderConfig.WdaBundleID,
		config.ProviderConfig.WdaBundleID,
		"WebDriverAgentRunner.xctest",
		device.GoIOSDeviceEntry,
		nil,
		nil,
		nil,
		testmanagerd.NewTestListener(io.Discard, io.Discard, os.TempDir()))
	if err != nil {
		resetLocalDevice(device)
	}
}

func updateIOSScreenSize(device *models.Device, deviceMachineCode string) error {
	if dimensions, ok := constants.IOSDeviceInfoMap[deviceMachineCode]; ok {
		device.ScreenHeight = dimensions.Height
		device.ScreenWidth = dimensions.Width
	} else {
		return fmt.Errorf("Could not find `%s` device machine code in the IOSDeviceInfoMap map, please update the map", deviceMachineCode)
	}

	// Update the device with the new dimensions in the DB
	err := db.UpsertDeviceDB(device)
	if err != nil {
		return fmt.Errorf("Failed to update DB with new device dimensions - %s", err)
	}

	return nil
}
