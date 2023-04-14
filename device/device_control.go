package device

import (
	"GADS/db"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

type Device struct {
	Container             *DeviceContainer `json:"container,omitempty"`
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

// Get specific device info from DB
func getDBDevice(udid string) Device {
	// Get a cursor of the specific device document from the "devices" table
	cursor, err := r.Table("devices").Get(udid).Run(db.DBSession)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_devices_db",
		}).Error("Could not get device from DB, err: " + err.Error())
		return Device{}
	}
	defer cursor.Close()

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

// Load a specific device page
func GetDevicePage(w http.ResponseWriter, r *http.Request) {
	var err error

	vars := mux.Vars(r)
	udid := vars["device_udid"]

	device := getDBDevice(udid)

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
