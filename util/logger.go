package util

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LogsWSClient struct {
	Conn           net.Conn
	CollectionName string
	Ctx            context.Context
	Cancel         context.CancelFunc
}

func (client *LogsWSClient) sendLiveLogs() {

	collection := mongoClient.Database("logs").Collection(client.CollectionName)

	lastPolledDocTS := time.Now().UnixMilli()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	defer fmt.Println("STOP SENDING LIVE LOGS")

	for {
		<-ticker.C
		// Query for documents created or modified after the last poll
		filter := bson.D{{Key: "timestamp", Value: bson.D{{Key: "$gt", Value: lastPolledDocTS}}}}

		// Sort the documents based on the timestamp field
		findOptions := options.Find()
		findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})

		cursor, err := collection.Find(client.Ctx, filter, findOptions)
		if err != nil {
			err = wsutil.WriteServerText(client.Conn, []byte(fmt.Sprintf("Failed to get db cursor for logs from collection `%s` - %s", client.CollectionName, err)))
			if err != nil {
				client.Conn.Close()
				return
			}
			continue
		}
		defer cursor.Close(client.Ctx)

		var documents []map[string]interface{}
		if err := cursor.All(client.Ctx, &documents); err != nil {
			err = wsutil.WriteServerText(client.Conn, []byte(fmt.Sprintf("Failed to get read documents from cursor for logs from collection `%s` - %s", client.CollectionName, err)))
			if err != nil {
				client.Conn.Close()
				return
			}
		}

		// Update the last poll timestamp
		if len(documents) > 0 {
			// The documents come in descending order so the first one is essentially the latest one logged in the DB
			lastDocument := documents[0]
			lastPolledDocTS = lastDocument["timestamp"].(int64)

			// Sleep for a while before the next poll
			jsonData, err := json.Marshal(documents)
			if err != nil {
				fmt.Println("MARSHAL ERROR " + err.Error())
				err = wsutil.WriteServerText(client.Conn, []byte(fmt.Sprintf("Failed to get marshal documents from cursor for logs from collection `%s` - %s", client.CollectionName, err)))
				if err != nil {
					client.Conn.Close()
					return
				}
			}

			err = wsutil.WriteServerText(client.Conn, jsonData)
			if err != nil {
				client.Conn.Close()
				return
			}
		}
	}

}

func (client *LogsWSClient) sendLogsInitial(limit int) {

	var logs []map[string]interface{}

	collection := mongoClient.Database("logs").Collection(client.CollectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	findOptions.SetLimit(int64(limit))

	cursor, err := collection.Find(client.Ctx, bson.D{{}}, findOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(client.Ctx)

	if err := cursor.All(client.Ctx, &logs); err != nil {
		log.Fatal(err)
	}
	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}

	jsonData, err := json.Marshal(logs)
	if err != nil {
		err = wsutil.WriteServerText(client.Conn, []byte(fmt.Sprintf("Failed to marshal documents from cursor for logs from collection `%s` - %s", client.CollectionName, err)))
		if err != nil {
			client.Conn.Close()
			return
		}
	}

	err = wsutil.WriteServerText(client.Conn, jsonData)
	if err != nil {
		client.Conn.Close()
	}
}

func LogsWS(c *gin.Context) {
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		fmt.Println(err)
	}

	logLimit, err := strconv.Atoi(c.DefaultQuery("logLimit", "100"))
	if err != nil {
		fmt.Println("Could not convert provided limit to int")
	}

	collectionName := c.DefaultQuery("collection", "")
	if collectionName == "" {
		fmt.Println("Empty collection name requested")
		return
	}

	ctx, cancel := context.WithCancel(mongoClientCtx)

	wsClient := &LogsWSClient{
		Conn:           conn,
		CollectionName: collectionName,
		Ctx:            ctx,
		Cancel:         cancel,
	}

	wsClient.sendLogsInitial(logLimit)
	go wsClient.sendLiveLogs()
	go wsClient.closeHandler()
	go wsClient.keepAlive()
}

func (client *LogsWSClient) Close() {
	client.Conn.Close()
	client.Cancel()
}

func (client *LogsWSClient) closeHandler() {
	defer client.Close()

	for {
		_, r, err := wsutil.ReadClientData(client.Conn)
		if err != nil {
			fmt.Println("Client closed")
			break
		}

		if r == ws.OpClose {
			fmt.Println("CLIENT CLOSED")
			break
		}
	}
}

func (client *LogsWSClient) keepAlive() {
	defer client.Close()

	for {
		err := wsutil.WriteServerMessage(client.Conn, ws.OpPing, []byte{})
		if err != nil {
			return
		}
		time.Sleep(1 * time.Second)
	}
}
