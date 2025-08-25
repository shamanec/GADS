/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package providerutil

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"runtime"

	"GADS/common"
	"GADS/common/cli"
	"GADS/common/utils"
	"GADS/provider/config"
	"GADS/provider/logger"

	"github.com/Masterminds/semver"
)

var UsedPorts = make(map[string]bool)
var gadsStreamURL = "https://github.com/shamanec/GADS-Android-stream/releases/latest/download/gads-stream.apk"

// Use this function to get a free port on the host for any service that might need one
// We keep a map of used ports so we don't allocate same ports to different services
func GetFreePort() (string, error) {
	const (
		maxAttempts = 10
		baseBackoff = 50 * time.Millisecond
	)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.ProviderLogger.LogDebug("port_allocation", "Trying to get a free port")

		a, err := net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			logger.ProviderLogger.LogError("port_allocation", fmt.Sprintf("Failed to resolve tcp address trying to get new port - %s", err))
			return "", fmt.Errorf("Failed to resolve tcp address trying to get new port - %s", err)
		}
		logger.ProviderLogger.LogDebug("port_allocation", "Resolved TCP address successfully")

		logger.ProviderLogger.LogDebug("port_allocation", "Attempting to listen on the resolved TCP address")
		l, err := net.ListenTCP("tcp", a)
		if err != nil {
			logger.ProviderLogger.LogError("port_allocation", fmt.Sprintf("Failed to listen tcp trying to get new port - %s", err))
			return "", fmt.Errorf("Failed to listen tcp trying to get new port - %s", err)
		}
		logger.ProviderLogger.LogDebug("port_allocation", "Listening on TCP address successfully")

		portInt := l.Addr().(*net.TCPAddr).Port
		portString := strconv.Itoa(portInt)

		logger.ProviderLogger.LogDebug("port_allocation", fmt.Sprintf("Acquired free port: %s", portString))

		logger.ProviderLogger.LogDebug("port_allocation", "Attempting to acquire lock for used ports")
		common.MutexManager.LocalDevicePorts.Lock()
		logger.ProviderLogger.LogDebug("port_allocation", "Successfully acquired lock for used ports")
		if _, exists := UsedPorts[portString]; !exists {
			UsedPorts[portString] = true
			logger.ProviderLogger.LogDebug("port_allocation", fmt.Sprintf("Port %s is free and has been allocated", portString))
			logger.ProviderLogger.LogDebug("port_allocation", "Releasing lock for used ports")
			common.MutexManager.LocalDevicePorts.Unlock()
			l.Close()
			return portString, nil
		}
		logger.ProviderLogger.LogDebug("port_allocation", fmt.Sprintf("Port %s is already in use, trying again", portString))
		logger.ProviderLogger.LogDebug("port_allocation", "Releasing lock for used ports")
		common.MutexManager.LocalDevicePorts.Unlock()
		l.Close()

		// Simple incremental backoff
		time.Sleep(time.Duration(attempt) * baseBackoff)
	}
	return "", fmt.Errorf("failed to find a free port after %d attempts", maxAttempts)
}

// Check if adb is available on the host by starting the server
func AdbAvailable() bool {
	logger.ProviderLogger.LogInfo("provider_setup", "Checking if adb is set up and available on the host PATH")

	cmd := exec.Command("adb", "start-server")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider_setup", fmt.Sprintf("adbAvailable: Error executing `adb start-server`, `adb` is not available on host or command failed - %s", err))
		return false
	}

	return true
}

// Check if Appium is installed and available on the host by checking its version
func AppiumAvailable() bool {
	logger.ProviderLogger.LogInfo("provider_setup", "Checking if Appium is set up and available on the host PATH")

	cmd := exec.Command("appium", "--version")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider_setup", fmt.Sprintf("AppiumAvailable: Appium is not available or command failed - %s", err))
		return false
	}
	return true
}

// Check if the GADS Appium plugin is installed on NPM
func IsAppiumPluginInstalledNPM() bool {
	cmd := exec.Command("npm", "list", "-g", "appium-gads", "--depth=0")
	err := cmd.Run()
	if err != nil {
		return false
	}
	return true
}

// Get the version of the Appium plugin currently installed on NPM
func GetAppiumPluginNPMVersion() (string, error) {
	cmd := exec.Command("npm", "list", "-g", "appium-gads", "--depth=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "appium-gads@") {
			parts := strings.Split(line, "@")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}
	return "", fmt.Errorf("Could not get GADS Appium plugin version on NPM")
}

// Check if the currently installed Appium GADS plugin version corresponds to the expected one for the current GADS binary
func ShouldUpdateAppiumPluginNPM(targetVersion string) bool {
	currentVersion, err := GetAppiumPluginNPMVersion()
	if err != nil {
		return false
	}

	targetSemver := semver.MustParse(targetVersion)
	currentSemver := semver.MustParse(currentVersion)
	versionCompareResult := targetSemver.Compare(currentSemver)

	return versionCompareResult != 0
}

// Install the GADS Appium plugin on NPM
func InstallAppiumPluginNPM(targetVersion string) error {
	cmd := exec.Command("npm", "install", "-g", fmt.Sprintf("appium-gads@%s", targetVersion))

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// Check if the GADS plugin is installed on Appium
func IsAppiumPluginInstalled() bool {
	cmd := exec.Command("appium", "plugin", "list")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "gads") {
			return true
		}
	}
	return false
}

