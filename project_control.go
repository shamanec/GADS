package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

var sudo_password = GetEnvValue("sudo_password")

//=================//
//=====STRUCTS=====//

type DeviceControlInfo struct {
	RunningContainers []string            `json:"running-containers"`
	IOSInfo           []IOSDeviceInfo     `json:"ios-devices-info"`
	AndroidInfo       []AndroidDeviceInfo `json:"android-devices-info"`
	InstallableApps   []string            `json:"installable-apps"`
}

type IOSDevice struct {
	AppiumPort         int    `json:"appium_port"`
	DeviceName         string `json:"device_name"`
	DeviceOSVersion    string `json:"device_os_version"`
	DeviceUDID         string `json:"device_udid"`
	WdaMjpegPort       int    `json:"wda_mjpeg_port"`
	WdaPort            int    `json:"wda_port"`
	WdaURL             string `json:"wda_url"`
	WdaMjpegURL        string `json:"wda_stream_url"`
	DeviceModel        string `json:"device_model"`
	DeviceViewportSize string `json:"viewport_size"`
}

type AndroidDevice struct {
	AppiumPort      int    `json:"appium_port"`
	DeviceName      string `json:"device_name"`
	DeviceOSVersion string `json:"device_os_version"`
	DeviceUDID      string `json:"device_udid"`
	StreamSize      string `json:"stream_size"`
	StreamPort      string `json:"stream_port"`
}

type WdaConfig struct {
	WdaURL       string `json:"wda_url"`
	WdaStreamURL string `json:"wda_stream_url"`
}

type SudoPasswordRequest struct {
	SudoPassword string `json:"sudo_password"`
}

//=======================//
//=====API FUNCTIONS=====//

// @Summary      Update project configuration
// @Description  Updates one  or multiple configuration values
// @Tags         configuration
// @Param        config body ProjectConfig true "Update config"
// @Accept		 json
// @Produce      json
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /configuration/update-config [put]
func UpdateProjectConfigHandler(w http.ResponseWriter, r *http.Request) {
	var requestData AppiumConfig
	err := UnmarshalRequestBody(r.Body, &requestData)

	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not unmarshal config.json file when trying to change configuration values.")
		return
	}

	if requestData.DevicesHost != "" {
		configData.AppiumConfig.DevicesHost = requestData.DevicesHost
	}
	if requestData.SeleniumHubHost != "" {
		configData.AppiumConfig.SeleniumHubHost = requestData.SeleniumHubHost
	}
	if requestData.SeleniumHubPort != "" {
		configData.AppiumConfig.SeleniumHubPort = requestData.SeleniumHubPort
	}
	if requestData.SeleniumHubProtocolType != "" {
		configData.AppiumConfig.SeleniumHubProtocolType = requestData.SeleniumHubProtocolType
	}
	if requestData.WDABundleID != "" {
		configData.AppiumConfig.WDABundleID = requestData.WDABundleID
	}

	bs, err := json.MarshalIndent(configData, "", "  ")

	err = ioutil.WriteFile("./configs/config.json", bs, 0644)
	if err != nil {
		JSONError(w, "config_file_interaction", "Could not write to the config.json file.", 500)
		return
	}
	SimpleJSONResponse(w, "Successfully updated project config in ./configs/config.json", 200)
}

