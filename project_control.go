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
	"strings"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var sudo_password = GetEnvValue("sudo_password")

//=================//
//=====STRUCTS=====//

type SudoPassword struct {
	SudoPassword string `json:"sudo_password"`
}

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

//=======================//
//=====API FUNCTIONS=====//

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
		}).Error("Elevated permissions are required to perform this action. Please set your sudo password in './env.json' or via the '/configuration/set-sudo-password' endpoint.")
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

	err = CopyFileShell("./configs/usbmuxd.service", "/lib/systemd/system/", sudo_password)
	if err != nil {
		DeleteTempUdevFiles()
		JSONError(w, "setup_udev_listener", "Could not copy usbmuxd.service. Error: "+err.Error(), 500)
		return
	}

	err = EnableUsbmuxdService()
	if err != nil {
		DeleteTempUdevFiles()
		JSONError(w, "setup_udev_listener", "Could not enable usbmuxd.service. Error: "+err.Error(), 500)
		return
	}

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
// @Description  Sets your sudo password in ./env.json. The password is needed for operations requiring elevated permissions like setting up udev.
// @Tags         configuration
// @Accept		 json
// @Produce      json
// @Param        config body SudoPassword true "Sudo password value"
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /configuration/set-sudo-password [put]
func SetSudoPassword(w http.ResponseWriter, r *http.Request) {
	requestBody, _ := ioutil.ReadAll(r.Body)
	sudo_password := gjson.Get(string(requestBody), "sudo_password").Str
	byteValue, err := ReadJSONFile("./env.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "set_sudo_password",
		}).Error("Could not read ./env.json while attempting to set sudo password. Error: " + err.Error())
		JSONError(w, "set_sudo_password", "Could not set sudo password", 500)
		return
	}
	updatedJSON, _ := sjson.Set(string(byteValue), "sudo_password", sudo_password)

	// Prettify the json so it looks good inside the file
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, []byte(updatedJSON), "", "  ")

	// Write the new json to the config.json file
	err = ioutil.WriteFile("./env.json", []byte(prettyJSON.String()), 0644)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "set_sudo_password",
		}).Error("Could not write ./env.json while attempting to set sudo password. Error: " + err.Error())
		JSONError(w, "set_sudo_password", "Could not set sudo password", 500)
		return
	}
	log.WithFields(log.Fields{
		"event": "set_sudo_password",
	}).Info("Successfully set sudo password.")
	SimpleJSONResponse(w, "Successfully set '"+sudo_password+"' as sudo password. This password will not be exposed anywhere except inside the ./env.json file. Make sure you don't commit this file to public repos :D", 200)
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

