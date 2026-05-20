package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
	"github.com/priivacy-ai/agent-log-analyzer/internal/localstore"
)

func TestProcessOnceCompletesPaidBundleJob(t *testing.T) {
	store, err := localstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	job := app.Job{
		ID:                   "job-paid-worker",
		Status:               app.StatusUploading,
		ScanType:             app.ScanTypePaidBundle,
		MaxUploadBytes:       250 * 1024 * 1024,
		UploadTokenHash:      "unused",
		ReportTokenHash:      "unused",
		UploadTokenExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := store.CreateUploadSession(job); err != nil {
		t.Fatal(err)
	}
	job, err = store.StoreUploadSession(job, workerPaidBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinalizeUploadSession(job); err != nil {
		t.Fatal(err)
	}

	if err := processOnce(store); err != nil {
		t.Fatal(err)
	}
	completed, err := store.GetJob("job-paid-worker")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != app.StatusCompleted {
		t.Fatalf("expected completed job, got %#v", completed)
	}
	report, err := store.GetReport("job-paid-worker")
	if err != nil {
		t.Fatal(err)
	}
	if report.Metrics.SessionCount != 2 {
		t.Fatalf("expected aggregate paid report, got %#v", report.Metrics)
	}
	if report.AggregateEvent.ParserType != "paid_bundle" {
		t.Fatalf("expected paid bundle parser type, got %#v", report.AggregateEvent)
	}
}

func workerPaidBundle(t *testing.T) []byte {
	t.Helper()
	files := map[string]string{
		"logs/session-1.jsonl": `{"type":"tool","command":"cat src/auth.ts","output":"ok"}` + "\n",
		"logs/session-2.jsonl": `{"type":"tool","command":"go test ./...","error":"failed"}` + "\n",
	}
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		data := []byte(content)
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
