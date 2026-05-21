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
	ScanTypeFullScan   ScanType = "full_scan_bundle"
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
	WaiverAcceptedAt     time.Time `json:"waiver_accepted_at,omitempty"`
	ReportPath           string    `json:"report_path,omitempty"`
	Error                string    `json:"error,omitempty"`
	QueueReceipt         string    `json:"-"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	CompletedAt          time.Time `json:"completed_at,omitempty"`
}

type EmailUnlockStatus string

const (
	EmailUnlockPending   EmailUnlockStatus = "pending"
	EmailUnlockConfirmed EmailUnlockStatus = "confirmed"
	EmailUnlockUsed      EmailUnlockStatus = "used"
)

type EmailUnlock struct {
	ID                           string            `json:"id"`
	Email                        string            `json:"email"`
	EmailHash                    string            `json:"email_hash"`
	MarketingOptIn               bool              `json:"marketing_opt_in"`
	SourceReportJobID            string            `json:"source_report_job_id,omitempty"`
	ConfirmationTokenHash        string            `json:"confirmation_token_hash,omitempty"`
	FullScanTokenHash            string            `json:"full_scan_token_hash,omitempty"`
	FullScanTokenExpiresAt       time.Time         `json:"full_scan_token_expires_at,omitempty"`
	FullScanJobID                string            `json:"full_scan_job_id,omitempty"`
	Status                       EmailUnlockStatus `json:"status"`
	CreatedAt                    time.Time         `json:"created_at"`
	UpdatedAt                    time.Time         `json:"updated_at"`
	ConfirmedAt                  time.Time         `json:"confirmed_at,omitempty"`
	LastTransactionalEmailSentAt time.Time         `json:"last_transactional_email_sent_at,omitempty"`
}