// @Summary      Sets up iOS device listener
// @Description  Creates udev rules, moves them to /etc/udev/rules.d and reloads udev. Copies usbmuxd.service to /lib/systemd/system and enables it
// @Tags         configuration
// @Produce      json
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /configuration/setup-ios-listener [post]
func SetupUdevListener(w http.ResponseWriter, r *http.Request) {
	// Open /lib/systemd/system/systemd-udevd.service
	// Add IPAddressAllow=127.0.0.1 at the bottom
	// This is to allow curl calls from the udev rules to the GADS server
	if sudo_password == "undefined" {
		log.WithFields(log.Fields{
			"event": "setup_udev_listener",
		}).Error("Elevated permissions are required to perform this action. Please set your sudo password in './configs/config.json' or via the '/configuration/set-sudo-password' endpoint.")
		JSONError(w, "setup_udev_listener", "Elevated permissions are required to perform this action.", 500)
		return
	}
	DeleteTempUdevFiles()
	err := CreateUdevRules()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "setup_udev_listener",
		}).Error("Could not create udev rules file. Error:" + err.Error())
		JSONError(w, "setup_udev_listener", "Could not create udev rules", 500)
		DeleteTempUdevFiles()
		return
	}

	err = SetUdevRules()
	if err != nil {
		JSONError(w, "setup_udev_listener", err.Error(), 500)
		DeleteTempUdevFiles()
		return
	}

	// err = CopyFileShell("./configs/usbmuxd.service", "/lib/systemd/system/", sudo_password)
	// if err != nil {
	// 	DeleteTempUdevFiles()
	// 	JSONError(w, "setup_udev_listener", "Could not copy usbmuxd.service. Error: "+err.Error(), 500)
	// 	return
	// }

	// err = EnableUsbmuxdService()
	// if err != nil {
	// 	DeleteTempUdevFiles()
	// 	JSONError(w, "setup_udev_listener", "Could not enable usbmuxd.service. Error: "+err.Error(), 500)
	// 	return
	// }

	DeleteTempUdevFiles()
	SimpleJSONResponse(w, "Successfully set udev rules.", 200)
}

// @Summary      Removes iOS device listener
// @Description  Deletes udev rules from /etc/udev/rules.d and reloads udev
// @Tags         configuration
// @Produce      json
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /configuration/remove-ios-listener [post]
func RemoveUdevListener(w http.ResponseWriter, r *http.Request) {
	err := DeleteFileShell("/etc/udev/rules.d/90-device.rules", sudo_password)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "remove_udev_listener",
		}).Error("Could not delete udev rules file. Error: " + err.Error())
		JSONError(w, "remove_udev_listener", "Could not delete usbmuxd rules file.", 500)
		return
	}
	commandString := "echo '" + sudo_password + "' | sudo -S udevadm control --reload-rules"
	cmd := exec.Command("bash", "-c", commandString)
	err = cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "remove_udev_listener",
		}).Error("Could not reload udev rules file. Error: " + err.Error())
		JSONError(w, "remove_udev_listener", "Could not reload udev rules", 500)
		return
	}
}

// @Summary      Set sudo password
// @Description  Sets your sudo password in ./configs/config.json. The password is needed for operations requiring elevated permissions like setting up udev.
// @Tags         configuration
// @Accept		 json
// @Produce      json
// @Param        config body SudoPassword true "Sudo password value"
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /configuration/set-sudo-password [put]
func SetSudoPassword(w http.ResponseWriter, r *http.Request) {
	var requestData SudoPasswordRequest

	err := UnmarshalRequestBody(r.Body, &requestData)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_create",
		}).Error("Could not unmarshal request body when creating container: " + err.Error())
		return
	}

	sudo_password := requestData.SudoPassword

	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not unmarshal ./configs/config.json file when setting sudo password")
		return
	}

	configData.EnvConfig.SudoPassword = sudo_password

	bs, err := json.MarshalIndent(configData, "", "  ")

	// Write the new json to the config.json file
	err = ioutil.WriteFile("./configs/config.json", bs, 0644)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "set_sudo_password",
		}).Error("Could not write ./configs/config.json while attempting to set sudo password. Error: " + err.Error())
		JSONError(w, "set_sudo_password", "Could not set sudo password", 500)
		return
	}
	log.WithFields(log.Fields{
		"event": "set_sudo_password",
	}).Info("Successfully set sudo password.")
	SimpleJSONResponse(w, "Successfully set '"+sudo_password+"' as sudo password. This password will not be exposed anywhere except inside the ./configs/config.json file. Make sure you don't commit this file to public repos :D", 200)
}

