package main

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

type AvailableDevicesInfo struct {
	DevicesInfo []DeviceInfo `json:"devices-info"`
}

type DeviceInfo struct {
	DeviceModel               string `json:"device_model"`
	DeviceOSVersion           string `json:"device_os_version"`
	DeviceOS                  string `json:"device_os"`
	DeviceContainerServerPort int    `json:"container_server_port"`
	DeviceUDID                string `json:"device_udid"`
	DeviceImage               string `json:"device_image"`
}

// Available devices html page
func LoadAvailableDevices(w http.ResponseWriter, r *http.Request) {
	var runningContainerNames = getRunningDeviceContainerNames()
	// Generate the data for each device container row in a slice of ContainerRow
	rows := getAvailableDevicesInfo(runningContainerNames)

	// Make functions available in the html template
	funcMap := template.FuncMap{
		"contains": strings.Contains,
	}

	// Parse the template and return response with the container table rows
	var tmpl = template.Must(template.New("device_selection.html").Funcs(funcMap).ParseFiles("static/device_selection.html", "static/device_selection_table.html"))
	if err := tmpl.ExecuteTemplate(w, "device_selection.html", rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// @Summary      Refresh the available devices
// @Description  Refreshes the currently available devices by returning an updated HTML table
// @Produce      html
// @Success      200
// @Failure      500
// @Router       /refresh-device-containers [post]
func RefreshAvailableDevices(w http.ResponseWriter, r *http.Request) {
	var runningContainerNames = getRunningDeviceContainerNames()
	// Generate the data for each device container row in a slice of ContainerRow
	rows := getAvailableDevicesInfo(runningContainerNames)

	// Make functions available in html template
	funcMap := template.FuncMap{
		// The name "title" is what the function will be called in the template text.
		"contains": strings.Contains,
	}

	// Parse the template and return response with the container table rows
	// This will generate only the device table, not the whole page
	var tmpl = template.Must(template.New("device_selection_table").Funcs(funcMap).ParseFiles("static/device_selection_table.html"))

	// Reply with the new table
	if err := tmpl.ExecuteTemplate(w, "device_selection_table", rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// @Summary      Get available devices info
// @Description  Provides info of the currently available devices
// @Tags         devices
// @Produce      json
// @Success      200 {object} AvailableDevicesInfo
// @Failure      500 {object} JsonErrorResponse
// @Router       /devices/available-devices [post]
func GetAvailableDevicesInfo(w http.ResponseWriter, r *http.Request) {
	var runningContainerNames = getRunningDeviceContainerNames()
	var info = AvailableDevicesInfo{
		DevicesInfo: getAvailableDevicesInfo(runningContainerNames),
	}
	fmt.Fprintf(w, PrettifyJSON(ConvertToJSONString(info)))
}

func getAvailableDevicesInfo(runningContainers []string) []DeviceInfo {
	var combinedInfo []DeviceInfo

	for _, containerName := range runningContainers {
		// Extract the device UDID from the container name
		re := regexp.MustCompile("[^_]*$")
		device_udid := re.FindStringSubmatch(containerName)

		var device_config *DeviceInfo
		device_config = getDeviceInfo(device_udid[0])

		combinedInfo = append(combinedInfo, *device_config)
	}

	return combinedInfo
}

func getDeviceInfo(device_udid string) *DeviceInfo {
	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not unmarshal config.json file when trying to create a container for device with udid: " + device_udid)
		return nil
	}

	var deviceConfig DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			deviceConfig = v
		}
	}

	return &DeviceInfo{
		DeviceModel:               deviceConfig.DeviceModel,
		DeviceOSVersion:           deviceConfig.DeviceOSVersion,
		DeviceOS:                  deviceConfig.OS,
		DeviceContainerServerPort: deviceConfig.ContainerServerPort,
		DeviceUDID:                deviceConfig.DeviceUDID,
		DeviceImage:               deviceConfig.DeviceImage,
	}
}
