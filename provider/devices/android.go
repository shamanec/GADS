package devices

import (
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Check if the GADS-stream service is running on the device
func isGadsStreamServiceRunning(device *models.Device) (bool, error) {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Checking if GADS-stream is already running on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "dumpsys", "activity", "services", "com.shamanec.stream/.ScreenCaptureService")
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
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "am", "stopservice", "com.shamanec.stream/.ScreenCaptureService")

	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogWarn("android_device_setup", fmt.Sprintf("Failed to stop GADS-stream service properly - %s", err))
	}
}

// Install gads-stream.apk on the device
func installGadsStream(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Installing GADS-stream apk on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "install", "-r", fmt.Sprintf("%s/gads-stream.apk", config.Config.EnvConfig.ProviderFolder))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("installGadsStream: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Uninstall the GADS stream app from the Android device
func uninstallGadsStream(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Uninstalling GADS-stream from device `%v`", device.UDID))

	return UninstallApp(device, "com.shamanec.stream")
}

// Add recording permissions to gads-stream app to avoid popup on start
func addGadsStreamRecordingPermissions(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Adding GADS-stream recording permissions on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "appops", "set", "com.shamanec.stream", "PROJECT_MEDIA", "allow")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("addGadsStreamRecordingPermissions: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Start the gads-stream app using adb
func startGadsStreamApp(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Starting GADS-stream app on `%s`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "am", "start", "-n", "com.shamanec.stream/com.shamanec.stream.ScreenCaptureActivity")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("startGadsStreamApp: Error executing `%s` - %s", cmd.Args, err)
	}

	return nil
}

// Press the Home button using adb to hide the transparent gads-stream activity
func pressHomeButton(device *models.Device) {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Pressing Home button with adb on device `%v`", device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "input", "keyevent", "KEYCODE_HOME")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("pressHomeButton: Could not 'press' Home button with `%v`, you need to press it yourself to hide the transparent activity of GADS-stream:\n %v", cmd.Path, err))
	}
}

// Forward the GADS stream tcp to a host port that is already assigned for the device
func forwardGadsStream(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Trying to forward GADS-stream port(1991) to host port `%v` for device `%s`", device.StreamPort, device.UDID))

	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "forward", "tcp:"+device.StreamPort, "tcp:1991")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("forwardGadsStream: Error executing `%s` while trying to forward GADS-stream socket to host - %s", cmd.Args, err)
	}

	return nil
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

	return nil
}

// Get all installed apps on an Android device
func getInstalledAppsAndroid(device *models.Device) []string {
	var installedApps []string
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "cmd", "package", "list", "packages", "-3")

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("getInstalledAppsAndroid: Error executing `%s` trying to get installed apps - %v", cmd.Args, err))
		return installedApps
	}

	// Get the command output to string
	result := strings.TrimSpace(outBuffer.String())
	// Get all lines with package names
	lines := strings.Split(result, "\n")

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
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "install", "-r", fmt.Sprintf("%s/%s", config.Config.EnvConfig.ProviderFolder, appName))

	if err := cmd.Run(); err != nil {
		device.Logger.LogError("install_app", fmt.Sprintf("installAppAndroid: Error executing `%s` trying to install app - %v", cmd.Args, err))
		return err
	}

	return nil
}
