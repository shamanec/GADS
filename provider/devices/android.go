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
	"GADS/common"
	"GADS/common/auth"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
	"GADS/provider/providerutil"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// AndroidDevice holds Android-specific runtime state alongside the shared RuntimeState.
type AndroidDevice struct {
	RuntimeState
	StreamPort              string // host port forwarded to device port 1991 (video stream)
	AndroidIMEPort          string // host port forwarded to device port 1993 (IME keyboard)
	AndroidRemoteServerPort string // host port forwarded to device port 1994 (remote control server)
}

var remoteServerNetClient = &http.Client{
	Timeout: time.Second * 120,
}

// Port accessors for router access via type assertion.
func (d *AndroidDevice) GetStreamPort() string              { return d.StreamPort }
func (d *AndroidDevice) GetAndroidIMEPort() string          { return d.AndroidIMEPort }
func (d *AndroidDevice) GetAndroidRemoteServerPort() string { return d.AndroidRemoteServerPort }

// Setup runs the full Android device provisioning sequence.
func (d *AndroidDevice) Setup() error {
	d.SetupMutex.Lock()
	defer d.SetupMutex.Unlock()

	d.SetProviderState("preparing")
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Running setup for device `%v`", d.GetUDID()))

	d.detectHardwareModel()

	if err := d.updateScreenSizeIfNeeded(); err != nil {
		return d.resetWithError("update screen dimensions with adb", err)
	}
	if err := d.disableAutoRotation(); err != nil {
		return d.resetWithError("disable auto-rotation", err)
	}
	if err := d.allocatePorts(); err != nil {
		return d.resetWithError("allocate free host ports", err)
	}
	if err := d.cleanupOldApps(); err != nil {
		return err // already reset inside cleanupOldApps
	}
	if err := d.installAndPushGadsSettings(); err != nil {
		return d.resetWithError("install/push GADS Settings on Android device", err)
	}
	if err := d.startServicesAndStreaming(); err != nil {
		return err // already reset inside
	}
	if err := d.forwardAndSetupPorts(); err != nil {
		return err // already reset inside
	}
	if err := d.applyStreamConfig(); err != nil {
		return d.resetWithError("apply device stream settings", err)
	}
	if err := d.setupAppiumIfNeeded(); err != nil {
		return err
	}

	d.SetProviderState("live")
	return nil
}

func (d *AndroidDevice) detectHardwareModel() {
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Retrieving hardware model for device `%s`", d.GetUDID()))
	d.getHardwareModel()
}

func (d *AndroidDevice) updateScreenSizeIfNeeded() error {
	if d.DBDevice.ScreenHeight != "" && d.DBDevice.ScreenWidth != "" {
		return nil
	}
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Updating screen dimensions for device `%v`", d.GetUDID()))
	if err := d.updateScreenSizeADB(); err != nil {
		return err
	}
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Successfully updated screen dimensions for device `%v`", d.GetUDID()))
	return nil
}

func (d *AndroidDevice) allocatePorts() error {
	streamPort, err := providerutil.GetFreePort()
	if err != nil {
		return fmt.Errorf("could not allocate free host port for GADS-stream - %w", err)
	}
	d.StreamPort = streamPort

	imePort, err := providerutil.GetFreePort()
	if err != nil {
		return fmt.Errorf("could not allocate free host port for GADS Android IME - %w", err)
	}
	d.AndroidIMEPort = imePort

	remoteServerPort, err := providerutil.GetFreePort()
	if err != nil {
		return fmt.Errorf("could not allocate free host port for GADS Android remote control server - %w", err)
	}
	d.AndroidRemoteServerPort = remoteServerPort
	return nil
}

func (d *AndroidDevice) cleanupOldApps() error {
	d.InstalledApps = d.GetInstalledAppBundleIDs()
	logger.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Updated installed apps for Android device `%v`", d.GetUDID()))

	for _, pkg := range []string{"com.gads.settings", "com.gads.webrtc", "com.shamanec.stream", "com.gads.gads_ime"} {
		if slices.Contains(d.InstalledApps, pkg) {
			if err := d.UninstallApp(pkg); err != nil {
				logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not uninstall %s from Android device `%v` - %v", pkg, d.GetUDID(), err))
				d.Reset(fmt.Sprintf("Failed to uninstall %s from Android device.", pkg))
				return err
			}
			time.Sleep(3 * time.Second)
		}
	}
	return nil
}

