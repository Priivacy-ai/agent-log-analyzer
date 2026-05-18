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
	job        app.Job
	queueDepth int
	direct     app.DirectUpload
	finalized  string
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

func (f fakeStore) QueueDepth() (int, error) {
	return f.queueDepth, nil
}

func (f fakeStore) GetReport(id string) (analyzer.Report, error) {
	return analyzer.Report{}, errors.New("not implemented")
}

func (f fakeStore) CreateDirectUpload(jobID string, expiresIn time.Duration, maxBytes int64) (app.DirectUpload, error) {
	upload := f.direct
	upload.JobID = jobID
	upload.MaxBytes = maxBytes
	return upload, nil
}

func (f fakeStore) FinalizeDirectUpload(jobID string) error {
	if f.finalized != "" && f.finalized != jobID {
		return errors.New("unexpected job id")
	}
	return nil
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

func TestCreateJobHandlerRejectsWhenQueueIsBusyBeforeParsingUpload(t *testing.T) {
	store := fakeStore{queueDepth: 10}
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader("not multipart"))
	rec := httptest.NewRecorder()

	createJobHandler(store, 10).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before multipart parsing, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected Retry-After header, got %q", rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), "analysis queue is busy") {
		t.Fatalf("expected busy queue response, got %s", rec.Body.String())
	}
}

func TestMultipartUploadDisabledHandlerRejectsUpload(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader("not multipart"))
	rec := httptest.NewRecorder()

	multipartUploadDisabledHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected status 410, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "direct upload") {
		t.Fatalf("expected direct upload guidance, got %s", rec.Body.String())
	}
}

func TestCreateDirectUploadHandlerReturnsUploadSpec(t *testing.T) {
	store := fakeStore{
		direct: app.DirectUpload{
			Method:       "POST",
			URL:          "https://uploads.example.test",
			Fields:       map[string]string{"key": "uploads/job.log"},
			ExpiresAt:    time.Now().UTC().Add(15 * time.Minute),
			FinalizePath: "/api/jobs/job-1234567890/finalize",
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/upload-url", nil)
	rec := httptest.NewRecorder()

	createDirectUploadHandler(store, 100, 15*time.Minute).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var upload app.DirectUpload
	if err := json.Unmarshal(rec.Body.Bytes(), &upload); err != nil {
		t.Fatalf("response is not valid upload JSON: %v", err)
	}
	if upload.JobID == "" || upload.URL == "" || upload.Method != "POST" || upload.MaxBytes == 0 {
		t.Fatalf("unexpected upload response: %#v", upload)
	}
}

func TestFinalizeDirectUploadHandlerQueuesJob(t *testing.T) {
	store := fakeStore{finalized: "job-1234567890"}
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/job-1234567890/finalize", nil)
	req.SetPathValue("id", "job-1234567890")
	rec := httptest.NewRecorder()

	finalizeDirectUploadHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("expected pending response, got %s", rec.Body.String())
	}
}

func TestMultipartUploadsDisabledByDefaultForAWSBackend(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_BACKEND", "aws")
	t.Setenv("CLAUDE_ANALYZER_ENABLE_MULTIPART_UPLOADS", "")

	if multipartUploadsEnabled() {
		t.Fatal("expected multipart uploads disabled for aws backend")
	}
}

func TestMultipartUploadsCanBeExplicitlyEnabled(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_BACKEND", "aws")
	t.Setenv("CLAUDE_ANALYZER_ENABLE_MULTIPART_UPLOADS", "true")

	if !multipartUploadsEnabled() {
		t.Fatal("expected explicit flag to enable multipart uploads")
	}
}
