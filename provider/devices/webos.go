package devices

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"GADS/common/cli"
	"GADS/common/models"
	"GADS/common/utils"
	"GADS/provider/config"
	"GADS/provider/logger"
	"GADS/provider/providerutil"
)

// connectedWebOSDevice represents a WebOS device returned by ares-setup-device --list
type connectedWebOSDevice struct {
	name string
	ip   string
}

func setupWebOSDevice(device *models.Device) {
	device.SetupMutex.Lock()
	defer device.SetupMutex.Unlock()

	device.ProviderState = "preparing"
	logger.ProviderLogger.LogInfo("webos_device_setup", fmt.Sprintf("Running setup for WebOS device `%v`", device.UDID))

	err := cli.KillDeviceAppiumProcess(device.UDID)
	if err != nil {
		logger.ProviderLogger.LogError("webos_device_setup", fmt.Sprintf("Failed attempt to kill existing Appium processes for device `%s` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to kill existing Appium processes.")
		return
	}

	appiumPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("webos_device_setup", fmt.Sprintf("Could not allocate free host port for Appium for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free host port for Appium")
		return
	}
	device.AppiumPort = appiumPort
	device.IPAddress = device.UDID

	go startAppium(device)

	timeout := time.After(30 * time.Second)
	tick := time.Tick(200 * time.Millisecond)
AppiumLoop:
	for {
		select {
		case <-timeout:
			logger.ProviderLogger.LogError("webos_device_setup", fmt.Sprintf("Did not successfully start Appium for device `%v` in 30 seconds", device.UDID))
			ResetLocalDevice(device, "Failed to start Appium for device.")
			return
		case <-tick:
			if device.IsAppiumUp {
				logger.ProviderLogger.LogInfo("webos_device_setup", fmt.Sprintf("Successfully started Appium for device `%v` on port %v", device.UDID, device.AppiumPort))
				break AppiumLoop
			}
		}
	}

	device.ProviderState = "live"
}

// getConnectedDevicesWebOS gets the connected WebOS devices using ares-setup-device
func getConnectedDevicesWebOS() []string {
	cmd := exec.Command("ares-setup-device", "--list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("webos_device_detection", fmt.Sprintf("Failed to get WebOS devices: %s", err))
		return []string{}
	}

	var connectedDevices []string
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		// Skip empty lines, header and emulator lines
		if line == "" || strings.Contains(line, "name") || strings.Contains(line, "----") || strings.Contains(line, "emulator") {
			continue
		}

		// ares-setup-device --list output format:
		// name            deviceinfo                connection  profile
		// TVLG (default)  prisoner@10.1.16.22:9922  ssh         tv
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			// Find the field that contains @ and : (deviceinfo field)
			var deviceInfo string
			for _, field := range fields {
				if strings.Contains(field, "@") && strings.Contains(field, ":") {
					deviceInfo = field
					break
				}
			}

			// Extract IP from deviceinfo if found
			if deviceInfo != "" {
				// Format is user@IP:PORT, extract just the IP
				parts := strings.Split(deviceInfo, "@")
				if len(parts) == 2 {
					ipPort := parts[1]
					ipParts := strings.Split(ipPort, ":")
					if len(ipParts) >= 1 {
						ip := ipParts[0]
						connectedDevices = append(connectedDevices, ip)
					}
				}
			}
		}
	}

	return connectedDevices
}

func installAppWebOS(device *models.Device, appName string) error {
	appPath := fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName)

	if strings.HasSuffix(appName, ".ipk") {
		logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Installing .ipk file directly on device %s", device.UDID))

		installCmd := exec.Command("ares-install", "--device", device.Name, appPath)
		output, err := installCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install .ipk: %s. Output: %s", err, string(output))
		}

		logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Successfully installed app on device %s", device.UDID))
		return nil
	}

	tempDir := fmt.Sprintf("%s/webos_temp_%s", os.TempDir(), device.UDID)

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Extracting source code for device %s", device.UDID))

	if err := utils.ExtractZipToDir(appPath, tempDir); err != nil {
		return fmt.Errorf("failed to extract app file: %w", err)
	}

	logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Packaging app for device %s", device.UDID))

	packageCmd := exec.Command("ares-package", tempDir)
	output, err := packageCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to package app: %s. Output: %s", err, string(output))
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read current directory: %s", err)
	}

	var ipkFile string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".ipk") {
			ipkFile = entry.Name()
			break
		}
	}

	if ipkFile == "" {
		return fmt.Errorf("no .ipk file found after packaging")
	}

	defer os.Remove(ipkFile)

	logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Installing app on device %s", device.UDID))

	installCmd := exec.Command("ares-install", "--device", device.Name, ipkFile)
	output, err = installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install app: %s. Output: %s", err, string(output))
	}

	logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Successfully installed app on device %s", device.UDID))
	return nil
}

