package config

import (
	"GADS/common/db"
	"GADS/common/models"
	"bytes"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"io"
	"log"
	"os"
)

var Config = &models.ConfigJsonData{}

func SetupConfig(nickname, folder string) {
	provider, err := db.GetProviderFromDB(nickname)
	if err != nil {
		log.Fatalf("Failed to gte provider data from DB - %s", err)
	}
	if provider.Nickname == "" {
		log.Fatal("Provider with this nickname is not registered in the DB")
	}
	provider.ProviderFolder = folder
	Config.EnvConfig = provider
}

func SetupSeleniumJar() error {
	mongoDb := db.MongoClient().Database("gads")
	bucket, err := gridfs.NewBucket(mongoDb, nil)

	// Create a filter and search the bucket for the selenium.jar file
	filter := bson.D{{"filename", "selenium.jar"}}
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
	err = cursor.All(db.MongoCtx(), &foundFiles)
	if err != nil {
		return fmt.Errorf("Failed to get files from DB cursor - %s", err)
	}

	// If no found files
	if len(foundFiles) == 0 {
		return fmt.Errorf("Selenium jar file is not present in MongoDB, you have to upload it via the hub admin UI")
	}

	// If more than 1 found file
	if len(foundFiles) > 1 {
		return fmt.Errorf("There is more than 1 file with the same selenium jar file name stored in MongoDB")
	}

	// Create the filepath and remove the selenium jar if present
	filePath := fmt.Sprintf("%s/%s", Config.EnvConfig.ProviderFolder, "selenium.jar")
	err = os.Remove(filePath)
	if err != nil {
		fmt.Printf("There is no Selenium jar file located at `%s`, nothing to remove\n", filePath)
	}

	// Get the ObjectID from the file ID in Mongo
	id, err := primitive.ObjectIDFromHex(foundFiles[0].ID)
	downloadStream, err := bucket.OpenDownloadStream(id)
	if err != nil {
		return fmt.Errorf("Failed to open download stream from the GridFS bucket - %s", err)
	}

	// Create a new buffer and read the download stream to it
	fileBuffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(fileBuffer, downloadStream); err != nil {
		return fmt.Errorf("Failed to copy download stream to the bytes buffer - %s", err)
	}

	// Create the file on the provider host
	actualFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Failed to create file with path `%s` - %s", filePath, err)
	}
	defer actualFile.Close()

	// Write the file contents to the file
	_, err = actualFile.Write(fileBuffer.Bytes())
	if err != nil {
		return fmt.Errorf("Failed to write byte to file with path `%s` - %s", filePath, err)
	}

	return nil
}