func (d *AndroidDevice) installAndPushGadsSettings() error {
	if err := d.installGadsSettingsApp(); err != nil {
		return fmt.Errorf("could not install GADS Settings - %w", err)
	}
	if err := d.pushGadsSettingsInTmpLocal(); err != nil {
		return fmt.Errorf("could not push GADS Settings to /tmp/local - %w", err)
	}
	return nil
}

func (d *AndroidDevice) startServicesAndStreaming() error {
	time.Sleep(2 * time.Second)
	go d.startRemoteControlServer()
	time.Sleep(2 * time.Second)

	// Start the respective video stream
	if d.DBDevice.StreamType == models.AndroidWebRTCGadsH264StreamTypeId {
		go d.startH264Stream()
	} else {
		if err := d.addStreamRecordingPermissions(); err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not add GADS Settings stream recording permissions on Android device `%v` - %v", d.GetUDID(), err))
			d.Reset("Failed to add GADS Settings stream recording permissions on Android device.")
			return err
		}
		time.Sleep(2 * time.Second)

		if d.SemVer.Major() >= 15 {
			if err := d.addStreamPostNotificationsPermission(); err != nil {
				logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not add GADS Settings POST_NOTIFICATIONS permissions on Android device `%v` - %v", d.GetUDID(), err))
				d.Reset("Failed to add GADS Settings POST_NOTIFICATIONS permissions on Android device.")
				return err
			}
			time.Sleep(1 * time.Second)
		}

		if err := d.startStreaming(); err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not start GADS streaming on Android device `%v` - %v", d.GetUDID(), err))
			d.Reset("Failed to start GADS streaming on Android device.")
			return err
		}
	}
	time.Sleep(2 * time.Second)

	// Forward the video stream to the host
	if err := d.forwardStream(); err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS streaming port to host port %v for Android device `%v` - %v", d.StreamPort, d.GetUDID(), err))
		d.Reset("Failed to forward GADS-stream port to host port.")
		return err
	}

	// Send TURN configuration to WebRTC service (non-fatal)
	if models.IsWebRTCStreamType(d.DBDevice.StreamType) {
		if err := d.UpdateWebRTCTURNConfig(); err != nil {
			logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Could not send TURN config to device `%s` - %v (WebRTC will use STUN only)", d.GetUDID(), err))
		}
	}
	return nil
}

func (d *AndroidDevice) forwardAndSetupPorts() error {
	// Setup GADS Android IME
	if err := d.setupIME(); err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Failed to setup GADS Android IME for Android device `%v` - %v", d.GetUDID(), err))
		d.Reset("Failed to setup GADS Android IME.")
		return err
	}

	// Forward IME port
	if err := d.forwardIME(); err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS Android IME port for Android device `%v` - %v", d.GetUDID(), err))
		d.Reset("Failed to forward GADS Android IME port to host port.")
		return err
	}

	// Forward remote server port
	if err := d.forwardRemoteServer(); err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS Android Settings port for Android device `%v` - %v", d.GetUDID(), err))
		d.Reset("Failed to forward GADS Android Settings port to host port.")
		return err
	}
	return nil
}

func (d *AndroidDevice) applyStreamConfig() error {
	if err := d.ApplyStreamSettings(); err != nil {
		return fmt.Errorf("could not apply device stream settings - %w", err)
	}
	if err := d.UpdateStreamSettingsOnDevice(); err != nil {
		return fmt.Errorf("could not update GADS stream settings on device - %w", err)
	}
	return nil
}

func (d *AndroidDevice) setupAppiumIfNeeded() error {
	if !config.ProviderConfig.SetupAppiumServers {
		return nil
	}
	// Uninstall old Appium packages before starting
	for _, pkg := range []string{"io.appium.settings", "io.appium.uiautomator2.server", "io.appium.uiautomator2.server.test"} {
		if slices.Contains(d.InstalledApps, pkg) {
			if err := d.UninstallApp(pkg); err != nil {
				logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to uninstall %s on device %s - %s", pkg, d.GetUDID(), err))
			}
		}
	}
	return setupAppiumForDevice(d)
}

