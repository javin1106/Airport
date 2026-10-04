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

	_, err := queue.client.TxPipelined(ctx, func(pipeline redis.Pipeliner) error {
		pipeline.HSet(ctx, deploymentStatusKey(deploymentID), "status", "queued", "error", "")
		pipeline.LPush(ctx, buildQueueKey, deploymentID)
		return nil
	})
	if err != nil {
		return fmt.Errorf("enqueue deployment %q: %w", deploymentID, err)
	}

	return nil
}

func (queue *RedisQueue) DequeueBuild(ctx context.Context) (string, error) {
	result, err := queue.client.BRPop(ctx, 0, buildQueueKey).Result()
	if err != nil {
		return "", fmt.Errorf("dequeue build: %w", err)
	}
	if len(result) != 2 || strings.TrimSpace(result[1]) == "" {
		return "", fmt.Errorf("dequeue build: invalid Redis response")
	}

	return result[1], nil
}

func (queue *RedisQueue) HasDeployment(ctx context.Context, deploymentID string) (bool, error) {
	count, err := queue.client.Exists(ctx, deploymentStatusKey(deploymentID)).Result()
	if err != nil {
		return false, fmt.Errorf("check deployment %q: %w", deploymentID, err)
	}
	return count > 0, nil
}

func (queue *RedisQueue) SetStatus(ctx context.Context, deploymentID, status, message string) error {
	if err := queue.client.HSet(ctx, deploymentStatusKey(deploymentID), "status", status, "error", message).Err(); err != nil {
		return fmt.Errorf("update deployment %q status: %w", deploymentID, err)
	}
	return nil
}

func deploymentStatusKey(deploymentID string) string {
	return "deployment:" + deploymentID
}

func (queue *RedisQueue) Close() error {
	return queue.client.Close()
}
