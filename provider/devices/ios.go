package devices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	"github.com/danielpaulus/go-ios/ios/instruments"
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
		ResetLocalDevice(device, "Failed to forward device port to host port due to an error.")
		return
	}

	// Close the forward connection if device context is done
	select {
	case <-device.Context.Done():
		cl.Close()
		return
	}
}

// Create a new WebDriverAgent session and update stream settings
func updateWebDriverAgent(device *models.Device) error {
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Updating WebDriverAgent session and mjpeg stream settings for device `%s`", device.UDID))

	err := UpdateWebDriverAgentStreamSettings(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Could not update WebDriverAgent stream settings for device %v - %v", device.UDID, err))
		return err
	}

	return nil
}

func UpdateWebDriverAgentStreamSettings(device *models.Device) error {
	var mjpegProperties models.WDAMjpegProperties
	mjpegProperties.MjpegServerFramerate = device.StreamTargetFPS
	mjpegProperties.MjpegServerScreenshotQuality = device.StreamJpegQuality
	mjpegProperties.MjpegServerScalingFactor = device.StreamScalingFactor

	mjpegSettings := models.WDAMjpegSettings{
		Settings: mjpegProperties,
	}

	// Marshal the struct to JSON
	requestBody, err := json.Marshal(mjpegSettings)
	if err != nil {
		return err
	}

	fmt.Println("Updating Appium settings")
	fmt.Println(bytes.NewBuffer(requestBody))

	var url = fmt.Sprintf("http://localhost:%v/appium/settings", device.WDAPort)

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

func mountDeveloperImageIOS(device *models.Device) {
	basedir := fmt.Sprintf("%s/devimages", config.ProviderConfig.ProviderFolder)

	path, err := imagemounter.DownloadImageFor(device.GoIOSDeviceEntry, basedir)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to download DDI for device `%s` to path `%s` - %s", device.UDID, basedir, err))
		ResetLocalDevice(device, "Failed to download Developer Disk Image (DDI) for the device.")
		return
	}

	err = imagemounter.MountImage(device.GoIOSDeviceEntry, path)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to mount DDI on device `%s` from path `%s` - %s", device.UDID, path, err))
		ResetLocalDevice(device, "Failed to mount Developer Disk Image (DDI) on the device.")
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

	if config.ProviderConfig.SupervisionPassword == "" {
		logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Supervision profile exists but no password provided, falling back to unsupervised pairing for device `%s`", device.UDID))
		err = ios.Pair(device.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("Could not perform unsupervised pairing successfully - %s", err)
		}
		return nil
	}
	err = ios.PairSupervised(device.GoIOSDeviceEntry, p12, config.ProviderConfig.SupervisionPassword)
	if err != nil {
		logger.ProviderLogger.LogWarn("ios_device_setup", fmt.Sprintf("Failed to perform supervised pairing on device `%s`, device unsupervised or unknown error - %s. Falling back to unsupervised pairing", device.UDID, err))
		err = ios.Pair(device.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("Could not perform unsupervised pairing successfully - %s", err)
		}
		return nil
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
		ResetLocalDevice(device, "Failed to create zipconduit connection for app installation.")
		return err
	}
	err = conn.SendFile(appPath)

	return nil
}

func launchAppIOS(device *models.Device, bundleID string, killExisting bool) error {
	pControl, err := instruments.NewProcessControl(device.GoIOSDeviceEntry)
	if err != nil {
		return fmt.Errorf("launchAppIOS: Failed to initiate process control launching app with bundleID `$s` - %s", bundleID, err)
	}

	opts := map[string]any{}
	if killExisting {
		opts["KillExisting"] = 1
	}
	_, err = pControl.LaunchAppWithArgs(bundleID, nil, nil, opts)
	if err != nil {
		ResetLocalDevice(device, "Failed to launch app with bundleID due to process control error.")
		return fmt.Errorf("launchAppIOS: Failed to launch app with bundleID `%s` - %s", bundleID, err)
	}

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
			ResetLocalDevice(device, "WebDriverAgent did not respond within the expected time.")
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
	rsdService, err := ios.NewWithAddrPortDevice(device.GoIOSTunnel.Address, device.GoIOSTunnel.RsdPort, device.GoIOSDeviceEntry)
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
	testConfig := testmanagerd.TestConfig{
		BundleId:           config.ProviderConfig.WdaBundleID,
		TestRunnerBundleId: config.ProviderConfig.WdaBundleID,
		XctestConfigName:   "WebDriverAgentRunner.xctest",
		Env:                nil,
		Args:               nil,
		TestsToRun:         nil,
		TestsToSkip:        nil,
		XcTest:             false,
		Device:             device.GoIOSDeviceEntry,
		Listener:           testmanagerd.NewTestListener(io.Discard, io.Discard, os.TempDir()),
	}
	_, err := testmanagerd.RunTestWithConfig(
		context.Background(),
		testConfig)
	if err != nil {
		ResetLocalDevice(device, "Failed to run WebDriverAgent due to an error.")
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
