package db

import (
	"GADS/common/errors"
	"GADS/common/models"
	"context"
	"fmt"
	"time"

	"slices"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	log "github.com/sirupsen/logrus"
)

var mongoClient *mongo.Client
var mongoClientCtx context.Context
var mongoClientCtxCancel context.CancelFunc

func InitMongoClient(mongoDb string) {
	var err error
	connectionString := "mongodb://" + mongoDb + "/?keepAlive=true"

	// Set up a context for the connection.
	mongoClientCtx, mongoClientCtxCancel = context.WithCancel(context.Background())

	// Create a MongoDB client with options.
	clientOptions := options.Client().ApplyURI(connectionString)
	mongoClient, err = mongo.Connect(mongoClientCtx, clientOptions)
	if err != nil {
		log.Fatalf("Could not connect to Mongo server at `%s` - %s", connectionString, err)
	}

	go checkDBConnection()
}

func MongoClient() *mongo.Client {
	if mongoClient == nil {
		errors.ExitWithErrorMessage("Mongo client is not initialized")
	}
	return mongoClient
}

func MongoCtx() context.Context {
	return mongoClientCtx
}

func MongoCtxCancel() context.CancelFunc {
	return mongoClientCtxCancel
}

func CloseMongoConn() {
	err := mongoClient.Disconnect(mongoClientCtx)
	if err != nil {
		log.Fatalf("Failed to close mongo connection when stopping provider - %s", err)
	}
}

func checkDBConnection() {
	errorCounter := 0
	for {
		if errorCounter < 10 {
			time.Sleep(1 * time.Second)
			err := mongoClient.Ping(mongoClientCtx, nil)
			if err != nil {
				fmt.Println("FAILED PINGING MONGO")
				errorCounter++
				continue
			}
		} else {
			log.Fatal("Lost connection to MongoDB server for more than 10 seconds!")
		}
	}
}

func GetProviderFromDB(nickname string) (models.ProviderDB, error) {
	var provider models.ProviderDB
	coll := mongoClient.Database("gads").Collection("providers")
	filter := bson.D{{Key: "nickname", Value: nickname}}

	err := coll.FindOne(context.TODO(), filter).Decode(&provider)
	if err != nil {
		return models.ProviderDB{}, err
	}
	return provider, nil
}

func GetProvidersFromDB() []models.ProviderDB {
	var providers []models.ProviderDB
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

func CreateCappedCollection(dbName, collectionName string, maxDocuments, mb int64) error {

	database := MongoClient().Database(dbName)
	collections, err := database.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		return err
	}

	if slices.Contains(collections, collectionName) {
		return err
	}

	// Create capped collection options with limit of documents or 20 mb size limit
	// Seems reasonable for now, I have no idea what is a proper amount
	collectionOptions := options.CreateCollection()
	collectionOptions.SetCapped(true)
	collectionOptions.SetMaxDocuments(maxDocuments)
	collectionOptions.SetSizeInBytes(mb * 1024 * 1024)

	// Create the actual collection
	err = database.CreateCollection(MongoCtx(), collectionName, collectionOptions)
	if err != nil {
		return err
	}

	return nil
}

func CollectionExists(dbName, collectionName string) (bool, error) {
	database := MongoClient().Database(dbName)
	collections, err := database.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		return false, err
	}

	if slices.Contains(collections, collectionName) {
		return true, nil
	}

	return false, nil
}

func AddCollectionIndex(dbName, collectionName string, indexModel mongo.IndexModel) error {
	ctx, cancel := context.WithCancel(MongoCtx())
	defer cancel()

	db := MongoClient().Database(dbName)
	_, err := db.Collection(collectionName).Indexes().CreateOne(ctx, indexModel)
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

func AddOrUpdateProvider(provider models.ProviderDB) error {
	update := bson.M{
		"$set": provider,
	}
	coll := mongoClient.Database("gads").Collection("providers")
	filter := bson.D{{Key: "nickname", Value: provider.Nickname}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(mongoClientCtx, filter, update, opts)
	if err != nil {
		return err
	}
	return nil
}

func GetDBDevices() []models.Device {
	var dbDevices []models.Device
	// Access the database and collection
	collection := MongoClient().Database("gads").Collection("devices")

	cursor, err := collection.Find(context.Background(), bson.D{{}}, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Could not get db cursor when trying to get latest device info from db - %s", err))
	}

	if err := cursor.All(context.Background(), &dbDevices); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Could not get devices latest info from db cursor - %s", err))
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
	}

	cursor.Close(context.TODO())

	return dbDevices
}

func GetDBDevicesUDIDs() []string {
	dbDevices := GetDBDevices()
	var udids []string

	for _, dbDevice := range dbDevices {
		udids = append(udids, dbDevice.UDID)
	}

	return udids
}

func UpsertDeviceDB(device models.Device) error {
	update := bson.M{
		"$set": device,
	}
	coll := mongoClient.Database("gads").Collection("devices")
	filter := bson.D{{Key: "udid", Value: device.UDID}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(mongoClientCtx, filter, update, opts)
	if err != nil {
		return err
	}
	return nil
}

func GetDevices() []*models.Device {
	// Access the database and collection
	collection := MongoClient().Database("gads").Collection("devices")
	latestDevices := []*models.Device{}

	cursor, err := collection.Find(context.Background(), bson.D{{}}, options.Find())
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Could not get db cursor when trying to get latest device info from db - %s", err))
		return latestDevices
	}

	if err := cursor.All(context.Background(), &latestDevices); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Could not get devices latest info from db cursor - %s", err))
		return latestDevices
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
		return latestDevices
	}

	err = cursor.Close(context.TODO())
	if err != nil {
		//stuff
	}

	return latestDevices
}
