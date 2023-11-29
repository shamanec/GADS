package device

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

var clients = make(map[net.Conn]bool)
var broadcast = make(chan []byte)
var mu sync.Mutex

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
	mu.Lock()
	clients[conn] = true
	mu.Unlock()
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
				mu.Lock()
				delete(clients, client)
				mu.Unlock()
			}
		}
	}
}

func GetDevices() {
	go keepAlive()

	for {
		jsonData, _ := json.Marshal(&latestDevices)

		for client := range clients {
			err := wsutil.WriteServerText(client, jsonData)
			if err != nil {
				client.Close()
				mu.Lock()
				delete(clients, client)
				mu.Unlock()
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func LoadDevices(c *gin.Context) {

	// Parse the template and return response with the created template
	var tmpl = template.Must(template.New("device_selection_new.html").ParseFiles("static/device_selection_new.html"))
	err := tmpl.ExecuteTemplate(c.Writer, "device_selection_new.html", nil)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}
