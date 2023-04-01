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
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

type Device struct {
	Container             *DeviceContainer `json:"container,omitempty"`
	State                 string           `json:"state"`
	Connected             bool             `json:"connected,omitempty"`
	Healthy               bool             `json:"healthy,omitempty"`
	LastHealthyTimestamp  int64            `json:"last_healthy_timestamp,omitempty"`
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

// Get all the devices registered in the DB
func GetDBDevices() []Device {
	var devicesDB []Device

	// Get a cursor of the whole "devices" table
	cursor, err := r.Table("devices").Run(session)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_devices_db",
		}).Error("Could not get devices from DB, err: " + err.Error())
		return []Device{}
	}
	defer cursor.Close()

	// Retrieve all documents from the DB into the Device slice
	err = cursor.All(&devicesDB)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_devices_db",
		}).Error("Could not get devices from DB, err: " + err.Error())
		return []Device{}
	}

	return devicesDB
}

// Get specific device info from DB
func GetDBDevice(udid string) Device {
	// Get a cursor of the specific device document from the "devices" table
	cursor, err := r.Table("devices").Get(udid).Run(session)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_devices_db",
		}).Error("Could not get device from DB, err: " + err.Error())
		return Device{}
	}

	// Retrieve a single document from the cursor
	var device Device
	err = cursor.One(&device)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_devices_db",
		}).Error("Could not get device from DB, err: " + err.Error())
		return Device{}
	}

	return device
}

func AvailableDevicesWSLocal(conn *websocket.Conn) {
	devices := GetDBDevices()
	writeDevicesWS(conn, devices)

	for {
		devices = GetDBDevices()
		writeDevicesWS(conn, devices)

		time.Sleep(1 * time.Second)
	}
}

// This is an additional check on top of the "Healthy" field in the DB.
// The reason is that the device might have old data in the DB where it is still "Connected" and "Healthy".
// So we also check the timestamp of the last time the device was "Healthy".
// It is used inside the html template.
func isHealthy(timestamp int64) bool {
	currentTime := time.Now().UnixMilli()
	diff := currentTime - timestamp
	if diff > 2000 {
		return false
	}

	return true
}

// Write the html device selection table to the websocket
// As a live feed update of the list with the latest information
func writeDevicesWS(conn *websocket.Conn, devices []Device) {
	var html_message []byte

	// Make functions available in html template
	funcMap := template.FuncMap{
		"contains":    strings.Contains,
		"healthCheck": isHealthy,
	}

	var tmpl = template.Must(template.New("device_selection_table").Funcs(funcMap).ParseFiles("static/device_selection_table.html"))

	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, "device_selection_table", devices)

	if err != nil {
		log.WithFields(log.Fields{
			"event": "send_devices_over_ws",
		}).Error("Could not execute template when sending devices over ws: " + err.Error())
		time.Sleep(2 * time.Second)
		return
	}

	if devices == nil {
		html_message = []byte(`<h1 style="align-items: center;">No devices available</h1>`)
	} else {
		html_message = []byte(buf.String())
	}

	if err := conn.WriteMessage(1, html_message); err != nil {
		return
	}
}

// Available devices html page
func LoadAvailableDevices(w http.ResponseWriter, r *http.Request) {
	// Make functions available in the html template
	funcMap := template.FuncMap{
		"contains":    strings.Contains,
		"healthCheck": isHealthy,
	}

	// Parse the template and return response with the created template
	var tmpl = template.Must(template.New("device_selection.html").Funcs(funcMap).ParseFiles("static/device_selection.html", "static/device_selection_table.html"))
	if err := tmpl.ExecuteTemplate(w, "device_selection.html", ConfigData.GadsHostAddress); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Load a specific device page
func GetDevicePage(w http.ResponseWriter, r *http.Request) {
	var err error

	vars := mux.Vars(r)
	udid := vars["device_udid"]

	device := GetDBDevice(udid)

	// If the device does not exist in the cached devices
	if device == (Device{}) {
		fmt.Println("error")
		return
	}

	var webDriverAgentSessionID = ""
	if device.OS == "ios" {
		webDriverAgentSessionID, err = CheckWDASession(device.Host + ":" + device.WDAPort)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	var appiumSessionID = ""
	if device.OS == "android" {
		appiumSessionID, err = checkAppiumSession(device.Host + ":" + device.AppiumPort)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	// Calculate the width and height for the canvas
	canvasWidth, canvasHeight := calculateCanvasDimensions(device.ScreenSize)

	pageData := struct {
		Device                  Device
		CanvasWidth             string
		CanvasHeight            string
		WebDriverAgentSessionID string
		AppiumSessionID         string
	}{
		Device:                  device,
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
