package devices

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"GADS/common/cli"
	"GADS/common/models"
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

	if err := extractZipToDir(appPath, tempDir); err != nil {
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
