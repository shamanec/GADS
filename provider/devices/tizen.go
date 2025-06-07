package devices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"GADS/common/cli"
	"GADS/common/models"
	"GADS/provider/logger"
	"GADS/provider/providerutil"
)

type TizenTVInfo struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Version   string      `json:"version"`
	Device    TizenDevice `json:"device"`
	Type      string      `json:"type"`
	URI       string      `json:"uri"`
	Remote    string      `json:"remote"`
	IsSupport string      `json:"isSupport"`
}

type TizenDevice struct {
	Type              string `json:"type"`
	DUID              string `json:"duid"`
	Model             string `json:"model"`
	ModelName         string `json:"modelName"`
	Description       string `json:"description"`
	NetworkType       string `json:"networkType"`
	SSID              string `json:"ssid"`
	IP                string `json:"ip"`
	FirmwareVersion   string `json:"firmwareVersion"`
	Name              string `json:"name"`
	ID                string `json:"id"`
	UDN               string `json:"udn"`
	Resolution        string `json:"resolution"`
	CountryCode       string `json:"countryCode"`
	MSFVersion        string `json:"msfVersion"`
	SmartHubAgreement string `json:"smartHubAgreement"`
	VoiceSupport      string `json:"VoiceSupport"`
	GamePadSupport    string `json:"GamePadSupport"`
	WifiMac           string `json:"wifiMac"`
	DeveloperMode     string `json:"developerMode"`
	DeveloperIP       string `json:"developerIP"`
	OS                string `json:"OS"`
}

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
	deviceIP    string
	retryCount  int
	lastAttempt time.Time
	isPaused    bool
	pauseUntil  time.Time
}

func setupTizenDevice(device *models.Device) {
	device.SetupMutex.Lock()
	defer device.SetupMutex.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)

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

	go startAppium(device, &wg)
	go checkAppiumUp(device)

	select {
	case <-device.AppiumReadyChan:
		logger.ProviderLogger.LogInfo("tizen_device_setup", fmt.Sprintf("Successfully started Appium for device `%v` on port %v", device.UDID, device.AppiumPort))
		break
	case <-time.After(30 * time.Second):
		logger.ProviderLogger.LogError("tizen_device_setup", fmt.Sprintf("Did not successfully start Appium for device `%v` in 60 seconds", device.UDID))
		ResetLocalDevice(device, "Appium did not start within the expected time.")
		return
	}

	device.ProviderState = "live"
	wg.Wait()
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

	var tvInfo TizenTVInfo
	if err := json.NewDecoder(resp.Body).Decode(&tvInfo); err != nil {
		return fmt.Errorf("failed to decode TV info - %s", err)
	}

	// Atualizar informações do device
	device.Name = tvInfo.Device.Name
	device.HardwareModel = tvInfo.Device.ModelName
	device.OSVersion = tvInfo.Version
	device.IPAddress = tvInfo.Device.IP
	device.DeviceAddress = device.UDID

	// Extrair dimensões da resolução
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
func connectTizenDevice(deviceIP string) error {
	logger.ProviderLogger.LogInfo("tizen_connection", fmt.Sprintf("Attempting to connect to Tizen device at %s", deviceIP))

	cmd := exec.Command("sdb", "connect", deviceIP)
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.ProviderLogger.LogError("tizen_connection", fmt.Sprintf("Failed to connect to Tizen device %s - %s. Output: %s", deviceIP, err, string(output)))
		return fmt.Errorf("failed to connect to Tizen device %s: %s", deviceIP, err)
	}

	logger.ProviderLogger.LogInfo("tizen_connection", fmt.Sprintf("Successfully connected to Tizen device %s. Output: %s", deviceIP, string(output)))
	return nil
}

// isTizenDeviceConnected checks if a Tizen device is currently connected using sdb devices
func isTizenDeviceConnected(deviceIP string) bool {
	cmd := exec.Command("sdb", "devices")
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.ProviderLogger.LogError("tizen_connection", fmt.Sprintf("Failed to check connected Tizen devices - %s", err))
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "List of devices attached") || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "device" {
			connectedDeviceID := fields[0]
			// Check if the connected device ID matches our device IP
			if connectedDeviceID == deviceIP {
				logger.ProviderLogger.LogDebug("tizen_connection", fmt.Sprintf("Tizen device %s is connected", deviceIP))
				return true
			}
		}
	}

	logger.ProviderLogger.LogDebug("tizen_connection", fmt.Sprintf("Tizen device %s is not connected", deviceIP))
	return false
}

func getTizenRetryState(deviceIP string) *tizenRetryState {
	tizenRetryMutex.RLock()
	defer tizenRetryMutex.RUnlock()
	return tizenRetryTracker[deviceIP]
}

