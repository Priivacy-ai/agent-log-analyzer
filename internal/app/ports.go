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
