package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/danluki/qrgen/internal/app"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	qrcode "github.com/skip2/go-qrcode"
)

func main() {
	redisAddr := getEnv("REDIS_ADDR", "redis:6379")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	concurrency := 10

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})
	defer rdb.Close()

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr, Password: redisPassword},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"default": 1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(app.TaskTypeGenerateQR, func(ctx context.Context, task *asynq.Task) error {
		var payload app.TaskPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		now := time.Now().UTC()
		record, err := app.LoadTask(ctx, rdb, payload.TaskID)
		if err != nil {
			return err
		}
		record.Status = "processing"
		record.Error = ""
		record.UpdatedAt = now
		if err := app.SaveTask(ctx, rdb, record); err != nil {
			return err
		}

		png, err := qrcode.Encode(payload.Content, qrcode.Medium, payload.Size)
		if err != nil {
			record.Status = "failed"
			record.Error = err.Error()
			record.UpdatedAt = time.Now().UTC()
			_ = app.SaveTask(context.Background(), rdb, record)
			return err
		}

		if err := rdb.Set(ctx, app.ResultKey(payload.TaskID), png, 24*time.Hour).Err(); err != nil {
			record.Status = "failed"
			record.Error = err.Error()
			record.UpdatedAt = time.Now().UTC()
			_ = app.SaveTask(context.Background(), rdb, record)
			return err
		}

		record.Status = "completed"
		record.Error = ""
		record.UpdatedAt = time.Now().UTC()
		return app.SaveTask(ctx, rdb, record)
	})

	log.Printf("qrgen worker connected to %s", redisAddr)
	if err := srv.Run(mux); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
