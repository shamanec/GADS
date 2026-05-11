package minio

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioClient struct {
	client *minio.Client
}

func InitMinioClientWithConfig(endpoint, accessKeyID, secretAccessKey string, useSSL bool) (MinioClient, error) {
	var minioClient MinioClient

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return minioClient, fmt.Errorf("failed to create Minio client: %v", err)
	}

	minioClient = MinioClient{
		client: client,
	}

	return minioClient, nil
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