// AppiumCapabilities returns the Android-specific Appium server capabilities.
func (d *AndroidDevice) AppiumCapabilities() models.AppiumServerCapabilities {
	return models.AppiumServerCapabilities{
		UDID:           d.GetUDID(),
		AutomationName: "UiAutomator2",
		PlatformName:   "Android",
		DeviceName:     d.DBDevice.Name,
	}
}

// Reset overrides RuntimeState.Reset to free Android-specific ports.
func (d *AndroidDevice) Reset(reason string) {
	if d.ResetBase(reason) {
		common.MutexManager.LocalDevicePorts.Lock()
		delete(providerutil.UsedPorts, d.StreamPort)
		delete(providerutil.UsedPorts, d.AndroidIMEPort)
		delete(providerutil.UsedPorts, d.AndroidRemoteServerPort)
		common.MutexManager.LocalDevicePorts.Unlock()
	}
}

func (d *AndroidDevice) androidRemoteServerRequest(method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", d.AndroidRemoteServerPort, endpoint)
	d.Logger.LogDebug("androidRemoteServerRequest", fmt.Sprintf("Calling `%s` for device `%s`", url, d.GetUDID()))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return remoteServerNetClient.Do(req)
}

func (d *AndroidDevice) isStreamServiceRunning() (bool, error) {
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "dumpsys", "activity", "services", d.getStreamServiceName())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("isStreamServiceRunning: Error executing `%s` with combined output - %s", cmd.Args, err)
	}
	if strings.Contains(string(output), "(nothing)") {
		return false, nil
	}
	return true, nil
}

func (d *AndroidDevice) stopStreamService() {
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "am", "stopservice", d.getStreamServiceName())
	if err := cmd.Run(); err != nil {
		logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to stop GADS-stream service properly - %s", err))
	}
}

func (d *AndroidDevice) installGadsSettingsApp() error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Installing GADS Settings apk on device `%v`", d.GetUDID()))
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "install", "-r", fmt.Sprintf("%s/gads-settings.apk", config.ProviderConfig.ProviderFolder))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installGadsSettingsApp: Error executing `%s` - %s", cmd.Args, err)
	}
	return nil
}

func (d *AndroidDevice) pushGadsSettingsInTmpLocal() error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Pushing GADS Settings apk to /tmp/local on device `%v`", d.GetUDID()))
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "push", fmt.Sprintf("%s/gads-settings.apk", config.ProviderConfig.ProviderFolder), "/data/local/tmp/gads-settings")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pushGadsSettingsInTmpLocal: Error executing `%s` - %s", cmd.Args, err)
	}
	return nil
}

func (d *AndroidDevice) startRemoteControlServer() {
	killCmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "pkill -f RemoteControlServerKt")
	_ = killCmd.Run()
	time.Sleep(1 * time.Second)

	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell",
		"CLASSPATH=/data/local/tmp/gads-settings app_process / com.gads.settings.RemoteControlServerKt 1994")

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error executing `%s` for device `%v` - %v", cmd.Args, d.GetUDID(), err))
		d.Reset("Failed to execute GADS Remote server.")
		return
	}
	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startRemoteControlServer: Error waiting for command to finish, device `%v` - %v", d.GetUDID(), err))
		d.Reset("GADS Android remote server failed.")
	}
}

func (d *AndroidDevice) startH264Stream() {
	killCmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "pkill -f H264Server")
	_ = killCmd.Run()
	time.Sleep(1 * time.Second)

	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell",
		"CLASSPATH=/data/local/tmp/gads-settings app_process / com.gads.settings.server.H264Server")

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error executing `%s` for device `%v` - %v", cmd.Args, d.GetUDID(), err))
		d.Reset("Failed to execute GADS H264 server.")
		return
	}
	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startH264Stream: Error waiting for command to finish, device `%v` - %v", d.GetUDID(), err))
		d.Reset("GADS Android H264 server failed.")
	}
}

func (d *AndroidDevice) setupIME() error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Enabling GADS Android IME on device `%v`", d.GetUDID()))
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "ime", "enable", "com.gads.settings/.GADSKeyboardIME")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("enableGadsAndroidIME: Error executing `%s` - %s", cmd.Args, err)
	}
	time.Sleep(1 * time.Second)

	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Setting GADS Android IME as active on device `%v`", d.GetUDID()))
	cmd = exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "ime", "set", "com.gads.settings/.GADSKeyboardIME")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setGadsAndroidIMEAsActive: Error executing `%s` - %s", cmd.Args, err)
	}
	return nil
}

