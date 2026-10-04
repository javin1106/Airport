package storage

import (
	"context"
	"fmt"
	"io/fs"
	"mime"
	"path"
	"path/filepath"
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

	objectKey := sourceObjectKey(deploymentID)

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

func (client *Client) DeleteArchive(ctx context.Context, deploymentID string) error {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return fmt.Errorf("deployment ID is required")
	}

	if err := client.api.RemoveObject(ctx, client.bucket, sourceObjectKey(deploymentID), minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete source archive: %w", err)
	}

	return nil
}

func (client *Client) DownloadArchive(ctx context.Context, deploymentID, destination string) error {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return fmt.Errorf("deployment ID is required")
	}
	if strings.TrimSpace(destination) == "" {
		return fmt.Errorf("destination is required")
	}

	if err := client.api.FGetObject(ctx, client.bucket, sourceObjectKey(deploymentID), destination, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("download source archive: %w", err)
	}

	return nil
}

func (client *Client) UploadDirectory(ctx context.Context, deploymentID, directory string) (int, error) {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return 0, fmt.Errorf("deployment ID is required")
	}
	if strings.TrimSpace(directory) == "" {
		return 0, fmt.Errorf("directory is required")
	}

	count := 0
	err := filepath.WalkDir(directory, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported build output entry %q", filePath)
		}

		relativePath, err := filepath.Rel(directory, filePath)
		if err != nil {
			return err
		}
		objectKey := path.Join("dist", deploymentID, filepath.ToSlash(relativePath))
		contentType := mime.TypeByExtension(filepath.Ext(filePath))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		if _, err := client.api.FPutObject(ctx, client.bucket, objectKey, filePath, minio.PutObjectOptions{
			ContentType: contentType,
		}); err != nil {
			return fmt.Errorf("upload build output %q: %w", relativePath, err)
		}
		count++
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("upload build output: %w", err)
	}
	if count == 0 {
		return 0, fmt.Errorf("build output directory is empty")
	}

	return count, nil
}

func sourceObjectKey(deploymentID string) string {
	return path.Join("sources", deploymentID, "source.tar.gz")
}
