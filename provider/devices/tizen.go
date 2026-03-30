package devices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"GADS/common/models"
	"GADS/common/utils"
	"GADS/provider/config"
	"GADS/provider/logger"
)

// TizenDevice holds Tizen TV-specific runtime state alongside the shared RuntimeState.
type TizenDevice struct {
	RuntimeState
	DeviceAddress string // HOST_IP:PORT address of the Tizen TV
}

// Tizen auto-connection constants
const (
	tizenMaxRetries    = 5
	tizenRetryInterval = 30 * time.Second
	tizenPauseAfterMax = 5 * time.Minute
)

// Tizen retry tracking
var (
	tizenRetryTracker = make(map[string]*tizenRetryState)
	tizenRetryMutex   sync.RWMutex
)

type tizenRetryState struct {
	deviceID    string
	retryCount  int
	lastAttempt time.Time
	isPaused    bool
	pauseUntil  time.Time
}

// Setup runs the full Tizen device provisioning sequence.
func (d *TizenDevice) Setup() error {
	d.SetupMutex.Lock()
	defer d.SetupMutex.Unlock()

	d.SetProviderState("preparing")
	logger.ProviderLogger.LogInfo("tizen_device_setup", fmt.Sprintf("Running setup for Tizen device `%v`", d.GetUDID()))

	if err := d.getTVInfo(); err != nil {
		logger.ProviderLogger.LogError("tizen_device_setup", fmt.Sprintf("Failed to get TV info for device `%v` - %v", d.GetUDID(), err))
		d.Reset("Failed to retrieve TV information.")
		return err
	}

	if err := setupAppiumForDevice(d); err != nil {
		return err
	}

	d.SetProviderState("live")
	return nil
}

// AppiumCapabilities returns the Tizen-specific Appium server capabilities.
func (d *TizenDevice) AppiumCapabilities() models.AppiumServerCapabilities {
	chromeDriverPath := filepath.Join(config.ProviderConfig.ProviderFolder, "drivers/chromedriver")
	absolutePath, _ := filepath.Abs(chromeDriverPath)
	return models.AppiumServerCapabilities{
		AutomationName:         "TizenTV",
		PlatformName:           "TizenTV",
		UDID:                   d.GetUDID(),
		DeviceAddress:          d.DeviceAddress,
		DeviceName:             d.DBDevice.Name,
		ChromeDriverExecutable: absolutePath,
	}
}

func getTizenTVHost(tvID string) (string, error) {
	if matched, _ := regexp.MatchString(`^([0-9]{1,3}\.){3}[0-9]{1,3}:\d+$`, tvID); matched {
		host := strings.Split(tvID, ":")[0]
		return host, nil
	}
	return "", fmt.Errorf("invalid format for host: %s", tvID)
}

func (d *TizenDevice) getTVInfo() error {
	tvHost, err := getTizenTVHost(d.GetUDID())
	if err != nil {
		return fmt.Errorf("failed to get TV host - %s", err)
	}

	url := fmt.Sprintf("http://%s:8001/api/v2/", tvHost)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get TV info - %s", err)
	}
	defer resp.Body.Close()

	var tvInfo models.TizenTVInfo
	if err := json.NewDecoder(resp.Body).Decode(&tvInfo); err != nil {
		return fmt.Errorf("failed to decode TV info - %s", err)
	}

	d.DBDevice.HardwareModel = tvInfo.Device.ModelName
	d.DBDevice.OSVersion = tvInfo.Version
	d.DBDevice.IPAddress = tvInfo.Device.IP
	d.DeviceAddress = d.GetUDID()

	if tvInfo.Device.Resolution != "" {
		dimensions := strings.Split(tvInfo.Device.Resolution, "x")
		if len(dimensions) == 2 {
			d.DBDevice.ScreenWidth = dimensions[0]
			d.DBDevice.ScreenHeight = dimensions[1]
		}
	}
	return nil
}

func connectTizenDevice(deviceUDID string) error {
	deviceIP, err := getTizenTVHost(deviceUDID)
	if err != nil {
		return fmt.Errorf("failed to extract IP from device UDID %s: %s", deviceUDID, err)
	}

	cmd := exec.Command("sdb", "connect", deviceIP)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect to Tizen device %s: %s. Output: %s", deviceUDID, err, string(output))
	}
	return nil
}

func isTizenDeviceConnected(deviceUDID string) bool {
	connectedDevices := getConnectedDevicesTizen()
	return slices.Contains(connectedDevices, deviceUDID)
}

func getTizenRetryState(deviceID string) *tizenRetryState {
	tizenRetryMutex.RLock()
	defer tizenRetryMutex.RUnlock()
	return tizenRetryTracker[deviceID]
}

