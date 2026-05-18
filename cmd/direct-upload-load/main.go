package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
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
	Status string `json:"status"`
}

type sample struct {
	CreateMS      int64 `json:"create_ms"`
	UploadMS      int64 `json:"upload_ms"`
	FinalizeMS    int64 `json:"finalize_ms"`
	WaitMS        int64 `json:"wait_ms"`
	ReportMS      int64 `json:"report_ms"`
	TotalMS       int64 `json:"total_ms"`
	UploadRetries int   `json:"upload_retries"`
}

type result struct {
	Count         int      `json:"count"`
	Concurrency   int      `json:"concurrency"`
	Failures      int      `json:"failures"`
	Errors        []string `json:"errors,omitempty"`
	CreateP95     int64    `json:"create_p95_ms"`
	UploadP95     int64    `json:"upload_p95_ms"`
	FinalizeP95   int64    `json:"finalize_p95_ms"`
	WaitP95       int64    `json:"wait_p95_ms"`
	ReportP95     int64    `json:"report_p95_ms"`
	TotalP95      int64    `json:"total_p95_ms"`
	UploadRetries int      `json:"upload_retries"`
}

func main() {
	baseURL := flag.String("url", getenv("CLAUDE_ANALYZER_URL", "http://127.0.0.1:8080"), "analyzer base URL")
	fixture := flag.String("fixture", getenv("CLAUDE_ANALYZER_FIXTURE", "testdata/fixtures/sample-claude.jsonl"), "JSONL fixture path")
	count := flag.Int("n", 10, "jobs to run")
	concurrency := flag.Int("concurrency", 5, "concurrent jobs")
	timeout := flag.Duration("timeout", 3*time.Minute, "per-job completion timeout")
	flag.Parse()

	data, err := os.ReadFile(*fixture)
	if err != nil {
		fail("read fixture: %v", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	jobs := make(chan int)
	results := make(chan sample, *count)
	errors := make(chan error, *count)

	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				s, err := runJob(client, *baseURL, data, *timeout)
				if err != nil {
					errors <- err
					continue
				}
				results <- s
			}
		}()
	}
	for i := 0; i < *count; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(results)
	close(errors)

	var samples []sample
	for item := range results {
		samples = append(samples, item)
	}
	var messages []string
	for err := range errors {
		if len(messages) < 10 {
			messages = append(messages, err.Error())
		}
	}
	out := summarize(*count, *concurrency, samples, messages)
	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fail("marshal result: %v", err)
	}
	fmt.Println(string(encoded))
	if out.Failures > 0 {
		os.Exit(1)
	}
}

func runJob(client *http.Client, baseURL string, data []byte, timeout time.Duration) (sample, error) {
	start := time.Now()
	var s sample
	upload, elapsed, err := createUpload(client, baseURL)
	if err != nil {
		return s, err
	}
	s.CreateMS = elapsed.Milliseconds()
	var retries int
	elapsed, retries, err = putUpload(client, upload, data)
	if err != nil {
		return s, err
	}
	s.UploadMS = elapsed.Milliseconds()
	s.UploadRetries = retries
	elapsed, err = finalizeUpload(client, baseURL, upload)
	if err != nil {
		return s, err
	}
	s.FinalizeMS = elapsed.Milliseconds()
	elapsed, err = waitForCompletion(client, baseURL, upload.JobID, timeout)
	if err != nil {
		return s, err
	}
	s.WaitMS = elapsed.Milliseconds()
	elapsed, err = verifyReport(client, baseURL, upload.JobID)
	if err != nil {
		return s, err
	}
	s.ReportMS = elapsed.Milliseconds()
	s.TotalMS = time.Since(start).Milliseconds()
	return s, nil
}

func createUpload(client *http.Client, baseURL string) (directUpload, time.Duration, error) {
	start := time.Now()
	resp, err := client.Post(baseURL+"/api/upload-url", "application/json", nil)
	if err != nil {
		return directUpload{}, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return directUpload{}, 0, fmt.Errorf("create upload status %d: %s", resp.StatusCode, string(body))
	}
	var upload directUpload
	if err := json.NewDecoder(resp.Body).Decode(&upload); err != nil {
		return directUpload{}, 0, err
	}
	return upload, time.Since(start), nil
}

