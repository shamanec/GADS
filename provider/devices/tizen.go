package devices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
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
		resetLocalDevice(device, "Failed to kill existing Appium processes.")
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

	device.OS = "tizen"

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