// @Summary      Get device control info
// @Description  Provides the running containers, IOS devices info and apps available for installing
// @Tags         devices
// @Produce      json
// @Success      200 {object} DeviceControlInfo
// @Failure      500 {object} ErrorJSON
// @Router       /devices/device-control [post]
func GetDeviceControlInfo(w http.ResponseWriter, r *http.Request) {
	var runningContainerNames = getRunningContainerNames()
	var info = DeviceControlInfo{
		RunningContainers: runningContainerNames,
		IOSInfo:           getIOSDevicesInfo(runningContainerNames),
		InstallableApps:   getInstallableApps(),
		AndroidInfo:       getAndroidDevicesInfo(runningContainerNames),
	}
	fmt.Fprintf(w, PrettifyJSON(ConvertToJSONString(info)))
}

//=======================//
//=====FUNCTIONS=====//

// Check if the sudo password in ./configs/config.json is different than "undefined" meaning something is set
func CheckSudoPasswordSet() bool {
	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "check_sudo_password",
		}).Error("Could not unmarshal ./configs/config.json file when checking sudo password")
		return false
	}
	sudo_password := configData.EnvConfig.SudoPassword
	if sudo_password == "undefined" || sudo_password == "" {
		return false
	}
	return true
}

// Delete the temporary iOS udev rule file
func DeleteTempUdevFiles() {
	DeleteFileShell("./90-device.rules", sudo_password)
}

// Check if the iOS udev rules and usbmuxd service are set
func UdevIOSListenerState() (status string) {
	_, ruleErr := os.Stat("/etc/udev/rules.d/90-device.rules")
	_, usbmuxdErr := os.Stat("/lib/systemd/system/usbmuxd.service")
	if ruleErr != nil || usbmuxdErr != nil {
		log.WithFields(log.Fields{
			"event": "udev_rules_state",
		}).Error("Udev rules are not set.")
		status = "Udev rules not set."
		return
	} else {
		status = "Udev rules set."
		return
	}
}

// Generate the temporary iOS udev rule file
func CreateUdevRules() error {
	log.WithFields(log.Fields{
		"event": "create_udev_rules",
	}).Info("Creating udev rules")
	// Create the rules file that will start usbmuxd on the first connected device
	create_container_rules, err := os.Create("./90-device.rules")
	if err != nil {
		return errors.New("Could not create 90-device.rules")
	}
	defer create_container_rules.Close()

	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "create_udev_rules",
		}).Error("Could not unmarshal config.json file when creating udev rules")
		return err
	}

	devices_list := configData.DeviceConfig

	for _, device := range devices_list {
		rule_line1 := `SUBSYSTEM=="usb", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", MODE="0666", SYMLINK+="device_` + device.DeviceUDID + `"`
		rule_line2 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"udid\":\"` + device.DeviceUDID + `\"}' http://localhost:10000/device-containers/remove"`
		rule_line3 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"device_type\":\"` + device.OS + `\", \"udid\":\"` + device.DeviceUDID + `\"}' http://localhost:10000/device-containers/create"`
		//rule_line2 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUdid + `", RUN+="/usr/local/bin/docker-cli start-device-container --device_type=` + device.OS + ` --udid=` + device.DeviceUdid + `"`
		//rule_line3 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUdid + `", RUN+="/usr/local/bin/docker-cli remove-device-container --udid=` + device.DeviceUdid + `"`

		if _, err := create_container_rules.WriteString(rule_line1 + "\n"); err != nil {
			return errors.New("Could not write to 90-device.rules")
		}

		if _, err := create_container_rules.WriteString(rule_line2 + "\n"); err != nil {
			return errors.New("Could not write to 90-device.rules")
		}

		if _, err := create_container_rules.WriteString(rule_line3 + "\n"); err != nil {
			return errors.New("Could not write to 90-device.rules")
		}
	}

	return nil
}

