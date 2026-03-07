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
	"GADS/common/auth"
	"GADS/common/constants"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/common/utils"
	"GADS/provider/config"
	"GADS/provider/logger"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// Check if the GADS-stream service is running on the device
func isGadsStreamServiceRunning(device *models.Device) (bool, error) {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Checking if GADS-stream is already running on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "dumpsys", "activity", "services", GetStreamServiceName(device))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("isGadsStreamServiceRunning: Error executing `%s` with combined output - %s", cmd.Args, err)
	}

	// If command returned "(nothing)" then the service is not running
	if strings.Contains(string(output), "(nothing)") {
		return false, nil
	}

	return true, nil
}

func stopGadsStreamService(device *models.Device) {
	logger.ProviderLogger.LogInfo("android_device_setup", "Stopping GADS-stream service")

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "am", "stopservice", GetStreamServiceName(device))

	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to stop GADS-stream service properly - %s", err))
	}
}

// Installs the GADS-Settings apk on Android devices.
// The GADS-Settings provides the GADS IME and GADS mjpeg video stream service
func installGadsSettingsApp(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Installing GADS Settings apk on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "install", "-r", fmt.Sprintf("%s/gads-settings.apk", config.ProviderConfig.ProviderFolder))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("installGadsSettingsApp: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Pushes the GADS-Settings apk withou an extension to /data/local/tmp on Android devices.
// This can be started as app_process which in turn contains the remote control server
func pushGadsSettingsInTmpLocal(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Pushing GADS Settings apk to /tmp/local on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "push", fmt.Sprintf("%s/gads-settings.apk", config.ProviderConfig.ProviderFolder), "/data/local/tmp/gads-settings")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("installGadsSettingsInTmpLocal: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Starts the GADS-Settings remote control server as app_process from /data/local/tmp.
// The remote control server provides endpoints for tapping/swiping and other interactions independent from Appium server
func startGadsRemoteControlServer(device *models.Device) {
	// Kill existing process first
	killCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "pkill -f RemoteControlServerKt")
	_ = killCmd.Run() // Ignore error - process might not exist

	time.Sleep(1 * time.Second)

	cmd := exec.CommandContext(
		device.Context,
		"adb",
		"-s",
		device.UDID,
		"shell",
		"CLASSPATH=/data/local/tmp/gads-settings app_process / com.gads.settings.RemoteControlServerKt 1994")

	logger.ProviderLogger.LogDebug("device_setup", fmt.Sprintf("Starting GADS Remote server on device `%s` with command `%s`", device.UDID, cmd.Args))

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error executing `%s` for device `%v` - %v", cmd.Args, device.UDID, err))
		ResetLocalDevice(device, "Failed to execute GADS Remote server.")
		return
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf(
			"startGadsRemoteControlServer: Error waiting for `%s` command to finish, it errored out or device `%v` was disconnected - %v",
			cmd.Args, device.UDID, err))

		ResetLocalDevice(device, "GADS Android remote server failed.")
	}
}

// Starts the GADS-Settings H264 server as app_process from /data/local/tmp.
// The H264 server provides h264 frames from the device screen over a websocket for Pion to serve WebRTC
func startGadsSettingsStream(device *models.Device) {
	// Kill existing process first
	killCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "pkill -f H264Server")
	_ = killCmd.Run() // Ignore error - process might not exist

	time.Sleep(1 * time.Second)

	cmd := exec.CommandContext(
		device.Context,
		"adb",
		"-s",
		device.UDID,
		"shell",
		"CLASSPATH=/data/local/tmp/gads-settings app_process / com.gads.settings.server.H264Server")

	logger.ProviderLogger.LogDebug("device_setup", fmt.Sprintf("Starting GADS Remote server on device `%s` with command `%s`", device.UDID, cmd.Args))

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error executing `%s` for device `%v` - %v", cmd.Args, device.UDID, err))
		ResetLocalDevice(device, "Failed to execute GADS Remote server.")
		return
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf(
			"startGadsSettingsStream: Error waiting for `%s` command to finish, it errored out or device `%v` was disconnected - %v",
			cmd.Args, device.UDID, err))

		ResetLocalDevice(device, "GADS Android H264 server failed.")
	}
}

