package device

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

var clients = make(map[net.Conn]bool)
var broadcast = make(chan []byte)

func AvailableDeviceWS(c *gin.Context) {
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		fmt.Println(err)
	}
	jsonData, _ := json.Marshal(&latestDevices)

	err = wsutil.WriteServerText(conn, jsonData)
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
			err := wsutil.WriteClientMessage(client, ws.OpPing, nil)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func GetDevices() {
	go keepAlive()

	for {
		jsonData, _ := json.Marshal(&latestDevices)

		for client := range clients {
			err := wsutil.WriteClientBinary(client, jsonData)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}

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
	if diff > 5000 {
		return false
	}

	return true
}

func LoadDevices(c *gin.Context) {

	// Parse the template and return response with the created template
	var tmpl = template.Must(template.New("device_selection_new.html").ParseFiles("static/device_selection_new.html"))
	err := tmpl.ExecuteTemplate(c.Writer, "device_selection_new.html", nil)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}
