package device

import (
	"GADS/util"
	"bytes"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}
var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []byte)

func AvailableDeviceWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "devices_ws",
		}).Error("Could not upgrade ws connection: " + err.Error())
		return
	}

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
		err := tmpl.ExecuteTemplate(&buf, "device_selection_table", latestDevices)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "send_devices_over_ws",
			}).Error("Could not execute template when sending devices over ws: " + err.Error())
			time.Sleep(2 * time.Second)
			return
		}

		// If no devices are in the DB show a generic message html
		// Or convert the generate html buffer to a slice of bytes
		if latestDevices == nil {
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
	err := tmpl.ExecuteTemplate(&buf, "device_selection_table", latestDevices)

	if err != nil {
		log.WithFields(log.Fields{
			"event": "send_devices_over_ws",
		}).Error("Could not execute template when sending devices over ws: " + err.Error())
	}

	if latestDevices == nil {
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

func LoadDevices(c *gin.Context) {
	funcMap := template.FuncMap{
		"contains":    strings.Contains,
		"healthCheck": isHealthy,
	}

	// Parse the template and return response with the created template
	var tmpl = template.Must(template.New("device_selection.html").Funcs(funcMap).ParseFiles("static/device_selection.html", "static/device_selection_table.html"))
	err := tmpl.ExecuteTemplate(c.Writer, "device_selection.html", util.ConfigData)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}
