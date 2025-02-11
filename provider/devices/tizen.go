package devices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"GADS/common/models"
	"GADS/provider/logger"
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

	// Iniciar o emparelhamento remoto com a TV e capturar o token
	// rcToken, err := pairRemoteWithTizenTV(device)
	// if err != nil {
	// 	resetLocalDevice(device)
	// 	return
	// }

	err := getTizenTVInfo(device)
	if err != nil {
		logger.ProviderLogger.LogError("tizen_device_setup", fmt.Sprintf("Failed to get TV info for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to pair remote with Tizen TV")
		return
	}

	// Armazenar o token no dispositivo
	// device.RCToken = rcToken
	device.OS = "tizen"

	go startAppium(device, &wg)
	go checkAppiumUp(device)

	select {
	case <-device.AppiumReadyChan:
		logger.ProviderLogger.LogInfo("tizen_device_setup", fmt.Sprintf("Successfully started Appium for device `%v` on port %v", device.UDID, device.AppiumPort))
		break
	case <-time.After(30 * time.Second):
		logger.ProviderLogger.LogError("tizen_device_setup", fmt.Sprintf("Did not successfully start Appium for device `%v` in 60 seconds", device.UDID))
		ResetLocalDevice(device, "Failed to start Appium")
		return
	}

	device.ProviderState = "live"
	wg.Wait()
}

// func pairRemoteWithTizenTV(device *models.Device) (string, error) {
// 	tvHost, err := getTizenTVHost(device.UDID)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get TV host - %s", err)
// 	}

// 	cmd := exec.Command("appium", "driver", "run", "tizentv", "pair-remote", "--host", tvHost)
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to pair remote with Tizen TV - %s: %s", err, string(output))
// 	}

// 	token := extractTokenFromOutput(string(output))
// 	if token == "" {
// 		return "", fmt.Errorf("pairing token not found in output")
// 	}

// 	logger.ProviderLogger.LogInfo("tizen_device_setup", "Remote pairing initiated successfully. Please accept the pairing on the TV.")

// 	return token, nil
// }

// func extractTokenFromOutput(output string) string {
// 	lines := strings.Split(output, "\n")
// 	for _, line := range lines {
// 		if strings.Contains(line, "pairing token") {
// 			parts := strings.Split(line, ":")
// 			if len(parts) > 1 {
// 				return strings.TrimSpace(parts[1])
// 			}
// 		}
// 	}
// 	return ""
// }

func getTizenTVHost(tvID string) (string, error) {
	cmd := exec.Command("sdb", "devices")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get Tizen devices - %s", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "List of devices attached") || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "device" && fields[len(fields)-1] == tvID {
			hostWithPort := fields[0]
			host := strings.Split(hostWithPort, ":")[0]
			return host, nil
		}
	}

	return "", fmt.Errorf("TV with ID %s not found in connected devices", tvID)
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
