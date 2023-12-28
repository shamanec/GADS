package device

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

var clients = make(map[net.Conn]bool)
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
			delete(clients, client)
			mu.Unlock()
			return
		}

		// If we get Op.Code = 8, then we received a close frame and we can remove the client
		if msg[0].OpCode == 8 {
			client.Close()
			mu.Lock()
			delete(clients, client)
			mu.Unlock()
			return
		}
	}
}

func GetDevices() {
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
