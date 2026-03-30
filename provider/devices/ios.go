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
	"GADS/provider/providerutil"

	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/forward"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/testmanagerd"
	"github.com/danielpaulus/go-ios/ios/tunnel"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	"golang.org/x/sync/errgroup"
)

// IOSDevice holds iOS-specific runtime state alongside the shared RuntimeState.
type IOSDevice struct {
	RuntimeState
	WDAPort          string          // host port for WebDriverAgent server (device port 8100)
	WDAStreamPort    string          // host port for WebDriverAgent MJPEG stream (device port 9100)
	StreamPort       string          // host port for device video stream (device port 8765)
	WDASessionID     string          // current WebDriverAgent session ID
	GoIOSDeviceEntry ios.DeviceEntry // go-ios library device entry for USB communication
	GoIOSTunnel      tunnel.Tunnel   // userspace tunnel for iOS 17.4+
	WdaReadyChan     chan bool       // signals WebDriverAgent is up after start
}

// Setup runs the full iOS device provisioning sequence.
func (d *IOSDevice) Setup() error {
	d.DBDevice.SetupMutex.Lock()
	defer d.DBDevice.SetupMutex.Unlock()

	d.SetProviderState("preparing")
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", d.GetUDID()))

	// Get go-ios DeviceEntry
	goIosDeviceEntry, err := ios.GetDevice(d.GetUDID())
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not get `go-ios` DeviceEntry for device - %v, err - %v", d.GetUDID(), err))
		d.Reset("Failed to get `go-ios` DeviceEntry for device.")
		return err
	}
	d.GoIOSDeviceEntry = goIosDeviceEntry
	d.DBDevice.GoIOSDeviceEntry = goIosDeviceEntry

	// Pair device
	if err := d.pair(); err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to pair device `%s` - %v", d.GetUDID(), err))
		d.Reset("Failed to pair device.")
		return err
	}

	// Check developer mode for iOS 16+
	if d.DBDevice.SemVer.Major() >= 16 {
		devModeEnabled, err := imagemounter.IsDevModeEnabled(d.GoIOSDeviceEntry)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not check developer mode status on device `%s` - %s", d.GetUDID(), err))
			d.Reset("Failed to check developer mode status on device.")
			return err
		}
		if !devModeEnabled {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Device `%s` is iOS 16+ but developer mode is not enabled!", d.GetUDID()))
			d.Reset("Device is iOS 16+ but developer mode is not enabled.")
			return fmt.Errorf("developer mode not enabled")
		}
	}

	// Mount DDI
	if err := d.mountDeveloperImage(); err != nil {
		d.Reset("Failed to mount Developer Disk Image (DDI) on the device.")
		return err
	}

	// Get device info
	plistValues, err := ios.GetValuesPlist(d.GoIOSDeviceEntry)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not get info plist values with go-ios `%v` - %v", d.GetUDID(), err))
		d.Reset("Failed to get info plist values with go-ios.")
		return err
	}
	d.DBDevice.HardwareModel = plistValues["HardwareModel"].(string)

	if d.DBDevice.ScreenHeight == "" || d.DBDevice.ScreenWidth == "" {
		if err := d.updateScreenSize(plistValues["ProductType"].(string)); err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to update screen dimensions for device `%s` - %s", d.GetUDID(), err))
			d.Reset("Failed to update screen dimensions for device.")
			return err
		}
	}

	// Allocate tunnel port
	tunnelPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", d.GetUDID(), err))
		d.Reset("Failed to allocate free WebDriverAgent port for device.")
		return err
	}
	intTunnelPort, _ := strconv.Atoi(tunnelPort)
	d.GoIOSDeviceEntry.UserspaceTUNPort = intTunnelPort

	// Create userspace tunnel for iOS 17.4+
	if d.DBDevice.SemVer.Compare(semver.MustParse("17.4.0")) >= 0 {
		deviceTunnel, err := d.createTunnel()
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to create userspace tunnel for device `%s` - %v", d.GetUDID(), err))
			d.Reset("Failed to create userspace tunnel for device.")
			return err
		}
		d.GoIOSTunnel = deviceTunnel
		d.DBDevice.GoIOSTunnel = deviceTunnel

		d.GoIOSDeviceEntry.UserspaceTUNPort = d.GoIOSTunnel.UserspaceTUNPort
		d.GoIOSDeviceEntry.UserspaceTUN = d.GoIOSTunnel.UserspaceTUN

		if err := d.deviceWithRsdProvider(); err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to create go-ios device entry with rsd provider for device `%s` - %v", d.GetUDID(), err))
			d.Reset("Failed to create go-ios device entry with rsd provider for device.")
			return err
		}
	}

	time.Sleep(1 * time.Second)

	// Disable memory limit for broadcast extension
	if d.DBDevice.StreamType == models.IOSWebRTCBroadcastExtensionId {
		pid, err := d.getProcessPid("gads-broadcast-extension")
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to get pid for GADS broadcast extension process on device `%s` - %s", d.GetUDID(), err))
		} else {
			if err := d.disableProcessMemoryLimit(pid); err != nil {
				logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to disable memory limit for GADS broadcast extension process on device `%s` - %s", d.GetUDID(), err))
			}
		}
	}

	// Allocate ports
	wdaPort, err := providerutil.GetFreePort()
	if err != nil {
		d.Reset("Failed to allocate free WebDriverAgent port for device.")
		return err
	}
	d.WDAPort = wdaPort
	d.DBDevice.WDAPort = wdaPort

	streamPort, err := providerutil.GetFreePort()
	if err != nil {
		d.Reset("Failed to allocate free iOS stream port for device.")
		return err
	}
	d.StreamPort = streamPort
	d.DBDevice.StreamPort = streamPort

	wdaStreamPort, err := providerutil.GetFreePort()
	if err != nil {
		d.Reset("Failed to allocate free WebDriverAgent stream port for device.")
		return err
	}
	d.WDAStreamPort = wdaStreamPort
	d.DBDevice.WDAStreamPort = wdaStreamPort

	// Forward ports
	go d.goIosForward(d.WDAPort, "8100")
	go d.goIosForward(d.StreamPort, "8765")
	go d.goIosForward(d.WDAStreamPort, "9100")

	// Install/launch WDA
	if d.DBDevice.SemVer.Major() < 17 || d.DBDevice.SemVer.Compare(semver.MustParse("17.4.0")) >= 0 {
		if err := d.installApp(fmt.Sprintf("%s/WebDriverAgent.ipa", config.ProviderConfig.ProviderFolder)); err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not install WebDriverAgent on device `%s` - %s", d.GetUDID(), err))
			d.Reset("Failed to install WebDriverAgent on device.")
			return err
		}
		go d.runWDA()
	} else {
		if err := d.launchApp(config.ProviderConfig.WdaBundleID, true); err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not launch WebDriverAgent on device `%s` - %s", d.GetUDID(), err))
			d.Reset("Failed to launch WebDriverAgent on device.")
			return err
		}
	}

	go d.checkWebDriverAgentUp()

	select {
	case <-d.WdaReadyChan:
		logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Successfully started WebDriverAgent for device `%v` forwarded on port %v", d.GetUDID(), d.WDAPort))
	case <-time.After(60 * time.Second):
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully start WebDriverAgent on device `%v` in 60 seconds", d.GetUDID()))
		d.Reset("Failed to start WebDriverAgent on device.")
		return fmt.Errorf("WDA did not start in time")
	}

	// Apply stream settings
	if err := d.ApplyStreamSettings(); err != nil {
		d.Reset("Failed to apply device stream settings.")
		return err
	}

	if err := d.UpdateStreamSettingsOnDevice(); err != nil {
		d.Reset("Failed to create WebDriverAgent session or update its stream settings.")
		return err
	}

	// Setup Appium if configured
	if config.ProviderConfig.SetupAppiumServers {
		if err := setupAppiumForDevice(d); err != nil {
			return err
		}
	}

	d.DBDevice.InstalledApps = d.GetInstalledAppBundleIDs()
	d.SetProviderState("live")
	return nil
}