func putUpload(client *http.Client, upload directUpload, data []byte) (time.Duration, int, error) {
	start := time.Now()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequest(upload.Method, rewriteURL(upload.URL), bytes.NewReader(data))
		if err != nil {
			return 0, attempt, err
		}
		for key, value := range upload.Headers {
			request.Header.Set(key, value)
		}
		if request.Header.Get("Content-Type") == "" {
			request.Header.Set("Content-Type", "application/octet-stream")
		}
		resp, err := client.Do(request)
		if err != nil {
			lastErr = err
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return time.Since(start), attempt, nil
			}
			lastErr = fmt.Errorf("upload status %d: %s", resp.StatusCode, string(body))
			if resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
				return 0, attempt, lastErr
			}
		}
		if attempt < 2 {
			time.Sleep(time.Duration(250*(1<<attempt)) * time.Millisecond)
		}
	}
	return 0, 2, lastErr
}

func finalizeUpload(client *http.Client, baseURL string, upload directUpload) (time.Duration, error) {
	start := time.Now()
	resp, err := client.Post(baseURL+upload.FinalizePath, "application/json", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("finalize status %d: %s", resp.StatusCode, string(body))
	}
	return time.Since(start), nil
}

func waitForCompletion(client *http.Client, baseURL, jobID string, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/api/jobs/" + jobID)
		if err != nil {
			return 0, err
		}
		var job jobStatus
		err = json.NewDecoder(resp.Body).Decode(&job)
		resp.Body.Close()
		if err != nil {
			return 0, err
		}
		switch job.Status {
		case "completed":
			return time.Since(start), nil
		case "failed":
			return 0, fmt.Errorf("job failed")
		}
		time.Sleep(time.Second)
	}
	return 0, fmt.Errorf("timed out waiting for job")
}

func verifyReport(client *http.Client, baseURL, jobID string) (time.Duration, error) {
	start := time.Now()
	resp, err := client.Get(baseURL + "/api/reports/" + jobID)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("report status %d: %s", resp.StatusCode, string(data))
	}
	body := string(data)
	for _, required := range []string{`"raw_transcript_sent_to_llm":false`, `"security_receipt"`} {
		if !strings.Contains(body, required) {
			return 0, fmt.Errorf("report missing %s", required)
		}
	}
	if strings.Contains(body, "sk-ant-") {
		return 0, fmt.Errorf("report leaked secret")
	}
	return time.Since(start), nil
}

func summarize(count, concurrency int, samples []sample, errors []string) result {
	out := result{
		Count:       count,
		Concurrency: concurrency,
		Failures:    count - len(samples),
		Errors:      errors,
	}
	out.CreateP95 = p95(samples, func(s sample) int64 { return s.CreateMS })
	out.UploadP95 = p95(samples, func(s sample) int64 { return s.UploadMS })
	out.FinalizeP95 = p95(samples, func(s sample) int64 { return s.FinalizeMS })
	out.WaitP95 = p95(samples, func(s sample) int64 { return s.WaitMS })
	out.ReportP95 = p95(samples, func(s sample) int64 { return s.ReportMS })
	out.TotalP95 = p95(samples, func(s sample) int64 { return s.TotalMS })
	for _, sample := range samples {
		out.UploadRetries += sample.UploadRetries
	}
	return out
}

func p95(samples []sample, value func(sample) int64) int64 {
	if len(samples) == 0 {
		return 0
	}
	values := make([]int64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, value(sample))
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	idx := int(float64(len(values))*0.95) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func rewriteURL(raw string) string {
	from := os.Getenv("CLAUDE_ANALYZER_UPLOAD_URL_REWRITE_FROM")
	to := os.Getenv("CLAUDE_ANALYZER_UPLOAD_URL_REWRITE_TO")
	if from == "" || to == "" || !strings.HasPrefix(raw, from) {
		return raw
	}
	parsed, err := url.Parse(to + strings.TrimPrefix(raw, from))
	if err != nil {
		return raw
	}
	return parsed.String()
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