func updateTizenRetryState(deviceID string, retryCount int, lastAttempt time.Time, isPaused bool, pauseUntil time.Time) {
	tizenRetryMutex.Lock()
	defer tizenRetryMutex.Unlock()
	tizenRetryTracker[deviceID] = &tizenRetryState{
		deviceID:    deviceID,
		retryCount:  retryCount,
		lastAttempt: lastAttempt,
		isPaused:    isPaused,
		pauseUntil:  pauseUntil,
	}
}

func resetTizenRetryState(deviceID string) {
	tizenRetryMutex.Lock()
	defer tizenRetryMutex.Unlock()
	if _, exists := tizenRetryTracker[deviceID]; exists {
		tizenRetryTracker[deviceID] = &tizenRetryState{deviceID: deviceID}
	}
}

func shouldAttemptTizenConnection(deviceID string) bool {
	state := getTizenRetryState(deviceID)
	now := time.Now()

	if state == nil {
		updateTizenRetryState(deviceID, 0, time.Time{}, false, time.Time{})
		return true
	}

	if state.isPaused {
		if now.Before(state.pauseUntil) {
			return false
		}
		updateTizenRetryState(deviceID, 0, time.Time{}, false, time.Time{})
		return true
	}

	if state.retryCount >= tizenMaxRetries {
		pauseUntil := now.Add(tizenPauseAfterMax)
		updateTizenRetryState(deviceID, state.retryCount, state.lastAttempt, true, pauseUntil)
		return false
	}

	if !state.lastAttempt.IsZero() && now.Sub(state.lastAttempt) < tizenRetryInterval {
		return false
	}
	return true
}

func handleTizenAutoConnection(connectedDevices []string) {
	for _, dev := range DevManager.All() {
		if dev.GetOS() != "tizen" || dev.GetDBDevice().Usage == "disabled" {
			continue
		}

		udid := dev.GetUDID()
		isConnectedViaSdb := isTizenDeviceConnected(udid)
		isInConnectedList := slices.Contains(connectedDevices, udid)

		if isConnectedViaSdb {
			state := getTizenRetryState(udid)
			if state != nil && state.retryCount > 0 {
				resetTizenRetryState(udid)
			}
		} else if !isInConnectedList {
			if shouldAttemptTizenConnection(udid) {
				attemptTizenConnection(udid)
			}
		}
	}
}

func attemptTizenConnection(deviceUDID string) {
	state := getTizenRetryState(deviceUDID)
	if state == nil {
		updateTizenRetryState(deviceUDID, 0, time.Time{}, false, time.Time{})
		state = getTizenRetryState(deviceUDID)
	}

	now := time.Now()
	newRetryCount := state.retryCount + 1

	err := connectTizenDevice(deviceUDID)
	if err != nil {
		updateTizenRetryState(deviceUDID, newRetryCount, now, false, time.Time{})
		if newRetryCount >= tizenMaxRetries {
			pauseUntil := now.Add(tizenPauseAfterMax)
			updateTizenRetryState(deviceUDID, newRetryCount, now, true, pauseUntil)
		}
	} else {
		resetTizenRetryState(deviceUDID)
	}
}

func getTizenCertificateName() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %s", err)
	}

	certDir := fmt.Sprintf("%s/SamsungCertificate", homeDir)
	entries, err := os.ReadDir(certDir)
	if err != nil {
		return "", fmt.Errorf("tizen certificate not found in %s: %s", certDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return entry.Name(), nil
		}
	}
	return "", fmt.Errorf("no certificate directory found in %s", certDir)
}

// InstallApp installs an app on the Tizen device.
func (d *TizenDevice) InstallApp(appName string) error {
	certName, err := getTizenCertificateName()
	if err != nil {
		return err
	}

	appPath := fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName)
	tempDir := fmt.Sprintf("%s/tizen_temp_%s", os.TempDir(), d.GetUDID())

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if strings.HasSuffix(appName, ".wgt") {
		if err := utils.ExtractZipToDir(appPath, tempDir); err != nil {
			return fmt.Errorf("failed to extract .wgt file: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported file format: %s. Expected .wgt", appName)
	}

	packageCmd := exec.Command("tizen", "package", "-t", "wgt", "-s", certName, "--", tempDir)
	output, err := packageCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to package app: %s. Output: %s", err, string(output))
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp directory: %s", err)
	}

	var wgtFile string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".wgt") {
			wgtFile = fmt.Sprintf("%s/%s", tempDir, entry.Name())
			break
		}
	}
	if wgtFile == "" {
		return fmt.Errorf("no .wgt file found after packaging")
	}

	installCmd := exec.Command("tizen", "install", "-n", wgtFile, "-s", d.GetUDID())
	output, err = installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install app: %s. Output: %s", err, string(output))
	}
	return nil
}