func (d *AndroidDevice) addStreamRecordingPermissions() error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Adding GADS-stream recording permissions on device `%v`", d.GetUDID()))
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "appops", "set", d.getStreamServicePackageName(), "PROJECT_MEDIA", "allow")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("addStreamRecordingPermissions: Error executing `%s` - %s", cmd.Args, err)
	}
	return nil
}

func (d *AndroidDevice) addStreamPostNotificationsPermission() error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Adding GADS app post notification permissions on device `%v`", d.GetUDID()))
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "pm", "grant", d.getStreamServicePackageName(), "android.permission.POST_NOTIFICATIONS")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("addStreamPostNotificationsPermission: Error executing `%s` - %s", cmd.Args, err)
	}
	return nil
}

func (d *AndroidDevice) startStreaming() error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Starting GADS-stream app on `%s`", d.GetUDID()))
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "am", "start", "-n", d.getStreamServiceActivityName())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("startStreaming: Error executing `%s` - %s", cmd.Args, err)
	}
	return nil
}

func (d *AndroidDevice) pressHomeButton() {
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "input", "keyevent", "KEYCODE_HOME")
	if err := cmd.Run(); err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("pressHomeButton: Could not 'press' Home button - %v", err))
	}
}

func (d *AndroidDevice) forwardPort(devicePort, hostPort string) error {
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "forward", "tcp:"+hostPort, "tcp:"+devicePort)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("forwardPort: Error forwarding device port %s to host port %s - %s", devicePort, hostPort, err)
	}
	return nil
}

func (d *AndroidDevice) forwardStream() error {
	return d.forwardPort("1991", d.StreamPort)
}

func (d *AndroidDevice) forwardIME() error {
	return d.forwardPort("1993", d.AndroidIMEPort)
}

func (d *AndroidDevice) forwardRemoteServer() error {
	return d.forwardPort("1994", d.AndroidRemoteServerPort)
}

func (d *AndroidDevice) updateScreenSizeADB() error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Attempting to automatically update the screen size for device `%v`", d.GetUDID()))

	var outBuffer bytes.Buffer
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "wm", "size")
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("updateScreenSizeADB: Error executing `%s` - %s", cmd.Args, err)
	}

	output := outBuffer.String()
	lines := strings.Split(output, "\n")
	if len(lines) == 2 {
		splitOutput := strings.Split(lines[0], ": ")
		screenDimensions := strings.Split(splitOutput[1], "x")
		d.DBDevice.ScreenWidth = strings.TrimSpace(screenDimensions[0])
		d.DBDevice.ScreenHeight = strings.TrimSpace(screenDimensions[1])
	}
	if len(lines) == 3 {
		splitOutput := strings.Split(lines[1], ": ")
		screenDimensions := strings.Split(splitOutput[1], "x")
		d.DBDevice.ScreenWidth = strings.TrimSpace(screenDimensions[0])
		d.DBDevice.ScreenHeight = strings.TrimSpace(screenDimensions[1])
	}

	if err := db.GlobalMongoStore.AddOrUpdateDevice(d.DBDevice); err != nil {
		return fmt.Errorf("Failed to upsert new device screen dimensions to DB - %s", err)
	}
	return nil
}

// GetInstalledAppBundleIDs returns the bundle identifiers (package names) of third-party installed apps.
func (d *AndroidDevice) GetInstalledAppBundleIDs() []string {
	installedApps := make([]string, 0)
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "cmd", "package", "list", "packages", "-3")

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		d.Logger.LogError("get_installed_apps", fmt.Sprintf("Error getting installed apps - %v", err))
		return installedApps
	}

	result := strings.TrimSpace(outBuffer.String())
	lines := regexp.MustCompile("\r?\n").Split(result, -1)
	for _, line := range lines {
		lineSplit := strings.Split(line, ":")
		if len(lineSplit) > 1 {
			installedApps = append(installedApps, lineSplit[1])
		}
	}
	return installedApps
}

