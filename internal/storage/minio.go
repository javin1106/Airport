package storage

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type Client struct {
	api    *minio.Client
	bucket string
}

func New(config Config) (*Client, error) {
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	config.AccessKey = strings.TrimSpace(config.AccessKey)
	config.Bucket = strings.TrimSpace(config.Bucket)

	if config.Endpoint == "" {
		return nil, fmt.Errorf("storage endpoint is required")
	}

	if config.AccessKey == "" {
		return nil, fmt.Errorf("storage access key is required")
	}

	if strings.TrimSpace(config.SecretKey) == "" {
		return nil, fmt.Errorf("storage secret key is required")
	}

	if config.Bucket == "" {
		return nil, fmt.Errorf("storage bucket is required")
	}

	minioClient, err := minio.New(
		config.Endpoint,
		&minio.Options{
			Creds: credentials.NewStaticV4(
				config.AccessKey,
				config.SecretKey,
				"",
			),
			Secure: config.UseSSL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}

	return &Client{
		api:    minioClient,
		bucket: config.Bucket,
	}, nil
}

func (client *Client) EnsureBucket(ctx context.Context) error {
	exists, err := client.api.BucketExists(ctx, client.bucket)
	if err != nil {
		return fmt.Errorf("check storage bucket: %w", err)
	}

	if exists {
		return nil
	}

	err = client.api.MakeBucket(
		ctx,
		client.bucket,
		minio.MakeBucketOptions{},
	)
	if err != nil {
		return fmt.Errorf("create storage bucket: %w", err)
	}

	return nil
}

func (client *Client) UploadArchive(
	ctx context.Context,
	deploymentID string,
	archivePath string,
) (string, error) {
	deploymentID = strings.TrimSpace(deploymentID)
	archivePath = strings.TrimSpace(archivePath)

	if deploymentID == "" {
		return "", fmt.Errorf("deployment ID is required")
	}

	if archivePath == "" {
		return "", fmt.Errorf("archive path is required")
	}

	objectKey := path.Join(
		"sources",
		deploymentID,
		"source.tar.gz",
	)

	_, err := client.api.FPutObject(
		ctx,
		client.bucket,
		objectKey,
		archivePath,
		minio.PutObjectOptions{
			ContentType: "application/gzip",
		},
	)
	if err != nil {
		return "", fmt.Errorf("upload source archive: %w", err)
	}

	return objectKey, nil
}
