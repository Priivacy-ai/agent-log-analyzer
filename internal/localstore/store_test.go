package localstore

import (
	"os"
	"testing"
	"time"
)

func TestSweepExpiredDeletesOldUploadsAndReports(t *testing.T) {
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

	result, err := store.SweepExpired(time.Now(), 15*time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("SweepExpired failed: %v", err)
	}
	if result.UploadsDeleted != 1 || result.ReportsDeleted != 1 {
		t.Fatalf("unexpected sweep result: %#v", result)
	}
	if _, err := os.Stat(uploadPath); !os.IsNotExist(err) {
		t.Fatalf("expected upload deleted, stat err: %v", err)
	}
	if _, err := os.Stat(store.root + "/reports/job-12345678.json"); !os.IsNotExist(err) {
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