// Enables the GADS Android IME and sets it as active for the device
func setupGadsAndroidIME(device *models.Device) error {
	err := enableGadsAndroidIME(device)
	if err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	err = setGadsAndroidIMEAsActive(device)
	if err != nil {
		return err
	}

	return nil
}

// Enable the GADS Android IME made available via the GADS-Settings apk
func enableGadsAndroidIME(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Enabling GADS Android IME on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "ime", "enable", "com.gads.settings/.GADSKeyboardIME")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("enableGadsAndroidIME: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Sets the GADS Android IME as the current active IME on the device
// The GADS Android IME has a server providing endpoint for typing text remotely
func setGadsAndroidIMEAsActive(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Setting GADS Android IME as active on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "ime", "set", "com.gads.settings/.GADSKeyboardIME")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("setGadsAndroidIMEAsActive: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Add recording permissions to GADS video streaming application to avoid popup on start
func addGadsStreamRecordingPermissions(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Adding GADS-stream recording permissions on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "appops", "set", GetStreamServicePackageName(device), "PROJECT_MEDIA", "allow")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("addGadsStreamRecordingPermissions: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Add POST_NOTIFICATIONS permission to the stream app because on some devices with newer Android startForeground() might throw an exception without it
func addGadsStreamPostNotificationsPermission(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Adding GADS app post notification permissions on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "pm", "grant", GetStreamServicePackageName(device), "android.permission.POST_NOTIFICATIONS")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("addGadsStreamRecordingPermissions: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Start the GADS video streaming foreground service using adb
func startGadsAndroidStreaming(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Starting GADS-stream app on `%s`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "am", "start", "-n", GetStreamServiceActivityName(device))
	logger.ProviderLogger.LogDebug("startGadsAndroidStreaming", fmt.Sprintf("Starting activity with `%v` on device `%s`", cmd.Args, device.UDID))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("startGadsStreamApp: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Press the Home button using adb to hide the transparent GADS video streaming activity
func pressHomeButton(device *models.Device) {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Pressing Home button with adb on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "input", "keyevent", "KEYCODE_HOME")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("pressHomeButton: Could not 'press' Home button with `%v`, you need to press it yourself to hide the transparent activity of GADS-stream:\n %v", cmd.Path, err))
	}
}

// Forward an Android device service port to a host port
func forwardAndroidPort(device *models.Device, devicePort, hostPort string) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Trying to forward Android device port `%v` to host port `%v` for device `%s`", devicePort, hostPort, device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "forward", "tcp:"+hostPort, "tcp:"+devicePort)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("forwardAndroidPort: Error executing `%s` while trying to forward Android device port to host - %s", cmd.Args, err)
	}

	return nil
}

// Forward the GADS Android stream tcp to a host port that is already assigned for the device
func forwardGadsStream(device *models.Device) error {
	return forwardAndroidPort(device, "1991", device.StreamPort)
}

// Forward the GADS Android IME tcp to a host port that is already assigned for the device
func forwardGadsAndroidIME(device *models.Device) error {
	return forwardAndroidPort(device, "1993", device.AndroidIMEPort)
}

// Forward the GADS Android remote control server tcp to a host port that is already assigned for the device
func forwardGadsRemoteServer(device *models.Device) error {
	return forwardAndroidPort(device, "1994", device.AndroidRemoteServerPort)
}

