package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
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
	DeviceHost                string `json:"device_host"`
}

type ContainerDeviceConfig struct {
	DeviceModel               string `json:"device_model"`
	DeviceOSVersion           string `json:"device_os_version"`
	DeviceOS                  string `json:"device_os"`
	DeviceContainerServerPort string `json:"container_server_port"`
	DeviceUDID                string `json:"device_udid"`
	DeviceHost                string `json:"device_host"`
	DeviceAppiumPort          string `json:"appium_port"`
	IOSWDAPort                string `json:"wda_port"`
	IOSMjpegPort              string `json:"wda_mjpeg_port"`
	IOSScreenSize             string `json:"screen_size"`
	AndroidStreamPort         string `json:"android_stream_port"`
}

type ContainerDeviceInfo struct {
	InstalledApps []string              `json:"installed_apps"`
	DeviceConfig  ContainerDeviceConfig `json:"device_config"`
}

// Available devices html page
func LoadAvailableDevices(w http.ResponseWriter, r *http.Request) {
	// Make functions available in the html template
	funcMap := template.FuncMap{
		"contains": strings.Contains,
	}

	// Parse the template and return response with the created template
	var tmpl = template.Must(template.New("device_selection.html").Funcs(funcMap).ParseFiles("static/device_selection.html", "static/device_selection_table.html"))
	if err := tmpl.ExecuteTemplate(w, "device_selection.html", nil); err != nil {
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
	//var runningContainerNames = getRunningDeviceContainerNames()
	// Generate the data for each device container row in a slice of ContainerRow
	//rows := getAvailableDevicesInfo(runningContainerNames)
	rows := getAvailableDevicesInfoAllProviders()

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
	var info = AvailableDevicesInfo{
		DevicesInfo: getAvailableDevicesInfoAllProviders(),
	}
	fmt.Fprintf(w, PrettifyJSON(ConvertToJSONString(info)))
}

type DeviceControlRequest struct {
	DeviceServer string `json:"device_server"`
}

func GetDevicePage(w http.ResponseWriter, r *http.Request) {
	var deviceRequestData DeviceControlRequest
	err := UnmarshalReader(r.Body, &deviceRequestData)
	if err != nil {
		return
	}

	response, err := http.Get("http://" + deviceRequestData.DeviceServer + "/device-info")
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(responseData))

	var containerDeviceInfo ContainerDeviceInfo

	UnmarshalJSONString(string(responseData), &containerDeviceInfo)
	fmt.Fprintf(w, "%v", ConvertToJSONString(containerDeviceInfo))

}

func getAvailableDevicesInfoAllProviders() []DeviceInfo {
	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		return nil
	}

	var allProviderDevicesInfo []DeviceInfo

	for _, v := range configData.EnvConfig.DeviceProviders {
		var providerDevicesInfo AvailableDevicesInfo
		response, err := http.Get("http://" + v + "/available-devices")
		if err != nil {
			fmt.Print(err.Error())

			return nil
		}

		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		}

		UnmarshalJSONString(string(responseData), &providerDevicesInfo)

		for _, v := range providerDevicesInfo.DevicesInfo {
			allProviderDevicesInfo = append(allProviderDevicesInfo, v)
		}
	}

	return allProviderDevicesInfo
}