// AppiumCapabilities returns the iOS-specific Appium server capabilities.
func (d *IOSDevice) AppiumCapabilities() models.AppiumServerCapabilities {
	return models.AppiumServerCapabilities{
		UDID:                  d.GetUDID(),
		WdaURL:                "http://localhost:" + d.WDAPort,
		WdaLocalPort:          d.WDAPort,
		WdaLaunchTimeout:      "120000",
		WdaConnectionTimeout:  "240000",
		ClearSystemFiles:      "false",
		PreventWdaAttachments: "true",
		SimpleIsVisibleCheck:  "false",
		AutomationName:        "XCUITest",
		PlatformName:          "iOS",
		DeviceName:            d.DBDevice.Name,
	}
}

func (d *IOSDevice) goIosForward(hostPort string, devicePort string) {
	hostPortInt, _ := strconv.Atoi(hostPort)
	devicePortInt, _ := strconv.Atoi(devicePort)

	cl, err := forward.Forward(d.GoIOSDeviceEntry, uint16(hostPortInt), uint16(devicePortInt))
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to forward device port %s to host port %s for device `%s` - %s", devicePort, hostPort, d.GetUDID(), err))
		d.Reset("Failed to forward device port to host port due to an error.")
		return
	}

	select {
	case <-d.DBDevice.Context.Done():
		cl.Close()
		return
	}
}

