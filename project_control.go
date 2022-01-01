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
	err = CreateDevice2DockerFile()
	if err != nil {
		JSONError(w, "create_device2docker_error", err.Error(), 500)
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
	DeleteFileShell("./90-usbmuxd.rules", sudo_password)
	DeleteFileShell("./39-usbmuxd.rules", sudo_password)
	DeleteFileShell("./ios_device2docker", sudo_password)
}

func UdevIOSListenerState() (status string) {
	_, firstRuleErr := os.Stat("/etc/udev/rules.d/90-usbmuxd.rules")
	_, secondRuleErr := os.Stat("/etc/udev/rules.d/39-usbmuxd.rules")
	if firstRuleErr != nil || secondRuleErr != nil {
		status = "Udev rules not set."
		return
	} else {
		status = "Udev rules set."
		return
	}
}

func CreateUdevRules() error {
	// Create the rules file that will start/remove containers on event
	create_container_rules, err := os.Create("./90-usbmuxd.rules")
	if err != nil {
		return errors.New("Could not create 90-usbmuxd.rules")
	}
	defer create_container_rules.Close()
	// Create the rules file that will start usbmuxd on the first connected device
	start_usbmuxd_rule, err := os.Create("./39-usbmuxd.rules")
	if err != nil {
		return errors.New("Could not create 39-usbmuxd.rules")
	}
	defer start_usbmuxd_rule.Close()
	// Open the configuration json file
	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		return errors.New("Could not open the config.json file.")
	}
	defer jsonFile.Close()

	// Read the configuration json file into byte array
	configJson, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return errors.New("Could not read the config.json file.")
	}

	// Get the UDIDs of all devices registered in the config.json
	jsonDevicesUDIDs := gjson.Get(string(configJson), "devicesList.#.device_udid")

	// For each udid create a new line inside the 90-usbmuxd.rules file
	for _, udid := range jsonDevicesUDIDs.Array() {
		rule_line1 := "ACTION==\"add\", SUBSYSTEM==\"usb\", ENV{DEVTYPE}==\"usb_device\", ATTR{manufacturer}==\"Apple Inc.\", ATTR{serial}==\"" + udid.Str + "\", OWNER=\"shamanec\", MODE=\"0666\", RUN+=\"/usr/local/bin/ios_device2docker " + udid.Str + "\""
		rule_line2 := "SUBSYSTEM==\"usb\", ENV{DEVTYPE}==\"usb_device\", ENV{PRODUCT}==\"5ac/12[9a][0-9a-f]/*|5ac/1901/*|5ac/8600/*\", ACTION==\"remove\", RUN+=\"/usr/local/bin/ios_device2docker\""
		if _, err := create_container_rules.WriteString(rule_line1 + "\n"); err != nil {
			return errors.New("Could not write to 90-usbmuxd.rules")
		}

		if _, err := create_container_rules.WriteString(rule_line2 + "\n"); err != nil {
			return errors.New("Could not write to 90-usbmuxd.rules")
		}
	}

	// Update the rule that starts usbmuxd
	// if _, err := start_usbmuxd_rule.WriteString("SUBSYSTEM==\"usb\", ENV{DEVTYPE}==\"usb_device\", ENV{PRODUCT}==\"5ac/12[9a][0-9a-f]/*|5ac/1901/*|5ac/8600/*\", OWNER=\"shamanec\", ACTION==\"add\", RUN+=\"/usr/local/sbin/usbmuxd -u -v -z -U shamanec\""); err != nil {
	if _, err := start_usbmuxd_rule.WriteString("SUBSYSTEM==\"usb\", ENV{DEVTYPE}==\"usb_device\", ENV{PRODUCT}==\"5ac/12[9a][0-9a-f]/*|5ac/1901/*|5ac/8600/*\", OWNER=\"shamanec\", ACTION==\"add\", RUN+=\"/bin/sleep 5\""); err != nil {
		return errors.New("Could not write to 39-usbmuxd.rules")
	}
	return nil
}

func CreateDevice2DockerFile() error {
	project_dir, err := os.Getwd()
	if err != nil {
		return errors.New("Could not get current project path")
	}
	// Execute the command to restart the container by container ID
	commandString := "sed -e \"s|project_dir|" + project_dir + "|g\" configs/default_ios_device2docker > ios_device2docker"
	cmd := exec.Command("bash", "-c", commandString)
	err = cmd.Run()
	if err != nil {
		return errors.New("Could not create ios_device2docker file")
	}
	return nil
}

func SetUdevRules() error {
	err := CopyFileShell("./90-usbmuxd.rules", "/etc/udev/rules.d/90-usbmuxd.rules", sudo_password)
	if err != nil {
		return err
	}
	err = CopyFileShell("./39-usbmuxd.rules", "/etc/udev/rules.d/39-usbmuxd.rules", sudo_password)
	if err != nil {
		return err
	}
	err = CopyFileShell("./ios_device2docker", "/usr/local/bin/ios_device2docker", sudo_password)
	if err != nil {
		return err
	}
	err = SetFilePermissionsShell("/usr/local/bin/ios_device2docker", "0777", sudo_password)
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
	err := DeleteFileShell("/etc/udev/rules.d/90-usbmuxd.rules", sudo_password)
	if err != nil {
		JSONError(w, "delete_file_error", err.Error(), 500)
	}
	err = DeleteFileShell("/etc/udev/rules.d/39-usbmuxd.rules", sudo_password)
	if err != nil {
		JSONError(w, "delete_file_error", err.Error(), 500)
	}
	err = DeleteFileShell("/usr/local/bin/ios_device2docker", sudo_password)
	if err != nil {
		JSONError(w, "delete_file_error", err.Error(), 500)
	}
	commandString := "echo '" + sudo_password + "' | sudo -S udevadm control --reload-rules"
	cmd := exec.Command("bash", "-c", commandString)
	err = cmd.Run()
	if err != nil {
		JSONError(w, "reload_udev_rules_error", "Could not reload udev rules: "+err.Error(), 500)
	}
}

func CheckSudoPasswordSet() bool {
	byteValue, err := ReadJSONFile("./env.json")
	if err != nil {
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
		fmt.Errorf(err.Error())
	}
	updatedJSON, _ := sjson.Set(string(byteValue), "sudo_password", sudo_password)

	// Prettify the json so it looks good inside the file
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, []byte(updatedJSON), "", "  ")

	// Write the new json to the config.json file
	err = ioutil.WriteFile("./env.json", []byte(prettyJSON.String()), 0644)
	if err != nil {
		JSONError(w, "env_file_error", "Could not write to the env.json file.", 400)
	}

}

func GetEnvValue(key string) string {
	byteValue, _ := ReadJSONFile("./env.json")
	value := gjson.Get(string(byteValue), key).Str
	return value
}
