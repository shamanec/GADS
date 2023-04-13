package device

import (
	"GADS/db"
	"GADS/util"
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

var LatestDevices []Device

// Get all the devices registered in the DB
func GetLatestDBDevices() {
	for {
		// Get a cursor of the whole "devices" table
		cursor, err := r.Table("devices").Run(db.DBSession)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "get_devices_db",
			}).Error("Could not get devices from DB, err: " + err.Error())
			LatestDevices = []Device{}
		}
		defer cursor.Close()

		// Retrieve all documents from the DB into the Device slice
		err = cursor.All(&LatestDevices)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "get_devices_db",
			}).Error("Could not get devices from DB, err: " + err.Error())
			LatestDevices = []Device{}
		}
		time.Sleep(1 * time.Second)
	}
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []byte)

func AvailableDevicesWS(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// upgrade this connection to a WebSocket
	// connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "devices_ws",
		}).Error("Could not upgrade ws connection: " + err.Error())
		return
	}

	// Generate devices list on websocket open
	// and send it over the websocket connection
	// If it fails just return and don't add the connection to the clients map
	html_message := generateDeviceSelectionHTML()
	err = conn.WriteMessage(1, html_message)
	if err != nil {
		conn.Close()
		return
	}

	// Add the new conn to clients map
	clients[conn] = true
}

func keepAlive() {
	for {
		// Send a ping message every 10 seconds
		time.Sleep(10 * time.Second)

		// Loop through the clients and send the message to each of them
		for client := range clients {
			err := client.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func GetDevices() {
	// Start a goroutine that will ping each websocket client
	// To keep the connection alive
	go keepAlive()

	// Define a slice of bytes to contain the last sent html over the websocket
	var lastHtmlMessage []byte

	// Start an endless loop polling the DB each second and sending an updated device selection html
	// To each websocket client
	for {
		// Get the devices from the DB
		var htmlMessage []byte

		// Make functions available in html template
		funcMap := template.FuncMap{
			"contains":    strings.Contains,
			"healthCheck": isHealthy,
		}

		// Generate the html for the device selection with the latest data
		var tmpl = template.Must(template.New("device_selection_table").Funcs(funcMap).ParseFiles("static/device_selection_table.html"))
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "device_selection_table", LatestDevices)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "send_devices_over_ws",
			}).Error("Could not execute template when sending devices over ws: " + err.Error())
			time.Sleep(2 * time.Second)
			return
		}

		// If no devices are in the DB show a generic message html
		// Or convert the generate html buffer to a slice of bytes
		if LatestDevices == nil {
			htmlMessage = []byte(`<h1 style="align-items: center;">No devices registered from providers in the DB</h1>`)
		} else {
			htmlMessage = []byte(buf.String())
		}

		// Check the generated html message to the last sent html message
		// If they are not the same - send the message to each websocket client
		// If they are the same just continue the loop
		// This is to avoid spamming identical html messages over the websocket each second
		// If there is no actual change
		if !bytes.Equal(htmlMessage, lastHtmlMessage) {
			for client := range clients {
				err := client.WriteMessage(1, htmlMessage)
				if err != nil {
					client.Close()
					delete(clients, client)
				}
			}
			lastHtmlMessage = htmlMessage
		}

		time.Sleep(1 * time.Second)
	}
}

func generateDeviceSelectionHTML() []byte {
	var html_message []byte

	// Make functions available in html template
	funcMap := template.FuncMap{
		"contains":    strings.Contains,
		"healthCheck": isHealthy,
	}

	var tmpl = template.Must(template.New("device_selection_table").Funcs(funcMap).ParseFiles("static/device_selection_table.html"))

	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, "device_selection_table", LatestDevices)

	if err != nil {
		log.WithFields(log.Fields{
			"event": "send_devices_over_ws",
		}).Error("Could not execute template when sending devices over ws: " + err.Error())
	}

	if LatestDevices == nil {
		html_message = []byte(`<h1 style="align-items: center;">No devices registered from providers in the DB</h1>`)
	} else {
		html_message = []byte(buf.String())
	}

	return html_message
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

// Available devices html page
func LoadDevices(w http.ResponseWriter, r *http.Request) {
	// Make functions available in the html template
	funcMap := template.FuncMap{
		"contains":    strings.Contains,
		"healthCheck": isHealthy,
	}

	// Parse the template and return response with the created template
	var tmpl = template.Must(template.New("device_selection.html").Funcs(funcMap).ParseFiles("static/device_selection.html", "static/device_selection_table.html"))
	if err := tmpl.ExecuteTemplate(w, "device_selection.html", util.ConfigData.GadsHostAddress); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