// Install the GADS Appium plugin on Appium
func InstallAppiumPlugin(targetVersion string) error {
	cmd := exec.Command("appium", "plugin", "install", "--source=npm", fmt.Sprintf("appium-gads@%s", targetVersion))

	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("provider_setup", fmt.Sprintf("Failed to install GADS Appium plugin - %s", string(out)))
		return err
	}
	return nil
}

// Uninstall the GADS Appium plugin from Appium
func UninstallAppiumPlugin() error {
	cmd := exec.Command("appium", "plugin", "uninstall", "gads")

	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("provider_setup", fmt.Sprintf("Failed to uninstall GADS Appium plugin - %s", string(out)))
		return err
	}
	return nil
}

// Update the GADS Appium plugin on Appium
func UpdateAppiumPlugin() error {
	cmd := exec.Command("appium", "plugin", "update", "gads")

	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.ProviderLogger.LogError("provider_setup", fmt.Sprintf("Failed to install GADS Appium plugin - %s", string(out)))
		return err
	}
	return nil
}

// Remove all adb forwarded ports(if any) on provider start
func RemoveAdbForwardedPorts() {
	logger.ProviderLogger.LogInfo("provider_setup", "Attempting to remove all `adb` forwarded ports on provider start")

	cmd := exec.Command("adb", "forward", "--remove-all")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider_setup", fmt.Sprintf("removeAdbForwardedPorts: Could not remove `adb` forwarded ports, there was an error or no devices are connected - %s", err))
	}
}

func GetAppiumVersion() (string, error) {
	versionOutput, err := cli.ExecuteCommand("appium", "-v")
	if err != nil {
		return "", err
	}

	return versionOutput, nil
}

func GetAppiumDriverVersion(driverName string) (string, error) {
	output, err := cli.ExecuteCommand("appium", "driver", "list")
	if err != nil {
		return "", err
	}
	// Appium driver list has coloured output
	// So we must strip the ANSI color codes
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	cleanedOutput := ansiRegex.ReplaceAllString(output, "")

	re := regexp.MustCompile(fmt.Sprintf(`(?m)-\s%s@([\d\.]+)\s\[installed`, driverName))
	match := re.FindStringSubmatch(cleanedOutput)
	if match == nil {
		return "", fmt.Errorf("driver %s not installed", driverName)
	}

	return match[1], nil
}

func GetXCUITestDriverVersion() (string, error) {
	return GetAppiumDriverVersion("xcuitest")
}

func GetUiAutomator2DriverVersion() (string, error) {
	return GetAppiumDriverVersion("uiautomator2")
}

// Check if sdb is available on the host by checking its version
func SdbAvailable() bool {
	logger.ProviderLogger.LogInfo("provider_setup", "Checking if sdb is set up and available on the host PATH")

	cmd := exec.Command("sdb", "version")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider_setup", fmt.Sprintf("sdbAvailable: sdb is not available or command failed - %s", err))
		return false
	}
	return true
}

// Check if ares-setup-device is available on the host by checking its version
func AresAvailable() bool {
	logger.ProviderLogger.LogInfo("provider_setup", "Checking if ares-setup-device is set up and available on the host PATH")

	cmd := exec.Command("ares", "-V")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider_setup", fmt.Sprintf("aresAvailable: ares-setup-device is not available or command failed - %s", err))
		return false
	}
	return true
}

// Check if chromedriver is located in the drivers provider folder
func isChromeDriverAvailable() bool {
	var chromedriverPath string
	if runtime.GOOS == "windows" {
		chromedriverPath = fmt.Sprintf("%s/drivers/chromedriver.exe", config.ProviderConfig.ProviderFolder)
	} else {
		chromedriverPath = fmt.Sprintf("%s/drivers/chromedriver", config.ProviderConfig.ProviderFolder)
	}

	_, err := os.Stat(chromedriverPath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

func CheckChromeDriverAndDownload() error {
	if isChromeDriverAvailable() {
		logger.ProviderLogger.LogInfo("provider_setup", "Chromedriver is available in the drivers provider folder, it will not be downloaded.")
		return nil
	}

	var url string
	switch runtime.GOOS {
	case "linux":
		url = "https://chromedriver.storage.googleapis.com/2.36/chromedriver_linux64.zip"
	case "darwin":
		url = "https://chromedriver.storage.googleapis.com/2.36/chromedriver_mac64.zip"
	case "windows":
		url = "https://chromedriver.storage.googleapis.com/2.36/chromedriver_win32.zip"
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Download the zip file
	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download ChromeDriver: %s", err)
	}
	defer response.Body.Close()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "chromedriver.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %s", err)
	}
	defer os.Remove(tempFile.Name())

	// Write the response body to the temp file
	_, err = io.Copy(tempFile, response.Body)
	if err != nil {
		return fmt.Errorf("failed to write to temp file: %s", err)
	}

	// Define the extraction directory as the provider folder
	extractionDir := fmt.Sprintf("%s/drivers", config.ProviderConfig.ProviderFolder)

	// Extract the zip file
	err = utils.Unzip(tempFile.Name(), extractionDir) // Specify the extraction directory
	if err != nil {
		return fmt.Errorf("failed to extract ChromeDriver: %s", err)
	}

	return nil
}
