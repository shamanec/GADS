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
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
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

type IOSDeviceInfo struct {
	BundleIDs    []string   `json:"installedAppsBundleIDs"`
	DeviceConfig *IOSDevice `json:"deviceConfig"`
}

type AndroidDeviceInfo struct {
	BundleIDs    []string   `json:"installedAppsBundleIDs"`
	DeviceConfig *IOSDevice `json:"deviceConfig"`
}

// @Summary      Get info for iOS device
// @Description  Get info for an iOS device - installed apps, Appium config
// @Tags         ios-devices
// @Produce      json
// @Param        device_udid path string true "Device UDID"
// @Success      200 {object} IOSDeviceInfo
// @Failure      500 {object} ErrorJSON
// @Router       /ios-devices/{device_udid}/info [get]
func GetIOSDeviceInfo(w http.ResponseWriter, r *http.Request) {
	// Get the parameters
	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	var installed_apps []string
	installed_apps, err := IOSDeviceApps(device_udid)
	if err != nil {
		installed_apps = append(installed_apps, "")
	}

	var device_config *IOSDevice
	device_config, err = IOSDeviceConfig(device_udid)

	device_info := IOSDeviceInfo{BundleIDs: installed_apps, DeviceConfig: device_config}
	fmt.Fprintf(w, PrettifyJSON(ConvertToJSONString(device_info)))
}

func IOSDeviceConfig(device_udid string) (*IOSDevice, error) {
	jsonFile, err := os.Open("./configs/config.json")

	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_ios_device_config",
		}).Error("Could not open ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		return nil, errors.New("Could not open ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_ios_device_config",
		}).Error("Could not read ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		return nil, errors.New("Could not read ./configs/config.json file when attempting to get info for device with UDID: '" + device_udid + "' . Error: " + err.Error())
	}
	appium_port := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").appium_port`)
	if appium_port.Raw == "" {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
		return nil, errors.New("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
	}
	device_name := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").device_name`)
	device_os_version := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").device_os_version`)
	wda_mjpeg_port := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").wda_mjpeg_port`)
	wda_port := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").wda_port`)

	return &IOSDevice{
			AppiumPort:      int(appium_port.Num),
			DeviceName:      device_name.Str,
			DeviceOSVersion: device_os_version.Str,
			WdaMjpegPort:    int(wda_mjpeg_port.Num),
			WdaPort:         int(wda_port.Num),
			DeviceUDID:      device_udid},
		nil
}

func IOSDeviceApps(device_udid string) ([]string, error) {
	device, err := ios.GetDevice(device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_apps",
		}).Error("Could not get device with UDID: '" + device_udid + "'. Error: " + err.Error())
		return nil, errors.New("Could not get device with UDID: '" + device_udid + "'. Error: " + err.Error())
	}

	svc, err := installationproxy.New(device)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_apps",
		}).Error("Could not create installation proxy for device with UDID: '" + device_udid + "'. Error: " + err.Error())
		return nil, errors.New("Could not create installation proxy for device with UDID: '" + device_udid + "'. Error: " + err.Error())
	}

	user_apps, err := svc.BrowseUserApps()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_apps",
		}).Error("Could not get user apps for device with UDID: '" + device_udid + "'. Error: " + err.Error())
		return nil, errors.New("Could not get user apps for device with UDID: '" + device_udid + "'. Error: " + err.Error())
	}

	var bundleIDs []string
	parsedBundleIDs := gjson.Get(ConvertToJSONString(user_apps), "#.CFBundleIdentifier")
	for _, bundleID := range parsedBundleIDs.Array() {
		bundleIDs = append(bundleIDs, bundleID.Str)
	}

	return bundleIDs, nil
}

type IOSAppInstall struct {
	IpaName string `json:"ipa_name"`
}

// @Summary      Install app on iOS device
// @Description  Installs *.ipa or *.app from the './ipa' folder
// @Tags         ios-devices
// @Produce      json
// @Param        device_udid path string true "Device UDID"
// @Param        config body IOSAppInstall true "Install iOS app"
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /ios-devices/{device_udid}/install-app [post]
func InstallIOSApp(w http.ResponseWriter, r *http.Request) {
	requestBody, _ := ioutil.ReadAll(r.Body)
	ipa_name := gjson.Get(string(requestBody), "ipa_name").Str

	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	err := InstallIOSAppLocal(device_udid, ipa_name)
	if err != nil {
		JSONError(w, "install_ios_app", "Failed to install app on device with UDID:'"+device_udid+"'", 500)
		return
	}
	SimpleJSONResponse(w, "install_ios_app", "Successfully installed '"+ipa_name+"'", 200)
}

