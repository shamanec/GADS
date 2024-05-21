package providerutil

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"GADS/provider/config"
	"GADS/provider/logger"
)

var mu sync.Mutex
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

// Check if xcodebuild is available on the host by checking its version
func XcodebuildAvailable() bool {
	logger.ProviderLogger.LogInfo("provider_setup", "Checking if xcodebuild is set up and available on the host (Xcode is installed)")

	cmd := exec.Command("xcodebuild", "-version")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider_setup", fmt.Sprintf("xcodebuildAvailable: xcodebuild is not available or command failed - %s", err))
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

// Check if go-ios binary is available
func GoIOSAvailable() bool {
	logger.ProviderLogger.LogInfo("provider_setup", "Checking if go-ios binary is set up and available on the host PATH")

	cmd := exec.Command("ios", "-h")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider_setup", fmt.Sprintf("goIOSAvailable: go-ios is not available on host or command failed - %s", err))
		return false
	}
	return true
}

// Build WebDriverAgent for testing with `xcodebuild`
func BuildWebDriverAgent() error {
	cmd := exec.Command("xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "generic/platform=iOS", "build-for-testing", "-derivedDataPath", "./build")
	cmd.Dir = config.Config.EnvConfig.WdaRepoPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	logger.ProviderLogger.LogInfo("provider_setup", fmt.Sprintf("Starting WebDriverAgent using xcodebuild in path `%s` with command `%s` ", config.Config.EnvConfig.WdaRepoPath, cmd.String()))
	if err := cmd.Start(); err != nil {
		return err
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		logger.ProviderLogger.LogDebug("webdriveragent_xcodebuild", line)
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("provider_setup", fmt.Sprintf("buildWebDriverAgent: Error waiting for build WebDriverAgent with `xcodebuild` command to finish - %s", err))
		logger.ProviderLogger.LogError("provider_setup", "buildWebDriverAgent: Building WebDriverAgent for testing was unsuccessful")
		os.Exit(1)
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

// Check if gads-stream.apk is available and if not - download the latest release
func CheckGadsStreamAndDownload() error {
	if isGadsStreamApkAvailable() {
		logger.ProviderLogger.LogInfo("provider_setup", "GADS-stream apk is available in the provider `conf` folder, it will not be downloaded. If you want to get the latest release, delete the file from conf folder and re-run the provider")
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

// Check if the gads-stream.apk file is located in the provider `conf` folder
func isGadsStreamApkAvailable() bool {
	_, err := os.Stat(fmt.Sprintf("%s/conf/gads-stream.apk", config.Config.EnvConfig.ProviderFolder))
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

// Download the latest release of GADS-Android-stream and put the apk in the provider `conf` folder
func downloadGadsStreamApk() error {
	logger.ProviderLogger.LogInfo("provider", "Downloading latest GADS-stream release apk file")
	outFile, err := os.Create(fmt.Sprintf("%s/conf/gads-stream.apk", config.Config.EnvConfig.ProviderFolder))
	if err != nil {
		return fmt.Errorf("Could not create file at %s/conf/gads-stream.apk - %s", config.Config.EnvConfig.ProviderFolder, err)
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

func GetAllAppFiles() []string {
	file, err := os.Open(fmt.Sprintf("%s/apps", config.Config.EnvConfig.ProviderFolder))
	if err != nil {
		logger.ProviderLogger.LogError("provider", fmt.Sprintf("Could not os.Open() apps directory - %s", err))
		return []string{}
	}
	defer file.Close()

	fileList, err := file.Readdir(-1)
	if err != nil {
		logger.ProviderLogger.LogError("provider", fmt.Sprintf("Could not Readdir on the apps directory - %s", err))
		return []string{}
	}

	var files []string
	for _, file := range fileList {
		files = append(files, file.Name())
		fmt.Println(file.Size())
	}

	return files
}