// UpdateStreamSettingsOnDevice updates WebDriverAgent stream settings.
func (d *IOSDevice) UpdateStreamSettingsOnDevice() error {
	var mjpegProperties models.WDAMjpegProperties
	mjpegProperties.MjpegServerFramerate = d.DBDevice.StreamTargetFPS
	mjpegProperties.MjpegServerScreenshotQuality = d.DBDevice.StreamJpegQuality
	mjpegProperties.MjpegServerScalingFactor = d.DBDevice.StreamScalingFactor

	mjpegSettings := models.WDAMjpegSettings{Settings: mjpegProperties}
	requestBody, err := json.Marshal(mjpegSettings)
	if err != nil {
		return err
	}

	var url = fmt.Sprintf("http://localhost:%v/appium/settings", d.WDAPort)
	response, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("could not successfully update WDA stream settings, status code=%v", response.StatusCode)
	}
	return nil
}

func (d *IOSDevice) mountDeveloperImage() error {
	basedir := fmt.Sprintf("%s/devimages", config.ProviderConfig.ProviderFolder)

	path, err := imagemounter.DownloadImageFor(d.GoIOSDeviceEntry, basedir)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to download DDI for device `%s` to path `%s` - %s", d.GetUDID(), basedir, err))
		return fmt.Errorf("failed to download DDI: %w", err)
	}

	err = imagemounter.MountImage(d.GoIOSDeviceEntry, path)
	if err != nil {
		if strings.Contains(err.Error(), "already mounted") || strings.Contains(err.Error(), "AlreadyMounted") {
			return nil
		}
		return fmt.Errorf("failed to mount DDI: %w", err)
	}
	return nil
}

func (d *IOSDevice) pair() (pairErr error) {
	if config.ProviderConfig.UseIOSPairCache {
		if err := restorePairRecordToUsbmuxd(d.GetUDID()); err == nil {
			logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Restored cached pairing record for device `%s`, skipping pairing", d.GetUDID()))
			return nil
		}
	}

	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Pairing device `%s`", d.GetUDID()))

	defer func() {
		if pairErr == nil && config.ProviderConfig.UseIOSPairCache {
			cachePairRecord(d.GetUDID())
		}
	}()

	p12, err := os.ReadFile(fmt.Sprintf("%s/supervision.p12", config.ProviderConfig.ProviderFolder))
	if err != nil {
		logger.ProviderLogger.LogWarn("ios_device_setup", fmt.Sprintf("Could not read supervision.p12 file when pairing device with UDID: %s, falling back to unsupervised pairing - %s", d.GetUDID(), err))
		err = ios.Pair(d.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("Could not perform unsupervised pairing successfully - %s", err)
		}
		return nil
	}

	if config.ProviderConfig.SupervisionPassword == "" {
		err = ios.Pair(d.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("Could not perform unsupervised pairing successfully - %s", err)
		}
		return nil
	}
	err = ios.PairSupervised(d.GoIOSDeviceEntry, p12, config.ProviderConfig.SupervisionPassword)
	if err != nil {
		logger.ProviderLogger.LogWarn("ios_device_setup", fmt.Sprintf("Failed to perform supervised pairing on device `%s`, falling back to unsupervised - %s", d.GetUDID(), err))
		err = ios.Pair(d.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("Could not perform unsupervised pairing successfully - %s", err)
		}
		return nil
	}
	return nil
}

