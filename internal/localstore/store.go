package localstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analytics"
	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

type Store struct {
	root string
}

func New(root string) (*Store, error) {
	if root == "" {
		root = "/tmp/agent-log-analyzer"
	}
	for _, dir := range []string{
		filepath.Join(root, "uploads"),
		filepath.Join(root, "jobs", "uploading"),
		filepath.Join(root, "jobs", "pending"),
		filepath.Join(root, "jobs", "processing"),
		filepath.Join(root, "jobs", "completed"),
		filepath.Join(root, "jobs", "failed"),
		filepath.Join(root, "reports"),
		filepath.Join(root, "analytics"),
		filepath.Join(root, "email_unlocks"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	return &Store{root: root}, nil
}

func (s *Store) CreateEmailUnlock(unlock app.EmailUnlock) error {
	if !validID(unlock.ID) {
		return errors.New("invalid email unlock id")
	}
	now := time.Now().UTC()
	if unlock.CreatedAt.IsZero() {
		unlock.CreatedAt = now
	}
	if unlock.UpdatedAt.IsZero() {
		unlock.UpdatedAt = now
	}
	if unlock.Status == "" {
		unlock.Status = app.EmailUnlockPending
	}
	return writeJSON(s.emailUnlockPath(unlock.ID), unlock)
}

func (s *Store) GetEmailUnlock(id string) (app.EmailUnlock, error) {
	var unlock app.EmailUnlock
	if !validID(id) {
		return unlock, errors.New("invalid email unlock id")
	}
	data, err := os.ReadFile(s.emailUnlockPath(id))
	if err != nil {
		return unlock, err
	}
	return unlock, json.Unmarshal(data, &unlock)
}

func (s *Store) GetEmailUnlockByFullScanTokenHash(tokenHash string) (app.EmailUnlock, error) {
	var unlock app.EmailUnlock
	if tokenHash == "" {
		return unlock, os.ErrNotExist
	}
	entries, err := os.ReadDir(filepath.Join(s.root, "email_unlocks"))
	if err != nil {
		return unlock, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		candidate, err := s.GetEmailUnlock(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		if candidate.FullScanTokenHash == tokenHash {
			return candidate, nil
		}
	}
	return unlock, os.ErrNotExist
}

func (s *Store) UpdateEmailUnlock(unlock app.EmailUnlock) error {
	if !validID(unlock.ID) {
		return errors.New("invalid email unlock id")
	}
	unlock.UpdatedAt = time.Now().UTC()
	return writeJSON(s.emailUnlockPath(unlock.ID), unlock)
}

func (s *Store) AppendAnalyticsEvent(event analytics.Event) error {
	data, err := analytics.MarshalJSONLine(event)
	if err != nil {
		return err
	}
	path := filepath.Join(s.root, "analytics", "events.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (s *Store) SaveUpload(jobID string, data []byte) (string, error) {
	path := filepath.Join(s.root, "uploads", jobID+".log")
	return path, os.WriteFile(path, data, 0o600)
}

func (s *Store) CreateUploadSession(job app.Job) error {
	job.Status = app.StatusUploading
	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	return s.writeJob("uploading", job)
}

func (s *Store) StoreUploadSession(job app.Job, data []byte) (app.Job, error) {
	uploadPath, err := s.SaveUpload(job.ID, data)
	if err != nil {
		return job, err
	}
	job.UploadPath = uploadPath
	job.UpdatedAt = time.Now().UTC()
	return job, s.writeJob("uploading", job)
}

func (s *Store) FinalizeUploadSession(job app.Job) error {
	if job.Status != app.StatusUploading {
		return nil
	}
	if job.UploadPath == "" {
		return errors.New("upload missing")
	}
	uploadingPath := s.jobPath("uploading", job.ID)
	job.Status = app.StatusPending
	job.UpdatedAt = time.Now().UTC()
	if err := s.writeJob("pending", job); err != nil {
		return err
	}
	return os.Remove(uploadingPath)
}

func (s *Store) CreateJob(job app.Job) error {
	job.Status = app.StatusPending
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	return s.writeJob("pending", job)
}

func (s *Store) ClaimNextJob() (app.Job, bool, error) {
	pending := filepath.Join(s.root, "jobs", "pending")
	entries, err := os.ReadDir(pending)
	if err != nil {
		return app.Job{}, false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		from := filepath.Join(pending, entry.Name())
		to := filepath.Join(s.root, "jobs", "processing", entry.Name())
		if err := os.Rename(from, to); err != nil {
			continue
		}
		job, err := readJob(to)
		if err != nil {
			return app.Job{}, false, err
		}
		job.Status = app.StatusProcessing
		job.UpdatedAt = time.Now().UTC()
		if err := writeJSON(to, job); err != nil {
			return app.Job{}, false, err
		}
		return job, true, nil
	}
	return app.Job{}, false, nil
}

func (s *Store) CompleteJob(job app.Job, report analyzer.Report) error {
	reportPath := filepath.Join(s.root, "reports", job.ID+".json")
	if err := writeJSON(reportPath, report); err != nil {
		return err
	}
	processingPath := s.jobPath("processing", job.ID)
	job.Status = app.StatusCompleted
	job.ReportPath = reportPath
	job.UpdatedAt = time.Now().UTC()
	job.CompletedAt = job.UpdatedAt
	if err := writeJSON(processingPath, job); err != nil {
		return err
	}
	return os.Rename(processingPath, s.jobPath("completed", job.ID))
}

func (s *Store) CreateCompletedReport(job app.Job, report analyzer.Report) error {
	reportPath := filepath.Join(s.root, "reports", job.ID+".json")
	if err := writeJSON(reportPath, report); err != nil {
		return err
	}
	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.Status = app.StatusCompleted
	job.ReportPath = reportPath
	job.UpdatedAt = now
	job.CompletedAt = now
	return s.writeJob("completed", job)
}

func (s *Store) FailJob(job app.Job, jobErr error) error {
	processingPath := s.jobPath("processing", job.ID)
	job.Status = app.StatusFailed
	job.Error = jobErr.Error()
	job.UpdatedAt = time.Now().UTC()
	if err := writeJSON(processingPath, job); err != nil {
		return err
	}
	return os.Rename(processingPath, s.jobPath("failed", job.ID))
}

func (s *Store) GetJob(id string) (app.Job, error) {
	if !validID(id) {
		return app.Job{}, errors.New("invalid job id")
	}
	for _, status := range []string{"uploading", "pending", "processing", "completed", "failed"} {
		path := s.jobPath(status, id)
		job, err := readJob(path)
		if err == nil {
			return job, nil
		}
	}
	return app.Job{}, os.ErrNotExist
}

func (s *Store) QueueDepth() (int, error) {
	total := 0
	for _, status := range []string{"pending", "processing"} {
		entries, err := os.ReadDir(filepath.Join(s.root, "jobs", status))
		if err != nil {
			return 0, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				total++
			}
		}
	}
	return total, nil
}

func (s *Store) GetReport(id string) (analyzer.Report, error) {
	if !validID(id) {
		return analyzer.Report{}, errors.New("invalid report id")
	}
	var report analyzer.Report
	data, err := os.ReadFile(filepath.Join(s.root, "reports", id+".json"))
	if err != nil {
		return report, err
	}
	return report, json.Unmarshal(data, &report)
}

func (s *Store) ReadUpload(path string) ([]byte, error) {
	cleanRoot, err := filepath.Abs(filepath.Join(s.root, "uploads"))
	if err != nil {
		return nil, err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return nil, errors.New("upload path outside store")
	}
	return os.ReadFile(cleanPath)
}

func (s *Store) SweepExpired(now time.Time, rawUploadTTL, reportTTL time.Duration) (app.SweepResult, error) {
	result := app.SweepResult{}
	if err := s.sweepDir(filepath.Join(s.root, "uploads"), now, rawUploadTTL, &result.UploadsDeleted); err != nil {
		return result, err
	}
	if err := s.sweepDir(filepath.Join(s.root, "reports"), now, reportTTL, &result.ReportsDeleted); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Store) sweepDir(dir string, now time.Time, ttl time.Duration, count *int) error {
	if ttl <= 0 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if now.Sub(info.ModTime()) <= ttl {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		*count++
	}
	return nil
}

func (s *Store) jobPath(status, id string) string {
	return filepath.Join(s.root, "jobs", status, id+".json")
}

func (s *Store) emailUnlockPath(id string) string {
	return filepath.Join(s.root, "email_unlocks", id+".json")
}

func (s *Store) writeJob(status string, job app.Job) error {
	if !validID(job.ID) {
		return errors.New("invalid job id")
	}
	return writeJSON(s.jobPath(status, job.ID), job)
}

func readJob(path string) (app.Job, error) {
	var job app.Job
	data, err := os.ReadFile(path)
	if err != nil {
		return job, err
	}
	return job, json.Unmarshal(data, &job)
}

func writeJSON(path string, value any) error {
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func validID(id string) bool {
	if len(id) < 8 || len(id) > 80 {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}
