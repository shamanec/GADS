package devices

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"

	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"
)

// Forward iOS device ports using `go-ios` CLI, for some reason using the library doesn't work properly
func goIOSForward(device *models.Device, hostPort string, devicePort string) {
	cmd := exec.CommandContext(device.Context, "ios",
		"forward",
		hostPort,
		devicePort,
		fmt.Sprintf("--udid=%s", device.UDID))
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("goIOSForward: Forwarding port with command `%s`", cmd.Args))

	// Start the port forward command
	err := cmd.Start()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("goIOSForward: Error executing `ios forward` for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("goIOSForward: Error waiting `ios forward` to finish for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
}

// Start the prebuilt WebDriverAgent with `xcodebuild`
func startWdaWithXcodebuild(device *models.Device) {
	cmd := exec.CommandContext(device.Context, "xcodebuild",
		"-project", "WebDriverAgent.xcodeproj",
		"-scheme", "WebDriverAgentRunner",
		"-destination", "platform=iOS,id="+device.UDID,
		"-derivedDataPath", "./build",
		"test-without-building")
	cmd.Dir = config.ProviderConfig.WdaRepoPath
	logger.ProviderLogger.LogDebug("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Starting WebDriverAgent with command `%v`", cmd.Args))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		device.Logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Error creating stdoutpipe while running WebDriverAgent with xcodebuild for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if err := cmd.Start(); err != nil {
		device.Logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Could not start WebDriverAgent with xcodebuild for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()

		//device.Logger.LogInfo("webdriveragent", strings.TrimSpace(line))

		if strings.Contains(line, "Restarting after") {
			resetLocalDevice(device)
			return
		}
	}

	if err := cmd.Wait(); err != nil {
		device.Logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Error waiting for WebDriverAgent(xcodebuild) command to finish, it errored out or device `%v` was disconnected - %v", device.UDID, err))
		resetLocalDevice(device)
	}
}

// Create a new WebDriverAgent session and update stream settings
func updateWebDriverAgent(device *models.Device) error {
	logger.ProviderLogger.LogDebug("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Updating WebDriverAgent session and mjpeg stream settings for device `%s`", device.UDID))

	err := createWebDriverAgentSession(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Could not create WebDriverAgent session for device %v - %v", device.UDID, err))
		return err
	}

	err = updateWebDriverAgentStreamSettings(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Could not update WebDriverAgent stream settings for device %v - %v", device.UDID, err))
		return err
	}

	return nil
}

func updateWebDriverAgentStreamSettings(device *models.Device) error {
	// Set 30 frames per second, without any scaling, half the original screenshot quality
	// TODO should make this configurable in some way, although can be easily updated the same way
	requestString := `{"settings": {"mjpegServerFramerate": 30, "mjpegServerScreenshotQuality": 75, "mjpegScalingFactor": 100}}`

	// Post the mjpeg server settings
	response, err := http.Post("http://localhost:"+device.WDAPort+"/session/"+device.WDASessionID+"/appium/settings", "application/json", strings.NewReader(requestString))
	if err != nil {
		return err
	}

	// TODO - potentially read the body to supply in the error
	if response.StatusCode != 200 {
		return fmt.Errorf("updateWebDriverAgentStreamSettings: Could not successfully update WDA stream settings, status code=%v", response.StatusCode)
	}

	return nil
}

// Create a new WebDriverAgent session
func createWebDriverAgentSession(device *models.Device) error {
	requestString := `{
		"capabilities": {
			"firstMatch": [{}],
			"alwaysMatch": {
				
			}
		}
	}`

	req, err := http.NewRequest(http.MethodPost, "http://localhost:"+device.WDAPort+"/session", strings.NewReader(requestString))
	if err != nil {
		return err
	}

	response, err := netClient.Do(req)
	if err != nil {
		return err
	}

	// Get the response into a byte slice
	responseBody, _ := io.ReadAll(response.Body)
	// Unmarshal response into a basic map
	var responseJson map[string]interface{}
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		return err
	}

	// Check the session ID from the map
	if responseJson["sessionId"] == "" {
		if err != nil {
			return fmt.Errorf("createWebDriverAgentSession: Could not get `sessionId` while creating a new WebDriverAgent session")
		}
	}

	device.WDASessionID = fmt.Sprintf("%v", responseJson["sessionId"])
	return nil
}

