package db

import (
	"GADS/common/errors"
	"GADS/common/models"
	"context"
	"fmt"
	"io"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"

	"slices"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	log "github.com/sirupsen/logrus"
)

var (
	mongoClient          *mongo.Client
	mongoClientCtx       context.Context
	mongoClientCtxCancel context.CancelFunc
	connectionString     string
	maxErrorCount        = 30                                         // Maximum number of errors before logging fatal
	maxTimeout           = time.Duration(maxErrorCount) * time.Second // Maximum timeout value for server selection and socket
)

func InitMongoClient(mongoDb string) {
	var err error
	connectionString = "mongodb://" + mongoDb + "/?keepAlive=true"

	// Set up a context for the connection.
	mongoClientCtx, mongoClientCtxCancel = context.WithCancel(context.Background())

	// Create a MongoDB client with options and timeout
	clientOptions := options.Client().
		ApplyURI(connectionString).
		SetServerSelectionTimeout(maxTimeout).
		SetConnectTimeout(5 * time.Second).
		SetSocketTimeout(maxTimeout).
		SetMaxPoolSize(200).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(5 * time.Minute)

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
		ctx, cancel := context.WithTimeout(mongoClientCtx, 1*time.Second)
		err := mongoClient.Ping(ctx, nil)
		cancel()

		if err != nil {
			errorCounter++
			log.WithFields(log.Fields{
				"error_count": errorCounter,
				"error":       err,
			}).Warn("Failed to ping MongoDB")

			if errorCounter >= maxErrorCount {
				log.Fatalf("Lost connection to MongoDB server and failed to reconnect after %d attempts!", maxErrorCount)
			}
		} else {
			if errorCounter > 0 {
				log.Info("MongoDB connection restored")
				errorCounter = 0
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func GetProviderFromDB(nickname string) (models.Provider, error) {
	var provider models.Provider
	coll := mongoClient.Database("gads").Collection("providers")
	filter := bson.D{{Key: "nickname", Value: nickname}}

	err := coll.FindOne(context.TODO(), filter).Decode(&provider)
	if err != nil {
		return models.Provider{}, err
	}
	return provider, nil
}

func GetProvidersFromDB() []models.Provider {
	var providers []models.Provider
	ctx, cancel := context.WithTimeout(mongoClientCtx, 10*time.Second)
	defer cancel()

	collection := mongoClient.Database("gads").Collection("providers")
	cursor, err := collection.Find(ctx, bson.D{{}}, options.Find())
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_providers",
		}).Error(fmt.Sprintf("Could not get db cursor when trying to get the providers info from db - %s", err))
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &providers); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_providers",
		}).Error(fmt.Sprintf("Could not get the providers info from db cursor - %s", err))
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_providers",
		}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
	}

	return providers
}

func AddOrUpdateUser(user models.User) error {
	update := bson.M{
		"$set": user,
	}
	coll := mongoClient.Database("gads").Collection("users")
	filter := bson.D{{Key: "username", Value: user.Username}}
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

func GetUserFromDB(username string) (models.User, error) {
	var user models.User

	coll := mongoClient.Database("gads").Collection("users")
	filter := bson.D{{Key: "username", Value: username}}
	err := coll.FindOne(context.TODO(), filter).Decode(&user)
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

func AddOrUpdateProvider(provider models.Provider) error {
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

func GetUsers() []models.User {
	var users []models.User
	collection := mongoClient.Database("gads").Collection("users")

	cursor, err := collection.Find(mongoClientCtx, bson.D{{}}, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_users",
		}).Error(fmt.Sprintf("Could not get db cursor when trying to get latest user info from db - %s", err))
		return users
	}

	if err := cursor.All(mongoClientCtx, &users); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_users",
		}).Error(fmt.Sprintf("Could not get users latest info from db cursor - %s", err))
		return users
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_devices",
		}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
		return users
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_users",
		}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
		return users
	}

	cursor.Close(mongoClientCtx)

	return users
}

func GetDBFiles() []models.DBFile {
	var files []models.DBFile
	collection := mongoClient.Database("gads").Collection("fs.files")

	cursor, err := collection.Find(mongoClientCtx, bson.D{{}}, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_files",
		}).Error(fmt.Sprintf("Could not get db cursor when trying to get latest files info from db - %s", err))
		return files
	}

	if err := cursor.All(mongoClientCtx, &files); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_files",
		}).Error(fmt.Sprintf("Could not get files latest info from db cursor - %s", err))
		return files
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_files",
		}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
		return files
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"event": "get_db_files",
		}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
		return files
	}

	cursor.Close(mongoClientCtx)

	return files
}

