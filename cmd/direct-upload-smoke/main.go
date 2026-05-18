package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type directUpload struct {
	JobID        string            `json:"job_id"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Fields       map[string]string `json:"fields"`
	Headers      map[string]string `json:"headers"`
	FinalizePath string            `json:"finalize_path"`
}

type jobStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func main() {
	baseURL := getenv("CLAUDE_ANALYZER_URL", "http://127.0.0.1:8080")
	fixture := getenv("CLAUDE_ANALYZER_FIXTURE", "testdata/fixtures/sample-claude.jsonl")
	upload, err := createUpload(baseURL)
	must(err)
	must(postUpload(upload, fixture))
	must(finalizeUpload(baseURL, upload))
	must(waitForCompletion(baseURL, upload.JobID, 120*time.Second))
	must(verifyReport(baseURL, upload.JobID))
	fmt.Printf("direct upload smoke ok: %s\n", upload.JobID)
}

func createUpload(baseURL string) (directUpload, error) {
	resp, err := http.Post(baseURL+"/api/upload-url", "application/json", nil)
	if err != nil {
		return directUpload{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return directUpload{}, fmt.Errorf("create upload status %d: %s", resp.StatusCode, string(body))
	}
	var upload directUpload
	return upload, json.NewDecoder(resp.Body).Decode(&upload)
}

func postUpload(upload directUpload, fixture string) error {
	data, err := os.ReadFile(fixture)
	if err != nil {
		return err
	}
	if len(upload.Fields) == 0 {
		return putUpload(upload, data)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range upload.Fields {
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	part, err := writer.CreateFormFile("file", filepath.Base(fixture))
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	request, err := http.NewRequest(upload.Method, rewriteURL(upload.URL), body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func putUpload(upload directUpload, data []byte) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequest(upload.Method, rewriteURL(upload.URL), bytes.NewReader(data))
		if err != nil {
			return err
		}
		for key, value := range upload.Headers {
			request.Header.Set(key, value)
		}
		if request.Header.Get("Content-Type") == "" {
			request.Header.Set("Content-Type", "application/octet-stream")
		}
		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			lastErr = err
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("upload status %d: %s", resp.StatusCode, string(body))
			if resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
				return lastErr
			}
		}
		if attempt < 2 {
			time.Sleep(time.Duration(250*(1<<attempt)) * time.Millisecond)
		}
	}
	return lastErr
}

func finalizeUpload(baseURL string, upload directUpload) error {
	resp, err := http.Post(baseURL+upload.FinalizePath, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("finalize status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func waitForCompletion(baseURL, jobID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/jobs/" + jobID)
		if err != nil {
			return err
		}
		var job jobStatus
		err = json.NewDecoder(resp.Body).Decode(&job)
		resp.Body.Close()
		if err != nil {
			return err
		}
		switch job.Status {
		case "completed":
			return nil
		case "failed":
			return fmt.Errorf("job failed")
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timed out waiting for %s", jobID)
}

func verifyReport(baseURL, jobID string) error {
	resp, err := http.Get(baseURL + "/api/reports/" + jobID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("report status %d: %s", resp.StatusCode, string(data))
	}
	body := string(data)
	for _, required := range []string{`"raw_transcript_sent_to_llm":false`, `"spec_kitty"`} {
		if !strings.Contains(body, required) {
			return fmt.Errorf("report missing %s", required)
		}
	}
	if strings.Contains(body, "sk-ant-") {
		return fmt.Errorf("report leaked secret")
	}
	return nil
}

func rewriteURL(raw string) string {
	from := os.Getenv("CLAUDE_ANALYZER_UPLOAD_URL_REWRITE_FROM")
	to := os.Getenv("CLAUDE_ANALYZER_UPLOAD_URL_REWRITE_TO")
	if from != "" && to != "" {
		return strings.Replace(raw, from, to, 1)
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Hostname() == "localstack" {
		parsed.Host = "127.0.0.1" + portSuffix(parsed)
		return parsed.String()
	}
	return raw
}

func portSuffix(parsed *url.URL) string {
	if parsed.Port() == "" {
		return ""
	}
	return ":" + parsed.Port()
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
