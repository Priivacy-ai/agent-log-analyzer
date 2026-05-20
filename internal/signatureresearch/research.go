package signatureresearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Sources []Source `json:"sources"`
}

type Source struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	URL      string `json:"url"`
	Category string `json:"category"`
	MaxPages int    `json:"max_pages,omitempty"`
}

type Report struct {
	SourceCount int         `json:"source_count"`
	Candidates  []Candidate `json:"candidates"`
}

type Candidate struct {
	Category          string            `json:"category"`
	ID                string            `json:"id"`
	Label             string            `json:"label"`
	Confidence        string            `json:"confidence"`
	EvidenceCount     int               `json:"evidence_count"`
	SuggestedPatterns []string          `json:"suggested_patterns"`
	Sources           []CandidateSource `json:"sources"`
}

type CandidateSource struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

func Collect(ctx context.Context, cfg Config, client HTTPClient) (Report, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	acc := map[string]*candidateAccumulator{}
	for _, source := range cfg.Sources {
		if source.ID == "" || source.URL == "" || source.Kind == "" || source.Category == "" {
			return Report{}, fmt.Errorf("invalid source config: %#v", source)
		}
		pages, err := fetchSource(ctx, client, source)
		if err != nil {
			return Report{}, fmt.Errorf("fetch %s: %w", source.ID, err)
		}
		for _, page := range pages {
			for _, item := range candidatesFromSource(source, page.Body) {
				key := item.Category + ":" + item.ID
				current := acc[key]
				if current == nil {
					current = &candidateAccumulator{
						Category: item.Category,
						ID:       item.ID,
						Label:    item.Label,
						Sources:  map[string]CandidateSource{},
						Patterns: map[string]bool{},
					}
					acc[key] = current
				}
				current.EvidenceCount += item.EvidenceCount
				for _, pattern := range item.SuggestedPatterns {
					current.Patterns[pattern] = true
				}
				current.Sources[source.ID] = CandidateSource{ID: source.ID, URL: page.URL}
			}
		}
	}

	report := Report{SourceCount: len(cfg.Sources)}
	for _, current := range acc {
		report.Candidates = append(report.Candidates, current.toCandidate())
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		left, right := report.Candidates[i], report.Candidates[j]
		if left.Category != right.Category {
			return left.Category < right.Category
		}
		if left.EvidenceCount != right.EvidenceCount {
			return left.EvidenceCount > right.EvidenceCount
		}
		return left.ID < right.ID
	})
	return report, nil
}

type fetchedPage struct {
	Body []byte
	URL  string
}

type candidateAccumulator struct {
	Category      string
	ID            string
	Label         string
	EvidenceCount int
	Sources       map[string]CandidateSource
	Patterns      map[string]bool
}

func (c *candidateAccumulator) toCandidate() Candidate {
	var sources []CandidateSource
	for _, source := range c.Sources {
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })

	patterns := make([]string, 0, len(c.Patterns))
	for pattern := range c.Patterns {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)

	confidence := "low"
	if c.EvidenceCount >= 3 || len(c.Sources) >= 2 {
		confidence = "medium"
	}
	if c.EvidenceCount >= 8 && len(c.Sources) >= 2 {
		confidence = "high"
	}

	return Candidate{
		Category:          c.Category,
		ID:                c.ID,
		Label:             c.Label,
		Confidence:        confidence,
		EvidenceCount:     c.EvidenceCount,
		SuggestedPatterns: patterns,
		Sources:           sources,
	}
}

func fetchSource(ctx context.Context, client HTTPClient, source Source) ([]fetchedPage, error) {
	if source.Kind != "mcp_registry" {
		body, finalURL, err := fetch(ctx, client, source.URL)
		if err != nil {
			return nil, err
		}
		return []fetchedPage{{Body: body, URL: finalURL}}, nil
	}

	maxPages := source.MaxPages
	if maxPages <= 0 {
		maxPages = 1
	}
	nextURL := source.URL
	var pages []fetchedPage
	for page := 0; page < maxPages && nextURL != ""; page++ {
		body, finalURL, err := fetch(ctx, client, nextURL)
		if err != nil {
			return nil, err
		}
		pages = append(pages, fetchedPage{Body: body, URL: finalURL})
		nextURL = nextMCPRegistryURL(source.URL, body)
	}
	return pages, nil
}

