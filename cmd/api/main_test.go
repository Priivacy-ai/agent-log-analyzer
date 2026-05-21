package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
	"github.com/priivacy-ai/agent-log-analyzer/internal/localstore"
)

type fakeStore struct {
	job        app.Job
	report     analyzer.Report
	queueDepth int
}

type captureEmailSender struct {
	messages []emailMessage
}

func (sender *captureEmailSender) Send(message emailMessage) error {
	sender.messages = append(sender.messages, message)
	return nil
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
	if id != f.job.ID {
		return analyzer.Report{}, errors.New("not found")
	}
	return f.report, nil
}

func TestSanitizePathRedactsDynamicIDs(t *testing.T) {
	for _, path := range []string{
		"/api/uploads/job-1234567890",
		"/api/uploads/job-1234567890/finalize",
		"/api/paid-uploads/job-1234567890",
		"/api/paid-uploads/job-1234567890/finalize",
		"/api/public-reports/job-1234567890/token-secret",
		"/api/public-artifacts/job-1234567890/token-secret/plugin.zip",
		"/api/jobs/job-1234567890",
		"/r/job-1234567890/token-secret",
	} {
		got := sanitizePath(path)
		if strings.Contains(got, "job-1234567890") || strings.Contains(got, "token-secret") {
			t.Fatalf("sanitizePath leaked job id for %q: %q", path, got)
		}
	}
}

