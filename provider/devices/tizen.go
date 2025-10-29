package devices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"GADS/common/cli"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
	"GADS/provider/providerutil"
)

// Tizen auto-connection constants
const (
	tizenMaxRetries    = 5                // Maximum consecutive connection attempts
	tizenRetryInterval = 30 * time.Second // Interval between connection attempts
	tizenPauseAfterMax = 5 * time.Minute  // Pause duration after max retries reached
)

// Tizen retry tracking
var (
	tizenRetryTracker = make(map[string]*tizenRetryState)
	tizenRetryMutex   sync.RWMutex
)

// tizenRetryState tracks connection attempts for a Tizen device
type tizenRetryState struct {
	deviceID    string
	retryCount  int
	lastAttempt time.Time
	isPaused    bool
	pauseUntil  time.Time
}

func setupTizenDevice(device *models.Device) {
	device.SetupMutex.Lock()
	defer device.SetupMutex.Unlock()

	device.ProviderState = "preparing"
	logger.ProviderLogger.LogInfo("tizen_device_setup", fmt.Sprintf("Running setup for Tizen device `%v`", device.UDID))

	err := cli.KillDeviceAppiumProcess(device.UDID)
	if err != nil {
		logger.ProviderLogger.LogError("tizen_device_setup", fmt.Sprintf("Failed attempt to kill existing Appium processes for device `%s` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to kill existing Appium processes.")
		return
	}

	appiumPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("tizen_device_setup", fmt.Sprintf("Could not allocate free host port for Appium for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to allocate free host port for Appium")
		return
	}
	device.AppiumPort = appiumPort

	err = getTizenTVInfo(device)
	if err != nil {
		logger.ProviderLogger.LogError("tizen_device_setup", fmt.Sprintf("Failed to get TV info for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to retrieve TV information.")
		return
	}

	go startAppium(device)

	timeout := time.After(30 * time.Second)
	tick := time.Tick(200 * time.Millisecond)
AppiumLoop:
	for {
		select {
		case <-timeout:
			logger.ProviderLogger.LogError("tizen_device_setup", fmt.Sprintf("Did not successfully start Appium for device `%v` in 60 seconds", device.UDID))
			ResetLocalDevice(device, "Failed to start Appium for device.")
			return
		case <-tick:
			if device.IsAppiumUp {
				logger.ProviderLogger.LogInfo("tizen_device_setup", fmt.Sprintf("Successfully started Appium for device `%v` on port %v", device.UDID, device.AppiumPort))
				break AppiumLoop
			}
		}
	}

	device.ProviderState = "live"
}

func getTizenTVHost(tvID string) (string, error) {
	// Check if the hostWithPort is in the format HOST_IP:PORT
	if matched, _ := regexp.MatchString(`^([0-9]{1,3}\.){3}[0-9]{1,3}:\d+$`, tvID); matched {
		host := strings.Split(tvID, ":")[0]
		return host, nil
	} else {
		return "", fmt.Errorf("invalid format for host: %s", tvID)
	}
}

func getTizenTVInfo(device *models.Device) error {
	tvHost, err := getTizenTVHost(device.UDID)
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

	// Update device information
	device.HardwareModel = tvInfo.Device.ModelName
	device.OSVersion = tvInfo.Version
	device.IPAddress = tvInfo.Device.IP
	device.DeviceAddress = device.UDID

	// Extract dimensions from resolution
	if tvInfo.Device.Resolution != "" {
		dimensions := strings.Split(tvInfo.Device.Resolution, "x")
		if len(dimensions) == 2 {
			device.ScreenWidth = dimensions[0]
			device.ScreenHeight = dimensions[1]
		}
	}

	return nil
}

// connectTizenDevice establishes a connection to a Tizen device using sdb connect
func connectTizenDevice(deviceUDID string) error {
	deviceIP, err := getTizenTVHost(deviceUDID)
	if err != nil {
		return fmt.Errorf("failed to extract IP from device UDID %s: %s", deviceUDID, err)
	}

	logger.ProviderLogger.LogInfo("tizen_connection", fmt.Sprintf("Attempting to connect to Tizen device %s (IP: %s)", deviceUDID, deviceIP))

	cmd := exec.Command("sdb", "connect", deviceIP)
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.ProviderLogger.LogError("tizen_connection", fmt.Sprintf("Failed to connect to Tizen device %s (IP: %s) - %s. Output: %s", deviceUDID, deviceIP, err, string(output)))
		return fmt.Errorf("failed to connect to Tizen device %s: %s", deviceUDID, err)
	}

	logger.ProviderLogger.LogInfo("tizen_connection", fmt.Sprintf("Successfully connected to Tizen device %s (IP: %s). Output: %s", deviceUDID, deviceIP, string(output)))
	return nil
}

// isTizenDeviceConnected checks if a Tizen device is currently connected using sdb devices
func isTizenDeviceConnected(deviceUDID string) bool {
	connectedDevices := getConnectedDevicesTizen()

	if slices.Contains(connectedDevices, deviceUDID) {
		logger.ProviderLogger.LogDebug("tizen_connection", fmt.Sprintf("Tizen device %s is connected", deviceUDID))
		return true
	}

	logger.ProviderLogger.LogDebug("tizen_connection", fmt.Sprintf("Tizen device %s is not connected", deviceUDID))
	return false
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
		tizenRetryTracker[deviceID] = &tizenRetryState{
			deviceID:    deviceID,
			retryCount:  0,
			lastAttempt: time.Time{},
			isPaused:    false,
			pauseUntil:  time.Time{},
		}
	}
}

