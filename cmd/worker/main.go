package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/javin1106/airport/internal/queue"
	"github.com/javin1106/airport/internal/storage"
	"github.com/javin1106/airport/internal/worker"
)

func main() {
	useSSL, err := strconv.ParseBool(os.Getenv("S3_USE_SSL"))
	if err != nil {
		log.Fatalf("invalid S3_USE_SSL value: %v", err)
	}

	objectStore, err := storage.New(storage.Config{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Bucket:    os.Getenv("S3_BUCKET"),
		UseSSL:    useSSL,
	})
	if err != nil {
		log.Fatalf("configure object storage: %v", err)
	}

	redisAddress := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if redisAddress == "" {
		log.Fatal("REDIS_ADDR is required")
	}
	buildQueue := queue.NewRedis(redisAddress)
	defer buildQueue.Close()

	storageContext, storageCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := objectStore.EnsureBucket(storageContext); err != nil {
		storageCancel()
		log.Fatalf("prepare object storage: %v", err)
	}
	storageCancel()

	redisContext, redisCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := buildQueue.Ping(redisContext); err != nil {
		redisCancel()
		log.Fatalf("connect to Redis: %v", err)
	}
	redisCancel()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	builder := worker.NewBuilder(objectStore)
	log.Print("Airport worker is waiting for builds")

	for ctx.Err() == nil {
		deploymentID, err := buildQueue.DequeueBuild(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				break
			}
			log.Printf("failed to dequeue build: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		log.Printf("building deployment %s", deploymentID)
		setStatus(buildQueue, deploymentID, "building", "")
		buildContext, cancelBuild := context.WithTimeout(ctx, 10*time.Minute)
		count, err := builder.Build(buildContext, deploymentID)
		cancelBuild()
		if err != nil {
			setStatus(buildQueue, deploymentID, "failed", err.Error())
			log.Printf("deployment %s failed: %v", deploymentID, err)
			continue
		}
		setStatus(buildQueue, deploymentID, "ready", "")
		log.Printf("deployment %s built: uploaded %d files", deploymentID, count)
	}
}

func setStatus(buildQueue *queue.RedisQueue, deploymentID, status, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := buildQueue.SetStatus(ctx, deploymentID, status, message); err != nil {
		log.Printf("failed to record deployment %s status %s: %v", deploymentID, status, err)
	}
}
