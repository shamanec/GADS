package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type Device struct {
	Container             *DeviceContainer `json:"container,omitempty"`
	State                 string           `json:"state"`
	Connected             bool             `json:"connected,omitempty"`
	Healthy               bool             `json:"healthy,omitempty"`
	LastUpdateTimestamp   int64            `json:"last_update_timestamp,omitempty"`
	UDID                  string           `json:"udid"`
	OS                    string           `json:"os"`
	AppiumPort            string           `json:"appium_port"`
	StreamPort            string           `json:"stream_port"`
	ContainerServerPort   string           `json:"container_server_port"`
	WDAPort               string           `json:"wda_port,omitempty"`
	Name                  string           `json:"name"`
	OSVersion             string           `json:"os_version"`
	ScreenSize            string           `json:"screen_size"`
	Model                 string           `json:"model"`
	Image                 string           `json:"image,omitempty"`
	Host                  string           `json:"host"`
	MinicapFPS            string           `json:"minicap_fps,omitempty"`
	MinicapHalfResolution string           `json:"minicap_half_resolution,omitempty"`
	UseMinicap            string           `json:"use_minicap,omitempty"`
}

type DeviceContainer struct {
	ContainerID     string `json:"id"`
	ContainerStatus string `json:"status"`
	ImageName       string `json:"image_name"`
	ContainerName   string `json:"container_name"`
}

func AvailableDevicesWSLocal(conn *websocket.Conn) {
	for {
		var html_message []byte

		// Make functions available in html template
		funcMap := template.FuncMap{
			"contains": strings.Contains,
		}

		var tmpl = template.Must(template.New("device_selection_table").Funcs(funcMap).ParseFiles("static/device_selection_table.html"))

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "device_selection_table", currentDevicesInfo)

		if err != nil {
			log.WithFields(log.Fields{
				"event": "send_devices_over_ws",
			}).Error("Could not execute template when sending devices over ws: " + err.Error())
			time.Sleep(2 * time.Second)
			return
		}

		if currentDevicesInfo == nil {
			html_message = []byte(`<h1 style="align-items: center;">No devices available</h1>`)
		} else {
			html_message = []byte(buf.String())
		}

		if err := conn.WriteMessage(1, html_message); err != nil {
			time.Sleep(2 * time.Second)
			return
		}

		time.Sleep(2 * time.Second)
	}

}

// Available devices html page
func LoadAvailableDevices(w http.ResponseWriter, r *http.Request) {
	// Make functions available in the html template
	funcMap := template.FuncMap{
		"contains": strings.Contains,
	}

	// Parse the template and return response with the created template
	var tmpl = template.Must(template.New("device_selection.html").Funcs(funcMap).ParseFiles("static/device_selection.html", "static/device_selection_table.html"))
	if err := tmpl.ExecuteTemplate(w, "device_selection.html", ConfigData.GadsHostAddress); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func GetDevicePage(w http.ResponseWriter, r *http.Request) {
	var err error

	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	var selected_device Device

	// Loop through the cached devices and search for the selected device
	for _, v := range currentDevicesInfo {
		if v.UDID == device_udid {
			selected_device = v
		}
	}

	// If the device does not exist in the cached devices
	if selected_device == (Device{}) {
		fmt.Println("error")
		return
	}

	var webDriverAgentSessionID = ""
	if selected_device.OS == "ios" {
		webDriverAgentSessionID, err = CheckWDASession(selected_device.Host + ":" + selected_device.WDAPort)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	var appiumSessionID = ""
	if selected_device.OS == "android" {
		appiumSessionID, err = checkAppiumSession(selected_device.Host + ":" + selected_device.AppiumPort)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	// Calculate the width and height for the canvas
	canvasWidth, canvasHeight := calculateCanvasDimensions(selected_device.ScreenSize)

	pageData := struct {
		Device                  Device
		CanvasWidth             string
		CanvasHeight            string
		WebDriverAgentSessionID string
		AppiumSessionID         string
	}{
		Device:                  selected_device,
		CanvasWidth:             canvasWidth,
		CanvasHeight:            canvasHeight,
		WebDriverAgentSessionID: webDriverAgentSessionID,
		AppiumSessionID:         appiumSessionID,
	}

	// Parse the template and return response with the container table rows
	// This will generate only the device table, not the whole page
	var tmpl = template.Must(template.ParseFiles("static/device_control_new.html"))

	// Reply with the new table
	if err = tmpl.Execute(w, pageData); err != nil {
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

// func getAvailableDevicesInfoAllProviders() {
// 	// Forever loop and get data from all providers every 2 seconds
// 	for {
// 		// Create an intermediate value to hold the currently built device config before updating the cached config
// 		intermediateConfig := []Device{}

// 		// Loop through the registered providers
// 		for _, v := range ConfigData.DeviceProviders {
// 			var providerDevices []Device

// 			// Get the available devices from the current provider
// 			response, err := http.Get("http://" + v + "/device/list")
// 			if err != nil {
// 				// If the current provider is not available start next loop iteration
// 				continue
// 			}

// 			// Read the response into a byte slice
// 			responseData, err := ioutil.ReadAll(response.Body)
// 			if err != nil {
// 				fmt.Println(err.Error())
// 			}

// 			// Read the response byte slice into the providerDevicesInfo struct
// 			err = UnmarshalJSONString(string(responseData), &providerDevices)
// 			if err != nil {
// 				fmt.Println(err.Error())
// 			}

// 			// Append the current devices info to the intermediate config
// 			for _, v := range providerDevices {
// 				intermediateConfig = append(intermediateConfig, v)
// 			}
// 		}

// 		// After all providers are polled update the cachedDevicesConfig
// 		cachedDevicesConfig = intermediateConfig

// 		time.Sleep(2 * time.Second)
// 	}
// }
