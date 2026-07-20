/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

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
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (m *MongoStore) GetFiles() ([]models.DBFile, error) {
	coll := m.GetCollection("fs.files")
	return GetDocuments[models.DBFile](m.Ctx, coll, bson.D{{}})
}

// GetFilesByType returns only the GridFS files whose metadata.type matches the
// given discriminator (e.g. "app" for uploaded device apps).
func (m *MongoStore) GetFilesByType(fileType string) ([]models.DBFile, error) {
	coll := m.GetCollection("fs.files")
	return GetDocuments[models.DBFile](m.Ctx, coll, bson.D{{Key: "metadata.type", Value: fileType}})
}

// GetFileByID returns a single GridFS file document by its hex ObjectID.
func (m *MongoStore) GetFileByID(fileID string) (models.DBFile, error) {
	id, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return models.DBFile{}, fmt.Errorf("Failed to parse file id `%s` - %s", fileID, err)
	}
	coll := m.GetCollection("fs.files")
	return GetDocument[models.DBFile](m.Ctx, coll, bson.D{{Key: "_id", Value: id}})
}

func (m *MongoStore) UploadFile(file io.Reader, fileName string, force bool) error {
	return m.UploadFileWithMetadata(file, fileName, nil, force)
}

// UploadFileWithMetadata stores a file in the default GridFS bucket, optionally
// attaching custom metadata (used to record type/description/uploader for the
// WebDriverAgent IPAs and supervision profile). When force is true every
// existing file sharing the same filename is removed first so the upload
// replaces it; when false an existing filename is rejected.
func (m *MongoStore) UploadFileWithMetadata(file io.Reader, fileName string, metadata bson.M, force bool) error {
	_, err := m.UploadFileWithMetadataReturningID(file, fileName, metadata, force)
	return err
}

// UploadFileWithMetadataReturningID behaves like UploadFileWithMetadata but also
// returns the hex ObjectID of the stored GridFS file (used when the caller needs
// to reference the freshly uploaded file, e.g. to install it right after upload).
func (m *MongoStore) UploadFileWithMetadataReturningID(file io.Reader, fileName string, metadata bson.M, force bool) (string, error) {
	bucket, err := gridfs.NewBucket(m.GetDefaultDatabase(), nil)
	if err != nil {
		return "", fmt.Errorf("Failed to create GridFS bucket - %s", err)
	}

	filter := bson.D{{Key: "filename", Value: fileName}}
	cursor, err := bucket.Find(filter)
	if err != nil {
		return "", fmt.Errorf("Failed to get cursor from DB - %s", err)
	}

	type gridfsFile struct {
		Name string `bson:"filename"`
		ID   string `bson:"_id"`
	}

	var foundFiles []gridfsFile
	err = cursor.All(m.Ctx, &foundFiles)
	if err != nil {
		return "", fmt.Errorf("Failed to get files from DB cursor - %s", err)
	}

	if len(foundFiles) > 0 && !force {
		return "", fmt.Errorf("File with name `%s` is already present in MongoDB", fileName)
	}

	// Force replace - delete every existing file sharing this filename before upload
	for _, found := range foundFiles {
		id, err := primitive.ObjectIDFromHex(found.ID)
		if err != nil {
			return "", fmt.Errorf("Failed to get ObjectID from the Mongo file ID - %s", err)
		}
		if err := bucket.Delete(id); err != nil {
			return "", fmt.Errorf("File is force upload but failed to delete it from Mongo before upload - %s", err)
		}
	}

	uploadOpts := options.GridFSUpload()
	if metadata != nil {
		uploadOpts.SetMetadata(metadata)
	}
	fileID, err := bucket.UploadFromStream(fileName, file, uploadOpts)
	if err != nil {
		return "", fmt.Errorf("Failed to upload file `%s` to bucket - %s", fileName, err)
	}
	return fileID.Hex(), nil
}

// DownloadFileByID downloads a GridFS file identified by its hex ObjectID and
// writes it to downloadPath under localName. Used by the provider to fetch the
// specific WebDriverAgent IPA selected in its configuration while keeping the
// on-disk filename constant.
func (m *MongoStore) DownloadFileByID(fileID, downloadPath, localName string) error {
	bucket, err := gridfs.NewBucket(m.GetDefaultDatabase(), nil)
	if err != nil {
		return err
	}

	id, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return fmt.Errorf("Failed to parse file id `%s` - %s", fileID, err)
	}

	downloadStream, err := bucket.OpenDownloadStream(id)
	if err != nil {
		return fmt.Errorf("Failed to open download stream from the GridFS bucket - %s", err)
	}

	// Remove any stale local copy before writing the fresh download
	filePath := filepath.Join(downloadPath, localName)
	if err := os.Remove(filePath); err != nil {
		fmt.Printf("There is no %s file located at `%s`, nothing to remove\n", localName, filePath)
	}

	fileBuffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(fileBuffer, downloadStream); err != nil {
		return fmt.Errorf("Failed to copy download stream to the bytes buffer - %s", err)
	}

	actualFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Failed to create file with path `%s` - %s", filePath, err)
	}
	defer actualFile.Close()

	if _, err := actualFile.Write(fileBuffer.Bytes()); err != nil {
		return fmt.Errorf("Failed to write byte to file with path `%s` - %s", filePath, err)
	}
	return nil
}

// DeleteFileByID removes a GridFS file identified by its hex ObjectID.
func (m *MongoStore) DeleteFileByID(fileID string) error {
	bucket, err := gridfs.NewBucket(m.GetDefaultDatabase(), nil)
	if err != nil {
		return err
	}
	id, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return fmt.Errorf("Failed to parse file id `%s` - %s", fileID, err)
	}
	if err := bucket.Delete(id); err != nil {
		return fmt.Errorf("Failed to delete file `%s` from Mongo - %s", fileID, err)
	}
	return nil
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
