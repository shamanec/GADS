package minio

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var GlobalMinioClient *MinioClient

type MinioClient struct {
	client *minio.Client
}

func InitMinioClient() error {
	endpoint := "localhost:9000"
	accessKeyID := "minioadmin"
	secretAccessKey := "minioadmin"
	useSSL := false

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return fmt.Errorf("failed to create Minio client: %v", err)
	}

	GlobalMinioClient = &MinioClient{
		client: client,
	}

	return nil
}

func InitMinioClientWithConfig(endpoint, accessKeyID, secretAccessKey string, useSSL bool) error {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return fmt.Errorf("failed to create Minio client: %v", err)
	}

	GlobalMinioClient = &MinioClient{
		client: client,
	}

	return nil
}

func (mc *MinioClient) EnsureBucket(bucketName string) error {
	ctx := context.Background()
	exists, err := mc.client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists: %v", err)
	}

	if !exists {
		err = mc.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}
	}

	return nil
}

func (mc *MinioClient) StoreAppiumScreenshot(buildID, sessionID, filename string, base64Data string) (string, error) {
	bucketName := "appium-report-screenshots"

	// Ensure bucket exists
	if err := mc.EnsureBucket(bucketName); err != nil {
		return "", err
	}
	// Decode base64 to bytes
	imageData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %v", err)
	}

	// Create object path: buildID/sessionID/filename
	objectPath := fmt.Sprintf("%s/%s/%s", buildID, sessionID, filename)

	// Upload to Minio
	ctx := context.Background()
	_, err = mc.client.PutObject(ctx, bucketName, objectPath, bytes.NewReader(imageData), int64(len(imageData)), minio.PutObjectOptions{
		ContentType: "image/jpeg",
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload screenshot to Minio: %v", err)
	}

	return objectPath, nil
}

func (mc *MinioClient) GetAppiumScreenshot(buildID, sessionID, filename string) (io.ReadCloser, error) {
	bucketName := "appium-report-screenshots"

	// Create object path: buildID/sessionID/filename
	objectPath := fmt.Sprintf("%s/%s/%s", buildID, sessionID, filename)

	// Check if object exists first
	ctx := context.Background()
	_, err := mc.client.StatObject(ctx, bucketName, objectPath, minio.StatObjectOptions{})
	if err != nil {
		// Check if it's a not found error
		if err.Error() == "The specified key does not exist." {
			return nil, fmt.Errorf("The specified key does not exist.")
		}
		return nil, fmt.Errorf("failed to stat screenshot object: %v", err)
	}

	// Get object from Minio
	reader, err := mc.client.GetObject(ctx, bucketName, objectPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get screenshot from Minio: %v", err)
	}

	return reader, nil
}
