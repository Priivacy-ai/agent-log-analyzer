package app

import (
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analytics"
	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
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

type DirectReportStore interface {
	CreateCompletedReport(job Job, report analyzer.Report) error
}

type AnalyticsStore interface {
	AppendAnalyticsEvent(event analytics.Event) error
}

type EmailUnlockStore interface {
	CreateEmailUnlock(unlock EmailUnlock) error
	GetEmailUnlock(id string) (EmailUnlock, error)
	GetEmailUnlockByFullScanTokenHash(tokenHash string) (EmailUnlock, error)
	UpdateEmailUnlock(unlock EmailUnlock) error
}

type APIStore interface {
	UploadStore
	JobQueue
	ReportStore
}

type TokenUploadStore interface {
	CreateUploadSession(job Job) error
	StoreUploadSession(job Job, data []byte) (Job, error)
	FinalizeUploadSession(job Job) error
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