// Get the Android device screen size with adb
func updateAndroidScreenSizeADB(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Attempting to automatically update the screen size for device `%v`", device.UDID))

	var outBuffer bytes.Buffer
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "wm", "size")
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("updateAndroidScreenSizeADB: Error executing `%s` - %s", cmd.Args, err)
	}

	output := outBuffer.String()
	// Some devices return more than one line with device screen info
	// Physical size and Override size
	// That's why we'll process the response respectively
	// Specifically this was applied when caught on Samsung S20 and S9, might apply for others
	lines := strings.Split(output, "\n")
	// If the split lines are 2 then we have only one size returned
	// and one empty line
	if len(lines) == 2 {
		splitOutput := strings.Split(lines[0], ": ")
		screenDimensions := strings.Split(splitOutput[1], "x")

		device.ScreenWidth = strings.TrimSpace(screenDimensions[0])
		device.ScreenHeight = strings.TrimSpace(screenDimensions[1])
	}

	// If the split lines are 3 then we have two sizes returned
	// and one empty line
	// We need the second size here
	if len(lines) == 3 {
		splitOutput := strings.Split(lines[1], ": ")
		screenDimensions := strings.Split(splitOutput[1], "x")

		device.ScreenWidth = strings.TrimSpace(screenDimensions[0])
		device.ScreenHeight = strings.TrimSpace(screenDimensions[1])
	}

	err := db.GlobalMongoStore.AddOrUpdateDevice(device)
	if err != nil {
		return fmt.Errorf("Failed to uspert new device screen dimensions to DB - %s", err)
	}

	return nil
}

// Get all installed apps on an Android device
func GetInstalledAppsAndroid(device *models.Device) []string {
	installedApps := make([]string, 0)
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "cmd", "package", "list", "packages", "-3")

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("GetInstalledAppsAndroid: Error executing `%s` trying to get installed apps - %v", cmd.Args, err))
		return installedApps
	}

	// Get the command output to string
	result := strings.TrimSpace(outBuffer.String())
	// Get all lines with package names
	lines := regexp.MustCompile("\r?\n").Split(result, -1)

	// Clean the package names and add them to the device installed apps
	for _, line := range lines {
		lineSplit := strings.Split(line, ":")
		if len(lineSplit) > 1 {
			packageName := lineSplit[1]
			installedApps = append(installedApps, packageName)
		} else {
			device.Logger.LogWarn("get_installed_apps", "Could not parse package line: "+line)
		}
	}

	return installedApps
}

// Uninstall app from Android device by package name
func uninstallAppAndroid(device *models.Device, packageName string) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "uninstall", packageName)

	if err := cmd.Run(); err != nil {
		device.Logger.LogError("uninstall_app", fmt.Sprintf("uninstallAppAndroid: Error executing `%s` trying to uninstall app - %v", cmd.Args, err))
		return err
	}

	return nil
}

// Install app on Android device by apk name
func installAppAndroid(device *models.Device, appName string) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "install", "-r", fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName))

	if err := cmd.Run(); err != nil {
		device.Logger.LogError("install_app", fmt.Sprintf("installAppAndroid: Error executing `%s` trying to install app - %v", cmd.Args, err))
		return err
	}

	return nil
}

func disableAutoRotationAndroid(device *models.Device) error {
	// 0 disable, 1 enable
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "settings", "put", "system", "accelerometer_rotation", "0")

	if err := cmd.Run(); err != nil {
		device.Logger.LogError("ChangeRotationAndroid", fmt.Sprintf("ChangeRotationAndroid: Error executing `%s` trying to change device rotation - %v", cmd.Args, err))
		return err
	}
	return nil
}

func GetCurrentRotationAndroid(device *models.Device) (string, error) {
	// 0 portrait, 1 landscape
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "settings", "get", "system", "user_rotation")

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("getCurrentRotationAndroid", fmt.Sprintf("getCurrentRotationAndroid: Error executing `%s` trying to get current device rotation - %v", cmd.Args, err))
		return "portrait", err
	}

	// Get the command output to string
	result := strings.TrimSpace(outBuffer.String())

	// Return the parsed rotation string
	if result == "1" {
		return "landscape", nil
	} else {
		return "portrait", nil
	}
}

// Change screen rotation on Android device
func ChangeRotationAndroid(device *models.Device, rotation string) error {
	// 0 is for portrait, 1 is for landscape
	var adbRotationValue = "0"
	if rotation == "landscape" {
		adbRotationValue = "1"
	}

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "settings", "put", "system", "user_rotation", adbRotationValue)

	if err := cmd.Run(); err != nil {
		device.Logger.LogError("ChangeRotationAndroid", fmt.Sprintf("ChangeRotationAndroid: Error executing `%s` trying to change device rotation - %v", cmd.Args, err))
		return err
	}
	return nil
}

