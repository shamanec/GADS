package devices

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"GADS/common/models"
	"GADS/provider/logger"
)

func setupTizenDevice(device *models.Device) {
	device.SetupMutex.Lock()
	defer device.SetupMutex.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)

	device.ProviderState = "preparing"
	logger.ProviderLogger.LogInfo("tizen_device_setup", fmt.Sprintf("Running setup for Tizen device `%v`", device.UDID))

	// Iniciar o emparelhamento remoto com a TV e capturar o token
	rcToken, err := pairRemoteWithTizenTV(device)
	if err != nil {
		ResetLocalDevice(device, "Failed to pair remote with Tizen TV")
		return
	}

	// Armazenar o token no dispositivo
	device.RCToken = rcToken
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

func pairRemoteWithTizenTV(device *models.Device) (string, error) {
	tvHost, err := getTizenTVHost(device.UDID)
	if err != nil {
		return "", fmt.Errorf("failed to get TV host - %s", err)
	}

	cmd := exec.Command("appium", "driver", "run", "tizentv", "pair-remote", "--host", tvHost)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to pair remote with Tizen TV - %s: %s", err, string(output))
	}

	token := extractTokenFromOutput(string(output))
	if token == "" {
		return "", fmt.Errorf("pairing token not found in output")
	}

	logger.ProviderLogger.LogInfo("tizen_device_setup", "Remote pairing initiated successfully. Please accept the pairing on the TV.")

	return token, nil
}

func extractTokenFromOutput(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "pairing token") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

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
