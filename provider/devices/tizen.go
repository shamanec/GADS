package devices

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"GADS/common/models"
	"GADS/provider/logger"
)

func setupTizenDevice(device *models.Device) {
	device.ProviderState = "preparing"
	logger.ProviderLogger.LogInfo("tizen_device_setup", fmt.Sprintf("Running setup for Tizen device `%v`", device.UDID))

	// Iniciar o emparelhamento remoto com a TV e capturar o token
	rcToken, err := pairRemoteWithTizenTV(device)
	if err != nil {
		resetLocalDevice(device)
		return
	}

	// Armazenar o token no dispositivo
	device.RCToken = rcToken
	device.OS = "tizen"

	go startAppium(device)
	go checkAppiumUp(device)

	select {
	case <-device.AppiumReadyChan:
		logger.ProviderLogger.LogInfo("tizen_device_setup", fmt.Sprintf("Successfully started Appium for device `%v` on port %v", device.UDID, device.AppiumPort))
		break
	case <-time.After(30 * time.Second):
		logger.ProviderLogger.LogError("tizen_device_setup", fmt.Sprintf("Did not successfully start Appium for device `%v` in 60 seconds", device.UDID))
		resetLocalDevice(device)
		return
	}
}

func pairRemoteWithTizenTV(device *models.Device) (string, error) {
	cmd := exec.Command("appium", "driver", "run", "tizentv", "pair-remote", "--host", device.IPAddress)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to pair remote with Tizen TV - %s: %s", err, string(output))
	}

	// Extrair o token de pareamento do output
	token := extractTokenFromOutput(string(output))
	if token == "" {
		return "", fmt.Errorf("pairing token not found in output")
	}

	logger.ProviderLogger.LogInfo("tizen_device_setup", "Remote pairing initiated successfully. Please accept the pairing on the TV.")

	return token, nil
}

func extractTokenFromOutput(output string) string {
	// Lógica para extrair o token do output
	// Supondo que o token é impresso em uma linha específica, você pode ajustar conforme necessário
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "pairing token") { // Ajuste a condição conforme necessário
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1]) // Retorna o token
			}
		}
	}
	return ""
}
