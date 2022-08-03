package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type AvailableDevicesInfo struct {
	DevicesInfo []ContainerDeviceConfig `json:"devices-info"`
}

type ContainerDeviceConfig struct {
	DeviceModel               string `json:"device_model"`
	DeviceOSVersion           string `json:"device_os_version"`
	DeviceOS                  string `json:"device_os"`
	DeviceContainerServerPort string `json:"container_server_port"`
	DeviceUDID                string `json:"device_udid"`
	DeviceHost                string `json:"device_host"`
	DeviceAppiumPort          string `json:"appium_port"`
	WdaPort                   string `json:"wda_port"`
	StreamPort                string `json:"stream_port"`
	ScreenSize                string `json:"screen_size"`
	DeviceImage               string `json:"device_image"`
}

func AvailableDevicesWSLocal(conn *websocket.Conn) {
	for {
		devices := cachedDevicesConfig
		var html_message []byte

		// Make functions available in html template
		funcMap := template.FuncMap{
			"contains": strings.Contains,
		}

		var tmpl = template.Must(template.New("device_selection_table").Funcs(funcMap).ParseFiles("static/device_selection_table.html"))

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "device_selection_table", devices)

		if err != nil {
			log.WithFields(log.Fields{
				"event": "send_devices_over_ws",
			}).Error("Could not execute template when sending devices over ws: " + err.Error())
			return
		}

		if devices == nil {
			html_message = []byte(`<h1 style="align-items: center;">No devices available</h1>`)
		} else {
			html_message = []byte(buf.String())
		}

		if err := conn.WriteMessage(1, html_message); err != nil {
			log.WithFields(log.Fields{
				"event": "send_devices_over_ws",
			}).Error("Could not send devices template over ws: " + err.Error())
			return
		}

		time.Sleep(2 * time.Second)
	}

}

// This var is used to store last devices update from all providers
var cachedDevicesConfig []ContainerDeviceConfig

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

func RefreshAvailableDevices(w http.ResponseWriter, r *http.Request) {
	devices := cachedDevicesConfig

	// Make functions available in html template
	funcMap := template.FuncMap{
		// The name "title" is what the function will be called in the template text.
		"contains": strings.Contains,
	}

	// Parse the template and return response with the updated devices list
	// This will generate only the devices list, not the whole page
	var tmpl = template.Must(template.New("device_selection_table").Funcs(funcMap).ParseFiles("static/device_selection_table.html"))

	// Reply with the new devices list
	if err := tmpl.ExecuteTemplate(w, "device_selection_table", devices); err != nil {
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
		DevicesInfo: cachedDevicesConfig,
	}

	fmt.Fprintf(w, PrettifyJSON(ConvertToJSONString(info)))
}

// @Summary      Load the page for a selected device
// @Description  Loads the page for a selected device from the device selection page
// @Produce      html
// @Success      200
// @Failure      500
// @Router       /devices/control/{device_host}/{device_udid} [post]
func GetDevicePage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	var selected_device ContainerDeviceConfig

	// Loop through the cached devices and search for the selected device
	for _, v := range cachedDevicesConfig {
		if v.DeviceUDID == device_udid {
			selected_device = v
		}
	}

	// If the device does not exist in the cached devices
	if selected_device == (ContainerDeviceConfig{}) {
		fmt.Println("error")
		return
	}

	// Calculate the width and height for the canvas
	canvasWidth, canvasHeight := calculateCanvasDimensions(selected_device.ScreenSize)

	pageData := struct {
		ContainerDeviceConfig ContainerDeviceConfig
		CanvasWidth           string
		CanvasHeight          string
	}{
		ContainerDeviceConfig: selected_device,
		CanvasWidth:           canvasWidth,
		CanvasHeight:          canvasHeight,
	}

	// Parse the template and return response with the container table rows
	// This will generate only the device table, not the whole page
	var tmpl = template.Must(template.ParseFiles("static/device_control_new.html"))

	// Reply with the new table
	if err := tmpl.Execute(w, pageData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

// Calculate the device stream canvas dimensions
func calculateCanvasDimensions(size string) (canvasWidth string, canvasHeight string) {
	// Get the width and height provided
	dimensions := strings.Split(size, "x")
	widthString := dimensions[0]
	heightString := dimensions[1]

	// Convert them to ints
	width, _ := strconv.Atoi(widthString)
	height, _ := strconv.Atoi(heightString)

	screen_ratio := float64(width) / float64(height)

	canvasHeight = "850"
	canvasWidth = fmt.Sprintf("%f", 850*screen_ratio)

	return
}

func getAvailableDevicesInfoAllProviders() {
	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		fmt.Println("error 1")
		return
	}

	// Forever loop and get data from all providers every 2 seconds
	for {
		// Create an intermediate value to hold the currently built device config before updating the cached config
		intermediateConfig := []ContainerDeviceConfig{}

		// Loop through the registered providers
		for _, v := range configData.DeviceProviders {
			var providerDevicesInfo AvailableDevicesInfo

			// Get the available devices from the current provider
			response, err := http.Get("http://" + v + "/available-devices")
			if err != nil {
				// If the current provider is not available start next loop iteration
				continue
			}

			// Read the response into a byte slice
			responseData, err := ioutil.ReadAll(response.Body)
			if err != nil {
				fmt.Println(err.Error())
			}

			// Read the response byte slice into the providerDevicesInfo struct
			UnmarshalJSONString(string(responseData), &providerDevicesInfo)

			// Append the current devices info to the intermediate config
			for _, v := range providerDevicesInfo.DevicesInfo {
				intermediateConfig = append(intermediateConfig, v)
			}
		}

		// After all providers are polled update the cachedDevicesConfig
		cachedDevicesConfig = intermediateConfig

		time.Sleep(2 * time.Second)
	}
}

func iOSGenerateJSONForTreeFromXML() {

}
