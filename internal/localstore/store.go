package localstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robertDouglass/claude-log-analyzer/internal/analyzer"
	"github.com/robertDouglass/claude-log-analyzer/internal/app"
)

type Store struct {
	root string
}

func New(root string) (*Store, error) {
	if root == "" {
		root = "/tmp/claude-log-analyzer"
	}
	for _, dir := range []string{
		filepath.Join(root, "uploads"),
		filepath.Join(root, "jobs", "pending"),
		filepath.Join(root, "jobs", "processing"),
		filepath.Join(root, "jobs", "completed"),
		filepath.Join(root, "jobs", "failed"),
		filepath.Join(root, "reports"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	return &Store{root: root}, nil
}

func (s *Store) SaveUpload(jobID string, data []byte) (string, error) {
	path := filepath.Join(s.root, "uploads", jobID+".log")
	return path, os.WriteFile(path, data, 0o600)
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
	for _, status := range []string{"pending", "processing", "completed", "failed"} {
		path := s.jobPath(status, id)
		job, err := readJob(path)
		if err == nil {
			return job, nil
		}
	}
	return app.Job{}, os.ErrNotExist
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

func (s *Store) SweepExpired(now time.Time, rawUploadTTL, reportTTL time.Duration) (SweepResult, error) {
	result := SweepResult{}
	if err := s.sweepDir(filepath.Join(s.root, "uploads"), now, rawUploadTTL, &result.UploadsDeleted); err != nil {
		return result, err
	}
	if err := s.sweepDir(filepath.Join(s.root, "reports"), now, reportTTL, &result.ReportsDeleted); err != nil {
		return result, err
	}
	return result, nil
}

type SweepResult struct {
	UploadsDeleted int `json:"uploads_deleted"`
	ReportsDeleted int `json:"reports_deleted"`
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
