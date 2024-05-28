package devices

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
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
	cmd.Dir = config.Config.EnvConfig.WdaRepoPath
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

		if strings.Contains(line, "ServerURLHere") {
			// device.DeviceIP = strings.Split(strings.Split(line, "//")[1], ":")[0]
			device.WdaReadyChan <- true
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

// Start WebDriverAgent with the go-ios binary
func startWdaWithGoIOS(device *models.Device) {
	cmd := exec.CommandContext(context.Background(), "ios", "runwda", "--bundleid="+config.Config.EnvConfig.WdaBundleID, "--testrunnerbundleid="+config.Config.EnvConfig.WdaBundleID, "--xctestconfig=WebDriverAgentRunner.xctest", "--udid="+device.UDID)
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

		if strings.Contains(line, "ServerURLHere") {
			// device.DeviceIP = strings.Split(strings.Split(line, "//")[1], ":")[0]
			device.WdaReadyChan <- true
		}
	}

	err = cmd.Wait()
	if err != nil {
		device.Logger.LogError("webdriveragent", fmt.Sprintf("startWdaWithGoIOS: Error waiting for `%s` to finish, it errored out or device `%v` was disconnected - %v", cmd.Args, device.UDID, err))
		resetLocalDevice(device)
	}
}

// Start an XCUITest(similar to WebDriverAgent) that will enable the broadcast stream if the GADS app is used
func startGadsIosBroadcastViaXCTestGoIOS(device *models.Device) error {
	cmd := exec.CommandContext(context.Background(), "ios", "runwda", "--bundleid=com.shamanec.iosstreamUITests.xctrunner", "--testrunnerbundleid=com.shamanec.iosstreamUITests.xctrunner", "--xctestconfig=iosstreamUITests.xctest", "--udid="+device.UDID)
	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startGadsIosBroadcastViaXCTestGoIOS: Error creating stdoutpipe while starting GADS broadcast with XCUITest, xcodebuild and go-ios for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return err
	}

	// Create a pipe to capture the command's error output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startGadsIosBroadcastViaXCTestGoIOS: Error creating stderrpipe while starting GADS broadcast with XCUITest, xcodebuild and go-ios for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return err
	}

	err = cmd.Start()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startGadsIosBroadcastViaXCTestGoIOS: Failed executing `%s` - %v", cmd.Args, err))
		resetLocalDevice(device)
		return err
	}

	// Create a combined reader from stdout and stderr
	combinedReader := io.MultiReader(stderr, stdout)
	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(combinedReader)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "didFinishExecutingTestPlan received. Closing test.") {
			if killErr := cmd.Process.Kill(); killErr != nil {
				return killErr
			}
			return nil
		}
	}

	err = cmd.Wait()
	if err != nil {
		device.Logger.LogError("gads_broadcast_startup", fmt.Sprintf("startGadsIosBroadcastViaXCTestGoIOS: Error waiting for `%s` to finish, it errored out or device `%v` was disconnected - %v", cmd.Args, device.UDID, err))
		resetLocalDevice(device)
		return err
	}

	return nil
}

// Mount a developer disk image on an iOS device with the go-ios library
func mountDeveloperImageIOS(device *models.Device) error {
	basedir := fmt.Sprintf("%s/devimages", config.Config.EnvConfig.ProviderFolder)

	var err error
	path, err := imagemounter.DownloadImageFor(device.GoIOSDeviceEntry, basedir)
	if err != nil {
		return fmt.Errorf("Could not download developer disk image with go-ios - %s", err)
	}

	err = imagemounter.MountImage(device.GoIOSDeviceEntry, path)
	if err != nil {
		return fmt.Errorf("Could not mount developer disk image with go-ios - %s", err)
	}

	return nil
}

// Pair an iOS device with host with/without supervision
func pairIOS(device *models.Device) error {
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Pairing device `%s`", device.UDID))

	p12, err := os.ReadFile(fmt.Sprintf("%s/supervision.p12", config.Config.EnvConfig.ProviderFolder))
	if err != nil {
		logger.ProviderLogger.LogWarn("ios_device_setup", fmt.Sprintf("Could not read supervision.p12 file when pairing device with UDID: %s, falling back to unsupervised pairing - %s", device.UDID, err))
		err = ios.Pair(device.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("Could not perform unsupervised pairing successfully - %s", err)
		}
		return nil
	}

	err = ios.PairSupervised(device.GoIOSDeviceEntry, p12, config.Config.EnvConfig.SupervisionPassword)
	if err != nil {
		return fmt.Errorf("Could not perform supervised pairing successfully - %s", err)
	}

	return nil
}

// Get all installed apps on an iOS device
func getInstalledAppsIOS(device *models.Device) []string {
	var installedApps []string
	cmd := exec.CommandContext(device.Context, "ios", "apps", "--udid="+device.UDID)

	device.InstalledApps = []string{}

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("getInstalledAppsIOS: Failed executing `%s` to get installed apps - %v", cmd.Args, err))
		return installedApps
	}

	// Get the command output json string
	jsonString := strings.TrimSpace(outBuffer.String())

	var appsData []struct {
		BundleID string `json:"CFBundleIdentifier"`
	}

	err := json.Unmarshal([]byte(jsonString), &appsData)
	if err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("getInstalledAppsIOS: Error unmarshalling `%s` output json - %v", cmd.Args, err))
		return installedApps
	}

	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	for _, appData := range appsData {
		installedApps = append(installedApps, appData.BundleID)
	}

	return installedApps
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

// Install app with the go-ios binary from provided path
func installAppWithPathIOS(device *models.Device, path string) error {
	if config.Config.EnvConfig.OS == "windows" {
		if strings.HasPrefix(path, "./") {
			path = strings.TrimPrefix(path, "./")
		}
	}

	cmd := exec.CommandContext(device.Context, "ios", "install", fmt.Sprintf("--path=%s", path), "--udid="+device.UDID)
	logger.ProviderLogger.LogDebug("install_app", fmt.Sprintf("installAppWithPathIOS: Installing with command `%s`", cmd.Args))
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("install_app", fmt.Sprintf("Failed executing `%s` - %v", cmd.Args, err))
		return err
	}

	return nil
}

func installAppIOS(device *models.Device, appName string) error {
	appPath := fmt.Sprintf("%s/%s", config.Config.EnvConfig.ProviderFolder, appName)
	if config.Config.EnvConfig.OS == "darwin" {
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
func isAboveIOS17(device *models.Device) (bool, error) {
	majorVersion := strings.Split(device.OSVersion, ".")[0]
	convertedVersion, err := strconv.Atoi(majorVersion)
	if err != nil {
		return false, fmt.Errorf("isAboveIOS17: Failed converting `%s` to int - %s", majorVersion, err)
	}
	if convertedVersion >= 17 {
		return true, nil
	}
	return false, nil
}
