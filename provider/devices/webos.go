package devices

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"GADS/common/models"
	"GADS/common/utils"
	"GADS/provider/config"
	"GADS/provider/logger"
)

// WebOSDevice holds WebOS TV-specific runtime state alongside the shared RuntimeState.
type WebOSDevice struct {
	RuntimeState
	DeviceAddress string // IP address of the WebOS TV (same as UDID for WebOS)
}

// connectedWebOSDevice represents a WebOS device returned by ares-setup-device --list
type connectedWebOSDevice struct {
	name string
	ip   string
}

// Setup runs the full WebOS device provisioning sequence.
func (d *WebOSDevice) Setup() error {
	d.SetupMutex.Lock()
	defer d.SetupMutex.Unlock()

	d.SetProviderState("preparing")
	logger.ProviderLogger.LogInfo("webos_device_setup", fmt.Sprintf("Running setup for WebOS device `%v`", d.GetUDID()))

	d.DBDevice.IPAddress = d.GetUDID()

	if err := setupAppiumForDevice(d); err != nil {
		return err
	}

	d.SetProviderState("live")
	return nil
}

// AppiumCapabilities returns the WebOS-specific Appium server capabilities.
func (d *WebOSDevice) AppiumCapabilities() models.AppiumServerCapabilities {
	chromeDriverPath := filepath.Join(config.ProviderConfig.ProviderFolder, "drivers/chromedriver")
	absolutePath, _ := filepath.Abs(chromeDriverPath)
	return models.AppiumServerCapabilities{
		AutomationName:         "webos",
		PlatformName:           "lgtv",
		UDID:                   d.GetUDID(),
		DeviceHost:             d.GetUDID(),
		DeviceName:             d.DBDevice.Name,
		ChromeDriverExecutable: absolutePath,
	}
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
		if line == "" || strings.Contains(line, "name") || strings.Contains(line, "----") || strings.Contains(line, "emulator") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			var deviceInfo string
			for _, field := range fields {
				if strings.Contains(field, "@") && strings.Contains(field, ":") {
					deviceInfo = field
					break
				}
			}

			if deviceInfo != "" {
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

// InstallApp installs an app on the WebOS device.
func (d *WebOSDevice) InstallApp(appName string) error {
	appPath := fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName)

	if strings.HasSuffix(appName, ".ipk") {
		logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Installing .ipk file directly on device %s", d.GetUDID()))

		installCmd := exec.Command("ares-install", "--device", d.DBDevice.Name, appPath)
		output, err := installCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install .ipk: %s. Output: %s", err, string(output))
		}

		logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Successfully installed app on device %s", d.GetUDID()))
		return nil
	}

	tempDir := fmt.Sprintf("%s/webos_temp_%s", os.TempDir(), d.GetUDID())

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Extracting source code for device %s", d.GetUDID()))

	if err := utils.ExtractZipToDir(appPath, tempDir); err != nil {
		return fmt.Errorf("failed to extract app file: %w", err)
	}

	logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Packaging app for device %s", d.GetUDID()))

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

	logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Installing app on device %s", d.GetUDID()))

	installCmd := exec.Command("ares-install", "--device", d.DBDevice.Name, ipkFile)
	output, err = installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install app: %s. Output: %s", err, string(output))
	}

	logger.ProviderLogger.LogInfo("webos_install_app", fmt.Sprintf("Successfully installed app on device %s", d.GetUDID()))
	return nil
}

// UninstallApp uninstalls an app from the WebOS device.
func (d *WebOSDevice) UninstallApp(appID string) error {
	logger.ProviderLogger.LogInfo("webos_uninstall_app", fmt.Sprintf("Uninstalling app %s from device %s", appID, d.GetUDID()))

	cmd := exec.Command("ares-install", "--device", d.DBDevice.Name, "--remove", appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("webos_uninstall_app", fmt.Sprintf("Failed to uninstall app %s from device %s: %v. Output: %s", appID, d.GetUDID(), err, string(output)))
		return fmt.Errorf("failed to uninstall app %s: %w", appID, err)
	}

	logger.ProviderLogger.LogInfo("webos_uninstall_app", fmt.Sprintf("Successfully uninstalled app %s from device %s", appID, d.GetUDID()))
	return nil
}

type WebOSApp struct {
	AppID     string `json:"appId"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	IsDevApp  bool   `json:"isDevApp"`
	SystemApp bool   `json:"systemApp"`
}

// GetInstalledApps returns installed apps info (returns as []models.DeviceApp for the interface).
func (d *WebOSDevice) GetInstalledApps() ([]models.DeviceApp, error) {
	webosApps := d.getInstalledAppsWebOS()
	var result []models.DeviceApp
	for _, app := range webosApps {
		result = append(result, models.DeviceApp{
			AppName:          app.Title,
			BundleIdentifier: app.AppID,
			CanUninstall:     app.IsDevApp,
		})
	}
	return result, nil
}

func (d *WebOSDevice) getInstalledAppsWebOS() []WebOSApp {
	apps := []WebOSApp{}

	cmd := exec.Command("ares-install", "--device", d.DBDevice.Name, "--listfull")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("webos_list_apps", fmt.Sprintf("Failed to list apps for device %s: %v. Output: %s", d.GetUDID(), err, string(output)))
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

	logger.ProviderLogger.LogInfo("webos_list_apps", fmt.Sprintf("Found %d installed apps on device %s", len(apps), d.GetUDID()))
	return apps
}

// GetInstalledAppBundleIDs returns bundle identifiers of installed apps.
func (d *WebOSDevice) GetInstalledAppBundleIDs() []string {
	var ids []string
	for _, app := range d.getInstalledAppsWebOS() {
		ids = append(ids, app.AppID)
	}
	return ids
}

// LaunchApp launches an app on the WebOS device.
func (d *WebOSDevice) LaunchApp(appID string) error {
	logger.ProviderLogger.LogInfo("webos_launch_app", fmt.Sprintf("Launching app %s on device %s", appID, d.GetUDID()))

	cmd := exec.Command("ares-launch", "--device", d.DBDevice.Name, appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("webos_launch_app", fmt.Sprintf("Failed to launch app %s on device %s: %v. Output: %s", appID, d.GetUDID(), err, string(output)))
		return fmt.Errorf("failed to launch app %s: %w", appID, err)
	}

	logger.ProviderLogger.LogInfo("webos_launch_app", fmt.Sprintf("Successfully launched app %s on device %s", appID, d.GetUDID()))
	return nil
}

// CloseApp closes an app on the WebOS device.
func (d *WebOSDevice) CloseApp(appID string) error {
	logger.ProviderLogger.LogInfo("webos_close_app", fmt.Sprintf("Closing app %s on device %s", appID, d.GetUDID()))

	cmd := exec.Command("ares-launch", "--device", d.DBDevice.Name, "--close", appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("webos_close_app", fmt.Sprintf("Failed to close app %s on device %s: %v. Output: %s", appID, d.GetUDID(), err, string(output)))
		return fmt.Errorf("failed to close app %s: %w", appID, err)
	}

	logger.ProviderLogger.LogInfo("webos_close_app", fmt.Sprintf("Successfully closed app %s on device %s", appID, d.GetUDID()))
	return nil
}

// KillApp kills an app on WebOS (same as CloseApp).
func (d *WebOSDevice) KillApp(appID string) error {
	return d.CloseApp(appID)
}