func TestGetPublicArtifactRequiresPaidWaiver(t *testing.T) {
	store := fakeStore{
		job: app.Job{
			ID:              "job-1234567890",
			Status:          app.StatusCompleted,
			ScanType:        app.ScanTypeSingle,
			ReportTokenHash: tokenHash("report-token"),
		},
		report: analyzer.Report{JobID: "job-1234567890", Version: "test"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/public-artifacts/job-1234567890/report-token/plugin.zip", nil)
	req.SetPathValue("id", "job-1234567890")
	req.SetPathValue("token", "report-token")
	rec := httptest.NewRecorder()

	getPublicArtifactHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden artifact status, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetPublicArtifactReturnsPluginZipForEmailFullScan(t *testing.T) {
	store := fakeStore{
		job: app.Job{
			ID:              "job-1234567890",
			Status:          app.StatusCompleted,
			ScanType:        app.ScanTypeFullScan,
			ReportTokenHash: tokenHash("report-token"),
		},
		report: analyzer.Report{
			JobID:   "job-1234567890",
			Version: "test",
			Findings: []analyzer.Finding{
				{ID: "repeated_file_reads", Severity: "high", CostImpact: "high"},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/public-artifacts/job-1234567890/report-token/plugin.zip", nil)
	req.Host = "example.test"
	req.SetPathValue("id", "job-1234567890")
	req.SetPathValue("token", "report-token")
	rec := httptest.NewRecorder()

	getPublicArtifactHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected plugin zip status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("PK")) {
		t.Fatalf("expected zip response, got %q", rec.Body.String())
	}
}

func TestReportPageServerRendersCompletedReport(t *testing.T) {
	store := fakeStore{
		job: app.Job{
			ID:              "job-1234567890",
			Status:          app.StatusCompleted,
			ScanType:        app.ScanTypeSingle,
			ReportTokenHash: tokenHash("report-token"),
		},
		report: analyzer.Report{
			JobID:   "job-1234567890",
			Version: "test",
			Score:   42,
			EstimatedWaste: analyzer.WasteRange{
				Low:  22,
				High: 30,
			},
			Metrics: analyzer.Metrics{
				Turns:            12,
				EstimatedTokens:  12000,
				ToolOutputTokens: 4000,
			},
			Findings: []analyzer.Finding{{
				ID:             "tool_output_bloat",
				Title:          "Large shell/tool output overhead",
				Severity:       "high",
				CostImpact:     "high",
				Evidence:       analyzer.FindingEvidence{TokenShare: 68},
				Recommendation: "Cap command output.",
			}},
			Timeline: []analyzer.TimelinePoint{{
				Turn:            10,
				EstimatedTokens: 10000,
				ToolTokens:      3000,
				Rereads:         2,
				Retries:         1,
			}},
			SourceReports: []analyzer.SourceReport{
				{
					SourceID:       "claude_code",
					SourceLabel:    "Claude Code",
					LogCount:       1,
					Score:          42,
					EstimatedWaste: analyzer.WasteRange{Low: 22, High: 30},
					Metrics:        analyzer.Metrics{EstimatedTokens: 10000, ToolOutputTokens: 3000, Rereads: 2, FailedCommands: 1},
					Findings: []analyzer.Finding{{
						Title:      "Large shell/tool output overhead",
						Severity:   "high",
						CostImpact: "high",
					}},
					Timeline: []analyzer.TimelinePoint{{
						Turn:            10,
						EstimatedTokens: 10000,
						ToolTokens:      3000,
						Rereads:         2,
						Retries:         1,
					}},
				},
				{
					SourceID:       "codex",
					SourceLabel:    "Codex",
					LogCount:       1,
					Score:          55,
					EstimatedWaste: analyzer.WasteRange{Low: 12, High: 20},
					Metrics:        analyzer.Metrics{EstimatedTokens: 2000, ToolOutputTokens: 200},
					Timeline: []analyzer.TimelinePoint{{
						Turn:            5,
						EstimatedTokens: 2000,
						ToolTokens:      200,
					}},
				},
			},
			ImmediateFixes: []string{"Use narrower shell commands."},
			Ecosystem: analyzer.Ecosystem{
				Client:          "claude-code",
				OperatingSystem: "macos",
				MCPServersKnown: []string{"github"},
				ToolingUtilization: analyzer.ToolingUtilization{
					MCP: analyzer.MCPUtilization{
						WarningBand:              "high",
						UtilizationRatioPct:      10,
						ServerCountBucket:        "many",
						ContextTokenBucket:       "15k_50k",
						CallCount:                3,
						UniqueKnownCalledIDs:     []string{"github"},
						UniqueUnknownCalledCount: 1,
					},
				},
			},
			SecurityReceipt: analyzer.SecurityReceipt{
				SecretsRedacted: 3,
				RawLogTTL:       "not uploaded",
			},
			Recommendation: &analyzer.RecommendationSet{
				Primary: &analyzer.TokenSavingRecommendation{
					PrimaryToolID:   "rtk",
					PrimaryToolName: "RTK (Rust Token Killer, rtk-ai/rtk)",
					PrimaryToolURL:  "https://github.com/rtk-ai/rtk",
					Reason:          analyzer.ReasonAbsent,
					SignalIDs:       []analyzer.Signal{analyzer.SignalToolOutputBloat},
					Confidence:      analyzer.ConfidenceLow,
					RiskLevel:       analyzer.RiskHigh,
					InstallPolicy:   analyzer.PolicyRecommendWithWaiver,
				},
				RegistryVersion: analyzer.RegistryVersion(),
				EngineVersion:   analyzer.EngineVersion(),
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/r/job-1234567890/report-token", nil)
	req.SetPathValue("id", "job-1234567890")
	req.SetPathValue("token", "report-token")
	rec := httptest.NewRecorder()

	reportPageHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %q", contentType)
	}
	if cacheControl := rec.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store cache control, got %q", cacheControl)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"42",
		"Large shell/tool output overhead",
		"Use narrower shell commands.",
		"RTK (Rust Token Killer, rtk-ai/rtk)",
		"https://github.com/rtk-ai/rtk",
		"Raw log TTL: not uploaded",
		"MCP:",
		"Agent Logs Analyzed",
		"Claude Code",
		"Codex",
		"timeline-bar",
		"potential savings",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("server-rendered report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<table>") || strings.Contains(body, "<th>Turn</th>") {
		t.Fatalf("server-rendered report regressed to tabular timeline: %s", body)
	}
	if strings.Contains(body, "Find out what&#39;s wasting your Claude Code tokens") || strings.Contains(body, "Run the local analyzer") {
		t.Fatalf("server-rendered report returned onboarding shell instead of report: %s", body)
	}
}

func TestReportPageRequiresReportToken(t *testing.T) {
	store := fakeStore{
		job: app.Job{
			ID:              "job-1234567890",
			Status:          app.StatusCompleted,
			ReportTokenHash: tokenHash("report-token"),
		},
		report: analyzer.Report{JobID: "job-1234567890", Version: "test"},
	}
	req := httptest.NewRequest(http.MethodGet, "/r/job-1234567890/wrong-token", nil)
	req.SetPathValue("id", "job-1234567890")
	req.SetPathValue("token", "wrong-token")
	rec := httptest.NewRecorder()

	reportPageHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetPublicArtifactReturnsPluginZipForPaidWaiver(t *testing.T) {
	store := fakeStore{
		job: app.Job{
			ID:                   "job-1234567890",
			Status:               app.StatusCompleted,
			ScanType:             app.ScanTypePaidBundle,
			ReportTokenHash:      tokenHash("report-token"),
			WaiverAcceptedAt:     time.Now().UTC(),
			UploadTokenExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		},
		report: analyzer.Report{
			JobID:   "job-1234567890",
			Version: "test",
			Findings: []analyzer.Finding{
				{ID: "repeated_file_reads", Severity: "high", CostImpact: "high"},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/public-artifacts/job-1234567890/report-token/plugin.zip", nil)
	req.Host = "example.test"
	req.SetPathValue("id", "job-1234567890")
	req.SetPathValue("token", "report-token")
	rec := httptest.NewRecorder()

	getPublicArtifactHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected plugin zip status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/zip" {
		t.Fatalf("expected zip content type, got %q", contentType)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("PK")) {
		t.Fatalf("expected zip response, got %q", rec.Body.String())
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

func TestClientReportUploadStoresSanitizedReportOnly(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report := analyzer.Report{
		JobID:   "local",
		Version: analyzer.Version,
		Score:   72,
		Metrics: analyzer.Metrics{Turns: 12, EstimatedTokens: 2400},
		SecurityReceipt: analyzer.SecurityReceipt{
			RawTranscriptSentToLLM: false,
			OutboundDuringAnalysis: false,
			SecretsRedacted:        2,
			RawLogTTL:              "not uploaded",
		},
		AggregateEvent: analyzer.AggregateSafeEvent{Event: "analysis_completed", ParserType: "jsonl"},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/client-reports", bytes.NewReader(body))
	req.Host = "example.test"
	rec := httptest.NewRecorder()

	createClientReportHandler(store, 15*time.Minute).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected client report status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var session analysisSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("response is not valid session JSON: %v", err)
	}
	if session.Token != "" || session.UploadPath != "" || !strings.Contains(session.ReportURL, "/r/") {
		t.Fatalf("expected report-only response, got %#v", session)
	}
	stored, err := store.GetReport(session.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.JobID != session.JobID || stored.SecurityReceipt.RawLogTTL != "not uploaded" {
		t.Fatalf("stored report mismatch: %#v", stored)
	}
	job, err := store.GetJob(session.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != app.StatusCompleted || job.UploadPath != "" || job.UploadTokenHash != "" {
		t.Fatalf("expected completed report-only job, got %#v", job)
	}
}

func TestClientReportUploadRejectsUnsafeReceipt(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report := analyzer.Report{
		Version: analyzer.Version,
		Metrics: analyzer.Metrics{Turns: 1},
		SecurityReceipt: analyzer.SecurityReceipt{
			RawTranscriptSentToLLM: true,
		},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/client-reports", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	createClientReportHandler(store, 15*time.Minute).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected unsafe report status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPaidClientReportUploadStoresSanitizedAggregateAndArtifactWorks(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_ENABLE_LOCAL_PAID_SESSIONS", "true")
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report := paidAggregateReport()
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/paid-client-reports", bytes.NewReader(body))
	req.Host = "example.test"
	req.Header.Set("X-Waiver-Accepted", "true")
	req.Header.Set("X-Waiver-Acknowledgment", "I accept at my own risk")
	rec := httptest.NewRecorder()

	createPaidClientReportHandler(store, 15*time.Minute).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected paid report status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var session analysisSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("response is not valid session JSON: %v", err)
	}
	if session.Token != "" || session.UploadPath != "" || !strings.Contains(session.ReportURL, "/r/") {
		t.Fatalf("expected report-only paid response, got %#v", session)
	}
	job, err := store.GetJob(session.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.ScanType != app.ScanTypePaidBundle || job.Status != app.StatusCompleted || job.UploadPath != "" || job.WaiverAcceptedAt.IsZero() {
		t.Fatalf("expected completed waiver-gated paid report job without upload path, got %#v", job)
	}
	stored, err := store.GetReport(session.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AggregateEvent.ParserType != "paid_bundle" || stored.SecurityReceipt.RawLogTTL != "not uploaded" {
		t.Fatalf("stored paid report is not sanitized aggregate: %#v", stored)
	}

	artifactReq := httptest.NewRequest(http.MethodGet, "/api/public-artifacts/"+session.JobID+"/token/plugin.zip", nil)
	artifactReq.Host = "example.test"
	artifactReq.SetPathValue("id", session.JobID)
	artifactReq.SetPathValue("token", strings.TrimPrefix(session.ReportPath, "/r/"+session.JobID+"/"))
	artifactRec := httptest.NewRecorder()
	getPublicArtifactHandler(store).ServeHTTP(artifactRec, artifactReq)

	if artifactRec.Code != http.StatusOK {
		t.Fatalf("expected plugin zip from sanitized paid report, got %d: %s", artifactRec.Code, artifactRec.Body.String())
	}
	if !bytes.HasPrefix(artifactRec.Body.Bytes(), []byte("PK")) {
		t.Fatalf("expected zip response")
	}
}

func TestPaidClientReportUploadRejectsNonAggregateReport(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_ENABLE_LOCAL_PAID_SESSIONS", "true")
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report := paidAggregateReport()
	report.AggregateEvent.ParserType = "jsonl"
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/paid-client-reports", bytes.NewReader(body))
	req.Header.Set("X-Waiver-Accepted", "true")
	req.Header.Set("X-Waiver-Acknowledgment", "I accept at my own risk")
	rec := httptest.NewRecorder()

	createPaidClientReportHandler(store, 15*time.Minute).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected non-aggregate paid report rejection 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEmailUnlockConfirmAndFullScanUploadLifecycle(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sender := &captureEmailSender{}
	sourceReport := analyzer.Report{
		JobID:   "source-job",
		Version: analyzer.Version,
		Score:   72,
		Metrics: analyzer.Metrics{Turns: 12, EstimatedTokens: 2400},
		SecurityReceipt: analyzer.SecurityReceipt{
			RawTranscriptSentToLLM: false,
			OutboundDuringAnalysis: false,
			RawLogTTL:              "not uploaded",
		},
	}
	sourceJob := app.Job{
		ID:              "job-source-1234",
		Status:          app.StatusCompleted,
		ScanType:        app.ScanTypeSingle,
		ReportTokenHash: tokenHash("source-token"),
	}
	if err := store.CreateCompletedReport(sourceJob, sourceReport); err != nil {
		t.Fatal(err)
	}

	form := "email=dev%40example.com&marketing_opt_in=1&source_report_job_id=job-source-1234&source_report_token=source-token"
	req := httptest.NewRequest(http.MethodPost, "/api/email-unlocks", strings.NewReader(form))
	req.Host = "example.test"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	createEmailUnlockHandler(store, sender).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected email unlock page status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0].Body, "/email/confirm/") {
		t.Fatalf("expected confirmation email, got %#v", sender.messages)
	}
	confirmPath := extractPathFromEmail(sender.messages[0].Body, "/email/confirm/")
	parts := strings.Split(confirmPath, "/")
	if len(parts) < 5 {
		t.Fatalf("could not parse confirm path %q", confirmPath)
	}
	confirmReq := httptest.NewRequest(http.MethodGet, confirmPath, nil)
	confirmReq.Host = "example.test"
	confirmReq.SetPathValue("id", parts[3])
	confirmReq.SetPathValue("token", parts[4])
	confirmRec := httptest.NewRecorder()
	confirmEmailUnlockHandler(store, sender).ServeHTTP(confirmRec, confirmReq)

	if confirmRec.Code != http.StatusOK {
		t.Fatalf("expected confirm status 200, got %d: %s", confirmRec.Code, confirmRec.Body.String())
	}
	if !strings.Contains(confirmRec.Body.String(), "npx --yes agent-analyzer@latest full-scan --token") {
		t.Fatalf("confirmation page missing full-scan command: %s", confirmRec.Body.String())
	}
	if len(sender.messages) != 2 || !strings.Contains(sender.messages[1].Body, "full-scan --token") {
		t.Fatalf("expected full-scan command email, got %#v", sender.messages)
	}
	fullScanToken := extractTokenAfter(sender.messages[1].Body, "--token '")
	fullScanReport := paidAggregateReport()
	fullScanReport.AggregateEvent.ParserType = "full_scan_bundle"
	body, err := json.Marshal(fullScanReport)
	if err != nil {
		t.Fatal(err)
	}
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/full-scan-client-reports", bytes.NewReader(body))
	uploadReq.Host = "example.test"
	uploadReq.Header.Set("Authorization", "Bearer "+fullScanToken)
	uploadReq.Header.Set("Content-Type", "application/json")
	uploadRec := httptest.NewRecorder()
	createFullScanClientReportHandler(store, sender, 15*time.Minute).ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected full-scan upload status 201, got %d: %s", uploadRec.Code, uploadRec.Body.String())
	}
	var response analysisSessionResponse
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("response is not valid session JSON: %v", err)
	}
	job, err := store.GetJob(response.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.ScanType != app.ScanTypeFullScan || job.Status != app.StatusCompleted {
		t.Fatalf("expected completed full-scan job, got %#v", job)
	}
	if len(sender.messages) != 3 || !strings.Contains(sender.messages[2].Body, "/api/public-artifacts/") {
		t.Fatalf("expected plugin-ready email, got %#v", sender.messages)
	}
	reuseReq := httptest.NewRequest(http.MethodPost, "/api/full-scan-client-reports", bytes.NewReader(body))
	reuseReq.Header.Set("Authorization", "Bearer "+fullScanToken)
	reuseReq.Header.Set("Content-Type", "application/json")
	reuseRec := httptest.NewRecorder()
	createFullScanClientReportHandler(store, sender, 15*time.Minute).ServeHTTP(reuseRec, reuseReq)
	if reuseRec.Code != http.StatusConflict {
		t.Fatalf("expected reused token conflict, got %d: %s", reuseRec.Code, reuseRec.Body.String())
	}
}

func extractPathFromEmail(body, marker string) string {
	index := strings.Index(body, marker)
	if index < 0 {
		return ""
	}
	end := index
	for end < len(body) && body[end] != '\n' && body[end] != '\r' && body[end] != ' ' {
		end++
	}
	return body[index:end]
}

func extractTokenAfter(body, marker string) string {
	index := strings.Index(body, marker)
	if index < 0 {
		return ""
	}
	index += len(marker)
	end := index
	for end < len(body) && body[end] != '\'' && body[end] != '\n' && body[end] != '\r' {
		end++
	}
	return body[index:end]
}

func TestPaidBundleUploadRequiresPaidTokenAndLimit(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token := "paid-token"
	job := app.Job{
		ID:                   "job-paid-token",
		Status:               app.StatusUploading,
		ScanType:             app.ScanTypePaidBundle,
		MaxUploadBytes:       maxPaidUploadBytes,
		UploadTokenHash:      tokenHash(token),
		ReportTokenHash:      tokenHash("report-token"),
		UploadTokenExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := store.CreateUploadSession(job); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/paid-uploads/job-paid-token?limit=100", bytes.NewReader(testPaidBundle(t)))
	req.SetPathValue("id", "job-paid-token")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("X-Scan-Limit", "100")
	rec := httptest.NewRecorder()
	paidBundleUploadHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected paid upload status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	finalizeReq := httptest.NewRequest(http.MethodPost, "/api/paid-uploads/job-paid-token/finalize", nil)
	finalizeReq.SetPathValue("id", "job-paid-token")
	finalizeReq.Header.Set("Authorization", "Bearer "+token)
	finalizeRec := httptest.NewRecorder()
	finalizeTokenUploadHandler(store).ServeHTTP(finalizeRec, finalizeReq)

	if finalizeRec.Code != http.StatusAccepted {
		t.Fatalf("expected finalize status 202, got %d: %s", finalizeRec.Code, finalizeRec.Body.String())
	}
	stored, err := store.GetJob("job-paid-token")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != app.StatusPending || stored.ScanType != app.ScanTypePaidBundle {
		t.Fatalf("expected pending paid bundle job, got %#v", stored)
	}
}

func TestCreatePaidSessionRequiresLocalEnablementAndWaiver(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	disabledReq := httptest.NewRequest(http.MethodPost, "/api/paid-sessions", strings.NewReader(`{"waiver_accepted":true,"acknowledgment":"at my own risk"}`))
	disabledRec := httptest.NewRecorder()
	createPaidSessionHandler(store, 15*time.Minute).ServeHTTP(disabledRec, disabledReq)
	if disabledRec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected disabled paid session status 402, got %d: %s", disabledRec.Code, disabledRec.Body.String())
	}

	t.Setenv("CLAUDE_ANALYZER_ENABLE_LOCAL_PAID_SESSIONS", "true")
	missingWaiverReq := httptest.NewRequest(http.MethodPost, "/api/paid-sessions", strings.NewReader(`{"waiver_accepted":false}`))
	missingWaiverRec := httptest.NewRecorder()
	createPaidSessionHandler(store, 15*time.Minute).ServeHTTP(missingWaiverRec, missingWaiverReq)
	if missingWaiverRec.Code != http.StatusBadRequest {
		t.Fatalf("expected missing waiver status 400, got %d: %s", missingWaiverRec.Code, missingWaiverRec.Body.String())
	}
}

func TestCreatePaidSessionReturnsLocalFirstPaidCommandByDefault(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_ENABLE_LOCAL_PAID_SESSIONS", "true")
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/paid-sessions", strings.NewReader(`{"waiver_accepted":true,"acknowledgment":"I accept at my own risk"}`))
	req.Host = "example.test"
	rec := httptest.NewRecorder()

	createPaidSessionHandler(store, 15*time.Minute).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected paid session status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var session analysisSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("response is not valid session JSON: %v", err)
	}
	for _, forbidden := range []string{"/api/paid-uploads/", "tar -C", "application/gzip", "raw bundle", "bundles my 100"} {
		if strings.Contains(session.Command, forbidden) || strings.Contains(session.Prompt, forbidden) || strings.Contains(session.UploadPath, forbidden) {
			t.Fatalf("public paid session exposed raw upload instruction %q: %#v", forbidden, session)
		}
	}
	for _, want := range []string{"npx --yes agent-analyzer@latest analyze --paid --limit 100", "/api/paid-client-reports", "sanitized aggregate", "does not upload raw logs"} {
		if !strings.Contains(session.Command, want) && !strings.Contains(session.Prompt, want) {
			t.Fatalf("paid local-first session missing %q: %#v", want, session)
		}
	}
	if session.Token != "" || session.JobID != "" || session.FinalizePath != "" {
		t.Fatalf("public local-first paid session should not mint a raw upload job/token: %#v", session)
	}
}

func TestCreatePaidSessionReturnsLegacyPaidBundleOnlyWhenExplicit(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_ENABLE_LOCAL_PAID_SESSIONS", "true")
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/paid-sessions?legacy_raw_bundle=1", strings.NewReader(`{"waiver_accepted":true,"acknowledgment":"I accept at my own risk"}`))
	req.Host = "example.test"
	rec := httptest.NewRecorder()

	createPaidSessionHandler(store, 15*time.Minute).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected paid session status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var session analysisSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("response is not valid session JSON: %v", err)
	}
	if !strings.Contains(session.UploadPath, "/api/paid-uploads/") || !strings.Contains(session.Command, "limit=100") || !strings.Contains(session.Command, "X-Scan-Limit: 100") {
		t.Fatalf("expected explicit legacy paid bundle command, got %#v", session)
	}
	job, err := store.GetJob(session.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.ScanType != app.ScanTypePaidBundle || job.WaiverAcceptedAt.IsZero() {
		t.Fatalf("expected waiver-gated paid bundle job, got %#v", job)
	}
}

func TestPaidBundleUploadRejectsFreeToken(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token := "free-token"
	job := app.Job{
		ID:                   "job-free-token",
		Status:               app.StatusUploading,
		ScanType:             app.ScanTypeSingle,
		MaxUploadBytes:       maxUploadBytes,
		UploadTokenHash:      tokenHash(token),
		ReportTokenHash:      tokenHash("report-token"),
		UploadTokenExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := store.CreateUploadSession(job); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/paid-uploads/job-free-token?limit=100", bytes.NewReader(testPaidBundle(t)))
	req.SetPathValue("id", "job-free-token")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("X-Scan-Limit", "100")
	rec := httptest.NewRecorder()
	paidBundleUploadHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected free token rejection 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPaidBundleUploadRequiresScanLimitContract(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token := "paid-token"
	job := app.Job{
		ID:                   "job-paid-limit",
		Status:               app.StatusUploading,
		ScanType:             app.ScanTypePaidBundle,
		MaxUploadBytes:       maxPaidUploadBytes,
		UploadTokenHash:      tokenHash(token),
		ReportTokenHash:      tokenHash("report-token"),
		UploadTokenExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := store.CreateUploadSession(job); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/paid-uploads/job-paid-limit", bytes.NewReader(testPaidBundle(t)))
	req.SetPathValue("id", "job-paid-limit")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/gzip")
	rec := httptest.NewRecorder()
	paidBundleUploadHandler(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected missing limit rejection 400, got %d: %s", rec.Code, rec.Body.String())
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

func testPaidBundle(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)
	data := []byte(`{"type":"tool","command":"cat src/auth.ts","output":"ok"}` + "\n")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "logs/session.jsonl", Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func paidAggregateReport() analyzer.Report {
	return analyzer.Report{
		JobID:   "local-paid",
		Version: analyzer.Version,
		Score:   68,
		Metrics: analyzer.Metrics{Turns: 12, SessionCount: 2, EstimatedTokens: 2400},
		SecurityReceipt: analyzer.SecurityReceipt{
			RawTranscriptSentToLLM: false,
			OutboundDuringAnalysis: false,
			SecretsRedacted:        1,
			RawLogTTL:              "not uploaded",
		},
		AggregateEvent: analyzer.AggregateSafeEvent{
			Event:      "analysis_completed",
			ParserType: "paid_bundle",
			Findings:   map[string]string{"repeated_file_reads": "high"},
		},
		Findings: []analyzer.Finding{
			{ID: "repeated_file_reads", Severity: "high", CostImpact: "medium-high", Evidence: analyzer.FindingEvidence{Count: 6}},
		},
	}
}
