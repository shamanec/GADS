package minio

import (
	"context"
	"fmt"

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
