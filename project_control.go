package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var sudo_password = GetEnvValue("sudo_password")

func SetupUdevListener(w http.ResponseWriter, r *http.Request) {
	DeleteTempUdevFiles()
	err := CreateUdevRules()
	if err != nil {
		JSONError(w, "create_udev_rules_error", err.Error(), 500)
		DeleteTempUdevFiles()
		return
	}

	err = SetUdevRules()
	if err != nil {
		JSONError(w, "setup_udev_rules_error", err.Error(), 500)
		DeleteTempUdevFiles()
		return
	}
	DeleteTempUdevFiles()
	fmt.Fprintf(w, "Successfully set udev rules.")
}

func DeleteTempUdevFiles() {
	DeleteFileShell("./39-usbmuxd.rules", sudo_password)
}

func UdevIOSListenerState() (status string) {
	_, ruleErr := os.Stat("/etc/udev/rules.d/39-usbmuxd.rules")
	if ruleErr != nil {
		status = "Udev rules not set."
		return
	} else {
		status = "Udev rules set."
		return
	}
}

func CreateUdevRules() error {
	log.WithFields(log.Fields{
		"event": "create_udev_rules",
	}).Info("Creating udev rules")
	// Create the rules file that will start usbmuxd on the first connected device
	create_container_rules, err := os.Create("./39-usbmuxd.rules")
	if err != nil {
		return errors.New("Could not create 39-usbmuxd.rules")
	}
	defer create_container_rules.Close()

	// Create rules for add and remove udev events
	rule_line1 := "ACTION==\"add\", SUBSYSTEM==\"usb\", ENV{DEVTYPE}==\"usb_device\", ATTR{manufacturer}==\"Apple Inc.\", ENV{PRODUCT}==\"5ac/12[9a][0-9a-f]/*|5ac/190[1-4]/*|5ac/8600/*\", MODE=\"0666\", RUN+=\"/usr/bin/wget --post-data='' http://localhost:10000/ios-containers/update\""
	rule_line2 := "SUBSYSTEM==\"usb\", ENV{DEVTYPE}==\"usb_device\", ENV{PRODUCT}==\"5ac/12[9a][0-9a-f]/*|5ac/1901/*|5ac/8600/*\", ACTION==\"remove\", RUN+=\"/usr/bin/wget --post-data='' http://localhost:10000/ios-containers/update\""
	if _, err := create_container_rules.WriteString(rule_line1 + "\n"); err != nil {
		return errors.New("Could not write to 39-usbmuxd.rules")
	}

	if _, err := create_container_rules.WriteString(rule_line2 + "\n"); err != nil {
		return errors.New("Could not write to 39-usbmuxd.rules")
	}
	return nil
}

func SetUdevRules() error {
	err := CopyFileShell("./39-usbmuxd.rules", "/etc/udev/rules.d/39-usbmuxd.rules", sudo_password)
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

func RemoveUdevRules(w http.ResponseWriter, r *http.Request) {
	err := DeleteFileShell("/etc/udev/rules.d/39-usbmuxd.rules", sudo_password)
	if err != nil {
		JSONError(w, "delete_file_error", err.Error(), 500)
		return
	}
	commandString := "echo '" + sudo_password + "' | sudo -S udevadm control --reload-rules"
	cmd := exec.Command("bash", "-c", commandString)
	err = cmd.Run()
	if err != nil {
		JSONError(w, "reload_udev_rules_error", "Could not reload udev rules: "+err.Error(), 500)
		return
	}
}

func CheckSudoPasswordSet() bool {
	byteValue, err := ReadJSONFile("./env.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "check_sudo_password_set",
		}).Error("Could not read ./env.json while checking sudo password status.")
		return false
	}
	sudo_password := gjson.Get(string(byteValue), "sudo_password").Str
	if sudo_password == "undefined" {
		return false
	}
	return true
}

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
	SimpleJSONResponse(w, "set_sudo_password", "Successfully set '"+sudo_password+"' as sudo password. This password will not be exposed anywhere except inside the ./env.json file. Make sure you don't commit this file to public repos :D", 200)
}

func CheckWDAProvided() bool {
	_, err := os.Stat("WebDriverAgent")
	if err != nil {
		return false
	}
	return true
}

func GetEnvValue(key string) string {
	byteValue, _ := ReadJSONFile("./env.json")
	value := gjson.Get(string(byteValue), key).Str
	return value
}
