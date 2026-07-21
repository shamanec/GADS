package devices

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
)

// AndroidTvDevice holds Android TV-specific runtime state alongside the shared RuntimeState.
type AndroidTvDevice struct {
	RuntimeState
	DeviceAddress string // HOST_IP:PORT address of the Android TV
}

// Setup runs the full Android TV device provisioning sequence.
func (d *AndroidTvDevice) Setup() error {
	d.SetupMutex.Lock()
	defer d.SetupMutex.Unlock()

	d.SetProviderState("preparing")
	logger.ProviderLogger.LogInfo("androidtv_device_setup", fmt.Sprintf("Running setup for Android TV device `%v`", d.GetUDID()))

	if err := connectAndroidTvDevice(d.GetUDID()); err != nil {
		logger.ProviderLogger.LogError("androidtv_device_setup", fmt.Sprintf("Failed to connect to Android TV device `%v` - %v", d.GetUDID(), err))
		d.Reset("Failed to connect to Android TV.")
		return err
	}

	d.getTVInfo()
	d.DeviceAddress = d.GetUDID()

	if err := setupAppiumForDevice(d); err != nil {
		return err
	}

	d.SetProviderState("live")
	return nil
}

// AppiumCapabilities returns the Android TV-specific Appium server capabilities.
func (d *AndroidTvDevice) AppiumCapabilities() models.AppiumServerCapabilities {
	return models.AppiumServerCapabilities{
		AutomationName: "UiAutomator2",
		PlatformName:   "Android",
		UDID:           d.GetUDID(),
		DeviceName:     d.DBDevice.Name,
	}
}

func connectAndroidTvDevice(deviceUDID string) error {
	cmd := exec.Command("adb", "connect", deviceUDID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect to Android TV device %s: %s. Output: %s", deviceUDID, err, string(output))
	}
	if strings.Contains(string(output), "failed") || strings.Contains(string(output), "cannot") {
		return fmt.Errorf("failed to connect to Android TV device %s. Output: %s", deviceUDID, string(output))
	}
	return nil
}

// handleAndroidTvAutoConnection issues `adb connect` for configured Android TV devices
// that aren't connected yet. TVs connect over the network (IP:PORT) and won't show up in
// `adb devices` until connected, so without this the device never reaches Setup. The first
// connect for an unauthorized TV triggers the on-device authorization prompt; the periodic
// retry picks the device up once it's authorized. `adb connect` is idempotent.
func handleAndroidTvAutoConnection(connectedDevices []string) {
	for _, dev := range DevManager.All() {
		if dev.GetOS() != "androidtv" || dev.GetDBDevice().Usage == "disabled" {
			continue
		}

		udid := dev.GetUDID()
		if slices.Contains(connectedDevices, udid) {
			continue
		}

		if err := connectAndroidTvDevice(udid); err != nil {
			logger.ProviderLogger.LogDebug("androidtv_autoconnect", fmt.Sprintf("Auto-connect attempt for Android TV %s failed: %v", udid, err))
		}
	}
}

func (d *AndroidTvDevice) getTVInfo() {
	brand := d.getProp("ro.product.brand")
	model := d.getProp("ro.product.model")
	d.HardwareModel = strings.TrimSpace(fmt.Sprintf("%s %s", brand, model))
}

func (d *AndroidTvDevice) getProp(prop string) string {
	cmd := exec.Command("adb", "-s", d.GetUDID(), "shell", "getprop", prop)
	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		logger.ProviderLogger.LogError("androidtv_device_setup", fmt.Sprintf("Failed to get prop %s for device %s: %v", prop, d.GetUDID(), err))
		return ""
	}
	return strings.TrimSpace(outBuffer.String())
}

// InstallApp installs an app on the Android TV device.
func (d *AndroidTvDevice) InstallApp(appName string) error {
	appPath := fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName)
	cmd := exec.Command("adb", "-s", d.GetUDID(), "install", "-r", appPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install app %s: %s. Output: %s", appName, err, string(output))
	}
	return nil
}

// UninstallApp uninstalls an app from the Android TV device.
func (d *AndroidTvDevice) UninstallApp(packageName string) error {
	cmd := exec.Command("adb", "-s", d.GetUDID(), "uninstall", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to uninstall app %s: %s. Output: %s", packageName, err, string(output))
	}
	return nil
}

// GetInstalledApps returns installed apps info (returns as []models.DeviceApp for the interface).
func (d *AndroidTvDevice) GetInstalledApps() ([]models.DeviceApp, error) {
	var result []models.DeviceApp
	for _, packageName := range d.getInstalledAppsAndroidTv() {
		result = append(result, models.DeviceApp{
			AppName:          packageName,
			BundleIdentifier: packageName,
			CanUninstall:     true,
		})
	}
	return result, nil
}

func (d *AndroidTvDevice) getInstalledAppsAndroidTv() []string {
	installedApps := make([]string, 0)

	cmd := exec.Command("adb", "-s", d.GetUDID(), "shell", "cmd", "package", "list", "packages", "-3")
	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		logger.ProviderLogger.LogError("androidtv_list_apps", fmt.Sprintf("Failed to list apps for device %s: %v", d.GetUDID(), err))
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

// GetInstalledAppBundleIDs returns bundle identifiers (package names) of installed apps.
func (d *AndroidTvDevice) GetInstalledAppBundleIDs() []string {
	return d.getInstalledAppsAndroidTv()
}

// LaunchApp launches an app on the Android TV device.
func (d *AndroidTvDevice) LaunchApp(packageName string) error {
	cmd := exec.Command("adb", "-s", d.GetUDID(), "shell", "monkey", "-p", packageName, "-c", "android.intent.category.LAUNCHER", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to launch app %s: %s. Output: %s", packageName, err, string(output))
	}
	return nil
}

// KillApp force-stops an app on the Android TV device.
func (d *AndroidTvDevice) KillApp(packageName string) error {
	cmd := exec.Command("adb", "-s", d.GetUDID(), "shell", "am", "force-stop", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to kill app %s: %s. Output: %s", packageName, err, string(output))
	}
	return nil
}