func startXCTestWithGoIOS(device *models.Device, bundleId string, xctestConfig string) {
	cmd := exec.CommandContext(context.Background(),
		"ios",
		"runtest",
		fmt.Sprintf("--bundle-id=%s", bundleId),
		fmt.Sprintf("--test-runner-bundle-id=%s", bundleId),
		fmt.Sprintf("--xctest-config=%s", xctestConfig),
		fmt.Sprintf("--udid=%s", device.UDID))
	logger.ProviderLogger.LogDebug("device_setup", fmt.Sprintf("startWdaWithGoIOS: Starting with command `%v`", cmd.Args))
	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startWdaWithGoIOS: Error creating stdoutpipe while running WebDriverAgent with go-ios for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	// Create a pipe to capture the command's error output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startWdaWithGoIOS: Error creating stderrpipe while running WebDriverAgent with go-ios for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	err = cmd.Start()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startWdaWithGoIOS: Failed executing `%s` - %v", cmd.Args, err))
		resetLocalDevice(device)
		return
	}

	// Create a combined reader from stdout and stderr
	combinedReader := io.MultiReader(stderr, stdout)
	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(combinedReader)

	for scanner.Scan() {
		line := scanner.Text()

		device.Logger.LogDebug("webdriveragent", strings.TrimSpace(line))

		//if strings.Contains(line, "ServerURLHere") {
		//	// device.DeviceIP = strings.Split(strings.Split(line, "//")[1], ":")[0]
		//	device.WdaReadyChan <- true
		//}
	}

	err = cmd.Wait()
	if err != nil {
		device.Logger.LogError("webdriveragent", fmt.Sprintf("startWdaWithGoIOS: Error waiting for `%s` to finish, it errored out or device `%v` was disconnected - %v", cmd.Args, device.UDID, err))
		resetLocalDevice(device)
	}
}

//	cmd := exec.CommandContext(context.Background(), "ios", "runwda", "--bundleid=com.shamanec.iosstreamUITests.xctrunner", "--testrunnerbundleid=com.shamanec.iosstreamUITests.xctrunner", "--xctestconfig=iosstreamUITests.xctest", "--udid="+device.UDID)

// Mount a developer disk image on an iOS device with the go-ios library
func mountDeveloperImageIOS(device *models.Device) error {
	basedir := fmt.Sprintf("%s/devimages", config.ProviderConfig.ProviderFolder)

	cmd := exec.CommandContext(device.Context, "ios", "image", "auto", fmt.Sprintf("--basedir=%s", basedir))
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Mounting DDI on device `%s` with command `%s`, image will be stored/found in `%s`", device.UDID, cmd.Args, basedir))

	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mountDeveloperImageIOS: Failed creating stdout pipe - %s", err)
	}

	// Create a pipe to capture the command's error output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("mountDeveloperImageIOS: Failed creating stderr pipe - %s", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("mountDeveloperImageIOS: Failed starting command `%s` - %s", cmd.Args, err)
	}

	// Create a combined reader from stdout and stderr
	combinedReader := io.MultiReader(stderr, stdout)
	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(combinedReader)

	for scanner.Scan() {
		//line := scanner.Text()
		//fmt.Println(line)
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("mountDeveloperImageIOS: Failed to run command to mount DDI - %s", err)
	}

	return nil
}

// Pair an iOS device with host with/without supervision
func pairIOS(device *models.Device) error {
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Pairing device `%s`", device.UDID))

	p12, err := os.ReadFile(fmt.Sprintf("%s/supervision.p12", config.ProviderConfig.ProviderFolder))
	if err != nil {
		logger.ProviderLogger.LogWarn("ios_device_setup", fmt.Sprintf("Could not read supervision.p12 file when pairing device with UDID: %s, falling back to unsupervised pairing - %s", device.UDID, err))
		err = ios.Pair(device.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("Could not perform unsupervised pairing successfully - %s", err)
		}
		return nil
	}

	err = ios.PairSupervised(device.GoIOSDeviceEntry, p12, config.ProviderConfig.SupervisionPassword)
	if err != nil {
		return fmt.Errorf("Could not perform supervised pairing successfully - %s", err)
	}

	return nil
}

