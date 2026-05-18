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
	"github.com/robertDouglass/claude-log-analyzer/internal/localstore"
)

type fakeStore struct {
	job        app.Job
	queueDepth int
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

func TestSanitizePathRedactsDynamicIDs(t *testing.T) {
	for _, path := range []string{
		"/api/uploads/job-1234567890",
		"/api/uploads/job-1234567890/finalize",
		"/api/public-reports/job-1234567890/token-secret",
		"/api/jobs/job-1234567890",
		"/r/job-1234567890/token-secret",
	} {
		got := sanitizePath(path)
		if strings.Contains(got, "job-1234567890") || strings.Contains(got, "token-secret") {
			t.Fatalf("sanitizePath leaked job id for %q: %q", path, got)
		}
	}
}

func TestGetJobHandlerDoesNotReturnStoragePaths(t *testing.T) {
	store := fakeStore{
		job: app.Job{
			ID:              "job-1234567890",
			Status:          app.StatusCompleted,
			UploadPath:      "s3://private-upload-bucket/uploads/job-1234567890.log",
			ReportPath:      "s3://private-report-bucket/reports/job-1234567890.json",
			UploadTokenHash: "private-upload-token-hash",
			ReportTokenHash: "private-report-token-hash",
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
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
	if job.UploadTokenHash != "" || job.ReportTokenHash != "" {
		t.Fatalf("expected token hashes stripped from response: %#v", job)
	}
}

func TestLegacyUploadRoutesAreNotMounted(t *testing.T) {
	store := fakeStore{job: app.Job{ID: "job-1234567890", Status: app.StatusCompleted}}
	mux := buildMux(store)
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/jobs"},
		{http.MethodPost, "/api/upload-url"},
		{http.MethodPost, "/api/jobs/job-1234567890/finalize"},
		{http.MethodGet, "/api/reports/job-1234567890"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s should not be mounted, got status %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestAnalysisSessionCurlFlow(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/analysis-sessions", nil)
	req.Host = "example.test"
	rec := httptest.NewRecorder()

	createAnalysisSessionHandler(store, 100, 15*time.Minute).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var session analysisSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("response is not valid session JSON: %v", err)
	}
	if session.JobID == "" || session.Token == "" || session.Command == "" || session.Prompt == "" {
		t.Fatalf("unexpected session response: %#v", session)
	}
	if strings.Contains(session.Command, ".log?X-Amz") {
		t.Fatalf("expected token curl command, got signed URL command: %s", session.Command)
	}

	uploadReq := httptest.NewRequest(http.MethodPut, "/api/uploads/"+session.JobID, strings.NewReader("log line"))
	uploadReq.SetPathValue("id", session.JobID)
	uploadReq.Header.Set("Authorization", "Bearer "+session.Token)
	uploadRec := httptest.NewRecorder()
	tokenUploadHandler(store).ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected upload status 201, got %d: %s", uploadRec.Code, uploadRec.Body.String())
	}

	finalizeReq := httptest.NewRequest(http.MethodPost, "/api/uploads/"+session.JobID+"/finalize", nil)
	finalizeReq.SetPathValue("id", session.JobID)
	finalizeReq.Header.Set("Authorization", "Bearer "+session.Token)
	finalizeRec := httptest.NewRecorder()
	finalizeTokenUploadHandler(store).ServeHTTP(finalizeRec, finalizeReq)

	if finalizeRec.Code != http.StatusAccepted {
		t.Fatalf("expected finalize status 202, got %d: %s", finalizeRec.Code, finalizeRec.Body.String())
	}
	job, err := store.GetJob(session.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != app.StatusPending {
		t.Fatalf("expected pending job, got %#v", job)
	}

	secondUpload := httptest.NewRequest(http.MethodPut, "/api/uploads/"+session.JobID, strings.NewReader("again"))
	secondUpload.SetPathValue("id", session.JobID)
	secondUpload.Header.Set("Authorization", "Bearer "+session.Token)
	secondRec := httptest.NewRecorder()
	tokenUploadHandler(store).ServeHTTP(secondRec, secondUpload)

	if secondRec.Code != http.StatusConflict {
		t.Fatalf("expected reused upload status 409, got %d: %s", secondRec.Code, secondRec.Body.String())
	}
}

func TestTokenUploadRejectsExpiredToken(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token := "expired-token"
	job := app.Job{
		ID:                   "job-expired-token",
		Status:               app.StatusUploading,
		MaxUploadBytes:       maxUploadBytes,
		UploadTokenHash:      tokenHash(token),
		ReportTokenHash:      tokenHash("report-token"),
		UploadTokenExpiresAt: time.Now().UTC().Add(-time.Minute),
	}
	if err := store.CreateUploadSession(job); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/uploads/job-expired-token", strings.NewReader("log line"))
	req.SetPathValue("id", "job-expired-token")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	tokenUploadHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected expired token status 410, got %d: %s", rec.Code, rec.Body.String())
	}
}
