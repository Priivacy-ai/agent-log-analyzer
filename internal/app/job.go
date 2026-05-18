package app

import "time"

type JobStatus string
type ScanType string

const (
	StatusUploading  JobStatus = "uploading"
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

const (
	ScanTypeSingle     ScanType = "single"
	ScanTypePaidBundle ScanType = "paid_bundle"
)

type Job struct {
	ID                   string    `json:"id"`
	Status               JobStatus `json:"status"`
	ScanType             ScanType  `json:"scan_type,omitempty"`
	UploadPath           string    `json:"upload_path"`
	MaxUploadBytes       int64     `json:"max_upload_bytes,omitempty"`
	UploadTokenHash      string    `json:"upload_token_hash,omitempty"`
	ReportTokenHash      string    `json:"report_token_hash,omitempty"`
	UploadTokenExpiresAt time.Time `json:"upload_token_expires_at,omitempty"`
	ReportPath           string    `json:"report_path,omitempty"`
	Error                string    `json:"error,omitempty"`
	QueueReceipt         string    `json:"-"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	CompletedAt          time.Time `json:"completed_at,omitempty"`
}