func (d *IOSDevice) getAllApps() ([]installationproxy.AppInfo, error) {
	svc, err := installationproxy.New(d.GoIOSDeviceEntry)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to installation proxy for all apps: %w", err)
	}
	defer svc.Close()
	return svc.BrowseAllApps()
}

func (d *IOSDevice) getUserApps() ([]installationproxy.AppInfo, error) {
	svc, err := installationproxy.New(d.GoIOSDeviceEntry)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to installation proxy for user apps: %w", err)
	}
	defer svc.Close()
	return svc.BrowseUserApps()
}

// GetInstalledApps returns detailed info about installed apps.
func (d *IOSDevice) GetInstalledApps() ([]models.DeviceApp, error) {
	var installedApps = make([]models.DeviceApp, 0)
	var allApps, userApps []installationproxy.AppInfo

	g, _ := errgroup.WithContext(context.Background())
	g.Go(func() error {
		var err error
		allApps, err = d.getAllApps()
		return err
	})
	g.Go(func() error {
		var err error
		userApps, err = d.getUserApps()
		return err
	})
	if err := g.Wait(); err != nil {
		return installedApps, err
	}

	bundleIdToExecutable := make(map[string]string, len(allApps))
	for _, app := range allApps {
		bundleIdToExecutable[app.CFBundleIdentifier()] = app.CFBundleExecutable()
	}

	for _, userApp := range userApps {
		if !strings.Contains(userApp.CFBundleExecutable(), "WebDriverAgentRunner") && !strings.Contains(userApp.CFBundleExecutable(), "h264-broadcast-extension") {
			installedApps = append(installedApps, models.DeviceApp{AppName: userApp.CFBundleExecutable(), BundleIdentifier: userApp.CFBundleIdentifier(), CanUninstall: true})
		}
	}

	for _, bundleId := range constants.IOSSystemAppsBundleIds {
		appName := bundleIdToExecutable[bundleId]
		if appName == "" {
			appName = "Unknown name"
		}
		installedApps = append(installedApps, models.DeviceApp{AppName: appName, BundleIdentifier: bundleId, CanUninstall: false})
	}

	return installedApps, nil
}

// GetInstalledAppBundleIDs returns the bundle identifiers of all installed apps.
func (d *IOSDevice) GetInstalledAppBundleIDs() []string {
	var bundleIdentifiers = make([]string, 0)
	installedAppsInfo, err := d.GetInstalledApps()
	if err != nil {
		return bundleIdentifiers
	}
	for _, installedApp := range installedAppsInfo {
		bundleIdentifiers = append(bundleIdentifiers, installedApp.BundleIdentifier)
	}
	return bundleIdentifiers
}

// UninstallApp uninstalls an app by bundle ID.
func (d *IOSDevice) UninstallApp(bundleID string) error {
	svc, err := installationproxy.New(d.GoIOSDeviceEntry)
	if err != nil {
		return fmt.Errorf("failed creating installation proxy connection - %v", err)
	}
	return svc.Uninstall(bundleID)
}

// InstallApp installs an app from a file in the provider folder.
func (d *IOSDevice) InstallApp(appName string) error {
	appPath := fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName)
	return d.installApp(appPath)
}