func shouldAttemptTizenConnection(deviceID string) bool {
	state := getTizenRetryState(deviceID)
	now := time.Now()

	if state == nil {
		// First time seeing this device, initialize state
		updateTizenRetryState(deviceID, 0, time.Time{}, false, time.Time{})
		return true
	}

	// If device is paused, check if pause period has ended
	if state.isPaused {
		if now.Before(state.pauseUntil) {
			return false // Still in pause period
		}
		// Pause period ended, reset retry count
		updateTizenRetryState(deviceID, 0, time.Time{}, false, time.Time{})
		return true
	}

	if state.retryCount >= tizenMaxRetries {
		// Max retries reached, enter pause mode
		pauseUntil := now.Add(tizenPauseAfterMax)
		updateTizenRetryState(deviceID, state.retryCount, state.lastAttempt, true, pauseUntil)
		logger.ProviderLogger.LogWarn("tizen_auto_connect", fmt.Sprintf("Tizen device %s reached max retries (%d), pausing until %v", deviceID, tizenMaxRetries, pauseUntil))
		return false
	}

	// Check if enough time has passed since last attempt
	if !state.lastAttempt.IsZero() && now.Sub(state.lastAttempt) < tizenRetryInterval {
		return false // Not enough time has passed
	}

	return true
}

// handleTizenAutoConnection checks registered Tizen devices and attempts automatic connections
func handleTizenAutoConnection(connectedDevices []string) {
	for _, dbDevice := range DBDeviceMap {
		// Only process Tizen devices that are enabled and registered
		if dbDevice.OS != "tizen" || dbDevice.Usage == "disabled" {
			continue
		}

		isConnectedViaSdb := isTizenDeviceConnected(dbDevice.UDID)

		isInConnectedList := slices.Contains(connectedDevices, dbDevice.UDID)

		if isConnectedViaSdb {
			state := getTizenRetryState(dbDevice.UDID)
			if state != nil && state.retryCount > 0 {
				logger.ProviderLogger.LogInfo("tizen_auto_connect", fmt.Sprintf("Tizen device %s is now connected, resetting retry count", dbDevice.UDID))
				resetTizenRetryState(dbDevice.UDID)
			}
		} else if !isInConnectedList {
			if shouldAttemptTizenConnection(dbDevice.UDID) {
				attemptTizenConnection(dbDevice.UDID)
			}
		}
	}
}

// attemptTizenConnection tries to connect to a Tizen device and updates retry state
func attemptTizenConnection(deviceUDID string) {
	state := getTizenRetryState(deviceUDID)
	if state == nil {
		updateTizenRetryState(deviceUDID, 0, time.Time{}, false, time.Time{})
		state = getTizenRetryState(deviceUDID)
	}

	now := time.Now()
	newRetryCount := state.retryCount + 1

	logger.ProviderLogger.LogInfo("tizen_auto_connect", fmt.Sprintf("Attempting to connect to Tizen device %s - attempt %d/%d", deviceUDID, newRetryCount, tizenMaxRetries))

	err := connectTizenDevice(deviceUDID)
	if err != nil {
		logger.ProviderLogger.LogWarn("tizen_auto_connect", fmt.Sprintf("Failed to connect to Tizen device %s - attempt %d/%d: %v", deviceUDID, newRetryCount, tizenMaxRetries, err))
		updateTizenRetryState(deviceUDID, newRetryCount, now, false, time.Time{})

		if newRetryCount >= tizenMaxRetries {
			pauseUntil := now.Add(tizenPauseAfterMax)
			updateTizenRetryState(deviceUDID, newRetryCount, now, true, pauseUntil)
			logger.ProviderLogger.LogWarn("tizen_auto_connect", fmt.Sprintf("Tizen device %s reached max retries (%d), pausing until %v", deviceUDID, tizenMaxRetries, pauseUntil))
		}
	} else {
		logger.ProviderLogger.LogInfo("tizen_auto_connect", fmt.Sprintf("Successfully connected to Tizen device %s", deviceUDID))
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
		return "", fmt.Errorf("tizen certificate not found in %s. Please configure certificate as per documentation: %s", certDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			logger.ProviderLogger.LogInfo("tizen_certificate", fmt.Sprintf("Using Tizen certificate: %s", entry.Name()))
			return entry.Name(), nil
		}
	}

	return "", fmt.Errorf("no certificate directory found in %s. Please configure certificate as per documentation", certDir)
}

func installAppTizen(device *models.Device, appName string) error {
	certName, err := getTizenCertificateName()
	if err != nil {
		logger.ProviderLogger.LogError("tizen_install_app", err.Error())
		return err
	}

	appPath := fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName)
	tempDir := fmt.Sprintf("%s/tizen_temp_%s", os.TempDir(), device.UDID)

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if strings.HasSuffix(appName, ".wgt") {
		logger.ProviderLogger.LogInfo("tizen_install_app", fmt.Sprintf("Extracting .wgt file for device %s", device.UDID))

		if err := extractZipToDir(appPath, tempDir); err != nil {
			return fmt.Errorf("failed to extract .wgt file: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported file format: %s. Expected .wgt", appName)
	}

	logger.ProviderLogger.LogInfo("tizen_install_app", fmt.Sprintf("Packaging app with certificate %s for device %s", certName, device.UDID))

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

	logger.ProviderLogger.LogInfo("tizen_install_app", fmt.Sprintf("Installing app on device %s", device.UDID))

	installCmd := exec.Command("tizen", "install", "-n", wgtFile, "-s", device.UDID)
	output, err = installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install app: %s. Output: %s", err, string(output))
	}

	logger.ProviderLogger.LogInfo("tizen_install_app", fmt.Sprintf("Successfully installed app on device %s", device.UDID))
	return nil
}
