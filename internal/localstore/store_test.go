package localstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analytics"
)

func TestSweepExpiredDeletesOldUploadsButKeepsReportsWhenReportTTLDisabled(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	uploadPath, err := store.SaveUpload("job-12345678", []byte("secret raw log"))
	if err != nil {
		t.Fatalf("SaveUpload failed: %v", err)
	}
	reportPath := store.jobPath("completed", "job-12345678")
	if err := writeJSON(reportPath, map[string]string{"status": "completed"}); err != nil {
		t.Fatalf("write completed job failed: %v", err)
	}
	if err := writeJSON(store.root+"/reports/job-12345678.json", map[string]string{"report": "ok"}); err != nil {
		t.Fatalf("write report failed: %v", err)
	}

	old := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(uploadPath, old, old); err != nil {
		t.Fatalf("Chtimes upload failed: %v", err)
	}
	if err := os.Chtimes(store.root+"/reports/job-12345678.json", old, old); err != nil {
		t.Fatalf("Chtimes report failed: %v", err)
	}

	result, err := store.SweepExpired(time.Now(), 15*time.Minute, 0)
	if err != nil {
		t.Fatalf("SweepExpired failed: %v", err)
	}
	if result.UploadsDeleted != 1 || result.ReportsDeleted != 0 {
		t.Fatalf("unexpected sweep result: %#v", result)
	}
	if _, err := os.Stat(uploadPath); !os.IsNotExist(err) {
		t.Fatalf("expected upload deleted, stat err: %v", err)
	}
	if _, err := os.Stat(store.root + "/reports/job-12345678.json"); err != nil {
		t.Fatalf("expected report retained, stat err: %v", err)
	}
}

func TestSweepExpiredDeletesReportsWhenReportTTLConfigured(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	reportPath := store.root + "/reports/job-12345678.json"
	if err := writeJSON(reportPath, map[string]string{"report": "ok"}); err != nil {
		t.Fatalf("write report failed: %v", err)
	}
	old := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(reportPath, old, old); err != nil {
		t.Fatalf("Chtimes report failed: %v", err)
	}

	result, err := store.SweepExpired(time.Now(), 15*time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("SweepExpired failed: %v", err)
	}
	if result.ReportsDeleted != 1 {
		t.Fatalf("unexpected sweep result: %#v", result)
	}
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("expected report deleted, stat err: %v", err)
	}
}

func TestSweepExpiredKeepsFreshFiles(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	uploadPath, err := store.SaveUpload("job-12345678", []byte("fresh raw log"))
	if err != nil {
		t.Fatalf("SaveUpload failed: %v", err)
	}
	result, err := store.SweepExpired(time.Now(), 15*time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("SweepExpired failed: %v", err)
	}
	if result.UploadsDeleted != 0 || result.ReportsDeleted != 0 {
		t.Fatalf("unexpected sweep result: %#v", result)
	}
	if _, err := os.Stat(uploadPath); err != nil {
		t.Fatalf("expected upload retained, stat err: %v", err)
	}
}

func TestAppendAnalyticsEventWritesJSONLWithoutReportIdentifiers(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	event := analytics.Event{
		SchemaVersion: analytics.SchemaVersion,
		Event:         "analytics.report",
		ScanType:      "free",
		Ecosystem: analytics.EcosystemEvent{
			Client:          "claude_code",
			OperatingSystem: "macos",
		},
	}
	if err := store.AppendAnalyticsEvent(event); err != nil {
		t.Fatalf("AppendAnalyticsEvent failed: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(store.root, "analytics", "events.jsonl"))
	if err != nil {
		t.Fatalf("read analytics JSONL: %v", err)
	}
	if !strings.Contains(string(body), `"analytics.report"`) {
		t.Fatalf("analytics JSONL missing event payload: %s", body)
	}
	for _, forbidden := range []string{`"job_id"`, `"report_path"`, `"upload_path"`} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("analytics JSONL leaked forbidden field %s: %s", forbidden, body)
		}
	}
}