func InstallIOSAppLocal(device_udid string, ipa_name string) error {
	device, err := ios.GetDevice(device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "install_ios_app",
		}).Error("Could not get device with UDID: '" + device_udid + "'. Error: " + err.Error())
		return errors.New("Error")
	}

	conn, err := zipconduit.New(device)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "install_ios_app",
		}).Error("Failed connecting zipconduit when installing app:'" + ipa_name + "' on device with UDID: '" + device_udid + "'. Error: " + err.Error())
		return errors.New("Error")
	}

	// Disable logging from go-ios
	log.SetOutput(ioutil.Discard)
	err = conn.SendFile("./ipa/" + ipa_name)
	// Re-enable logging after finishing conn.SendFile()
	log.SetOutput(project_log_file)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "install_ios_app",
		}).Error("Failed writing app on device with UDID: '" + device_udid + "'. Error: " + err.Error())
		return errors.New("Error")
	}
	return nil
}

type IOSAppUninstall struct {
	BundleID string `json:"bundle_id"`
}

// @Summary      Uninstall app from iOS device
// @Description  Uninstalls app from iOS device by provided bundleID
// @Tags         ios-devices
// @Produce      json
// @Param        device_udid path string true "Device UDID"
// @Param        config body IOSAppUninstall true "Uninstall iOS app"
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /ios-devices/{device_udid}/uninstall-app [post]
func UninstallIOSApp(w http.ResponseWriter, r *http.Request) {
	requestBody, _ := ioutil.ReadAll(r.Body)
	bundle_id := gjson.Get(string(requestBody), "bundle_id").Str

	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	err := UninstallIOSAppLocal(device_udid, bundle_id)
	if err != nil {
		JSONError(w, "uninstall_ios_app", "Failed uninstalling app with bundleID:'"+bundle_id+"'", 500)
		return
	}
	SimpleJSONResponse(w, "uninstall_ios_app", "Successfully uninstalled app with bundleID:'"+bundle_id+"'", 200)
}

func UninstallIOSAppLocal(device_udid string, bundle_id string) error {
	device, err := ios.GetDevice(device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "uninstall_ios_app",
		}).Error("Could not get device with UDID: '" + device_udid + "' when uninstalling app with bundleID:'" + bundle_id + "'. Error: " + err.Error())
		return errors.New("Error")
	}

	svc, err := installationproxy.New(device)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "uninstall_ios_app",
		}).Error("Failed connecting installationproxy when uninstalling app with bundleID:'" + bundle_id + "'on device with UDID: '" + device_udid + "'. Error: " + err.Error())
		return errors.New("Error")
	}

	// Disable logging from go-ios
	log.SetOutput(ioutil.Discard)
	err = svc.Uninstall(bundle_id)
	// Re-enable logging after finishing svs.Uninstall()
	log.SetOutput(project_log_file)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "uninstall_ios_app",
		}).Error("Failed uninstalling app with bundleID:'" + bundle_id + "'on device with UDID: '" + device_udid + "'. Error: " + err.Error())
		return errors.New("Error")
	}
	return nil
}

func GetIOSDeviceMjpegStreamURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	jsonFile, err := os.Open("./logs/*" + device_udid + ".json")

	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_wda_url",
		}).Error("Could not open WDA url file for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		fmt.Fprintf(w, PrettifyJSON("{\"wda_url\":\"\""))
		return
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_wda_url",
		}).Error("Could not read WDA file for device with UDID: '" + device_udid + "' . Error: " + err.Error())
		fmt.Fprintf(w, PrettifyJSON("{\"wda_url\":\"\""))
		return
	}
	url := gjson.Get(string(byteValue), `wda_url`)
	fmt.Fprintf(w, PrettifyJSON("{\"wda_url\":\""+url.Str+"\""))
}
