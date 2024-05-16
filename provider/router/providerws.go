package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/devices"
	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

var providerClients = make(map[net.Conn]bool)
var mu sync.Mutex

func GetProviderDataWS(c *gin.Context) {
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		fmt.Println(err)
	}

	var deviceData []*models.Device
	for _, device := range devices.DeviceMap {
		deviceData = append(deviceData, device)
	}

	var providerData models.ProviderData
	providerData.ProviderData = config.Config.EnvConfig
	providerData.DeviceData = deviceData

	jsonData, _ := json.Marshal(&providerData)

	err = wsutil.WriteServerText(conn, jsonData)
	if err != nil {
		conn.Close()
		return
	}

	// Add the new conn to clients map
	mu.Lock()
	providerClients[conn] = true
	mu.Unlock()
	go monitorConnClose(conn)
}

// Wait to receive a message on the connection.
// If it errors out or the message is close frame,
// clean up the connection and remove the client from the map.
// With this we can immediately clean up resources instead of waiting for the browser to gracefully close the connection in around 30s
func monitorConnClose(client net.Conn) {
	for {
		msg, err := wsutil.ReadClientMessage(client, nil)
		// If we got io.EOF error then the ws was probably abnormally closed
		// Remove the client
		if err == io.EOF {
			client.Close()
			mu.Lock()
			delete(providerClients, client)
			mu.Unlock()
			return
		}

		// If we get Op.Code = 8, then we received a close frame and we can remove the client
		if len(msg) != 0 {
			if msg[0].OpCode == 8 {
				client.Close()
				mu.Lock()
				delete(providerClients, client)
				mu.Unlock()
				return
			}
		}
	}
}

func sendProviderLiveData() {
	for {
		var deviceData []*models.Device
		mu.Lock()
		for _, device := range devices.DeviceMap {
			deviceData = append(deviceData, device)
		}
		mu.Unlock()

		var providerData models.ProviderData
		providerData.ProviderData = config.Config.EnvConfig
		providerData.DeviceData = deviceData

		jsonData, _ := json.Marshal(&providerData)
		for client := range providerClients {
			err := wsutil.WriteServerText(client, jsonData)
			if err != nil {
				client.Close()
				mu.Lock()
				delete(providerClients, client)
				mu.Unlock()
			}
		}

		time.Sleep(1 * time.Second)
	}
}
