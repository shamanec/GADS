/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// This endpoint is for easier testing and debugging of WebRTC instead of building on a device each time
func WebRTCSocket(c *gin.Context) {
	udid := c.Param("udid")

	fmt.Println("DEVICE " + udid)
	// device := devices.DBDeviceMap[udid]
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	// u := url.URL{Scheme: "ws", Host: "localhost:" + device.StreamPort, Path: ""}
	u := url.URL{Scheme: "ws", Host: "localhost:1991", Path: ""}
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
			err = wsutil.WriteServerText(deviceConn, []byte("hangup"))
			if err != nil {
				log.Printf("Failed to send hangup to the android socket")
			}
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
					err = wsutil.WriteServerText(deviceConn, msg)
					if err != nil {
						log.Printf("Failed to write to android socket")
					}
					break
				case "candidate":
					fmt.Println("GOT CANDIDATE!!")
					err = wsutil.WriteServerText(deviceConn, msg)
					if err != nil {
						log.Printf("Failed to write to android socket")
					}
					break
				}
			}
		}
	}
}