// Get all installed apps on an iOS device
func GetInstalledAppsIOS(device *models.Device) []string {
	var installedApps []string
	cmd := exec.CommandContext(device.Context, "ios", "apps", "--udid="+device.UDID)

	device.InstalledApps = []string{}

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("GetInstalledAppsIOS: Failed executing `%s` to get installed apps - %v", cmd.Args, err))
		return installedApps
	}

	// Get the command output json string
	jsonString := strings.TrimSpace(outBuffer.String())

	var appsData []struct {
		BundleID string `json:"CFBundleIdentifier"`
	}

	err := json.Unmarshal([]byte(jsonString), &appsData)
	if err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("GetInstalledAppsIOS: Error unmarshalling `%s` output json - %v", cmd.Args, err))
		return installedApps
	}

	var mu sync.RWMutex
	mu.Lock()
	defer mu.Unlock()
	for _, appData := range appsData {
		installedApps = append(installedApps, appData.BundleID)
	}

	return installedApps
}

// To use for iOS 17+ when stable
func StartIOSTunnel() {
	cmd := exec.CommandContext(context.Background(), "ios", "tunnel", "start")

	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
	}

	// Create a pipe to capture the command's error output
	stderr, err := cmd.StderrPipe()
	if err != nil {
	}

	err = cmd.Start()
	if err != nil {
	}

	// Create a combined reader from stdout and stderr
	combinedReader := io.MultiReader(stderr, stdout)
	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(combinedReader)

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}
}

// Uninstall an app on an iOS device by bundle identifier
func uninstallAppIOS(device *models.Device, bundleID string) error {
	cmd := exec.CommandContext(device.Context, "ios", "uninstall", bundleID, "--udid="+device.UDID)
	err := cmd.Run()
	if err != nil {
		device.Logger.LogError("uninstall_app", fmt.Sprintf("uninstallAppIOS: Failed executing `%s` - %v", cmd.Args, err))
		return err
	}

	return nil
}

func installAppDefaultPath(device *models.Device, appName string) error {
	appPath := fmt.Sprintf("%s/%s", config.ProviderConfig.ProviderFolder, appName)

	return installAppIOS(device, appPath)
}

func installAppIOS(device *models.Device, appPath string) error {
	if config.ProviderConfig.OS == "windows" {
		appPath = strings.TrimPrefix(appPath, "./")
	}

	if config.ProviderConfig.OS == "darwin" && isAboveIOS16(device) {
		cmd := exec.CommandContext(device.Context,
			"xcrun",
			"devicectl",
			"device",
			"install",
			"app",
			"--device",
			device.UDID,
			appPath,
		)
		logger.ProviderLogger.LogInfo("install_app_ios", fmt.Sprintf("Attempting to install app `%s` on device `%s` with command `%s`", appPath, device.UDID, cmd.Args))
		if err := cmd.Run(); err != nil {
			return err
		}
	} else {
		cmd := exec.CommandContext(device.Context,
			"ios",
			"install",
			fmt.Sprintf("--path=%s", appPath),
			fmt.Sprintf("--udid=%s", device.UDID),
		)
		logger.ProviderLogger.LogInfo("install_app_ios", fmt.Sprintf("Attempting to install app `%s` on device `%s` with command `%s`", appPath, device.UDID, cmd.Args))
		if err := cmd.Run(); err != nil {
			device.Logger.LogError("install_app_ios", fmt.Sprintf("Failed executing `%s` - %v", cmd.Args, err))
			return err
		}
	}
	return nil
}

// Check if a device is above iOS 17
func isAboveIOS17(device *models.Device) bool {
	deviceOSVersion, _ := semver.NewVersion(device.OSVersion)

	return deviceOSVersion.Major() >= 17
}

func isAboveIOS16(device *models.Device) bool {
	deviceOSVersion, _ := semver.NewVersion(device.OSVersion)

	return deviceOSVersion.Major() >= 16
}

func checkWebDriverAgentUp(device *models.Device) {
	var netClient = &http.Client{
		Timeout: time.Second * 120,
	}

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%v/status", device.WDAPort), nil)

	loops := 0
	for {
		if loops >= 30 {
			return
		}
		resp, err := netClient.Do(req)
		if err != nil {
			time.Sleep(1 * time.Second)
		} else {
			if resp.StatusCode == http.StatusOK {
				device.WdaReadyChan <- true
				return
			}
		}
		loops++
	}
}

func checkAppiumUp(device *models.Device) {
	var netClient = &http.Client{
		Timeout: time.Second * 120,
	}

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%v/status", device.AppiumPort), nil)

	loops := 0
	for {
		if loops >= 30 {
			return
		}
		resp, err := netClient.Do(req)
		if err != nil {
			time.Sleep(1 * time.Second)
		} else {
			if resp.StatusCode == http.StatusOK {
				device.AppiumReadyChan <- true
				return
			}
		}
		loops++
	}
}