func (d *IOSDevice) installApp(appPath string) error {
	if config.ProviderConfig.OS == "windows" {
		appPath = strings.TrimPrefix(appPath, "./")
	}

	logger.ProviderLogger.LogInfo("install_app_ios", fmt.Sprintf("Attempting to install app `%s` on device `%s`", appPath, d.GetUDID()))
	conn, err := zipconduit.New(d.GoIOSDeviceEntry)
	if err != nil {
		logger.ProviderLogger.LogInfo("install_app_ios", fmt.Sprintf("Failed to create zipconduit connection when installing app `%s` on device `%s`", appPath, d.GetUDID()))
		d.Reset("Failed to create zipconduit connection for app installation.")
		return err
	}
	conn.SendFile(appPath)
	return nil
}

func (d *IOSDevice) launchApp(bundleID string, killExisting bool) error {
	pControl, err := instruments.NewProcessControl(d.GoIOSDeviceEntry)
	if err != nil {
		return fmt.Errorf("failed to initiate process control - %s", err)
	}

	opts := map[string]any{}
	if killExisting {
		opts["KillExisting"] = 1
	}
	_, err = pControl.LaunchAppWithArgs(bundleID, nil, nil, opts)
	if err != nil {
		d.Reset("Failed to launch app with bundleID due to process control error.")
		return fmt.Errorf("failed to launch app with bundleID `%s` - %s", bundleID, err)
	}
	return nil
}

// LaunchApp launches an app by bundle ID (for the PlatformDevice interface).
func (d *IOSDevice) LaunchApp(bundleID string) error {
	return d.launchApp(bundleID, true)
}

func (d *IOSDevice) checkWebDriverAgentUp() {
	var netClient = &http.Client{Timeout: time.Second * 30}
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%v/status", d.WDAPort), nil)

	loops := 0
	for {
		if loops >= 30 {
			d.Reset("WebDriverAgent did not respond within the expected time.")
			return
		}
		resp, err := netClient.Do(req)
		if err != nil {
			time.Sleep(1 * time.Second)
		} else {
			if resp.StatusCode == http.StatusOK {
				d.WdaReadyChan <- true
				return
			}
		}
		loops++
	}
}

func (d *IOSDevice) createTunnel() (tunnel.Tunnel, error) {
	tun, err := tunnel.ConnectUserSpaceTunnelLockdown(d.GoIOSDeviceEntry, d.GoIOSDeviceEntry.UserspaceTUNPort)
	tun.UserspaceTUN = true
	tun.UserspaceTUNPort = d.GoIOSDeviceEntry.UserspaceTUNPort
	return tun, err
}

func (d *IOSDevice) deviceWithRsdProvider() error {
	rsdService, err := ios.NewWithAddrPortDevice(d.GoIOSTunnel.Address, d.GoIOSTunnel.RsdPort, d.GoIOSDeviceEntry)
	if err != nil {
		return err
	}
	defer rsdService.Close()
	rsdProvider, err := rsdService.Handshake()
	if err != nil {
		return err
	}
	newEntry, err := ios.GetDeviceWithAddress(d.GetUDID(), d.GoIOSTunnel.Address, rsdProvider)
	newEntry.UserspaceTUN = d.GoIOSDeviceEntry.UserspaceTUN
	newEntry.UserspaceTUNPort = d.GoIOSDeviceEntry.UserspaceTUNPort
	d.GoIOSDeviceEntry = newEntry
	d.DBDevice.GoIOSDeviceEntry = newEntry
	if err != nil {
		return err
	}
	return nil
}

func (d *IOSDevice) runWDA() {
	testConfig := testmanagerd.TestConfig{
		BundleId:           config.ProviderConfig.WdaBundleID,
		TestRunnerBundleId: config.ProviderConfig.WdaBundleID,
		XctestConfigName:   "WebDriverAgentRunner.xctest",
		Device:             d.GoIOSDeviceEntry,
		Listener:           testmanagerd.NewTestListener(io.Discard, io.Discard, os.TempDir()),
	}
	_, err := testmanagerd.RunTestWithConfig(d.DBDevice.Context, testConfig)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Failed to run WebDriverAgent via testmanagerd on device `%s` - %s", d.GetUDID(), err))
		d.Reset("Failed to run WebDriverAgent due to an error.")
	}
}

