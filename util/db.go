package util

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	log "github.com/sirupsen/logrus"
)

var mongoClient *mongo.Client
var mongoClientCtx context.Context

// Create a new MongoDB Client to reuse for writing/reading from MongoDB
func InitMongo() {
	var err error
	connectionString := "mongodb://" + ConfigData.MongoDB

	// Set up a context for the connection.
	mongoClientCtx = context.Background()

	// Create a MongoDB client with options.
	clientOptions := options.Client().ApplyURI(connectionString)
	mongoClient, err = mongo.Connect(mongoClientCtx, clientOptions)
	if err != nil {
		panic(fmt.Sprintf("Could not new client for Mongo server at `%s` - %s", connectionString, err))
	}

	// Ping the client to see if the connection is alive
	err = mongoClient.Ping(mongoClientCtx, nil)
	if err != nil {
		panic(fmt.Sprintf("No initial connection to MongoDB server at `%s` was established - %s", connectionString, err))
	}

	go checkDBConnection()
}

func MongoClient() *mongo.Client {
	return mongoClient
}

func MongoCtx() context.Context {
	return mongoClientCtx
}

// Periodically check the MongoDB connection and attempt to create a new client if connection is lost
func checkDBConnection() {
	log.Info("Starting to periodically check MongoDB connection, will attempt to re-establish if it is lost!")
	for {
		err := mongoClient.Ping(mongoClientCtx, nil)
		if err != nil {
			log.Error(fmt.Sprintf("Lost connection to MongoDB server, attempting to create a new client - %s", err))
			InitMongo()
			break
		}
		time.Sleep(2 * time.Second)
	}
}

type ProviderData struct {
	Name        string `json:"name" bson:"_id"`
	Devices     int    `json:"devices" bson:"devices_in_config"`
	HostAddress string `json:"host_address" bson:"host_address"`
}

func GetProvidersFromDB() []ProviderData {
	var providers []ProviderData

	collection := MongoClient().Database("gads").Collection("providers")
	cursor, err := collection.Find(context.Background(), bson.D{{}}, options.Find())
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Could not get db cursor when trying to get latest device info from db - %s", err))
	}

	if err := cursor.All(MongoCtx(), &providers); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Could not get devices latest info from db cursor - %s", err))
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
	}

	cursor.Close(MongoCtx())

	return providers
}
