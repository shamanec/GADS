package router

import (
	"GADS/provider/devices"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type WebRTCMessage struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

func WebRTCSocket2(c *gin.Context) {
	udid := c.Param("udid")

	fmt.Println("DEVICE " + udid)
	device := devices.DBDeviceMap[udid]
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	u := url.URL{Scheme: "ws", Host: "localhost:" + device.StreamPort, Path: ""}
	deviceConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		log.Printf("Failed to dial device websocket - " + err.Error())
		return
	}
	defer deviceConn.Close()

	go func() {
		for {
			msg, op, err := wsutil.ReadServerData(deviceConn)
			if err != nil {
				log.Printf("Device disconnected: %s", err)
				return
			}

			if op == ws.OpText {
				fmt.Println("KOLEO2")
				fmt.Println(string(msg))
				err = wsutil.WriteServerText(conn, msg)
				if err != nil {
					log.Printf("Failed to write to client socket")
					return
				}
			}
		}
	}()

	for {
		msg, op, err := wsutil.ReadClientData(conn)
		if err != nil {
			log.Printf("Client disconnected: %s", err)
			return
		}

		if op == ws.OpText {
			fmt.Println("KOLEO")
			fmt.Println("MESSAGE")
			fmt.Println(string(msg))

			var message WebRTCMessage
			err := json.Unmarshal(msg, &message)
			fmt.Printf("MESSAGE TYPE IS %s\n", message.Type)
			if err != nil {
				fmt.Println("OMG")
			} else {
				switch message.Type {
				case "offer":
					fmt.Println("GOT OFFER!!")
					err = wsutil.WriteClientText(deviceConn, msg)
					if err != nil {
						log.Printf("Failed to write to android socket")
					}
					break
				case "candidate":
					fmt.Println("GOT CANDIDATE!!")
					err = wsutil.WriteClientText(deviceConn, msg)
					if err != nil {
						log.Printf("Failed to write to android socket")
					}
					break
				}
			}
		}
	}
}