func UpdateGadsStreamSettings(device *models.Device) error {
	// Prepare the WebSocket URL
	u := url.URL{Scheme: "ws", Host: "localhost:" + device.StreamPort, Path: ""}
	destConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		return fmt.Errorf("failed connecting to device `%s` stream port - %s", device.UDID, err)
	}
	defer destConn.Close()

	// Create the message to send
	socketMsg := fmt.Sprintf("targetFPS=%v:jpegQuality=%v:scalingFactor=%v",
		device.StreamTargetFPS, device.StreamJpegQuality, device.StreamScalingFactor)

	if device.StreamType == models.AndroidWebRTCGadsH264StreamTypeId {
		socketMsg = fmt.Sprintf("targetFPS=%v:scalingFactor=%v",
			device.StreamTargetFPS, device.StreamScalingFactor)
	}

	// Send the message over the WebSocket
	err = wsutil.WriteServerMessage(destConn, ws.OpText, []byte(socketMsg))
	if err != nil {
		fmt.Printf("failed sending stream settings to stream websocket - %s\n", err)
		return fmt.Errorf("failed sending stream settings to stream websocket - %s", err)
	}

	return nil
}

func UpdateWebRTCTURNConfig(device *models.Device) error {
	// Get TURN config from MongoDB
	turnConfig, err := db.GlobalMongoStore.GetTURNConfig()
	if err != nil {
		return fmt.Errorf("failed to get TURN config from DB - %s", err)
	}

	if !turnConfig.Enabled {
		device.Logger.LogDebug("webrtc_turn",
			fmt.Sprintf("TURN not enabled, skipping for device `%s`", device.UDID))
		return nil
	}

	// Validate TURN config
	if turnConfig.Server == "" || turnConfig.SharedSecret == "" {
		return fmt.Errorf("TURN config incomplete: server=%s, shared_secret configured=%t",
			turnConfig.Server, turnConfig.SharedSecret != "")
	}

	// Generate ephemeral TURN credentials
	ttl := turnConfig.TTL
	if ttl == 0 {
		ttl = 3600 // Default: 1 hour
	}
	username, password, _ := auth.GenerateTURNCredentials(turnConfig.SharedSecret, ttl, config.ProviderConfig.TURNUsernameSuffix)

	// Connect to device WebSocket (use forwarded port from device.StreamPort)
	u := url.URL{Scheme: "ws", Host: "localhost:" + device.StreamPort, Path: ""}

	destConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		return fmt.Errorf("failed connecting to WebRTC service WebSocket - %s", err)
	}
	defer destConn.Close()

	// Send TURN config with ephemeral credentials as JSON
	turnMsg := fmt.Sprintf(`{"type":"turn","server":"%s","port":%d,"username":"%s","password":"%s"}`,
		turnConfig.Server, turnConfig.Port, username, password)

	err = wsutil.WriteServerMessage(destConn, ws.OpText, []byte(turnMsg))
	if err != nil {
		return fmt.Errorf("failed sending TURN config to WebSocket - %s", err)
	}

	device.Logger.LogInfo("webrtc_turn",
		fmt.Sprintf("TURN config sent to device `%s`: %s:%d (ephemeral credentials)",
			device.UDID, turnConfig.Server, turnConfig.Port))

	return nil
}

func GetStreamServiceName(device *models.Device) string {
	switch device.StreamType {
	case models.MJPEGStreamTypeId:
		return "com.gads.settings/.ScreenCaptureService"
	case models.AndroidWebRTCGetStreamStreamTypeId:
		return "com.gads.settings/.WebRTCScreenCaptureService"
	default:
		return "com.gads.settings/.ScreenCaptureService"
	}
}

func GetStreamServicePackageName(device *models.Device) string {
	return "com.gads.settings"
}