func GetDBDeviceNew() []models.Device {
	var dbDevices []models.Device
	// Access the database and collection
	collection := MongoClient().Database("gads").Collection("new_devices")

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

func UpsertDeviceDB(device *models.Device) error {
	update := bson.M{
		"$set": device,
	}
	coll := mongoClient.Database("gads").Collection("new_devices")
	filter := bson.D{{Key: "udid", Value: device.UDID}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(mongoClientCtx, filter, update, opts)
	if err != nil {
		return err
	}
	return nil
}

func DeleteDeviceDB(udid string) error {
	coll := mongoClient.Database("gads").Collection("new_devices")
	filter := bson.M{"udid": udid}

	_, err := coll.DeleteOne(mongoClientCtx, filter)
	if err != nil {
		return err
	}

	return nil
}

func DeleteUserDB(nickname string) error {
	coll := mongoClient.Database("gads").Collection("users")
	filter := bson.M{"username": nickname}

	_, err := coll.DeleteOne(mongoClientCtx, filter)
	if err != nil {
		return err
	}

	return nil
}

func DeleteProviderDB(nickname string) error {
	coll := mongoClient.Database("gads").Collection("providers")
	filter := bson.M{"nickname": nickname}

	_, err := coll.DeleteOne(mongoClientCtx, filter)
	if err != nil {
		return err
	}

	return nil
}

func AddAdminUserIfMissing() error {
	dbUser, err := GetUserFromDB("admin")
	if err != nil && err != mongo.ErrNoDocuments {
		return fmt.Errorf("AddAdminUserIfMissing: Failed to check if admin user is in the DB - %s", err)
	}

	if dbUser != (models.User{}) {
		return nil
	}

	err = AddOrUpdateUser(models.User{Username: "admin", Password: "password", Role: "admin"})
	if err != nil {
		return fmt.Errorf("Failed to add/update admin user - %s", err)
	}
	return nil
}

func UploadFileGridFS(file io.Reader, fileName string, force bool) error {
	mongoDb := MongoClient().Database("gads")
	bucket, err := gridfs.NewBucket(mongoDb, nil)

	// Create a filter and search the bucket for the selenium.jar file
	filter := bson.D{{"filename", fileName}}
	cursor, err := bucket.Find(filter)
	if err != nil {
		return fmt.Errorf("Failed to get cursor from DB - %s", err)
	}

	// Try to get the found files from the cursor
	type gridfsFile struct {
		Name string `bson:"filename"`
		ID   string `bson:"_id"`
	}
	var foundFiles []gridfsFile
	err = cursor.All(MongoCtx(), &foundFiles)
	if err != nil {
		return fmt.Errorf("Failed to get files from DB cursor - %s", err)
	}

	// If there are found files fail upload
	if len(foundFiles) == 1 {
		if force {
			// Get the ObjectID for the file in Mongo
			id, err := primitive.ObjectIDFromHex(foundFiles[0].ID)
			if err != nil {
				return fmt.Errorf("Failed to get ObjectID from the Mongo file ID - %s", err)
			}

			// Delete the file in Mongo before attempting to upload
			err = bucket.Delete(id)
			if err != nil {
				return fmt.Errorf("File is force upload but failed to delete it from Mongo before upload - %s", err)
			}

			// Upload the file to the bucket
			_, err = bucket.UploadFromStream(fileName, file, nil)
			if err != nil {
				return fmt.Errorf("Failed to upload file `%s` to bucket - %s", fileName, err)
			}
			return nil
		}
		return fmt.Errorf("File with name `%s` is already present in MongoDB", fileName)
	} else {
		// File is not in bucket so upload it
		_, err = bucket.UploadFromStream(fileName, file, nil)
		if err != nil {
			return fmt.Errorf("Failed to upload file `%s` to bucket - %s", fileName, err)
		}
		return nil
	}
}

func GetGlobalStreamSettings() (models.StreamSettings, error) {
	var globalSettings models.GlobalSettings
	var streamSettings models.StreamSettings

	coll := mongoClient.Database("gads").Collection("global_settings")
	filter := bson.D{{Key: "type", Value: "stream-settings"}}

	err := coll.FindOne(mongoClientCtx, filter).Decode(&globalSettings)
	if err == mongo.ErrNoDocuments {
		streamSettings = models.StreamSettings{
			TargetFPS:            15,
			JpegQuality:          75,
			ScalingFactorAndroid: 50,
			ScalingFactoriOS:     50,
		}

		err = UpdateGlobalStreamSettings(streamSettings)
		if err != nil {
			return streamSettings, err
		}
	} else if err != nil {
		return streamSettings, err
	} else {
		settingsBytes, err := bson.Marshal(globalSettings.Settings)
		if err != nil {
			return streamSettings, fmt.Errorf("failed to marshal settings: %v", err)
		}

		err = bson.Unmarshal(settingsBytes, &streamSettings)
		if err != nil {
			return streamSettings, fmt.Errorf("failed to unmarshal settings: %v", err)
		}
	}

	return streamSettings, nil
}

func UpdateGlobalStreamSettings(settings models.StreamSettings) error {
	globalSettings := models.GlobalSettings{
		Type:        "stream-settings",
		Settings:    settings,
		LastUpdated: time.Now(),
	}

	update := bson.M{
		"$set": globalSettings,
	}

	coll := mongoClient.Database("gads").Collection("global_settings")
	filter := bson.D{{Key: "type", Value: "stream-settings"}}
	opts := options.Update().SetUpsert(true)

	_, err := coll.UpdateOne(mongoClientCtx, filter, update, opts)
	return err
}

func GetDeviceStreamSettings(udid string) (models.DeviceStreamSettings, error) {
	var deviceStreamSettings models.DeviceStreamSettings

	coll := mongoClient.Database("gads").Collection("device_stream_settings")
	filter := bson.D{{Key: "udid", Value: udid}}

	err := coll.FindOne(mongoClientCtx, filter).Decode(&deviceStreamSettings)
	if err != nil {
		return deviceStreamSettings, err // Return error if no settings were found or if any other error occurred
	}

	return deviceStreamSettings, nil
}

func UpdateDeviceStreamSettings(udid string, settings models.DeviceStreamSettings) error {
	coll := mongoClient.Database("gads").Collection("device_stream_settings")
	filter := bson.D{{Key: "udid", Value: udid}}
	update := bson.M{
		"$set": settings,
	}
	opts := options.Update().SetUpsert(true)

	_, err := coll.UpdateOne(mongoClientCtx, filter, update, opts)
	return err
}
