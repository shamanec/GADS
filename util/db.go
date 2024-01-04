package util

import (
	"context"
	"fmt"
	"time"

	"GADS/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	log "github.com/sirupsen/logrus"
)

var mongoClient *mongo.Client
var mongoClientCtx context.Context
var mongoClientCtxCancel context.CancelFunc

// Create a new MongoDB Client to reuse for writing/reading from MongoDB
func InitMongo() {
	var err error
	connectionString := "mongodb://" + ConfigData.MongoDB
	// Set up a context for the connection.
	mongoClientCtx, mongoClientCtxCancel = context.WithCancel(context.Background())

	// Create a MongoDB client with options
	// serverAPI := options.ServerAPI(options.ServerAPIVersion1)
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

func MongoClientCtx() context.Context {
	return mongoClientCtx
}

func MongoClientCtxCancel() context.CancelFunc {
	return mongoClientCtxCancel
}

// Periodically check the MongoDB connection and attempt to create a new client if connection is lost
func checkDBConnection() {
	log.Info("Starting to periodically check MongoDB connection!")
	errorCounter := 0
	for {
		if errorCounter < 10 {
			time.Sleep(2 * time.Second)
			err := mongoClient.Ping(mongoClientCtx, nil)
			if err != nil {
				log.WithFields(log.Fields{
					"event": "check_db_connection",
				}).Error(fmt.Sprintf("No connection to MongoDB server - %s", err))
				errorCounter++
				continue
			}
		} else {
			log.WithFields(log.Fields{
				"event": "check_db_connection",
			}).Error("Connection to MongoDB server was lost for more than 20 seconds!")
			panic("Connection to MongoDB server was lost for more than 20 seconds!")
		}
	}
}

type ProviderData struct {
	Name        string `json:"name" bson:"_id"`
	Devices     int    `json:"devices" bson:"devices_in_config"`
	HostAddress string `json:"host_address" bson:"host_address"`
}

func GetProvidersFromDB() []ProviderData {
	var providers []ProviderData
	ctx, cancel := context.WithTimeout(mongoClientCtx, 10*time.Second)
	defer cancel()

	collection := mongoClient.Database("gads").Collection("providers")
	cursor, err := collection.Find(ctx, bson.D{{}}, options.Find())
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Could not get db cursor when trying to get latest device info from db - %s", err))
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &providers); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Could not get devices latest info from db cursor - %s", err))
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
	}

	return providers
}

func AddOrUpdateUser(user models.User) error {
	update := bson.M{
		"$set": user,
	}
	coll := mongoClient.Database("gads").Collection("users")
	filter := bson.D{{Key: "email", Value: user.Email}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(mongoClientCtx, filter, update, opts)
	if err != nil {
		return err
	}
	return nil
}

func GetUserFromDB(email string) (models.User, error) {
	var user models.User

	coll := mongoClient.Database("gads").Collection("users")
	filter := bson.D{{Key: "email", Value: email}}
	err := coll.FindOne(context.TODO(), filter).Decode(&user)
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

type ProviderDB struct {
	OS                  string `json:"os" bson:"os"`
	Nickname            string `json:"nickname" bson:"nickname"`
	HostAddress         string `json:"host_address" bson:"host_address"`
	Port                int    `json:"port" bson:"port"`
	UseSeleniumGrid     bool   `json:"use_selenium_grid" bson:"use_selenium_grid"`
	SeleniumGrid        string `json:"selenium_grid" bson:"selenium_grid"`
	ProvideAndroid      bool   `json:"provide_android" bson:"provide_android"`
	ProvideIOS          bool   `json:"provide_ios" bson:"provide_ios"`
	WdaBundleID         string `json:"wda_bundle_id" bson:"wda_bundle_id"`
	WdaRepoPath         string `json:"wda_repo_path" bson:"wda_repo_path"`
	SupervisionPassword string `json:"supervision_password" bson:"supervision_password"`
}

func AddOrUpdateProvider(provider ProviderDB) error {
	update := bson.M{
		"$set": provider,
	}
	coll := mongoClient.Database("gads").Collection("providers_new")
	filter := bson.D{{Key: "nickname", Value: provider.Nickname}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(mongoClientCtx, filter, update, opts)
	if err != nil {
		return err
	}
	return nil
}

func GetProviderFromDB(nickname string) (ProviderDB, error) {
	var provider ProviderDB
	coll := mongoClient.Database("gads").Collection("providers_new")
	filter := bson.D{{Key: "nickname", Value: nickname}}

	err := coll.FindOne(context.TODO(), filter).Decode(&provider)
	if err != nil {
		return ProviderDB{}, err
	}
	return provider, nil
}