func GetStreamServiceActivityName(device *models.Device) string {
	switch device.StreamType {
	case models.MJPEGStreamTypeId:
		return "com.gads.settings/com.gads.settings.streaming.MjpegScreenCaptureActivity"
	case models.AndroidWebRTCGetStreamStreamTypeId:
		return "com.gads.settings/com.gads.settings.webrtc.WebRTCScreenCaptureActivity"
	default:
		return "com.gads.settings/com.gads.settings.streaming.MjpegScreenCaptureActivity"
	}
}

func DeleteAndroidSharedStorageFile(device *models.Device, filePath string) error {
	deleteFileCmd := exec.Command("adb", "-s", device.UDID, "shell", "rm", fmt.Sprintf("\"%s\"", filePath))
	_, err := deleteFileCmd.Output()
	if err != nil {
		return err
	}

	return nil
}

func PullAndroidSharedStorageFile(device *models.Device, filePath string, fileName string) (string, error) {
	var tempFilePath = filepath.Join(os.TempDir(), fileName)
	pullFileCmd := exec.Command("adb", "-s", device.UDID, "pull", filePath, tempFilePath)

	_, err := pullFileCmd.Output()
	if err != nil {
		return tempFilePath, err
	}

	return tempFilePath, nil
}

func GetAndroidSharedStorageFileTree(device *models.Device) (*models.AndroidFileNode, error) {
	// Collect file paths
	fileCmd := exec.Command("adb", "-s", device.UDID, "shell", "find", constants.AndroidSharedStorageRoot, "-type", "f")
	fileOutput, err := fileCmd.Output()
	if err != nil {
		return nil, err
	}

	// Collect directory paths
	dirCmd := exec.Command("adb", "-s", device.UDID, "shell", "find", constants.AndroidSharedStorageRoot, "-type", "d")
	dirOutput, err := dirCmd.Output()
	if err != nil {
		return nil, err
	}

	fileSet := make(map[string]bool)
	dirSet := make(map[string]bool)

	// Build file set
	scanner := bufio.NewScanner(strings.NewReader(string(fileOutput)))
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if isAndroidSharedStorageFilePathAllowed(path) {
			fileSet[path] = true
		}
	}

	// Build dir set
	scanner = bufio.NewScanner(strings.NewReader(string(dirOutput)))
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if isAndroidSharedStorageFilePathAllowed(path) {
			dirSet[path] = true
		}
	}

	// Merge all paths
	allPaths := make([]string, 0, len(fileSet)+len(dirSet))
	for p := range dirSet {
		allPaths = append(allPaths, p)
	}
	for p := range fileSet {
		allPaths = append(allPaths, p)
	}

	// Build tree
	root := &models.AndroidFileNode{
		Name:     constants.AndroidSharedStorageRoot,
		FullPath: constants.AndroidSharedStorageRoot,
		IsFile:   false,
	}
	for _, path := range allPaths {
		addAndroidSharedStorageFilePathNode(root, path, fileSet)
	}
	return root, nil
}

func isAndroidSharedStorageFilePathAllowed(path string) bool {
	if strings.Contains(path, "/.") {
		return false // skip hidden dirs like .thumbnails or .cache
	}
	return utils.StringStartsWithAny(path, constants.AndroidAllowedSharedStorageFolders...)
}

func addAndroidSharedStorageFilePathNode(root *models.AndroidFileNode, fullPath string, fileSet map[string]bool) {
	relativePath := strings.TrimPrefix(fullPath, constants.AndroidSharedStorageRoot)
	parts := strings.Split(strings.TrimPrefix(relativePath, "/"), "/")

	current := root
	currentPath := constants.AndroidSharedStorageRoot

	for i, part := range parts {
		if part == "" {
			continue
		}

		if current.Children == nil {
			current.Children = make(map[string]*models.AndroidFileNode)
		}

		currentPath = path.Join(currentPath, part)
		child, exists := current.Children[part]
		if !exists {
			child = &models.AndroidFileNode{
				Name:     part,
				FullPath: currentPath,
				IsFile:   false, // assume folder
			}
			current.Children[part] = child
		}

		// If this is the last segment and it's a known file, mark it
		if i == len(parts)-1 && fileSet[fullPath] {
			child.IsFile = true
			child.Children = nil // remove any children from a file
		}

		current = child
	}
}
