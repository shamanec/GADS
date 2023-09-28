package util

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
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
	connectedClients = make(map[*websocket.Conn]bool)
	connMutex        sync.Mutex
)

func ProviderLogsWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	connectedClients[conn] = true

	logLimit, err := strconv.Atoi(c.DefaultQuery("logLimit", "100"))
	if err != nil {
		fmt.Println("Could not convert provided limit to int")
	}

	logProvider := c.DefaultQuery("logProvider", "")
	if logProvider == "" {
		fmt.Println("Empty provider")
		return
	}

	initialLogs := getLogsInitial(logProvider, logLimit)
	jsonData, err := json.Marshal(initialLogs)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	sendLogsToClients(jsonData)

	go sendLiveLogsToClients(logProvider)
}

func getLogsInitial(collectionName string, limit int) []map[string]interface{} {
	var logs []map[string]interface{}

	collection := MongoClient().Database("logs").Collection(strings.ToLower(collectionName))
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

	return logs
}

func sendLogsToClients(data []byte) {
	for client := range connectedClients {
		connMutex.Lock()
		err := client.WriteMessage(1, data)
		connMutex.Unlock()
		if err != nil {
			client.Close()
			delete(connectedClients, client)
		}
	}
}

func sendLiveLogsToClients(collectionName string) {
	// Access the database and collection
	collection := MongoClient().Database("logs").Collection(strings.ToLower(collectionName))
	lastPollTimestamp := time.Now().UnixMilli()

	for {
		// Query for documents created or modified after the last poll
		// filter := bson.M{"timestamp": bson.M{"$gt": lastPollTimestamp}}
		filter := bson.D{{Key: "timestamp", Value: bson.D{{Key: "$gt", Value: lastPollTimestamp}}}}

		findOptions := options.Find()
		findOptions.SetSort(bson.D{{Key: "timestamp", Value: 1}})
		findOptions.SetLimit(10)

		cursor, err := collection.Find(context.Background(), filter, findOptions)
		if err != nil {
			log.Println(err)
			continue
		}

		// Process the retrieved documents
		var documents []map[string]interface{}
		if err := cursor.All(context.Background(), &documents); err != nil {
			log.Println(err)
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
				fmt.Println("Error:", err)
				return
			}
			sendLogsToClients(jsonData)
		}

		time.Sleep(2 * time.Second)
	}
}

// Periodically ping all connected websocket clients to keep the connection alive if no messages are sent
func keepAlive() {
	for {
		// Send a ping message every 10 seconds
		time.Sleep(10 * time.Second)

		// Loop through the clients and send the message to each of them
		for client := range connectedClients {
			err := client.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				client.Close()
				delete(connectedClients, client)
			}
		}
	}
}
