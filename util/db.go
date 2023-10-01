package util

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client
var mongoClientCtx context.Context

// Create a new MongoDB Client to reuse for writing/reading from MongoDB
func NewMongoClient() {
	var err error
	connectionString := "mongodb://" + ConfigData.MongoDB

	// Set up a context for the connection.
	mongoClientCtx = context.Background()

	// Create a MongoDB client with options.
	clientOptions := options.Client().ApplyURI(connectionString)
	mongoClient, err = mongo.Connect(mongoClientCtx, clientOptions)
	if err != nil {
		panic(fmt.Sprintf("Could not perform connect to Mongo server at `%s` - %s", connectionString, err))
	}

	// Ping the client to see if the connection is alive
	err = mongoClient.Ping(mongoClientCtx, nil)
	if err != nil {
		panic(fmt.Sprintf("No initial connection to MongoDB server at `%s` was established - %s", connectionString, err))
	}

	go checkDBConnection()
	go keepAlive()
}

// Access the
func MongoClient() *mongo.Client {
	return mongoClient
}

func MongoCtx() context.Context {
	return mongoClientCtx
}

// Check the MongoDB connection each second and attempt to create a new client if connection is lost
func checkDBConnection() {
	fmt.Println("Starting to periodically check MongoDB connection, will attempt to re-establish if it is lost!")
	for {
		err := mongoClient.Ping(mongoClientCtx, nil)
		if err != nil {
			fmt.Printf("Lost connection to MongoDB server, attempting to create a new client - %s", err)
			NewMongoClient()
			break
		}
		time.Sleep(1 * time.Second)
	}
}
