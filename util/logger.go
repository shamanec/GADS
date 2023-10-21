package util

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
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
	mu sync.Mutex
)

type LogsWSClient struct {
	Conn           *websocket.Conn
	CollectionName string
}

func (client *LogsWSClient) sendLiveLogs() {
	ctx, cancel := context.WithCancel(mongoClientCtx)
	defer cancel()

	collection := mongoClient.Database("logs").Collection(client.CollectionName)

	lastPolledDocTS := time.Now().UnixMilli()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		// Query for documents created or modified after the last poll
		filter := bson.D{{Key: "timestamp", Value: bson.D{{Key: "$gt", Value: lastPolledDocTS}}}}

		// Sort the documents based on the timestamp field
		findOptions := options.Find()
		findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})

		cursor, err := collection.Find(ctx, filter, findOptions)
		if err != nil {
			mu.Lock()
			defer mu.Unlock()
			err = client.Conn.WriteMessage(1, []byte(fmt.Sprintf("Failed to get db cursor for logs from collection `%s` - %s", client.CollectionName, err)))
			if err != nil {
				client.Conn.Close()
				return
			}
			continue
		}

		var documents []map[string]interface{}
		if err := cursor.All(ctx, &documents); err != nil {
			mu.Lock()
			defer mu.Unlock()
			err = client.Conn.WriteMessage(1, []byte(fmt.Sprintf("Failed to get read documents from cursor for logs from collection `%s` - %s", client.CollectionName, err)))
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
				err = client.Conn.WriteMessage(1, []byte(fmt.Sprintf("Failed to get marshal documents from cursor for logs from collection `%s` - %s", client.CollectionName, err)))
				if err != nil {
					client.Conn.Close()
					return
				}
			}
			mu.Lock()
			defer mu.Unlock()
			err = client.Conn.WriteMessage(1, jsonData)
			if err != nil {
				client.Conn.Close()
				return
			}
		}
	}
}

func (client *LogsWSClient) sendLogsInitial(limit int) {
	ctx, cancel := context.WithCancel(mongoClientCtx)
	defer cancel()

	var logs []map[string]interface{}

	collection := mongoClient.Database("logs").Collection(client.CollectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	findOptions.SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, bson.D{{}}, findOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &logs); err != nil {
		log.Fatal(err)
	}
	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}

	jsonData, err := json.Marshal(logs)
	if err != nil {
		err = client.Conn.WriteMessage(1, []byte(fmt.Sprintf("Failed to marshal documents from cursor for logs from collection `%s` - %s", client.CollectionName, err)))
		if err != nil {
			client.Conn.Close()
			return
		}
	}

	mu.Lock()
	defer mu.Unlock()
	err = client.Conn.WriteMessage(1, jsonData)
	if err != nil {
		client.Conn.Close()
	}
}

func LogsWS(c *gin.Context) {
	fmt.Println("ESTABLISHING WS")
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
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		err := client.Conn.WriteMessage(websocket.PingMessage, nil)
		if err != nil {
			fmt.Printf("CLOSING WS - %v\n", time.Now().UnixMilli())
			client.Conn.Close()
			break
		}
	}
}
