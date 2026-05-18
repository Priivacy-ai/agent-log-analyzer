package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/robertDouglass/claude-log-analyzer/internal/analyzer"
	"github.com/robertDouglass/claude-log-analyzer/internal/app"
)

type fakeStore struct {
	job app.Job
}

func (f fakeStore) SaveUpload(jobID string, data []byte) (string, error) {
	return "", errors.New("not implemented")
}

func (f fakeStore) ReadUpload(path string) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (f fakeStore) CreateJob(job app.Job) error {
	return errors.New("not implemented")
}

func (f fakeStore) ClaimNextJob() (app.Job, bool, error) {
	return app.Job{}, false, errors.New("not implemented")
}

func (f fakeStore) CompleteJob(job app.Job, report analyzer.Report) error {
	return errors.New("not implemented")
}

func (f fakeStore) FailJob(job app.Job, jobErr error) error {
	return errors.New("not implemented")
}

func (f fakeStore) GetJob(id string) (app.Job, error) {
	if id != f.job.ID {
		return app.Job{}, errors.New("not found")
	}
	return f.job, nil
}

func (f fakeStore) GetReport(id string) (analyzer.Report, error) {
	return analyzer.Report{}, errors.New("not implemented")
}

func TestSanitizePathRedactsDynamicIDs(t *testing.T) {
	for _, path := range []string{
		"/api/jobs/job-1234567890",
		"/api/reports/job-1234567890",
	} {
		got := sanitizePath(path)
		if strings.Contains(got, "job-1234567890") {
			t.Fatalf("sanitizePath leaked job id for %q: %q", path, got)
		}
	}
}

func TestGetJobHandlerDoesNotReturnStoragePaths(t *testing.T) {
	store := fakeStore{
		job: app.Job{
			ID:         "job-1234567890",
			Status:     app.StatusCompleted,
			UploadPath: "s3://private-upload-bucket/uploads/job-1234567890.log",
			ReportPath: "s3://private-report-bucket/reports/job-1234567890.json",
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/job-1234567890", nil)
	req.SetPathValue("id", "job-1234567890")
	rec := httptest.NewRecorder()

	getJobHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "private-upload-bucket") || strings.Contains(rec.Body.String(), "private-report-bucket") {
		t.Fatalf("job response leaked storage path: %s", rec.Body.String())
	}
	var job app.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatalf("response is not valid job JSON: %v", err)
	}
	if job.UploadPath != "" || job.ReportPath != "" {
		t.Fatalf("expected paths stripped from response: %#v", job)
	}
}