func (d *IOSDevice) updateScreenSize(deviceMachineCode string) error {
	if dimensions, ok := constants.IOSDeviceInfoMap[deviceMachineCode]; ok {
		d.DBDevice.ScreenHeight = dimensions.Height
		d.DBDevice.ScreenWidth = dimensions.Width
	} else {
		return fmt.Errorf("could not find `%s` device machine code in the IOSDeviceInfoMap map", deviceMachineCode)
	}

	if err := db.GlobalMongoStore.AddOrUpdateDevice(d.DBDevice); err != nil {
		return fmt.Errorf("failed to update DB with new device dimensions - %s", err)
	}
	return nil
}

func (d *IOSDevice) getProcessPid(processName string) (uint64, error) {
	svc, err := instruments.NewDeviceInfoService(d.GoIOSDeviceEntry)
	if err != nil {
		return 0, fmt.Errorf("failed to create device info service for device `%s`", d.GetUDID())
	}
	defer svc.Close()

	processList, err := svc.ProcessList()
	if err != nil {
		return 0, fmt.Errorf("failed to get process list for device `%s` - %s", d.GetUDID(), err)
	}

	for _, process := range processList {
		if process.Pid > 1 && process.Name == processName {
			return process.Pid, nil
		}
	}
	return 0, fmt.Errorf("no process with name `%s` found on device `%s`", processName, d.GetUDID())
}

func (d *IOSDevice) disableProcessMemoryLimit(pid uint64) error {
	pControl, err := instruments.NewProcessControl(d.GoIOSDeviceEntry)
	if err != nil {
		return fmt.Errorf("failed to create process control instance for device `%s` - %s", d.GetUDID(), err)
	}

	disabled, err := pControl.DisableMemoryLimit(pid)
	if err != nil {
		return fmt.Errorf("failed to disable memory limit for pid `%v` for device `%s` - %s", pid, d.GetUDID(), err)
	}
	if !disabled {
		return fmt.Errorf("failed to disable memory limit for pid `%v` for device `%s` without explicit error", pid, d.GetUDID())
	}
	return nil
}

// GetRunningApps returns a list of running apps on the device that are killable.
func (d *IOSDevice) GetRunningApps() ([]models.RunningApp, error) {
	var runningApps = make([]models.RunningApp, 0)

	var allApps, userApps []installationproxy.AppInfo
	var procList []instruments.ProcessInfo

	g, _ := errgroup.WithContext(context.Background())
	g.Go(func() error {
		svc, err := installationproxy.New(d.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("failed to connect to installation proxy for all apps: %w", err)
		}
		defer svc.Close()
		allApps, err = svc.BrowseAllApps()
		return err
	})
	g.Go(func() error {
		svc, err := installationproxy.New(d.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("failed to connect to installation proxy for user apps: %w", err)
		}
		defer svc.Close()
		userApps, err = svc.BrowseUserApps()
		return err
	})
	g.Go(func() error {
		svc, err := instruments.NewDeviceInfoService(d.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("failed to create device info service: %w", err)
		}
		defer svc.Close()
		procList, err = svc.ProcessList()
		return err
	})

	if err := g.Wait(); err != nil {
		return runningApps, err
	}

	execToBundleId := make(map[string]string, len(allApps))
	for _, app := range allApps {
		execToBundleId[app.CFBundleExecutable()] = app.CFBundleIdentifier()
	}

	appsAllowList := make(map[string]bool)
	for _, bundleId := range constants.IOSSystemAppsBundleIds {
		appsAllowList[bundleId] = true
	}
	for _, userApp := range userApps {
		if !strings.Contains(userApp.CFBundleExecutable(), "WebDriverAgentRunner") && !strings.Contains(userApp.CFBundleExecutable(), "h264-broadcast-extension") {
			appsAllowList[userApp.CFBundleIdentifier()] = true
		}
	}

	for _, proc := range procList {
		bundleID, found := execToBundleId[proc.Name]
		if !found {
			continue
		}
		if appsAllowList[bundleID] {
			runningApps = append(runningApps, models.RunningApp{AppName: proc.Name, BundleIdentifier: bundleID})
		}
	}

	return runningApps, nil
}