// UninstallApp uninstalls an app from the Tizen device.
func (d *TizenDevice) UninstallApp(appID string) error {
	cmd := exec.Command("tizen", "uninstall", "-s", d.GetUDID(), "-p", appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to uninstall app %s: %s. Output: %s", appID, err, string(output))
	}
	return nil
}

type TizenApp struct {
	AppID       string `json:"appId"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	IsDevApp    bool   `json:"isDevApp"`
	IsSystemApp bool   `json:"isSystemApp"`
}

// GetInstalledApps returns installed apps info (returns as []models.DeviceApp for the interface).
func (d *TizenDevice) GetInstalledApps() ([]models.DeviceApp, error) {
	tizenApps := d.getInstalledAppsTizen()
	var result []models.DeviceApp
	for _, app := range tizenApps {
		result = append(result, models.DeviceApp{
			AppName:          app.Title,
			BundleIdentifier: app.AppID,
			CanUninstall:     app.IsDevApp,
		})
	}
	return result, nil
}

func (d *TizenDevice) getInstalledAppsTizen() []TizenApp {
	apps := []TizenApp{}

	cmd := exec.Command("sdb", "-s", d.GetUDID(), "shell", "0", "vd_applist")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("tizen_list_apps", fmt.Sprintf("Failed to list apps for device %s: %v", d.GetUDID(), err))
		return apps
	}

	lines := strings.Split(string(output), "\n")
	var currentApp TizenApp
	var appType string
	var installedSourceType string
	inAppBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "----") && len(trimmed) > 50 && !strings.Contains(line, "=") {
			if inAppBlock && currentApp.AppID != "" {
				currentApp.IsDevApp = (appType == "user" || installedSourceType == "0")
				currentApp.IsSystemApp = false
				apps = append(apps, currentApp)
			}
			currentApp = TizenApp{}
			appType = ""
			installedSourceType = ""
			inAppBlock = true
			continue
		}

		if inAppBlock && strings.Contains(line, "=") {
			parts := strings.Split(line, "=")
			if len(parts) >= 2 {
				key := strings.TrimSpace(strings.ReplaceAll(parts[0], "-", ""))
				value := strings.Trim(strings.TrimSpace(parts[1]), "-")

				switch key {
				case "app_tizen_id":
					currentApp.AppID = value
				case "app_title":
					currentApp.Title = value
				case "app_version":
					currentApp.Version = value
				case "type":
					appType = value
				case "installed_source_type":
					installedSourceType = value
				}
			}
		}
	}

	if inAppBlock && currentApp.AppID != "" {
		currentApp.IsDevApp = (appType == "user" || installedSourceType == "0")
		currentApp.IsSystemApp = false
		apps = append(apps, currentApp)
	}
	return apps
}

// GetInstalledAppBundleIDs returns bundle identifiers of installed apps.
func (d *TizenDevice) GetInstalledAppBundleIDs() []string {
	var ids []string
	for _, app := range d.getInstalledAppsTizen() {
		ids = append(ids, app.AppID)
	}
	return ids
}

// LaunchApp launches an app on the Tizen device.
func (d *TizenDevice) LaunchApp(appID string) error {
	cmd := exec.Command("tizen", "run", "-s", d.GetUDID(), "-p", appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to launch app %s: %s. Output: %s", appID, err, string(output))
	}
	return nil
}

// CloseApp closes an app on the Tizen device.
func (d *TizenDevice) CloseApp(appID string) error {
	cmd := exec.Command("sdb", "-s", d.GetUDID(), "shell", "0", "was_kill", appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to close app %s: %s. Output: %s", appID, err, string(output))
	}
	return nil
}

// KillApp kills an app on Tizen (same as CloseApp).
func (d *TizenDevice) KillApp(appID string) error {
	return d.CloseApp(appID)
}

// --- Legacy exported functions used by the provider router ---

func GetInstalledAppsTizen(device *models.Device) []TizenApp {
	dev, ok := DevManager.Get(device.UDID)
	if !ok {
		return []TizenApp{}
	}
	tizenDev, ok := dev.(*TizenDevice)
	if !ok {
		return []TizenApp{}
	}
	return tizenDev.getInstalledAppsTizen()
}

func LaunchAppTizen(device *models.Device, appID string) error {
	dev, ok := DevManager.Get(device.UDID)
	if !ok {
		return fmt.Errorf("device not found")
	}
	return dev.LaunchApp(appID)
}

func CloseAppTizen(device *models.Device, appID string) error {
	dev, ok := DevManager.Get(device.UDID)
	if !ok {
		return fmt.Errorf("device not found")
	}
	tizenDev, ok := dev.(*TizenDevice)
	if !ok {
		return fmt.Errorf("device is not Tizen")
	}
	return tizenDev.CloseApp(appID)
}
