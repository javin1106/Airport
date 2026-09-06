package queue

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

const buildQueueKey = "build-queue"

type RedisQueue struct {
	client *redis.Client
}

func NewRedis(address string) *RedisQueue {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: "",
		DB:       0,
	})

	return &RedisQueue{
		client: client,
	}
}

func (queue *RedisQueue) Ping(ctx context.Context) error {
	if err := queue.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}

	return nil
}

func (queue *RedisQueue) EnqueueBuild(
	ctx context.Context,
	deploymentID string,
) error {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return fmt.Errorf("deployment ID is required")
	}

	if err := queue.client.LPush(ctx, buildQueueKey, deploymentID).Err(); err != nil {
		return fmt.Errorf("enqueue deployment %q: %w", deploymentID, err)
	}

	return nil
}

func (queue *RedisQueue) Close() error {
	return queue.client.Close()
}
