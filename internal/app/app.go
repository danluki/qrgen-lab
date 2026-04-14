package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	TaskTypeGenerateQR = "qr:generate"
)

type TaskPayload struct {
	TaskID  string `json:"task_id"`
	Content string `json:"content"`
	Size    int    `json:"size"`
}

type TaskRecord struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Size      int       `json:"size"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func TaskKey(taskID string) string {
	return fmt.Sprintf("qr:task:%s", taskID)
}

func ResultKey(taskID string) string {
	return fmt.Sprintf("qr:result:%s", taskID)
}

func SaveTask(ctx context.Context, rdb *redis.Client, record TaskRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return rdb.Set(ctx, TaskKey(record.ID), payload, 24*time.Hour).Err()
}

func LoadTask(ctx context.Context, rdb *redis.Client, taskID string) (TaskRecord, error) {
	payload, err := rdb.Get(ctx, TaskKey(taskID)).Bytes()
	if err != nil {
		return TaskRecord{}, err
	}

	var record TaskRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return TaskRecord{}, err
	}

	return record, nil
}