// Check if the sudo password in env.json is different than "undefined" meaning something is set
func CheckSudoPasswordSet() bool {
	byteValue, err := ReadJSONFile("./env.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "check_sudo_password",
		}).Error("Could not read ./env.json while checking sudo password status.")
		return false
	}
	sudo_password := gjson.Get(string(byteValue), "sudo_password").Str
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

	jsonBytes, _ := ReadJSONFile("./configs/config.json")
	android_udids := gjson.Get(string(jsonBytes), `android-devices-list.#.device_udid`)
	ios_udids := gjson.Get(string(jsonBytes), `ios-devices-list.#.device_udid`)

	// Add rule lines for each Android device in config.json
	for _, device_udid := range android_udids.Array() {
		device_name := gjson.Get(string(jsonBytes), `android-devices-list.#(device_udid="`+device_udid.Str+`").device_name`)
		rule_line1 := `SUBSYSTEM=="usb", ENV{ID_SERIAL_SHORT}=="` + device_udid.Str + `", MODE="0666", SYMLINK+="device-` + device_name.Str + `-` + device_udid.Str + `"`
		rule_line2 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device_udid.Str + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"os_type\":\"Android\"}' http://localhost:10000/device-containers/` + device_udid.Str + `/remove"`
		rule_line3 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device_udid.Str + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"os_type\":\"Android\"}' http://localhost:10000/device-containers/` + device_udid.Str + `/create"`

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

	// Add rule lines for each iOS device in config.json
	for _, device_udid := range ios_udids.Array() {
		device_name := gjson.Get(string(jsonBytes), `ios-devices-list.#(device_udid="`+device_udid.Str+`").device_name`)
		rule_line1 := `SUBSYSTEM=="usb", ENV{ID_SERIAL_SHORT}=="` + device_udid.Str + `", MODE="0666", SYMLINK+="device-` + device_name.Str + `-` + device_udid.Str + `"`
		rule_line2 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device_udid.Str + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"os_type\":\"iOS\"}' http://localhost:10000/device-containers/` + device_udid.Str + `/remove"`
		rule_line3 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device_udid.Str + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"os_type\":\"iOS\"}' http://localhost:10000/device-containers/` + device_udid.Str + `/create"`

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

// Get a value from env.json
func GetEnvValue(key string) string {
	byteValue, _ := ReadJSONFile("./env.json")
	value := gjson.Get(string(byteValue), key).Str
	return value
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
		if strings.Contains(containerName, "ios_device") || strings.Contains(containerName, "android_device") {
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
		if strings.Contains(containerName, "ios_device") {
			// Extract the device UDID from the container name
			re := regexp.MustCompile("[^-]*$")
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
		} else if strings.Contains(containerName, "android_device") {
			print("test")
		}
	}
	return combinedInfo
}

// For each running container extract the info for each respective device from ./configs/config.json to provide to the device-control info endpoint.
// Provides installed apps, configuration info, wda urls
func getAndroidDevicesInfo(runningContainers []string) []AndroidDeviceInfo {
	var combinedInfo []AndroidDeviceInfo
	for _, containerName := range runningContainers {
		if strings.Contains(containerName, "android_device") {
			// Extract the device UDID from the container name
			re := regexp.MustCompile("[^-]*$")
			device_udid := re.FindStringSubmatch(containerName)

			var device_config *AndroidDevice
			device_config, _ = androidDeviceConfig(device_udid[0])

			var deviceInfo = AndroidDeviceInfo{DeviceConfig: device_config}
			combinedInfo = append(combinedInfo, deviceInfo)
		} else if strings.Contains(containerName, "android_device") {
			print("test")
		}
	}
	return combinedInfo
}

// Get the WDA and WDA stream urls from the container logs folder for a specific device
func getIOSDeviceWdaURLs(device_udid string) (string, string) {
	// Get the path of the WDA url file using regex
	pattern := "./logs/*" + device_udid + "/ios-wda-url.json"
	matches, err := filepath.Glob(pattern)

	if err != nil {
		fmt.Println(err)
	}

	// Open the first match, should be only one file
	jsonFile, err := os.Open(matches[0])

	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_wda_url",
		}).Error("Could not open WDA url file for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		return "", ""
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_wda_url",
		}).Error("Could not read WDA url file for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		return "", ""
	}
	url := gjson.Get(string(byteValue), `wda_url`)
	stream_url := gjson.Get(string(byteValue), `wda_stream_url`)
	return url.Str, stream_url.Str
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
	jsonFile, err := os.Open("./configs/config.json")

	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_ios_device_config",
		}).Error("Could not open ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		return nil, errors.New("Could not open ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_ios_device_config",
		}).Error("Could not read ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		return nil, errors.New("Could not read ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
	}
	appium_port := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").appium_port`)
	if appium_port.Raw == "" {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
		return nil, errors.New("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
	}
	device_name := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").device_name`)
	device_os_version := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").device_os_version`)
	wda_mjpeg_port := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").wda_mjpeg_port`)
	wda_port := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").wda_port`)

	wda_url, wda_stream_url := getIOSDeviceWdaURLs(device_udid)

	model, viewport_size := getIOSModelAndViewport(string(byteValue), getIOSDeviceProductType(device_udid))

	return &IOSDevice{
			AppiumPort:         int(appium_port.Num),
			DeviceName:         device_name.Str,
			DeviceOSVersion:    device_os_version.Str,
			WdaMjpegPort:       int(wda_mjpeg_port.Num),
			WdaPort:            int(wda_port.Num),
			DeviceUDID:         device_udid,
			WdaMjpegURL:        wda_stream_url,
			WdaURL:             wda_url,
			DeviceModel:        model,
			DeviceViewportSize: viewport_size},
		nil
}

// Get the configuration info for iOS device from ./configs/config.json
func androidDeviceConfig(device_udid string) (*AndroidDevice, error) {
	jsonFile, err := os.Open("./configs/config.json")

	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_android_device_config",
		}).Error("Could not open ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		return nil, errors.New("Could not open ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_android_device_config",
		}).Error("Could not read ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		return nil, errors.New("Could not read ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
	}
	appium_port := gjson.Get(string(byteValue), `android-devices-list.#(device_udid="`+device_udid+`").appium_port`)
	if appium_port.Raw == "" {
		log.WithFields(log.Fields{
			"event": "get_android_device_config",
		}).Error("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
		return nil, errors.New("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
	}
	device_name := gjson.Get(string(byteValue), `android-devices-list.#(device_udid="`+device_udid+`").device_name`)
	device_os_version := gjson.Get(string(byteValue), `android-devices-list.#(device_udid="`+device_udid+`").device_os_version`)
	stream_port := gjson.Get(string(byteValue), `android-devices-list.#(device_udid="`+device_udid+`").stream_port`)
	stream_size, err := getAndroidDeviceMinicapStreamSize(device_udid)
	return &AndroidDevice{
			AppiumPort:      int(appium_port.Num),
			DeviceName:      device_name.Str,
			DeviceOSVersion: device_os_version.Str,
			DeviceUDID:      device_udid,
			StreamSize:      stream_size,
			StreamPort:      stream_port.Raw},
		nil
}

func getIOSDeviceProductType(device_udid string) string {
	deviceList, err := ios.ListDevices()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_connected_ios_devices",
		}).Error("Could not get connected devices. Error: " + err.Error())
		return ""
	}
	deviceValues, err := outputDetailedList(deviceList)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_connected_ios_devices",
		}).Error("Could not get connected devices detailed list. Error: " + err.Error())
		return ""
	}

	product_type := gjson.Get(deviceValues, `deviceList.#(Udid="`+device_udid+`").ProductType`).Str
	return product_type
}

func getIOSModelAndViewport(config_json string, product_type string) (string, string) {

	model := gjson.Get(config_json, `ios_models_sizes.#(product_type="`+product_type+`").device_model`).Str
	viewport_size := gjson.Get(config_json, `ios_models_sizes.#(product_type="`+product_type+`").screen_size`).Str
	return model, viewport_size
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
