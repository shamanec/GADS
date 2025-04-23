package db

import (
	"GADS/common/models"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

func (m *MongoStore) GetFiles() ([]models.DBFile, error) {
	coll := m.GetCollection("fs.files")
	return GetDocuments[models.DBFile](m.Ctx, coll, bson.D{{}})
}

func (m *MongoStore) UploadFile(file io.Reader, fileName string, force bool) error {
	bucket, err := gridfs.NewBucket(m.GetDefaultDatabase(), nil)
	filter := bson.D{{Key: "filename", Value: fileName}}

	cursor, err := bucket.Find(filter)
	if err != nil {
		return fmt.Errorf("Failed to get cursor from DB - %s", err)
	}

	type gridfsFile struct {
		Name string `bson:"filename"`
		ID   string `bson:"_id"`
	}

	var foundFiles []gridfsFile
	err = cursor.All(m.Ctx, &foundFiles)
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

func (m *MongoStore) DownloadFile(fileName, downloadPath string) error {
	bucket, err := gridfs.NewBucket(m.GetDefaultDatabase(), nil)
	if err != nil {
		return err
	}

	// Create a filter and search the bucket for the WebDriverAgent.ipa file
	filter := bson.D{{Key: "filename", Value: fileName}}
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
	err = cursor.All(m.Ctx, &foundFiles)
	if err != nil {
		return fmt.Errorf("Failed to get files from DB cursor - %s", err)
	}

	// If no found files
	if len(foundFiles) == 0 {
		return fmt.Errorf("%s is not present in MongoDB, have you uploaded it?", fileName)
	}

	// If more than 1 found file
	if len(foundFiles) > 1 {
		fmt.Printf("There is more than one %s file in MongoDB, will download the first one!\n", fileName)
	}

	// Create the filepath and remove the supervision profile file if present
	filePath := filepath.Join(downloadPath, fileName)
	err = os.Remove(filePath)
	if err != nil {
		fmt.Printf("There is no %s file located at `%s`, nothing to remove\n", fileName, filePath)
	}

	// Get the ObjectID from the file ID in Mongo
	id, err := primitive.ObjectIDFromHex(foundFiles[0].ID)
	if err != nil {
		return fmt.Errorf("Failed to get object id from hex - %s", err)
	}
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
