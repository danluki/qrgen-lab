package app

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTaskKey(t *testing.T) {
	t.Parallel()

	taskID := "task-123"
	want := "qr:task:task-123"

	if got := TaskKey(taskID); got != want {
		t.Fatalf("TaskKey(%q) = %q, want %q", taskID, got, want)
	}
}

func TestResultKey(t *testing.T) {
	t.Parallel()

	taskID := "task-456"
	want := "qr:result:task-456"

	if got := ResultKey(taskID); got != want {
		t.Fatalf("ResultKey(%q) = %q, want %q", taskID, got, want)
	}
}

func TestTaskRecordJSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 14, 9, 0, 0, 0, time.UTC)
	want := TaskRecord{
		ID:        "task-789",
		Content:   "https://example.com",
		Size:      256,
		Status:    "queued",
		CreatedAt: now,
		UpdatedAt: now,
	}

	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got TaskRecord
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got != want {
		t.Fatalf("JSON round-trip mismatch: got %+v, want %+v", got, want)
	}
}