// KillApp kills a running app by bundle identifier.
func (d *IOSDevice) KillApp(bundleIdentifier string) error {
	var allApps []installationproxy.AppInfo
	var processList []instruments.ProcessInfo

	g, _ := errgroup.WithContext(context.Background())
	g.Go(func() error {
		svc, err := installationproxy.New(d.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("failed to connect to installation proxy: %w", err)
		}
		defer svc.Close()
		allApps, err = svc.BrowseAllApps()
		return err
	})
	g.Go(func() error {
		infoService, err := instruments.NewDeviceInfoService(d.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("failed to create device info service - %w", err)
		}
		defer infoService.Close()
		processList, err = infoService.ProcessList()
		return err
	})

	if err := g.Wait(); err != nil {
		return err
	}

	pControl, err := instruments.NewProcessControl(d.GoIOSDeviceEntry)
	if err != nil {
		return fmt.Errorf("failed to create process control service - %w", err)
	}
	defer pControl.Close()

	var appProcessName string
	for _, app := range allApps {
		if app.CFBundleIdentifier() == bundleIdentifier {
			appProcessName = app.CFBundleExecutable()
		}
	}
	if appProcessName == "" {
		return fmt.Errorf("app with bundle identifier `%s` is not installed on device", bundleIdentifier)
	}

	for _, p := range processList {
		if p.Name == appProcessName {
			return pControl.KillProcess(p.Pid)
		}
	}
	return fmt.Errorf("app with bundle id `%s` is not running", bundleIdentifier)
}

// GetScreenSize returns the device screen dimensions.
func (d *IOSDevice) GetScreenSize() (width, height string, err error) {
	return d.DBDevice.ScreenWidth, d.DBDevice.ScreenHeight, nil
}

// GetHardwareModel returns the hardware model string.
func (d *IOSDevice) GetHardwareModel() (string, error) {
	return d.DBDevice.HardwareModel, nil
}

// GetCurrentRotation returns the current device rotation (iOS uses WDA for this, handled by router).
func (d *IOSDevice) GetCurrentRotation() (string, error) {
	return d.DBDevice.CurrentRotation, nil
}

// ChangeRotation is handled via WDA in the router for iOS.
func (d *IOSDevice) ChangeRotation(rotation string) error {
	return fmt.Errorf("iOS rotation is handled via WebDriverAgent")
}

// ApplyStreamSettings applies stream settings from DB to the device runtime state.
func (d *IOSDevice) ApplyStreamSettings() error {
	return applyDeviceStreamSettings(d.DBDevice)
}

// --- Legacy exported functions used by the provider router ---

func GetInstalledAppsIOS(device *models.Device) []models.DeviceApp {
	dev, ok := DevManager.Get(device.UDID)
	if !ok {
		return []models.DeviceApp{}
	}
	iosDev, ok := dev.(*IOSDevice)
	if !ok {
		return []models.DeviceApp{}
	}
	apps, _ := iosDev.GetInstalledApps()
	return apps
}

func GetRunningAppsIOS(device *models.Device) ([]models.RunningApp, error) {
	dev, ok := DevManager.Get(device.UDID)
	if !ok {
		return nil, fmt.Errorf("device not found")
	}
	iosDev, ok := dev.(*IOSDevice)
	if !ok {
		return nil, fmt.Errorf("device is not iOS")
	}
	return iosDev.GetRunningApps()
}

func KillAppIOS(device *models.Device, bundleIdentifier string) error {
	dev, ok := DevManager.Get(device.UDID)
	if !ok {
		return fmt.Errorf("device not found")
	}
	iosDev, ok := dev.(*IOSDevice)
	if !ok {
		return fmt.Errorf("device is not iOS")
	}
	return iosDev.KillApp(bundleIdentifier)
}

func UpdateWebDriverAgentStreamSettings(device *models.Device) error {
	dev, ok := DevManager.Get(device.UDID)
	if !ok {
		return fmt.Errorf("device not found")
	}
	iosDev, ok := dev.(*IOSDevice)
	if !ok {
		return fmt.Errorf("device is not iOS")
	}
	return iosDev.UpdateStreamSettingsOnDevice()
}