func updateTizenRetryState(deviceIP string, retryCount int, lastAttempt time.Time, isPaused bool, pauseUntil time.Time) {
	tizenRetryMutex.Lock()
	defer tizenRetryMutex.Unlock()

	tizenRetryTracker[deviceIP] = &tizenRetryState{
		deviceIP:    deviceIP,
		retryCount:  retryCount,
		lastAttempt: lastAttempt,
		isPaused:    isPaused,
		pauseUntil:  pauseUntil,
	}
}

func resetTizenRetryState(deviceIP string) {
	tizenRetryMutex.Lock()
	defer tizenRetryMutex.Unlock()

	if _, exists := tizenRetryTracker[deviceIP]; exists {
		tizenRetryTracker[deviceIP] = &tizenRetryState{
			deviceIP:    deviceIP,
			retryCount:  0,
			lastAttempt: time.Time{},
			isPaused:    false,
			pauseUntil:  time.Time{},
		}
	}
}

func shouldAttemptTizenConnection(deviceIP string) bool {
	state := getTizenRetryState(deviceIP)
	now := time.Now()

	if state == nil {
		// First time seeing this device, initialize state
		updateTizenRetryState(deviceIP, 0, time.Time{}, false, time.Time{})
		return true
	}

	// If device is paused, check if pause period has ended
	if state.isPaused {
		if now.Before(state.pauseUntil) {
			return false // Still in pause period
		}
		// Pause period ended, reset retry count
		updateTizenRetryState(deviceIP, 0, time.Time{}, false, time.Time{})
		return true
	}

	if state.retryCount >= tizenMaxRetries {
		// Max retries reached, enter pause mode
		pauseUntil := now.Add(tizenPauseAfterMax)
		updateTizenRetryState(deviceIP, state.retryCount, state.lastAttempt, true, pauseUntil)
		logger.ProviderLogger.LogWarn("tizen_auto_connect", fmt.Sprintf("Tizen device %s reached max retries (%d), pausing until %v", deviceIP, tizenMaxRetries, pauseUntil))
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

		deviceIP, err := getTizenTVHost(dbDevice.UDID)
		if err != nil {
			logger.ProviderLogger.LogError("tizen_auto_connect", fmt.Sprintf("Failed to extract IP from device UDID %s: %v", dbDevice.UDID, err))
			continue
		}

		isConnectedViaSdb := isTizenDeviceConnected(deviceIP)

		isInConnectedList := slices.Contains(connectedDevices, dbDevice.UDID)

		if isConnectedViaSdb {
			state := getTizenRetryState(deviceIP)
			if state != nil && state.retryCount > 0 {
				logger.ProviderLogger.LogInfo("tizen_auto_connect", fmt.Sprintf("Tizen device %s (%s) is now connected, resetting retry count", dbDevice.UDID, deviceIP))
				resetTizenRetryState(deviceIP)
			}
		} else if !isInConnectedList {
			if shouldAttemptTizenConnection(deviceIP) {
				attemptTizenConnection(deviceIP, dbDevice.UDID)
			}
		}
	}
}

// attemptTizenConnection tries to connect to a Tizen device and updates retry state
func attemptTizenConnection(deviceIP, deviceUDID string) {
	state := getTizenRetryState(deviceIP)
	if state == nil {
		updateTizenRetryState(deviceIP, 0, time.Time{}, false, time.Time{})
		state = getTizenRetryState(deviceIP)
	}

	now := time.Now()
	newRetryCount := state.retryCount + 1

	logger.ProviderLogger.LogInfo("tizen_auto_connect", fmt.Sprintf("Attempting to connect to Tizen device %s (%s) - attempt %d/%d", deviceUDID, deviceIP, newRetryCount, tizenMaxRetries))

	err := connectTizenDevice(deviceIP)
	if err != nil {
		logger.ProviderLogger.LogWarn("tizen_auto_connect", fmt.Sprintf("Failed to connect to Tizen device %s (%s) - attempt %d/%d: %v", deviceUDID, deviceIP, newRetryCount, tizenMaxRetries, err))
		updateTizenRetryState(deviceIP, newRetryCount, now, false, time.Time{})

		if newRetryCount >= tizenMaxRetries {
			pauseUntil := now.Add(tizenPauseAfterMax)
			updateTizenRetryState(deviceIP, newRetryCount, now, true, pauseUntil)
			logger.ProviderLogger.LogWarn("tizen_auto_connect", fmt.Sprintf("Tizen device %s (%s) reached max retries (%d), pausing until %v", deviceUDID, deviceIP, tizenMaxRetries, pauseUntil))
		}
	} else {
		logger.ProviderLogger.LogInfo("tizen_auto_connect", fmt.Sprintf("Successfully connected to Tizen device %s (%s)", deviceUDID, deviceIP))
		resetTizenRetryState(deviceIP)
	}
}