type WebOSApp struct {
	AppID     string `json:"appId"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	IsDevApp  bool   `json:"isDevApp"`  // always true (ares-install lists only dev apps)
	SystemApp bool   `json:"systemApp"` // parsed from output (always false for CLI apps)
}

func GetInstalledAppsWebOS(device *models.Device) []WebOSApp {
	apps := []WebOSApp{}

	cmd := exec.Command("ares-install", "--device", device.Name, "--listfull")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("webos_list_apps", fmt.Sprintf("Failed to list apps for device %s: %v. Output: %s", device.UDID, err, string(output)))
		return apps
	}

	lines := strings.Split(string(output), "\n")
	var currentApp WebOSApp

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			if currentApp.AppID != "" {
				currentApp.IsDevApp = true
				apps = append(apps, currentApp)
				currentApp = WebOSApp{}
			}
			continue
		}

		if strings.Contains(line, " : ") {
			parts := strings.SplitN(line, " : ", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "id":
					currentApp.AppID = value
				case "title":
					currentApp.Title = value
				case "version":
					currentApp.Version = value
				case "systemApp":
					currentApp.SystemApp = (value == "true")
				}
			}
		}
	}

	if currentApp.AppID != "" {
		currentApp.IsDevApp = true
		apps = append(apps, currentApp)
	}

	logger.ProviderLogger.LogInfo("webos_list_apps", fmt.Sprintf("Found %d installed apps on device %s", len(apps), device.UDID))
	return apps
}

func LaunchAppWebOS(device *models.Device, appID string) error {
	logger.ProviderLogger.LogInfo("webos_launch_app", fmt.Sprintf("Launching app %s on device %s", appID, device.UDID))

	cmd := exec.Command("ares-launch", "--device", device.Name, appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("webos_launch_app", fmt.Sprintf("Failed to launch app %s on device %s: %v. Output: %s", appID, device.UDID, err, string(output)))
		return fmt.Errorf("failed to launch app %s: %w", appID, err)
	}

	logger.ProviderLogger.LogInfo("webos_launch_app", fmt.Sprintf("Successfully launched app %s on device %s", appID, device.UDID))
	return nil
}

func CloseAppWebOS(device *models.Device, appID string) error {
	logger.ProviderLogger.LogInfo("webos_close_app", fmt.Sprintf("Closing app %s on device %s", appID, device.UDID))

	cmd := exec.Command("ares-launch", "--device", device.Name, "--close", appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("webos_close_app", fmt.Sprintf("Failed to close app %s on device %s: %v. Output: %s", appID, device.UDID, err, string(output)))
		return fmt.Errorf("failed to close app %s: %w", appID, err)
	}

	logger.ProviderLogger.LogInfo("webos_close_app", fmt.Sprintf("Successfully closed app %s on device %s", appID, device.UDID))
	return nil
}

func uninstallAppWebOS(device *models.Device, appID string) error {
	logger.ProviderLogger.LogInfo("webos_uninstall_app", fmt.Sprintf("Uninstalling app %s from device %s", appID, device.UDID))

	cmd := exec.Command("ares-install", "--device", device.Name, "--remove", appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("webos_uninstall_app", fmt.Sprintf("Failed to uninstall app %s from device %s: %v. Output: %s", appID, device.UDID, err, string(output)))
		return fmt.Errorf("failed to uninstall app %s: %w", appID, err)
	}

	logger.ProviderLogger.LogInfo("webos_uninstall_app", fmt.Sprintf("Successfully uninstalled app %s from device %s", appID, device.UDID))
	return nil
}
