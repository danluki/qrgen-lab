package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/danluki/qrgen/internal/app"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type createTaskRequest struct {
	Content string `json:"content"`
	Size    int    `json:"size"`
}

var (
	gatewayRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "qrgen_gateway_http_requests_total",
			Help: "Total number of HTTP requests handled by the gateway.",
		},
		[]string{"method", "route", "status"},
	)
	gatewayRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "qrgen_gateway_http_request_duration_seconds",
			Help:    "Duration of HTTP requests handled by the gateway.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
	gatewayTasksCreated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "qrgen_gateway_tasks_created_total",
			Help: "Total number of QR generation tasks created by the gateway.",
		},
	)
	gatewayTasksFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "qrgen_gateway_task_enqueue_failures_total",
			Help: "Total number of task enqueue failures in the gateway.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		gatewayRequestsTotal,
		gatewayRequestDuration,
		gatewayTasksCreated,
		gatewayTasksFailed,
	)
}

func main() {
	redisAddr := getEnv("REDIS_ADDR", "redis:6379")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	port := getEnv("PORT", "8080")

	redisOpt := asynq.RedisClientOpt{Addr: redisAddr, Password: redisPassword}
	client := asynq.NewClient(redisOpt)
	defer client.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})
	defer rdb.Close()

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), prometheusMiddleware())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.POST("/api/tasks", func(c *gin.Context) {
		var req createTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		req.Content = strings.TrimSpace(req.Content)
		if req.Content == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
			return
		}

		if req.Size == 0 {
			req.Size = 256
		}
		if req.Size < 128 || req.Size > 1024 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "size must be between 128 and 1024"})
			return
		}

		now := time.Now().UTC()
		taskID := uuid.NewString()
		record := app.TaskRecord{
			ID:        taskID,
			Content:   req.Content,
			Size:      req.Size,
			Status:    "queued",
			CreatedAt: now,
			UpdatedAt: now,
		}

		ctx := c.Request.Context()
		if err := app.SaveTask(ctx, rdb, record); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist task"})
			return
		}

		payload, err := json.Marshal(app.TaskPayload{TaskID: taskID, Content: req.Content, Size: req.Size})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare task payload"})
			return
		}

		task := asynq.NewTask(app.TaskTypeGenerateQR, payload)
		info, err := client.EnqueueContext(ctx, task, asynq.TaskID(taskID), asynq.MaxRetry(10), asynq.Timeout(2*time.Minute))
		if err != nil {
			gatewayTasksFailed.Inc()
			record.Status = "failed"
			record.Error = err.Error()
			record.UpdatedAt = time.Now().UTC()
			_ = app.SaveTask(ctx, rdb, record)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enqueue task"})
			return
		}

		gatewayTasksCreated.Inc()

		c.JSON(http.StatusAccepted, gin.H{
			"task": gin.H{
				"id":         record.ID,
				"content":    record.Content,
				"size":       record.Size,
				"status":     record.Status,
				"created_at": record.CreatedAt,
				"updated_at": record.UpdatedAt,
				"queue":      info.Queue,
				"image_url":  fmt.Sprintf("/api/tasks/%s/image", taskID),
				"status_url": fmt.Sprintf("/api/tasks/%s", taskID),
			},
		})
	})

	router.GET("/api/tasks/:id", func(c *gin.Context) {
		record, err := app.LoadTask(c.Request.Context(), rdb, c.Param("id"))
		if err == redis.Nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load task"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"task": gin.H{
				"id":         record.ID,
				"content":    record.Content,
				"size":       record.Size,
				"status":     record.Status,
				"error":      record.Error,
				"created_at": record.CreatedAt,
				"updated_at": record.UpdatedAt,
				"image_url":  fmt.Sprintf("/api/tasks/%s/image", record.ID),
			},
		})
	})

	router.GET("/api/tasks/:id/image", func(c *gin.Context) {
		taskID := c.Param("id")
		record, err := app.LoadTask(c.Request.Context(), rdb, taskID)
		if err == redis.Nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load task"})
			return
		}
		if record.Status != "completed" {
			c.JSON(http.StatusConflict, gin.H{"error": "task is not completed yet", "status": record.Status})
			return
		}

		data, err := rdb.Get(c.Request.Context(), app.ResultKey(taskID)).Bytes()
		if err == redis.Nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "qr code not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load qr code"})
			return
		}

		c.Data(http.StatusOK, "image/png", data)
	})

	router.GET("/api/queues/default", func(c *gin.Context) {
		inspector := asynq.NewInspector(redisOpt)
		defer inspector.Close()

		queueInfo, err := inspector.GetQueueInfo("default")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to inspect queue"})
			return
		}

		c.JSON(http.StatusOK, queueInfo)
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("gateway listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		gatewayRequestsTotal.WithLabelValues(c.Request.Method, route, status).Inc()
		gatewayRequestDuration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}
