package app

import (
	"time"

	"github.com/robertDouglass/claude-log-analyzer/internal/analyzer"
)

type UploadStore interface {
	SaveUpload(jobID string, data []byte) (string, error)
	ReadUpload(path string) ([]byte, error)
}

type JobQueue interface {
	CreateJob(job Job) error
	ClaimNextJob() (Job, bool, error)
	CompleteJob(job Job, report analyzer.Report) error
	FailJob(job Job, jobErr error) error
	GetJob(id string) (Job, error)
	QueueDepth() (int, error)
}

type ReportStore interface {
	GetReport(id string) (analyzer.Report, error)
}

type APIStore interface {
	UploadStore
	JobQueue
	ReportStore
}

type DirectUpload struct {
	JobID        string            `json:"job_id"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Fields       map[string]string `json:"fields,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at"`
	MaxBytes     int64             `json:"max_bytes"`
	FinalizePath string            `json:"finalize_path"`
}

type DirectUploadStore interface {
	CreateDirectUpload(jobID string, expiresIn time.Duration, maxBytes int64) (DirectUpload, error)
	FinalizeDirectUpload(jobID string) error
}

type WorkerStore interface {
	UploadStore
	JobQueue
}

type SweepResult struct {
	UploadsDeleted int `json:"uploads_deleted"`
	ReportsDeleted int `json:"reports_deleted"`
}

type SweeperStore interface {
	SweepExpired(now time.Time, rawUploadTTL, reportTTL time.Duration) (SweepResult, error)
}
