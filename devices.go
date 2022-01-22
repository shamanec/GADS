package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// @Summary      Get logs for iOS device container
// @Description  Get logs by type
// @Tags         device-logs
// @Produce      json
// @Param        log_type path string true "Log Type"
// @Param        device_udid path string true "Device UDID"
// @Success      200 {object} SimpleResponseJSON
// @Failure      200 {object} SimpleResponseJSON
// @Router       /device-logs/{log_type}/{device_udid} [get]
func GetDeviceLogs(w http.ResponseWriter, r *http.Request) {

	// Get the parameters
	vars := mux.Vars(r)
	key := vars["log_type"]
	key2 := vars["device_udid"]

	// Execute the command to restart the container by container ID
	commandString := "tail -n 1000 ./logs/*" + key2 + "/" + key + ".log"
	cmd := exec.Command("bash", "-c", commandString)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_device_logs",
		}).Error("Could not get logs of type: '" + key + "' for device with udid:" + key2)
		SimpleJSONResponse(w, "get_device_logs", "No logs of this type available for this container.", 200)
		return
	}
	log.WithFields(log.Fields{
		"event": "get_device_logs",
	}).Info("Successfully got logs of type: '" + key + "' for device with udid:" + key2)
	SimpleJSONResponse(w, "get_device_logs", out.String(), 200)
}

func ReturnDeviceInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	// Open our jsonFile
	jsonFile, err := os.Open("./configs/config.json")

	// if os.Open returns an error then handle it
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_device_info",
		}).Error("Could not open ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		JSONError(w, "get_device_info", "Could not open ./configs/config.json file. Error: "+err.Error(), 500)
		return
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_device_info",
		}).Error("Could not read ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		JSONError(w, "get_device_info", "Could not read ./configs/config.json file. Error: "+err.Error(), 500)
		return
	}
	json_object := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`")`)
	fmt.Fprintf(w, PrettifyJSON(json_object.Raw))
}

func GetConnectedIOSDevices(w http.ResponseWriter, r *http.Request) {
	deviceList, err := ios.ListDevices()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_connected_ios_devices",
		}).Error("Could not get connected devices. Error: " + err.Error())
		JSONError(w, "get_connected_ios_devices", "Could not get connected devices. Error: "+err.Error(), 500)
		return
	}
	deviceValues, err := outputDetailedList(deviceList)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_connected_ios_devices",
		}).Error("Could not get connected devices detailed list. Error: " + err.Error())
		JSONError(w, "get_connected_ios_devices", "Could not get connected devices detailed list. Error: "+err.Error(), 500)
		return
	}
	log.WithFields(log.Fields{
		"event": "get_connected_ios_devices",
	}).Info("Successfully got connected iOS devices detailed list.")

	fmt.Fprintf(w, PrettifyJSON(deviceValues))
}

type detailsEntry struct {
	Udid           string
	ProductName    string
	ProductType    string
	ProductVersion string
}

func outputDetailedList(deviceList ios.DeviceList) (string, error) {
	result := make([]detailsEntry, len(deviceList.DeviceList))
	for i, device := range deviceList.DeviceList {
		udid := device.Properties.SerialNumber
		allValues, err := ios.GetValues(device)
		if err != nil {
			return "", errors.New("Failed getting device values")
		}
		result[i] = detailsEntry{udid, allValues.Value.ProductName, allValues.Value.ProductType, allValues.Value.ProductVersion}
	}
	return ConvertToJSONString(map[string][]detailsEntry{
		"deviceList": result,
	}), nil
}

