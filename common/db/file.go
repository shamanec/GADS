package db

import (
	"GADS/common/models"
	"fmt"
	"io"

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
	filter := bson.D{{"filename", fileName}}

	cursor, err := bucket.Find(filter)
	if err != nil {
		return fmt.Errorf("Failed to get cursor from DB - %s", err)
	}

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
