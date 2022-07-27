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
	AppiumPort          int    `json:"appium_port"`
	DeviceName          string `json:"device_name"`
	DeviceOSVersion     string `json:"device_os_version"`
	DeviceUDID          string `json:"device_udid"`
	WdaMjpegPort        int    `json:"wda_mjpeg_port"`
	WdaPort             int    `json:"wda_port"`
	WdaURL              string `json:"wda_url"`
	WdaMjpegURL         string `json:"wda_stream_url"`
	DeviceModel         string `json:"device_model"`
	DeviceScreenSize    string `json:"screen_size"`
	ContainerServerPort int    `json:"container_server_port"`
	DeviceImage         string `json:"device_image"`
}

type AndroidDevice struct {
	AppiumPort      int    `json:"appium_port"`
	DeviceName      string `json:"device_name"`
	DeviceOSVersion string `json:"device_os_version"`
	DeviceUDID      string `json:"device_udid"`
	StreamSize      string `json:"stream_size"`
	StreamPort      string `json:"stream_port"`
	DeviceImage     string `json:"device_image"`
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
	err := UnmarshalReader(r.Body, &requestData)

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

	// Check the sudo password in the config
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

	// Create the common devices udev rules file
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

	// For each device generate the respective rule lines
	for _, device := range devices_list {
		rule_line1 := `SUBSYSTEM=="usb", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", MODE="0666", SYMLINK+="device_` + device.DeviceUDID + `"`
		rule_line2 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"udid\":\"` + device.DeviceUDID + `\"}' http://localhost:10000/device-containers/remove"`
		rule_line3 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"device_type\":\"` + device.OS + `\", \"udid\":\"` + device.DeviceUDID + `\"}' http://localhost:10000/device-containers/create"`
		//rule_line2 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUdid + `", RUN+="/usr/local/bin/docker-cli start-device-container --device_type=` + device.OS + ` --udid=` + device.DeviceUdid + `"`
		//rule_line3 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUdid + `", RUN+="/usr/local/bin/docker-cli remove-device-container --udid=` + device.DeviceUdid + `"`

		// Write the new lines for each device in the udev rules file
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
	// Copy the udev rules to /etc/udev/rules.d
	err := CopyFileShell("./90-device.rules", "/etc/udev/rules.d/90-device.rules", sudo_password)
	if err != nil {
		return err
	}

	// Reload the udev rules after updating them
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

// Get an env value from ./configs/config.json
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
