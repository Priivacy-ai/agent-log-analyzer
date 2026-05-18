package paidscan

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractAcceptsBoundedJSONLBundle(t *testing.T) {
	bundle := makeBundle(t, map[string]string{
		".claude/projects/a/session-1.jsonl": `{"type":"tool","command":"cat src/auth.ts","output":"ok"}` + "\n",
		".claude/projects/b/session-2.jsonl": `{"type":"tool","command":"go test ./...","error":"failed"}` + "\n",
	})

	entries, err := Extract(bundle, Options{MaxFiles: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Path == "" || !strings.HasSuffix(entries[0].Path, ".jsonl") {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestAnalyzeBundleAggregatesReports(t *testing.T) {
	bundle := makeBundle(t, map[string]string{
		"logs/session-1.jsonl": strings.Join([]string{
			`{"type":"user","message":"Claude Code on macOS using Spec Kitty"}`,
			`{"type":"tool","command":"cat src/auth.ts","output":"sk-ant-123456789012345678901234567890"}`,
			`{"type":"tool","command":"cat src/auth.ts","output":"again"}`,
			`{"type":"tool","command":"cat src/auth.ts","output":"third"}`,
		}, "\n"),
		"logs/session-2.jsonl": strings.Join([]string{
			`{"type":"tool","command":"go test ./...","error":"failed"}`,
			`{"type":"tool","command":"go test ./...","error":"failed"}`,
			`{"type":"tool","command":"go test ./...","error":"failed"}`,
		}, "\n"),
	})

	report, err := AnalyzeBundle("job-paid", bundle, Options{MaxFiles: 100})
	if err != nil {
		t.Fatal(err)
	}
	if report.Metrics.SessionCount != 2 {
		t.Fatalf("expected session count, got %#v", report.Metrics)
	}
	if report.AggregateEvent.ParserType != "paid_bundle" {
		t.Fatalf("expected paid_bundle parser type, got %#v", report.AggregateEvent)
	}
	if report.Redactions["anthropic_key"] == 0 {
		t.Fatalf("expected redaction aggregation, got %#v", report.Redactions)
	}
	body := string(mustJSON(t, report))
	for _, forbidden := range []string{"sk-ant-", "session-1.jsonl", ".claude/projects"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("paid report leaked %q: %s", forbidden, body)
		}
	}
}

func TestExtractRejectsHostileArchives(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries map[string]string
	}{
		{name: "absolute", entries: map[string]string{"/tmp/session.jsonl": "{}\n"}},
		{name: "traversal", entries: map[string]string{"../session.jsonl": "{}\n"}},
		{name: "non-jsonl", entries: map[string]string{"session.txt": "{}\n"}},
		{name: "windows", entries: map[string]string{`logs\session.jsonl`: "{}\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Extract(makeBundle(t, tc.entries), Options{MaxFiles: 100}); err == nil {
				t.Fatal("expected hostile archive to fail")
			}
		})
	}
}

func TestExtractRejectsTooManyFiles(t *testing.T) {
	entries := map[string]string{}
	for i := 0; i < 101; i++ {
		entries["logs/session-"+string(rune('a'+i%26))+"-"+string(rune('a'+i/26))+".jsonl"] = "{}\n"
	}
	if _, err := Extract(makeBundle(t, entries), Options{MaxFiles: 100}); err == nil {
		t.Fatal("expected too many files to fail")
	}
}

func TestExtractRejectsSymlink(t *testing.T) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "logs/session.jsonl", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Extract(buf.Bytes(), Options{MaxFiles: 100}); err == nil {
		t.Fatal("expected symlink to fail")
	}
}

func makeBundle(t *testing.T, files map[string]string) []byte {
	t.Helper()
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