func fetch(ctx context.Context, client HTTPClient, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json,text/html;q=0.8,*/*;q=0.5")
	req.Header.Set("User-Agent", "agent-log-analyzer-signature-research/0.1")
	if token := strings.TrimSpace(getenv("GITHUB_TOKEN")); token != "" && strings.Contains(req.URL.Host, "github.com") {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, "", err
	}
	return body, resp.Request.URL.String(), nil
}

var getenv = os.Getenv

func nextMCPRegistryURL(baseURL string, body []byte) string {
	var parsed struct {
		Metadata struct {
			NextCursor string `json:"nextCursor"`
		} `json:"metadata"`
	}
	if json.Unmarshal(body, &parsed) != nil || parsed.Metadata.NextCursor == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	values := u.Query()
	values.Set("cursor", parsed.Metadata.NextCursor)
	u.RawQuery = values.Encode()
	return u.String()
}

type rawCandidate struct {
	Category          string
	ID                string
	Label             string
	EvidenceCount     int
	SuggestedPatterns []string
}

func candidatesFromSource(source Source, body []byte) []rawCandidate {
	switch source.Kind {
	case "mcp_registry":
		return mcpRegistryCandidates(source, body)
	case "npm_search":
		return npmSearchCandidates(source, body)
	case "github_search_repositories":
		return githubRepoCandidates(source, body)
	default:
		return textCandidates(source, body)
	}
}

func mcpRegistryCandidates(source Source, body []byte) []rawCandidate {
	var parsed struct {
		Servers []struct {
			Server struct {
				Name        string `json:"name"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Repository  struct {
					URL string `json:"url"`
				} `json:"repository"`
				Packages []struct {
					Registry string `json:"registry"`
					Name     string `json:"name"`
				} `json:"packages"`
			} `json:"server"`
		} `json:"servers"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return textCandidates(source, body)
	}
	var out []rawCandidate
	for _, item := range parsed.Servers {
		label := firstNonEmpty(item.Server.Title, item.Server.Name)
		if candidateID := mcpID(item.Server.Name, item.Server.Title); candidateID != "" {
			out = append(out, rawCandidate{
				Category:          source.Category,
				ID:                candidateID,
				Label:             label,
				EvidenceCount:     2,
				SuggestedPatterns: patternsFor(source.Category, candidateID, label, item.Server.Name),
			})
		}
		for _, pkg := range item.Server.Packages {
			if pkg.Name == "" {
				continue
			}
			if candidateID := normalizeID(pkg.Name); candidateID != "" {
				out = append(out, rawCandidate{
					Category:          source.Category,
					ID:                candidateID,
					Label:             pkg.Name,
					EvidenceCount:     1,
					SuggestedPatterns: patternsFor(source.Category, candidateID, pkg.Name),
				})
			}
		}
		text := []byte(strings.Join([]string{item.Server.Name, item.Server.Title, item.Server.Description, item.Server.Repository.URL}, " "))
		out = append(out, textCandidates(source, text)...)
	}
	return out
}

func npmSearchCandidates(source Source, body []byte) []rawCandidate {
	var parsed struct {
		Objects []struct {
			Package struct {
				Name        string   `json:"name"`
				Description string   `json:"description"`
				Keywords    []string `json:"keywords"`
			} `json:"package"`
		} `json:"objects"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return textCandidates(source, body)
	}
	var out []rawCandidate
	for _, item := range parsed.Objects {
		name := item.Package.Name
		if !looksRelevant(name + " " + item.Package.Description + " " + strings.Join(item.Package.Keywords, " ")) {
			continue
		}
		if candidateID := normalizeID(name); candidateID != "" {
			out = append(out, rawCandidate{
				Category:          source.Category,
				ID:                candidateID,
				Label:             name,
				EvidenceCount:     2,
				SuggestedPatterns: patternsFor(source.Category, candidateID, name),
			})
		}
	}
	return out
}

func githubRepoCandidates(source Source, body []byte) []rawCandidate {
	var parsed struct {
		Items []struct {
			Name        string   `json:"name"`
			FullName    string   `json:"full_name"`
			Description string   `json:"description"`
			Topics      []string `json:"topics"`
		} `json:"items"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return textCandidates(source, body)
	}
	var out []rawCandidate
	for _, item := range parsed.Items {
		text := strings.Join([]string{item.Name, item.FullName, item.Description, strings.Join(item.Topics, " ")}, " ")
		if !looksRelevant(text) {
			continue
		}
		if candidateID := normalizeID(item.Name); candidateID != "" {
			out = append(out, rawCandidate{
				Category:          source.Category,
				ID:                candidateID,
				Label:             firstNonEmpty(item.FullName, item.Name),
				EvidenceCount:     1,
				SuggestedPatterns: patternsFor(source.Category, candidateID, item.Name, item.FullName),
			})
		}
		out = append(out, textCandidates(source, []byte(text))...)
	}
	return out
}

func textCandidates(source Source, body []byte) []rawCandidate {
	text := string(bytes.ToValidUTF8(body, nil))
	var out []rawCandidate
	for _, match := range regexp.MustCompile(`mcp__([A-Za-z0-9_-]{2,})__`).FindAllStringSubmatch(text, -1) {
		id := normalizeID(match[1])
		out = append(out, rawCandidate{Category: "mcp_servers", ID: id, Label: match[1], EvidenceCount: 1, SuggestedPatterns: patternsFor("mcp_servers", id, match[1])})
	}
	for _, match := range regexp.MustCompile(`(?i)\bclaude\s+mcp\s+add\s+([A-Za-z0-9_-]{2,})\b`).FindAllStringSubmatch(text, -1) {
		id := normalizeID(match[1])
		out = append(out, rawCandidate{Category: "mcp_servers", ID: id, Label: match[1], EvidenceCount: 1, SuggestedPatterns: patternsFor("mcp_servers", id, match[1])})
	}
	for _, match := range regexp.MustCompile(`(?i)\.claude/(?:skills|commands)/([A-Za-z0-9_-]{2,})`).FindAllStringSubmatch(text, -1) {
		id := normalizeID(match[1])
		out = append(out, rawCandidate{Category: "skills", ID: id, Label: match[1], EvidenceCount: 1, SuggestedPatterns: patternsFor("skills", id, match[1])})
	}
	for _, match := range regexp.MustCompile(`(?i)(?:^|[\s"'(:])/([A-Za-z][A-Za-z0-9_-]{2,})\b`).FindAllStringSubmatch(text, -1) {
		id := normalizeID(match[1])
		if id == "" || isCommonFalsePositive(id) {
			continue
		}
		out = append(out, rawCandidate{Category: "skills", ID: id, Label: "/" + match[1], EvidenceCount: 1, SuggestedPatterns: patternsFor("skills", id, match[1])})
	}
	return out
}

func looksRelevant(text string) bool {
	lower := strings.ToLower(text)
	for _, marker := range []string{"claude", "mcp", "model context protocol", "agent", "coding assistant", "codex", "cursor", "windsurf", "opencode", "skill"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func mcpID(name, title string) string {
	for _, raw := range []string{title, name} {
		candidate := normalizeID(lastSegment(raw))
		if candidate != "" && candidate != "mcp" && candidate != "server" {
			return candidate
		}
	}
	return ""
}

func patternsFor(category, id string, labels ...string) []string {
	seen := map[string]bool{}
	add := func(pattern string) {
		if pattern != "" {
			seen[pattern] = true
		}
	}
	switch category {
	case "mcp_servers":
		add(`(?i)\bmcp__` + idRegexp(id) + `__`)
		add(`(?i)\bclaude\s+mcp\s+add\s+` + idRegexp(id) + `\b`)
	case "skills":
		add(`(?i)(^|\s)/` + idRegexp(id) + `\b`)
		add(`(?i)\.claude/(skills|commands)/` + idRegexp(id) + `\b`)
	default:
		add(`(?i)\b` + idRegexp(id) + `\b`)
	}
	for _, label := range labels {
		if normalized := normalizeID(label); normalized != "" && normalized != id {
			add(`(?i)\b` + idRegexp(normalized) + `\b`)
		}
	}
	out := make([]string, 0, len(seen))
	for pattern := range seen {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

func idRegexp(id string) string {
	parts := strings.Split(id, "_")
	for idx, part := range parts {
		parts[idx] = regexp.QuoteMeta(part)
	}
	return strings.Join(parts, `[-_]`)
}

func normalizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "@")
	value = strings.TrimPrefix(value, "mcp-server-")
	value = strings.TrimPrefix(value, "mcp_")
	value = strings.TrimSuffix(value, "-mcp-server")
	value = strings.TrimSuffix(value, "_mcp_server")
	value = strings.TrimSuffix(value, "-mcp")
	value = strings.TrimSuffix(value, "_mcp")
	value = strings.NewReplacer("/", "_", "-", "_", ".", "_", "@", "", " ", "_").Replace(value)
	value = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(value, "_")
	value = regexp.MustCompile(`_+`).ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if len(value) < 2 || len(value) > 80 {
		return ""
	}
	return value
}

func lastSegment(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		value = strings.Trim(parsed.Path, "/")
	}
	if idx := strings.LastIndex(value, "/"); idx >= 0 {
		value = value[idx+1:]
	}
	return value
}

func isCommonFalsePositive(id string) bool {
	switch id {
	case "users", "home", "tmp", "var", "usr", "src", "api", "docs", "help", "login", "logout":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