func RegisterIOSDevice(w http.ResponseWriter, r *http.Request) {
	// Get the request json body and extract the device UDID and OS version
	requestBody, _ := ioutil.ReadAll(r.Body)
	device_udid := gjson.Get(string(requestBody), "device_udid")
	device_os_version := gjson.Get(string(requestBody), "device_os_version")
	device_name := gjson.Get(string(requestBody), "device_name")

	// Open the configuration json file
	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "register_ios_device",
		}).Error("Could not open ./configs/config.json when attempting to register iOS device with UDID: '" + device_udid.Str + "'")
		JSONError(w, "register_ios_device", "Could not open the config.json file.", 500)
	}
	defer jsonFile.Close()

	// Read the configuration json file into byte array
	configJson, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "register_ios_device",
		}).Error("Could not read ./configs/config.json when attempting to register iOS device with UDID: '" + device_udid.Str + "'")
		JSONError(w, "register_ios_device", "Could not read the config.json file.", 500)
	}

	// Get the UDIDs of all devices registered in the config.json
	jsonDevicesUDIDs := gjson.Get(string(configJson), "devicesList.#.device_udid")

	//Loop over the devices UDIDs and return message if device is already registered
	// for _, udid := range jsonDevicesUDIDs.Array() {
	// 	if udid.String() == device_udid.String() {
	// 		log.WithFields(log.Fields{
	// 			"event": "register_ios_device",
	// 		}).Error("Attempted to register an already registered iOS device with UDID: '" + device_udid.Str + "'")
	// 		JSONError(w, "device_registered", "The device with UDID: "+device_udid.Str+" is already registered.", 400)
	// 		return
	// 	}
	// }

	// Create the object for the new device
	var deviceInfo = Device{
		AppiumPort:      4841 + len(jsonDevicesUDIDs.Array()),
		DeviceName:      device_name.Str,
		DeviceOSVersion: device_os_version.String(),
		DeviceUDID:      device_udid.String(),
		WdaMjpegPort:    20101 + len(jsonDevicesUDIDs.Array()),
		WdaPort:         20001 + len(jsonDevicesUDIDs.Array())}

	// Append the new device object to the devicesList array
	updatedJSON, _ := sjson.Set(string(configJson), "devicesList.-1", deviceInfo)

	// Prettify the json so it looks good inside the file
	//var prettyJSON bytes.Buffer
	//json.Indent(&prettyJSON, []byte(updatedJSON), "", "  ")

	// Write the new json to the config.json file
	err = ioutil.WriteFile("./configs/config.json", []byte(PrettifyJSON(updatedJSON)), 0644)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "register_ios_device",
		}).Error("Could not write to ./configs/config.json when attempting to register iOS device with UDID: '" + device_udid.Str + "'")
		JSONError(w, "config_file_error", "Could not write to the config.json file.", 400)
	}
	log.WithFields(log.Fields{
		"event": "register_ios_device",
	}).Info("Successfully registered iOS device with UDID: '" + device_udid.Str + "' in ./configs/config.json")
}

func IOSDeviceState(w http.ResponseWriter, r *http.Request) {
	// Get the parameters
	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	device, err := ios.GetDevice(device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_state",
		}).Error("Could not get device with UDID: '" + device_udid + "'. Error: " + err.Error())
		JSONError(w, "ios_device_state", "Could not get device with UDID: '"+device_udid+"'", 500)
	}

	control, _ := instruments.NewDeviceStateControl(device)
	profileTypes, _ := control.List()

	if r.Method == "GET" {
		ConvertToJSONString(profileTypes)
		fmt.Fprintf(w, PrettifyJSON(ConvertToJSONString(profileTypes)))
	} else if r.Method == "POST" {
		requestBody, _ := ioutil.ReadAll(r.Body)
		profileTypeId := gjson.Get(string(requestBody), "profileTypeID").Str
		profileId := gjson.Get(string(requestBody), "profileID").Str

		pType, profile, _ := instruments.VerifyProfileAndType(profileTypes, profileTypeId, profileId)
		if pType.ActiveProfile == profileId {
			err = control.Disable(pType)
			SimpleJSONResponse(w, "ios_device_state", "Disabled profile with ID:'"+profileId+"' for profile type with ID:'"+profileTypeId+"' for device with UDID:'"+device_udid+"'", 200)
		} else {
			err = control.Enable(pType, profile)
			SimpleJSONResponse(w, "ios_device_state", "Enabled profile with ID:'"+profileId+"' for profile type with ID:'"+profileTypeId+"' for device with UDID:'"+device_udid+"'", 200)
		}
	}
}
