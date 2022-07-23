package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

//=================//
//=====STRUCTS=====//

type IOSDeviceInfo struct {
	BundleIDs    []string   `json:"installedAppsBundleIDs"`
	DeviceConfig *IOSDevice `json:"deviceConfig"`
}

type AndroidDeviceInfo struct {
	BundleIDs    []string       `json:"installedAppsBundleIDs"`
	DeviceConfig *AndroidDevice `json:"deviceConfig"`
}

type DeviceControlInfo struct {
	RunningContainers []string            `json:"running-containers"`
	IOSInfo           []IOSDeviceInfo     `json:"ios-devices-info"`
	AndroidInfo       []AndroidDeviceInfo `json:"android-devices-info"`
	InstallableApps   []string            `json:"installable-apps"`
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

// Load the device control page
func GetDeviceControlPage(w http.ResponseWriter, r *http.Request) {
	var device_control_page = template.Must(template.ParseFiles("static/device_control.html"))
	if err := device_control_page.Execute(w, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "device_control_page",
		}).Error("Couldn't load device_control.html")
		return
	}
}

// @Summary      Get device control info
// @Description  Provides the running containers, IOS devices info and apps available for installing
// @Tags         devices
// @Produce      json
// @Success      200 {object} DeviceControlInfo
// @Failure      500 {object} JsonErrorResponse
// @Router       /devices/device-control [post]
func GetDeviceControlInfo(w http.ResponseWriter, r *http.Request) {
	var runningContainerNames = getRunningDeviceContainerNames()
	var info = DeviceControlInfo{
		RunningContainers: runningContainerNames,
		IOSInfo:           getIOSDevicesInfo(runningContainerNames),
		InstallableApps:   getInstallableApps(),
		AndroidInfo:       getAndroidDevicesInfo(runningContainerNames),
	}
	fmt.Fprintf(w, PrettifyJSON(ConvertToJSONString(info)))
}

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

	err := UnmarshalReader(r.Body, &data)
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

	err := UnmarshalReader(r.Body, &data)
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
			AppiumPort:          deviceConfig.AppiumPort,
			DeviceName:          deviceConfig.DeviceName,
			DeviceOSVersion:     deviceConfig.DeviceOSVersion,
			WdaMjpegPort:        deviceConfig.WDAMjpegPort,
			WdaPort:             deviceConfig.WDAPort,
			DeviceUDID:          device_udid,
			WdaMjpegURL:         wda_stream_url,
			WdaURL:              wda_url,
			DeviceModel:         "Remove please",
			DeviceScreenSize:    deviceConfig.ScreenSize,
			ContainerServerPort: deviceConfig.ContainerServerPort},
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

// Get the names of the currently running containers(that are for devices)
func getRunningDeviceContainerNames() []string {
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
