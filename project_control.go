package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	log "github.com/sirupsen/logrus"
)

var sudo_password = GetEnvValue("sudo_password")

//=================//
//=====STRUCTS=====//

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

type ProjectConfigPageData struct {
	WebDriverAgentProvided bool
	SudoPasswordSet        bool
	UdevIOSListenerStatus  string
	ImageStatus            string
	AppiumConfigValues     AppiumConfig
}

//=======================//
//=====API FUNCTIONS=====//

// Load the general logs page
func GetLogsPage(w http.ResponseWriter, r *http.Request) {
	var logs_page = template.Must(template.ParseFiles("static/project_logs.html"))
	if err := logs_page.Execute(w, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "project_logs_page",
		}).Error("Couldn't load project_logs.html")
		return
	}
}

// Load the page with the project configuration info
func GetProjectConfigurationPage(w http.ResponseWriter, r *http.Request) {
	// Get the config data
	projectConfig, err := GetConfigJsonData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create the AppiumConfig
	var configRow = AppiumConfig{
		DevicesHost:             projectConfig.AppiumConfig.DevicesHost,
		SeleniumHubHost:         projectConfig.AppiumConfig.SeleniumHubHost,
		SeleniumHubPort:         projectConfig.AppiumConfig.SeleniumHubPort,
		SeleniumHubProtocolType: projectConfig.AppiumConfig.SeleniumHubProtocolType,
		WDABundleID:             projectConfig.AppiumConfig.WDABundleID}

	var index = template.Must(template.ParseFiles("static/project_config.html"))

	// Create the final data for the config page
	pageData := ProjectConfigPageData{WebDriverAgentProvided: CheckWDAProvided(), SudoPasswordSet: CheckSudoPasswordSet(), UdevIOSListenerStatus: UdevIOSListenerState(), ImageStatus: ImageExists(), AppiumConfigValues: configRow}
	if err := index.Execute(w, pageData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// @Summary      Get project logs
// @Description  Provides project logs as plain text response
// @Tags         project-logs
// @Produces	 text
// @Success      200
// @Failure      200
// @Router       /project-logs [get]
func GetLogs(w http.ResponseWriter, r *http.Request) {
	// Create the command string to read the last 1000 lines of project.log
	commandString := "tail -n 1000 ./logs/project.log"

	// Create the command
	cmd := exec.Command("bash", "-c", commandString)

	// Create a buffer for the output
	var out bytes.Buffer

	// Pipe the Stdout of the command to the buffer pointer
	cmd.Stdout = &out

	// Execute the command
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_project_logs",
		}).Error("Attempted to get project logs but no logs available.")

		// Reply with generic message on error
		fmt.Fprintf(w, "No logs available")
		return
	}

	// Reply with the read logs lines
	fmt.Fprintf(w, out.String())
}

// @Summary      Update project Appium configuration
// @Description  Updates one or multiple configuration values
// @Tags         configuration
// @Param        config body AppiumConfig true "Update config"
// @Accept		 json
// @Produce      json
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /configuration/update-config [put]
func UpdateProjectConfigHandler(w http.ResponseWriter, r *http.Request) {
	var requestData AppiumConfig
	// Get the request data
	err := UnmarshalRequestBody(r.Body, &requestData)

	// Get the config data from configs/config.json
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "update_appium_config",
		}).Error("Could not get config data: " + err.Error())
		JSONError(w, "update_appium_config", "An error occurred", 500)
		return
	}

	// Check each field from the request
	// If non-empty value is received update the respective value in the configData
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

	// Marshal back the configData into a byte slice
	bs, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "update_appium_config",
		}).Error("Could not marshal back config data: " + err.Error())
		JSONError(w, "update_appium_config", "An error occurred", 500)
		return
	}

	// Write the updated configData json into the file
	err = ioutil.WriteFile("./configs/config.json", bs, 0644)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "update_appium_config",
		}).Error("Could not write back config data: " + err.Error())
		JSONError(w, "update_appium_config", "An error occurred", 500)
		return
	}

	SimpleJSONResponse(w, "Successfully updated project config in ./configs/config.json", 200)
}

// @Summary      Sets up udev devices listener
// @Description  Creates udev rules, moves them to /etc/udev/rules.d and reloads udev. Copies usbmuxd.service to /lib/systemd/system and enables it
// @Tags         configuration
// @Produce      json
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /configuration/setup-udev-listener [post]
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
	err := SetupUdevListenerInternal()
	if err != nil {
		JSONError(w, "setup_udev_listener", "Could not setup udev rules", 500)
	}

	SimpleJSONResponse(w, "Successfully set udev rules.", 200)
}

// @Summary      Removes udev device listener
// @Description  Deletes udev rules from /etc/udev/rules.d and reloads udev
// @Tags         configuration
// @Produce      json
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /configuration/remove-device-listener [post]
func RemoveUdevListener(w http.ResponseWriter, r *http.Request) {
	err := RemoveUdevListenerInternal()
	if err != nil {
		JSONError(w, "remove_udev_listener", err.Error(), 500)
	}
}

// @Summary      Set sudo password
// @Description  Sets your sudo password in ./configs/config.json. The password is needed for operations requiring elevated permissions like setting up udev.
// @Tags         configuration
// @Accept		 json
// @Produce      json
// @Param        config body SudoPasswordRequest true "Sudo password value"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /configuration/set-sudo-password [put]
func SetSudoPassword(w http.ResponseWriter, r *http.Request) {
	var requestData SudoPasswordRequest

	// Get the request data
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

	// Update the password in configData
	configData.EnvConfig.SudoPassword = sudo_password

	// Create a byte slice with the updated data
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

//=======================//
//=====FUNCTIONS=====//

// Completely setup udev and usbmuxd
func SetupUdevListenerInternal() error {
	DeleteTempUdevFiles()

	err := CreateUdevRules()
	if err != nil {
		DeleteTempUdevFiles()
		return err
	}

	err = SetUdevRules()
	if err != nil {
		DeleteTempUdevFiles()
		return err
	}

	err = CopyFileShell("./configs/usbmuxd.service", "/lib/systemd/system/", sudo_password)
	if err != nil {
		DeleteTempUdevFiles()
		return err
	}

	err = EnableUsbmuxdService()
	if err != nil {
		DeleteTempUdevFiles()
		return err
	}

	DeleteTempUdevFiles()
	return nil
}

func RemoveUdevListenerInternal() error {
	err := DeleteFileShell("/etc/udev/rules.d/90-device.rules", sudo_password)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "remove_udev_listener",
		}).Error("Could not delete udev rules file. Error: " + err.Error())
		return err
	}

	commandString := "echo '" + sudo_password + "' | sudo -S udevadm control --reload-rules"
	cmd := exec.Command("bash", "-c", commandString)
	err = cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "remove_udev_listener",
		}).Error("Could not reload udev rules file. Error: " + err.Error())
		return err
	}

	return nil
}

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
