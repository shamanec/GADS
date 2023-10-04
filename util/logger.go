package util

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

type LogsWSClient struct {
	Conn           *websocket.Conn
	CollectionName string
}

func (client *LogsWSClient) sendLiveLogs() {
	// Access the database and collection
	collection := MongoClient().Database("logs").Collection(strings.ToLower(client.CollectionName))
	lastPollTimestamp := time.Now().UnixMilli()

	for {
		// Query for documents created or modified after the last poll
		filter := bson.D{{Key: "timestamp", Value: bson.D{{Key: "$gt", Value: lastPollTimestamp}}}}

		// Sort the documents based on the timestamp field
		findOptions := options.Find()
		findOptions.SetSort(bson.D{{Key: "timestamp", Value: 1}})
		// findOptions.SetLimit(10)

		cursor, err := collection.Find(context.Background(), filter, findOptions)
		if err != nil {
			err = client.Conn.WriteMessage(1, []byte(fmt.Sprintf("Failed to get db cursor for logs from collection `%s` - %s", client.CollectionName, err)))
			if err != nil {
				client.Conn.Close()
				break
			}
			continue
		}

		var documents []map[string]interface{}
		if err := cursor.All(context.Background(), &documents); err != nil {
			err = client.Conn.WriteMessage(1, []byte(fmt.Sprintf("Failed to get read documents from cursor for logs from collection `%s` - %s", client.CollectionName, err)))
			if err != nil {
				client.Conn.Close()
				break
			}
		}

		// Update the last poll timestamp
		if len(documents) > 0 {
			lastDocument := documents[len(documents)-1]
			lastPollTimestamp = lastDocument["timestamp"].(int64)

			// Close the cursor
			cursor.Close(context.Background())

			// Sleep for a while before the next poll
			jsonData, err := json.Marshal(documents)
			if err != nil {
				err = client.Conn.WriteMessage(1, []byte(fmt.Sprintf("Failed to get marshal documents from cursor for logs from collection `%s` - %s", client.CollectionName, err)))
				if err != nil {
					client.Conn.Close()
					break
				}
			}
			err = client.Conn.WriteMessage(1, jsonData)
			if err != nil {
				client.Conn.Close()
				break
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func (client *LogsWSClient) sendLogsInitial(limit int) {
	var logs []map[string]interface{}

	collection := MongoClient().Database("logs").Collection(client.CollectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	findOptions.SetLimit(int64(limit))

	cursor, err := collection.Find(MongoCtx(), bson.D{{}}, findOptions)
	if err != nil {
		log.Fatal(err)
	}

	if err := cursor.All(MongoCtx(), &logs); err != nil {
		log.Fatal(err)
	}
	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}

	cursor.Close(MongoCtx())

	jsonData, err := json.Marshal(logs)
	if err != nil {
		err = client.Conn.WriteMessage(1, []byte(fmt.Sprintf("Failed to marshal documents from cursor for logs from collection `%s` - %s", client.CollectionName, err)))
		if err != nil {
			client.Conn.Close()
			return
		}
	}

	err = client.Conn.WriteMessage(1, jsonData)
	if err != nil {
		client.Conn.Close()
	}
}

func LogsWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		return
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

	wsClient := &LogsWSClient{
		Conn:           conn,
		CollectionName: collectionName,
	}

	wsClient.sendLogsInitial(logLimit)
	go wsClient.sendLiveLogs()
	go wsClient.keepAlive()
}

// Periodically ping the websocket client to keep the connection alive if no messages are sent
func (client *LogsWSClient) keepAlive() {
	for {
		// Send a ping message every 10 seconds
		time.Sleep(10 * time.Second)
		err := client.Conn.WriteMessage(websocket.PingMessage, nil)
		if err != nil {
			fmt.Println("Closing connection")
			client.Conn.Close()
			break
		}
	}
}
