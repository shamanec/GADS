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
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

//=================//
//=====STRUCTS=====//

type detailsList struct {
	DetailsList []detailsEntry `json:"deviceList"`
}

type detailsEntry struct {
	Udid           string
	ProductName    string
	ProductType    string
	ProductVersion string
}

type IOSDeviceInfo struct {
	BundleIDs    []string   `json:"installedAppsBundleIDs"`
	DeviceConfig *IOSDevice `json:"deviceConfig"`
}

type AndroidDeviceInfo struct {
	BundleIDs    []string   `json:"installedAppsBundleIDs"`
	DeviceConfig *IOSDevice `json:"deviceConfig"`
}

type iOSAppInstall struct {
	IpaName string `json:"ipa_name"`
}

type iOSAppUninstall struct {
	BundleID string `json:"bundle_id"`
}

type iOSDevice struct {
	AppiumPort      int    `json:"appium_port"`
	DeviceName      string `json:"device_name"`
	DeviceOSVersion string `json:"device_os_version"`
	DeviceUDID      string `json:"device_udid"`
	WdaMjpegPort    int    `json:"wda_mjpeg_port"`
	WdaPort         int    `json:"wda_port"`
}

type registerIOSDevice struct {
	DeviceUDID      string `json:"device_udid"`
	DeviceName      string `json:"device_name"`
	DeviceOSVersion string `json:"device_os_version"`
}

//=======================//
//=====API FUNCTIONS=====//

// @Summary      Register a new iOS device
// @Description  Registers a new iOS device in config.json
// @Tags         ios-devices
// @Produce      json
// @Param        config body registerIOSDevice true "Register iOS device"
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /ios-devices/register [post]
func RegisterIOSDevice(w http.ResponseWriter, r *http.Request) {
	requestBody, _ := ioutil.ReadAll(r.Body)
	device_udid := gjson.Get(string(requestBody), "device_udid")
	device_os_version := gjson.Get(string(requestBody), "device_os_version")
	device_name := gjson.Get(string(requestBody), "device_name")

	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "register_ios_device",
		}).Error("Could not open ./configs/config.json when attempting to register iOS device with UDID: '" + device_udid.Str + "'")
		JSONError(w, "register_ios_device", "Could not open the config.json file.", 500)
	}
	defer jsonFile.Close()

	configJson, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "register_ios_device",
		}).Error("Could not read ./configs/config.json when attempting to register iOS device with UDID: '" + device_udid.Str + "'")
		JSONError(w, "register_ios_device", "Could not read the config.json file.", 500)
	}

	jsonDevicesUDIDs := gjson.Get(string(configJson), "ios-devices-list.#.device_udid")

	//Loop over the devices UDIDs and return message if device is already registered
	for _, udid := range jsonDevicesUDIDs.Array() {
		if udid.String() == device_udid.String() {
			log.WithFields(log.Fields{
				"event": "register_ios_device",
			}).Error("Attempted to register an already registered iOS device with UDID: '" + device_udid.Str + "'")
			JSONError(w, "device_registered", "The device with UDID: "+device_udid.Str+" is already registered.", 400)
			return
		}
	}

	var deviceInfo = iOSDevice{
		AppiumPort:      4841 + len(jsonDevicesUDIDs.Array()),
		DeviceName:      device_name.Str,
		DeviceOSVersion: device_os_version.String(),
		DeviceUDID:      device_udid.String(),
		WdaMjpegPort:    20101 + len(jsonDevicesUDIDs.Array()),
		WdaPort:         20001 + len(jsonDevicesUDIDs.Array())}

	updatedJSON, _ := sjson.Set(string(configJson), "ios-devices-list.-1", deviceInfo)

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
	SimpleJSONResponse(w, "Successfully registered iOS device with UDID: '"+device_udid.Str+"' in ./configs/config.json", 200)
}

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
	vars := mux.Vars(r)
	key := vars["log_type"]
	key2 := vars["device_udid"]

	commandString := "tail -n 1000 ./logs/*" + key2 + "/" + key + ".log"
	cmd := exec.Command("bash", "-c", commandString)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_device_logs",
		}).Error("Could not get logs of type: '" + key + "' for device with udid:" + key2)
		SimpleJSONResponse(w, "No logs of this type available for this container.", 200)
		return
	}
	log.WithFields(log.Fields{
		"event": "get_device_logs",
	}).Info("Successfully got logs of type: '" + key + "' for device with udid:" + key2)
	SimpleJSONResponse(w, out.String(), 200)
}

// @Summary      Get connected iOS devices
// @Description  Returns the connected iOS devices with go-ios
// @Tags         ios-devices
// @Produce      json
// @Success      200 {object} detailsList
// @Failure      500 {object} ErrorJSON
// @Router       /ios-devices [get]
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

// @Summary      Install app on iOS device
// @Description  Installs *.ipa or *.app from the './apps' folder with go-ios
// @Tags         ios-devices
// @Produce      json
// @Param        device_udid path string true "Device UDID"
// @Param        config body iOSAppInstall true "Install iOS app"
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
	SimpleJSONResponse(w, "Successfully installed '"+ipa_name+"'", 200)
}

// @Summary      Uninstall app from iOS device
// @Description  Uninstalls app from iOS device by provided bundleID with go-ios
// @Tags         ios-devices
// @Produce      json
// @Param        device_udid path string true "Device UDID"
// @Param        config body iOSAppUninstall true "Uninstall iOS app"
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /ios-devices/{device_udid}/uninstall-app [post]
func UninstallIOSApp(w http.ResponseWriter, r *http.Request) {
	requestBody, _ := ioutil.ReadAll(r.Body)
	bundle_id := gjson.Get(string(requestBody), "bundle_id").Str

	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	err := uninstallIOSApp(device_udid, bundle_id)
	if err != nil {
		JSONError(w, "uninstall_ios_app", "Failed uninstalling app with bundleID:'"+bundle_id+"'", 500)
		return
	}
	SimpleJSONResponse(w, "Successfully uninstalled app with bundleID:'"+bundle_id+"'", 200)
}

//===================//
//=====FUNCTIONS=====//

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
	err = conn.SendFile("./apps/" + ipa_name)
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

func uninstallIOSApp(device_udid string, bundle_id string) error {
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
