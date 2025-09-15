package minio

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var GlobalMinioClient *MinioClient

type MinioClient struct {
	client *minio.Client
	bucket string
}

func init() {
	InitMinioClient()
}

func InitMinioClient() {
	endpoint := "localhost:9000"
	accessKeyID := "minioadmin"
	secretAccessKey := "minioadmin"
	bucketName := "appium-report-screenshots"
	useSSL := false

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalf("Failed to create Minio client: %v", err)
	}

	GlobalMinioClient = &MinioClient{
		client: client,
		bucket: bucketName,
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		log.Fatalf("Failed to check if bucket exists: %v", err)
	}
	if !exists {
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Fatalf("Failed to create bucket: %v", err)
		}
	}
}

func (mc *MinioClient) StoreScreenshot(buildID, sessionID, filename string, base64Data string) (string, error) {
	// Decode base64 to bytes
	imageData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %v", err)
	}

	// Create object path: buildID/sessionID/filename
	objectPath := fmt.Sprintf("%s/%s/%s", buildID, sessionID, filename)

	// Upload to Minio
	ctx := context.Background()
	_, err = mc.client.PutObject(ctx, mc.bucket, objectPath, bytes.NewReader(imageData), int64(len(imageData)), minio.PutObjectOptions{
		ContentType: "image/jpeg",
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload screenshot to Minio: %v", err)
	}

	return objectPath, nil
}