// Copy the iOS udev rules to /etc/udev/rules.d and reload udev
func SetUdevRules() error {
	//err := CopyFileShell("./39-usbmuxd.rules", "/etc/udev/rules.d/39-usbmuxd.rules", sudo_password)
	err := CopyFileShell("./90-device.rules", "/etc/udev/rules.d/90-device.rules", sudo_password)
	if err != nil {
		return err
	}
	commandString := "echo '" + sudo_password + "' | sudo -S udevadm control --reload-rules"
	cmd := exec.Command("bash", "-c", commandString)
	err = cmd.Run()
	if err != nil {
		return errors.New("Could not reload udev rules")
	}
	return nil
}

// Check if WebDriverAgent.ipa exists in  the ./apps folder
func CheckWDAProvided() bool {
	_, err := os.Stat("apps/WebDriverAgent.ipa")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "wda_ipa_present",
		}).Error("Could not find WebDriverAgent.ipa at ./apps")
		return false
	}
	return true
}

// Get a value from ./configs/config.json
func GetEnvValue(key string) string {
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "check_sudo_password",
		}).Error("Could not unmarshal ./configs/config.json file when getting value")
	}

	if key == "sudo_password" {
		return configData.EnvConfig.SudoPassword
	} else if key == "supervision_password" {
		return configData.EnvConfig.SupervisionPassword
	} else if key == "connect_selenium_grid" {
		return strconv.FormatBool(configData.EnvConfig.ConnectSeleniumGrid)
	}
	return ""
}

// Get the names of the currently running containers(that are for devices)
func getRunningContainerNames() []string {
	var containerNames []string

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return containerNames
	}

	// Get the current containers list
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return containerNames
	}

	// Loop through the containers list
	for _, container := range containers {
		// Parse plain container name
		containerName := strings.Replace(container.Names[0], "/", "", -1)
		if (strings.Contains(containerName, "iosDevice") || strings.Contains(containerName, "androidDevice")) && strings.Contains(container.Status, "Up") {
			containerNames = append(containerNames, containerName)
		}
	}
	return containerNames
}

// For each running container extract the info for each respective device from ./configs/config.json to provide to the device-control info endpoint.
// Provides installed apps, configuration info, wda urls
func getIOSDevicesInfo(runningContainers []string) []IOSDeviceInfo {
	var combinedInfo []IOSDeviceInfo
	for _, containerName := range runningContainers {
		if strings.Contains(containerName, "iosDevice") {
			// Extract the device UDID from the container name
			re := regexp.MustCompile("[^_]*$")
			device_udid := re.FindStringSubmatch(containerName)

			var installed_apps []string
			installed_apps, err := IOSDeviceApps(device_udid[0])
			if err != nil {
				installed_apps = append(installed_apps, "")
			}

			var device_config *IOSDevice
			device_config, err = iOSDeviceConfig(device_udid[0])

			var deviceInfo = IOSDeviceInfo{BundleIDs: installed_apps, DeviceConfig: device_config}
			combinedInfo = append(combinedInfo, deviceInfo)
		}
	}
	return combinedInfo
}

// For each running container extract the info for each respective device from ./configs/config.json to provide to the device-control info endpoint.
// Provides installed apps, configuration info, wda urls
func getAndroidDevicesInfo(runningContainers []string) []AndroidDeviceInfo {
	var combinedInfo []AndroidDeviceInfo
	for _, containerName := range runningContainers {
		if strings.Contains(containerName, "androidDevice") {
			// Extract the device UDID from the container name
			re := regexp.MustCompile("[^_]*$")
			device_udid := re.FindStringSubmatch(containerName)

			var device_config *AndroidDevice
			device_config, _ = androidDeviceConfig(device_udid[0])

			var deviceInfo = AndroidDeviceInfo{DeviceConfig: device_config}
			combinedInfo = append(combinedInfo, deviceInfo)
		}
	}
	return combinedInfo
}

