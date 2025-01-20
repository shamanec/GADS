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
	"sync"
	"time"

	"GADS/common/cli"
	"GADS/provider/config"
	"GADS/provider/logger"
)

var mu sync.RWMutex
var UsedPorts = make(map[string]bool)
var gadsStreamURL = "https://github.com/shamanec/GADS-Android-stream/releases/latest/download/gads-stream.apk"

// Use this function to get a free port on the host for any service that might need one
// We keep a map of used ports so we don't allocate same ports to different services
func GetFreePort() (string, error) {
	mu.Lock()
	defer mu.Unlock()

	a, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", fmt.Errorf("Failed to resolve tcp address trying to get new port - %s", err)
	}

	var l *net.TCPListener
	l, err = net.ListenTCP("tcp", a)
	if err != nil {
		return "", fmt.Errorf("Failed to listen tcp trying to get new port - %s", err)
	}
	defer l.Close()

	portInt := l.Addr().(*net.TCPAddr).Port
	portString := strconv.Itoa(portInt)
	if _, ok := UsedPorts[portString]; ok {
		return GetFreePort()
	}
	UsedPorts[portString] = true
	return portString, nil
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

// Remove all adb forwarded ports(if any) on provider start
func RemoveAdbForwardedPorts() {
	logger.ProviderLogger.LogInfo("provider_setup", "Attempting to remove all `adb` forwarded ports on provider start")

	cmd := exec.Command("adb", "forward", "--remove-all")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider_setup", fmt.Sprintf("removeAdbForwardedPorts: Could not remove `adb` forwarded ports, there was an error or no devices are connected - %s", err))
	}
}

// Check if gads-stream.apk is available and if not - download the latest release
func CheckGadsStreamAndDownload() error {
	if isGadsStreamApkAvailable() {
		logger.ProviderLogger.LogInfo("provider_setup", "GADS-stream apk is available in the provider folder, it will not be downloaded. If you want to get the latest release, delete the file from conf folder and re-run the provider")
		return nil
	}

	err := downloadGadsStreamApk()
	if err != nil {
		return err
	}

	if !isGadsStreamApkAvailable() {
		return fmt.Errorf("GADS-stream download was reported successful but the .apk was not actually downloaded")
	}

	logger.ProviderLogger.LogInfo("provider_setup", "Latest GADS-stream release apk was successfully downloaded")
	return nil
}

// Check if the gads-stream.apk file is located in the provider folder
func isGadsStreamApkAvailable() bool {
	_, err := os.Stat(fmt.Sprintf("%s/gads-stream.apk", config.ProviderConfig.ProviderFolder))
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

// Download the latest release of GADS-Android-stream and put the apk in the provider folder
func downloadGadsStreamApk() error {
	logger.ProviderLogger.LogInfo("provider", "Downloading latest GADS-stream release apk file")
	outFile, err := os.Create(fmt.Sprintf("%s/gads-stream.apk", config.ProviderConfig.ProviderFolder))
	if err != nil {
		return fmt.Errorf("Could not create file at %s/gads-stream.apk - %s", config.ProviderConfig.ProviderFolder, err)
	}
	defer outFile.Close()

	req, err := http.NewRequest(http.MethodGet, gadsStreamURL, nil)
	if err != nil {
		return fmt.Errorf("Could not create new request - %s", err)
	}

	var netClient = &http.Client{
		Timeout: time.Second * 240,
	}
	resp, err := netClient.Do(req)
	if err != nil {
		return fmt.Errorf("Could not execute request to download - %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP response error: %s", resp.Status)
	}

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("Could not copy the response data to the file at apps/gads-stream.apk - %s", err)
	}

	return nil
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
