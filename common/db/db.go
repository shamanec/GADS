package db

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"

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

func UploadFileGridFS(file io.Reader, fileName string, force bool) error {
	mongoDb := GlobalMongoStore.Client.Database("gads")
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
	err = cursor.All(GlobalMongoStore.Ctx, &foundFiles)
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
