package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/danluki/qrgen/internal/app"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	qrcode "github.com/skip2/go-qrcode"
)

var (
	workerTasksProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "qrgen_worker_tasks_processed_total",
			Help: "Total number of tasks processed by the QR worker.",
		},
		[]string{"status"},
	)
	workerTaskDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "qrgen_worker_task_duration_seconds",
			Help:    "Duration of QR generation task processing.",
			Buckets: prometheus.DefBuckets,
		},
	)
	workerTasksInProgress = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "qrgen_worker_tasks_in_progress",
			Help: "Current number of QR generation tasks in progress.",
		},
	)
)

func init() {
	prometheus.MustRegister(workerTasksProcessed, workerTaskDuration, workerTasksInProgress)
}

func main() {
	redisAddr := getEnv("REDIS_ADDR", "redis:6379")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	concurrency := 10
	metricsPort := getEnv("METRICS_PORT", "2112")

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})
	defer rdb.Close()

	go startMetricsServer(metricsPort)

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
		startedAt := time.Now()
		workerTasksInProgress.Inc()
		defer workerTasksInProgress.Dec()

		var payload app.TaskPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			workerTasksProcessed.WithLabelValues("failed").Inc()
			return err
		}

		now := time.Now().UTC()
		record, err := app.LoadTask(ctx, rdb, payload.TaskID)
		if err != nil {
			workerTasksProcessed.WithLabelValues("failed").Inc()
			return err
		}
		record.Status = "processing"
		record.Error = ""
		record.UpdatedAt = now
		if err := app.SaveTask(ctx, rdb, record); err != nil {
			workerTasksProcessed.WithLabelValues("failed").Inc()
			return err
		}

		png, err := qrcode.Encode(payload.Content, qrcode.Medium, payload.Size)
		if err != nil {
			record.Status = "failed"
			record.Error = err.Error()
			record.UpdatedAt = time.Now().UTC()
			_ = app.SaveTask(context.Background(), rdb, record)
			workerTasksProcessed.WithLabelValues("failed").Inc()
			return err
		}

		if err := rdb.Set(ctx, app.ResultKey(payload.TaskID), png, 24*time.Hour).Err(); err != nil {
			record.Status = "failed"
			record.Error = err.Error()
			record.UpdatedAt = time.Now().UTC()
			_ = app.SaveTask(context.Background(), rdb, record)
			workerTasksProcessed.WithLabelValues("failed").Inc()
			return err
		}

		record.Status = "completed"
		record.Error = ""
		record.UpdatedAt = time.Now().UTC()
		if err := app.SaveTask(ctx, rdb, record); err != nil {
			workerTasksProcessed.WithLabelValues("failed").Inc()
			return err
		}

		workerTaskDuration.Observe(time.Since(startedAt).Seconds())
		workerTasksProcessed.WithLabelValues("completed").Inc()
		return nil
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

func startMetricsServer(port string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("qrgen metrics listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
