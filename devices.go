package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
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
	BundleIDs    []string       `json:"installedAppsBundleIDs"`
	DeviceConfig *AndroidDevice `json:"deviceConfig"`
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

type installIOSAppRequest struct {
	IpaName string `json:"ipa_name"`
}

type uninstallIOSAppRequest struct {
	BundleID string `json:"bundle_id"`
}

type goIOSAppList []struct {
	BundleID string `json:"CFBundleIdentifier"`
}

//=======================//
//=====API FUNCTIONS=====//

// @Summary      Get logs for iOS device container
// @Description  Get logs by type
// @Tags         device-logs
// @Produce      json
// @Param        log_type path string true "Log Type"
// @Param        device_udid path string true "Device UDID"
// @Success      200 {object} JsonResponse
// @Failure      200 {object} JsonResponse
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
// @Failure      500 {object} JsonErrorResponse
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
// @Param        config body installIOSAppRequest true "Install iOS app"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /ios-devices/{device_udid}/install-app [post]
func InstallIOSApp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var data installIOSAppRequest

	err := UnmarshalRequestBody(r.Body, &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_create",
		}).Error("Could not unmarshal request body when installing iOS app")
		return
	}

	err = InstallIOSAppLocal(vars["device_udid"], data.IpaName)
	if err != nil {
		JSONError(w, "install_ios_app", "Failed to install app on device with UDID:'"+vars["device_udid"]+"'", 500)
		return
	}
	SimpleJSONResponse(w, "Successfully installed '"+data.IpaName+"'", 200)
}

// @Summary      Uninstall app from iOS device
// @Description  Uninstalls app from iOS device by provided bundleID with go-ios
// @Tags         ios-devices
// @Produce      json
// @Param        device_udid path string true "Device UDID"
// @Param        config body uninstallIOSAppRequest true "Uninstall iOS app"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /ios-devices/{device_udid}/uninstall-app [post]
func UninstallIOSApp(w http.ResponseWriter, r *http.Request) {
	var data uninstallIOSAppRequest

	err := UnmarshalRequestBody(r.Body, &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_create",
		}).Error("Could not unmarshal request body when uninstalling iOS app")
		return
	}

	bundle_id := data.BundleID

	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	err = uninstallIOSApp(device_udid, bundle_id)
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

	var data goIOSAppList

	err = UnmarshalJSONString(ConvertToJSONString(user_apps), &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_create",
		}).Error("Could not unmarshal request body when uninstalling iOS app")
		return nil, errors.New("Could not unmarshal user apps json")
	}

	var bundleIDs []string
	for _, dataObject := range data {
		bundleIDs = append(bundleIDs, dataObject.BundleID)
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