// GetInstalledApps returns detailed info about installed apps via the GADS remote server.
func (d *AndroidDevice) GetInstalledApps() ([]models.DeviceApp, error) {
	var deviceApps = make([]models.DeviceApp, 0)

	runningAppsResp, err := d.androidRemoteServerRequest(http.MethodGet, "installed-apps", nil)
	if err != nil {
		d.Logger.LogError("get_installed_apps", fmt.Sprintf("Failed executing remote server request - %s", err.Error()))
		return deviceApps, err
	}
	defer runningAppsResp.Body.Close()

	payload, err := io.ReadAll(runningAppsResp.Body)
	if err != nil {
		d.Logger.LogError("get_installed_apps", fmt.Sprintf("Failed reading remote server response body - %s", err.Error()))
		return deviceApps, err
	}
	if err := json.Unmarshal(payload, &deviceApps); err != nil {
		d.Logger.LogError("get_installed_apps", fmt.Sprintf("Failed unmarshalling remote server response - %s", err.Error()))
		return deviceApps, err
	}
	return deviceApps, nil
}

// UninstallApp uninstalls an app by package name.
func (d *AndroidDevice) UninstallApp(packageName string) error {
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "uninstall", packageName)
	if err := cmd.Run(); err != nil {
		d.Logger.LogError("uninstall_app", fmt.Sprintf("Error uninstalling app `%s` - %v", packageName, err))
		return err
	}
	return nil
}

// InstallApp installs an app from a file in the provider folder.
func (d *AndroidDevice) InstallApp(appName string) error {
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "install", "-r", fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName))
	if err := cmd.Run(); err != nil {
		d.Logger.LogError("install_app", fmt.Sprintf("Error installing app `%s` - %v", appName, err))
		return err
	}
	return nil
}

func (d *AndroidDevice) disableAutoRotation() error {
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "settings", "put", "system", "accelerometer_rotation", "0")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// GetCurrentRotation returns "portrait" or "landscape".
func (d *AndroidDevice) GetCurrentRotation() (string, error) {
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "settings", "get", "system", "user_rotation")

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		return "portrait", err
	}

	result := strings.TrimSpace(outBuffer.String())
	if result == "1" {
		return "landscape", nil
	}
	return "portrait", nil
}

// ChangeRotation changes the device rotation to "portrait" or "landscape".
func (d *AndroidDevice) ChangeRotation(rotation string) error {
	var adbRotationValue = "0"
	if rotation == "landscape" {
		adbRotationValue = "1"
	}
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "settings", "put", "system", "user_rotation", adbRotationValue)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// UpdateStreamSettingsOnDevice sends stream settings to the device via WebSocket.
func (d *AndroidDevice) UpdateStreamSettingsOnDevice() error {
	u := url.URL{Scheme: "ws", Host: "localhost:" + d.StreamPort, Path: ""}
	destConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		return fmt.Errorf("failed connecting to device `%s` stream port - %s", d.GetUDID(), err)
	}
	defer destConn.Close()

	socketMsg := fmt.Sprintf("targetFPS=%v:jpegQuality=%v:scalingFactor=%v",
		d.StreamTargetFPS, d.StreamJpegQuality, d.StreamScalingFactor)

	if d.DBDevice.StreamType == models.AndroidWebRTCGadsH264StreamTypeId {
		socketMsg = fmt.Sprintf("targetFPS=%v:scalingFactor=%v",
			d.StreamTargetFPS, d.StreamScalingFactor)
	}

	if err := wsutil.WriteServerMessage(destConn, ws.OpText, []byte(socketMsg)); err != nil {
		return fmt.Errorf("failed sending stream settings to stream websocket - %s", err)
	}
	return nil
}

// UpdateWebRTCTURNConfig sends TURN configuration to the WebRTC service on the device.
func (d *AndroidDevice) UpdateWebRTCTURNConfig() error {
	turnConfig, err := db.GlobalMongoStore.GetTURNConfig()
	if err != nil {
		return fmt.Errorf("failed to get TURN config from DB - %s", err)
	}
	if !turnConfig.Enabled {
		return nil
	}
	if turnConfig.Server == "" || turnConfig.SharedSecret == "" {
		return fmt.Errorf("TURN config incomplete: server=%s, shared_secret configured=%t", turnConfig.Server, turnConfig.SharedSecret != "")
	}

	ttl := turnConfig.TTL
	if ttl == 0 {
		ttl = 3600
	}
	username, password, _ := auth.GenerateTURNCredentials(turnConfig.SharedSecret, ttl, config.ProviderConfig.TURNUsernameSuffix)

	u := url.URL{Scheme: "ws", Host: "localhost:" + d.StreamPort, Path: ""}
	destConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		return fmt.Errorf("failed connecting to WebRTC service WebSocket - %s", err)
	}
	defer destConn.Close()

	turnMsg := fmt.Sprintf(`{"type":"turn","server":"%s","port":%d,"username":"%s","password":"%s"}`,
		turnConfig.Server, turnConfig.Port, username, password)

	if err := wsutil.WriteServerMessage(destConn, ws.OpText, []byte(turnMsg)); err != nil {
		return fmt.Errorf("failed sending TURN config to WebSocket - %s", err)
	}
	return nil
}