// Get the WDA and WDA stream urls from the container logs folder for a specific device
func getIOSDeviceWdaURLs(device_udid string) (string, string, error) {
	// Get the path of the WDA url file using regex
	pattern := "./logs/*" + device_udid + "/ios-wda-url.json"
	matches, err := filepath.Glob(pattern)

	if err != nil {
		return "", "", err
	}

	var wdaConfig WdaConfig
	err = UnmarshalJSONFile(matches[0], &wdaConfig)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_wda_config",
		}).Error("Could not unmarshal json at path: " + matches[0])
		return "", "", err
	}

	return wdaConfig.WdaURL, wdaConfig.WdaStreamURL, nil
}

// Get the WDA and WDA stream urls from the container logs folder for a specific device
func getAndroidDeviceMinicapStreamSize(device_udid string) (string, error) {
	// Get the path of the WDA url file using regex
	pattern := "./logs/*" + device_udid + "/minicap.log"
	matches, err := filepath.Glob(pattern)

	command_string := "cat " + matches[0] + " | grep \"+ args='-P\""
	cmd := exec.Command("bash", "-c", command_string)
	var out bytes.Buffer
	cmd.Stdout = &out

	err = cmd.Run()
	if err != nil {
		return "", err
	}

	if out.String() == "" {
		log.WithFields(log.Fields{
			"event": "get_minicap_stream_size",
		}).Error("Error")
		return "", nil
	}

	// Get the stream size that is between the -P and @ in the cmd out string
	stream_size := GetStringInBetween(out.String(), "-P ", "@")
	return stream_size, nil
}

// Get the configuration info for iOS device from ./configs/config.json
func iOSDeviceConfig(device_udid string) (*IOSDevice, error) {
	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not unmarshal config.json file when trying to create a container for device with udid: " + device_udid)
		return nil, err
	}

	// Check if device is registered in config data
	var device_in_config bool
	var deviceConfig DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			device_in_config = true
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if !device_in_config {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
		return nil, err
	}

	wda_url, wda_stream_url, err := getIOSDeviceWdaURLs(device_udid)
	if err != nil {
		return nil, err
	}

	return &IOSDevice{
			AppiumPort:         deviceConfig.AppiumPort,
			DeviceName:         deviceConfig.DeviceName,
			DeviceOSVersion:    deviceConfig.DeviceOSVersion,
			WdaMjpegPort:       deviceConfig.WDAMjpegPort,
			WdaPort:            deviceConfig.WDAPort,
			DeviceUDID:         device_udid,
			WdaMjpegURL:        wda_stream_url,
			WdaURL:             wda_url,
			DeviceModel:        "Remove please",
			DeviceViewportSize: deviceConfig.ViewportSize},
		nil
}

// Get the configuration info for iOS device from ./configs/config.json
func androidDeviceConfig(device_udid string) (*AndroidDevice, error) {
	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not unmarshal config.json file when trying to create a container for device with udid: " + device_udid)
		return nil, err
	}

	// Check if device is registered in config data
	var device_in_config bool
	var deviceConfig DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			device_in_config = true
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if !device_in_config {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
		return nil, err
	}

	stream_size, err := getAndroidDeviceMinicapStreamSize(device_udid)
	return &AndroidDevice{
			AppiumPort:      deviceConfig.AppiumPort,
			DeviceName:      deviceConfig.DeviceName,
			DeviceOSVersion: deviceConfig.DeviceOSVersion,
			DeviceUDID:      device_udid,
			StreamSize:      stream_size,
			StreamPort:      strconv.Itoa(deviceConfig.StreamPort)},
		nil
}

// Get the installable apps list from the ./apps folder
func getInstallableApps() []string {
	var appNames []string

	files, err := ioutil.ReadDir("./apps/")
	if err != nil {
		return appNames
	}

	for _, file := range files {
		if strings.Contains(file.Name(), ".ipa") || strings.Contains(file.Name(), ".app") || strings.Contains(file.Name(), ".apk") {
			appNames = append(appNames, file.Name())
		}
	}
	return appNames
}