func (d *AndroidDevice) getStreamServiceName() string {
	switch d.DBDevice.StreamType {
	case models.MJPEGStreamTypeId:
		return "com.gads.settings/.ScreenCaptureService"
	case models.AndroidWebRTCGetStreamStreamTypeId:
		return "com.gads.settings/.WebRTCScreenCaptureService"
	default:
		return "com.gads.settings/.ScreenCaptureService"
	}
}

func (d *AndroidDevice) getStreamServicePackageName() string {
	return "com.gads.settings"
}

func (d *AndroidDevice) getStreamServiceActivityName() string {
	switch d.DBDevice.StreamType {
	case models.MJPEGStreamTypeId:
		return "com.gads.settings/com.gads.settings.streaming.MjpegScreenCaptureActivity"
	case models.AndroidWebRTCGetStreamStreamTypeId:
		return "com.gads.settings/com.gads.settings.webrtc.WebRTCScreenCaptureActivity"
	default:
		return "com.gads.settings/com.gads.settings.streaming.MjpegScreenCaptureActivity"
	}
}

// GetScreenSize is not needed for Android (screen size is retrieved via ADB during setup).
func (d *AndroidDevice) GetScreenSize() (width, height string, err error) {
	return d.DBDevice.ScreenWidth, d.DBDevice.ScreenHeight, nil
}

// GetHardwareModel returns the hardware model string.
func (d *AndroidDevice) GetHardwareModel() (string, error) {
	return d.HardwareModel, nil
}

func (d *AndroidDevice) getHardwareModel() {
	brandCmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "getprop", "ro.product.brand")
	var outBuffer bytes.Buffer
	brandCmd.Stdout = &outBuffer
	if err := brandCmd.Run(); err != nil {
		d.HardwareModel = "Unknown"
	}
	brand := outBuffer.String()
	outBuffer.Reset()

	modelCmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "getprop", "ro.product.model")
	modelCmd.Stdout = &outBuffer
	if err := modelCmd.Run(); err != nil {
		d.HardwareModel = "Unknown"
		return
	}
	model := outBuffer.String()
	d.HardwareModel = fmt.Sprintf("%s %s", strings.TrimSpace(brand), strings.TrimSpace(model))
}

// LaunchApp is not supported for Android via this interface (use Appium).
func (d *AndroidDevice) LaunchApp(bundleID string) error {
	return fmt.Errorf("LaunchApp not supported for Android via this interface")
}

// KillApp force-stops an Android app by package name.
func (d *AndroidDevice) KillApp(bundleIdentifier string) error {
	cmd := exec.CommandContext(d.Context, "adb", "-s", d.GetUDID(), "shell", "am", "force-stop", bundleIdentifier)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("KillApp: Failed killing app with package name `%s` via adb shell", bundleIdentifier)
	}
	return nil
}

// ApplyStreamSettings applies stream settings from DB to the device runtime state.
func (d *AndroidDevice) ApplyStreamSettings() error {
	return applyDeviceStreamSettings(d)
}

func DeleteAndroidSharedStorageFile(device *models.Device, filePath string) error {
	deleteFileCmd := exec.Command("adb", "-s", device.UDID, "shell", "rm", fmt.Sprintf("\"%s\"", filePath))
	_, err := deleteFileCmd.Output()
	return err
}

func PullAndroidSharedStorageFile(device *models.Device, filePath string, fileName string) (string, error) {
	var tempFilePath = filepath.Join(os.TempDir(), fileName)
	pullFileCmd := exec.Command("adb", "-s", device.UDID, "pull", filePath, tempFilePath)
	_, err := pullFileCmd.Output()
	return tempFilePath, err
}


