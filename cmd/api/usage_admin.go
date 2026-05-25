package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analytics"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

const (
	defaultUsageStatsWindow = 7 * 24 * time.Hour
	maxUsageStatsWindow     = 90 * 24 * time.Hour
	defaultUsageStatsLimit  = 20000
	maxUsageStatsLimit      = 100000
)

func usageStatsHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminTokenConfigured() {
			writeError(w, http.StatusServiceUnavailable, "usage stats auth is not configured")
			return
		}
		if !adminTokenMatches(bearerToken(r.Header.Get("Authorization"))) {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}
		usageStore, ok := store.(app.UsageStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "usage stats unavailable")
			return
		}
		since, until, err := usageStatsWindow(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		limit := usageStatsLimit(r)
		events, err := usageStore.ReadUsageEvents(since, limit)
		if err != nil {
			slog.Warn("usage stats read failed", "error_category", "usage_stats_read")
			writeError(w, http.StatusInternalServerError, "could not read usage stats")
			return
		}
		stats := analytics.SummarizeUsageEvents(events, since, until, limit > 0 && len(events) >= limit)
		writeJSON(w, http.StatusOK, stats)
	}
}

func logRequests(next http.Handler, store app.APIStore) http.Handler {
	usageStore, _ := store.(app.UsageStore)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		path := sanitizePath(r.URL.Path)
		event := analytics.NewUsageEvent(start)
		event.Method = r.Method
		event.Path = path
		event.Host = cleanHost(r.Host)
		event.Scheme = requestScheme(r)
		event.Status = recorder.status
		event.DurationMS = time.Since(start).Milliseconds()
		event.RequestBytes = r.ContentLength
		event.ResponseBytes = recorder.bytes
		event.AuthSurface = authSurface(path)
		event.Authenticated = requestAuthenticated(event.AuthSurface, recorder.status)
		event.ClientHash = clientHash(r)
		event.ClientIPVersion, event.ClientIPPrefix = clientIPSummary(r)
		event.UserAgent = userAgentFamily(r.UserAgent())
		event.Browser, event.BrowserMajor, event.OperatingSystem, event.OSMajor, event.DeviceClass, event.Bot = userAgentDetails(r.UserAgent())
		event.AcceptLanguage, event.Language, event.Region = languageDetails(r.Header.Get("Accept-Language"))
		event.ReferrerHost, event.ReferrerPath, event.ReferrerInternal = referrerDetails(r.Header.Get("Referer"), event.Host)
		event.UTM = utmParams(r.URL.Query())
		slog.Info("request",
			"method", r.Method,
			"path", path,
			"host", event.Host,
			"status", recorder.status,
			"duration_ms", event.DurationMS,
			"user_agent", event.UserAgent,
			"browser", event.Browser,
			"browser_major", event.BrowserMajor,
			"operating_system", event.OperatingSystem,
			"os_major", event.OSMajor,
			"device_class", event.DeviceClass,
			"bot", event.Bot,
			"language", event.Language,
			"region", event.Region,
			"client_ip_version", event.ClientIPVersion,
			"client_ip_prefix", event.ClientIPPrefix,
			"referrer_host", event.ReferrerHost,
			"referrer_path", event.ReferrerPath,
		)
		if usageStore == nil || path == "/healthz" {
			return
		}
		if err := usageStore.AppendUsageEvent(event); err != nil {
			slog.Warn("usage event append failed", "error_category", "usage_append")
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
	wrote  bool
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(data []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(w.status)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	return n, err
}

func adminTokenConfigured() bool {
	return os.Getenv("CLAUDE_ANALYZER_ADMIN_TOKEN_SHA256") != "" || os.Getenv("CLAUDE_ANALYZER_ADMIN_TOKEN") != ""
}

func adminTokenMatches(token string) bool {
	if token == "" {
		return false
	}
	if hash := os.Getenv("CLAUDE_ANALYZER_ADMIN_TOKEN_SHA256"); hash != "" {
		got := tokenHash(token)
		return subtle.ConstantTimeCompare([]byte(hash), []byte(got)) == 1
	}
	expected := os.Getenv("CLAUDE_ANALYZER_ADMIN_TOKEN")
	return expected != "" && subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1
}

func usageStatsWindow(r *http.Request) (time.Time, time.Time, error) {
	until := time.Now().UTC()
	window := defaultUsageStatsWindow
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		duration, err := time.ParseDuration(raw)
		if err != nil || duration <= 0 || duration > maxUsageStatsWindow {
			return time.Time{}, time.Time{}, httpError("since must be a duration between 1ns and 90d")
		}
		window = duration
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		days, err := strconv.Atoi(raw)
		if err != nil || days <= 0 || days > 90 {
			return time.Time{}, time.Time{}, httpError("days must be between 1 and 90")
		}
		window = time.Duration(days) * 24 * time.Hour
	}
	return until.Add(-window), until, nil
}

type httpError string

func (e httpError) Error() string { return string(e) }

func usageStatsLimit(r *http.Request) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return defaultUsageStatsLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return defaultUsageStatsLimit
	}
	if limit > maxUsageStatsLimit {
		return maxUsageStatsLimit
	}
	return limit
}

func authSurface(path string) string {
	switch {
	case path == "/api/admin/usage-stats":
		return "admin_token"
	case path == "/api/admin/email-unlocks":
		return "admin_token"
	case strings.HasPrefix(path, "/api/uploads/"), strings.HasPrefix(path, "/api/paid-uploads/"):
		return "upload_token"
	case strings.HasPrefix(path, "/api/public-reports/"), strings.HasPrefix(path, "/api/public-artifacts/"), strings.HasPrefix(path, "/r/"):
		return "report_token"
	case strings.HasPrefix(path, "/email/confirm/"):
		return "email_confirmation_token"
	default:
		return "none"
	}
}

func requestAuthenticated(surface string, status int) bool {
	if surface == "none" {
		return false
	}
	return status < http.StatusBadRequest
}

func clientHash(r *http.Request) string {
	salt := os.Getenv("CLAUDE_ANALYZER_USAGE_HASH_SALT")
	if salt == "" {
		return ""
	}
	client := firstClientIP(r)
	if client == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(salt + "\x00" + client))
	return hex.EncodeToString(sum[:])
}

func userAgentFamily(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case ua == "":
		return "unknown"
	case strings.Contains(ua, "curl/"):
		return "curl"
	case strings.Contains(ua, "npm/"), strings.Contains(ua, "node"):
		return "node"
	case strings.Contains(ua, "python-requests"):
		return "python-requests"
	case strings.Contains(ua, "go-http-client"):
		return "go-http-client"
	case strings.Contains(ua, "mozilla/"):
		return "browser"
	default:
		return "other"
	}
}

func cleanHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	return limitString(host, 120)
}

func requestScheme(r *http.Request) string {
	if proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); proto == "http" || proto == "https" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	if r.URL != nil && (r.URL.Scheme == "http" || r.URL.Scheme == "https") {
		return r.URL.Scheme
	}
	return "http"
}

func clientIPSummary(r *http.Request) (string, string) {
	client := firstClientIP(r)
	if client == "" {
		return "", ""
	}
	addr, err := netip.ParseAddr(client)
	if err != nil {
		return "", ""
	}
	if addr.Is4() {
		prefix, err := addr.Prefix(24)
		if err != nil {
			return "ipv4", ""
		}
		return "ipv4", prefix.Masked().String()
	}
	prefix, err := addr.Prefix(48)
	if err != nil {
		return "ipv6", ""
	}
	return "ipv6", prefix.Masked().String()
}

func firstClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		for _, candidate := range strings.Split(forwarded, ",") {
			candidate = strings.TrimSpace(candidate)
			if candidate != "" {
				return candidate
			}
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func userAgentDetails(userAgent string) (browser, browserMajor, operatingSystem, osMajor, deviceClass string, bot bool) {
	ua := strings.ToLower(userAgent)
	if ua == "" {
		return "unknown", "", "unknown", "", "unknown", false
	}
	bot = strings.Contains(ua, "bot") ||
		strings.Contains(ua, "crawler") ||
		strings.Contains(ua, "spider") ||
		strings.Contains(ua, "slurp") ||
		strings.Contains(ua, "ahrefs") ||
		strings.Contains(ua, "semrush") ||
		strings.Contains(ua, "bytespider") ||
		strings.Contains(ua, "facebookexternalhit")
	browser, browserMajor = browserDetails(userAgent, ua)
	operatingSystem, osMajor = osDetails(userAgent, ua)
	switch {
	case bot:
		deviceClass = "bot"
	case strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet"):
		deviceClass = "tablet"
	case strings.Contains(ua, "mobile") || strings.Contains(ua, "iphone") || strings.Contains(ua, "android"):
		deviceClass = "mobile"
	case browser == "curl" || browser == "node" || browser == "python-requests" || browser == "go-http-client":
		deviceClass = "automation"
	case browser == "unknown":
		deviceClass = "unknown"
	default:
		deviceClass = "desktop"
	}
	return browser, browserMajor, operatingSystem, osMajor, deviceClass, bot
}

func browserDetails(userAgent, ua string) (string, string) {
	switch {
	case strings.Contains(ua, "edg/"):
		return "edge", majorAfter(userAgent, "Edg/")
	case strings.Contains(ua, "firefox/"):
		return "firefox", majorAfter(userAgent, "Firefox/")
	case strings.Contains(ua, "fxios/"):
		return "firefox", majorAfter(userAgent, "FxiOS/")
	case strings.Contains(ua, "crios/"):
		return "chrome", majorAfter(userAgent, "CriOS/")
	case strings.Contains(ua, "chrome/"):
		return "chrome", majorAfter(userAgent, "Chrome/")
	case strings.Contains(ua, "safari/") && strings.Contains(ua, "version/"):
		return "safari", majorAfter(userAgent, "Version/")
	case strings.Contains(ua, "curl/"):
		return "curl", majorAfter(userAgent, "curl/")
	case strings.Contains(ua, "npm/"):
		return "node", majorAfter(userAgent, "npm/")
	case strings.Contains(ua, "node"):
		return "node", ""
	case strings.Contains(ua, "python-requests/"):
		return "python-requests", majorAfter(userAgent, "python-requests/")
	case strings.Contains(ua, "go-http-client/"):
		return "go-http-client", majorAfter(userAgent, "Go-http-client/")
	default:
		return "unknown", ""
	}
}

func osDetails(userAgent, ua string) (string, string) {
	switch {
	case strings.Contains(ua, "iphone os ") || strings.Contains(ua, "cpu os "):
		return "ios", majorWithRegexp(userAgent, regexp.MustCompile(`(?:iPhone OS|CPU OS) ([0-9]+)`))
	case strings.Contains(ua, "android "):
		return "android", majorWithRegexp(userAgent, regexp.MustCompile(`Android ([0-9]+)`))
	case strings.Contains(ua, "mac os x "):
		return "macos", majorWithRegexp(userAgent, regexp.MustCompile(`Mac OS X ([0-9]+)`))
	case strings.Contains(ua, "windows nt "):
		return "windows", majorWithRegexp(userAgent, regexp.MustCompile(`Windows NT ([0-9]+)`))
	case strings.Contains(ua, "linux"):
		return "linux", ""
	default:
		return "unknown", ""
	}
}

func majorAfter(value, marker string) string {
	index := strings.Index(value, marker)
	if index < 0 {
		index = strings.Index(strings.ToLower(value), strings.ToLower(marker))
	}
	if index < 0 {
		return ""
	}
	start := index + len(marker)
	end := start
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	return value[start:end]
}

func majorWithRegexp(value string, expr *regexp.Regexp) string {
	matches := expr.FindStringSubmatch(value)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func languageDetails(header string) (raw, language, region string) {
	raw = limitString(strings.TrimSpace(header), 256)
	if raw == "" {
		return "", "", ""
	}
	first := strings.TrimSpace(strings.Split(raw, ",")[0])
	first = strings.TrimSpace(strings.Split(first, ";")[0])
	first = strings.ReplaceAll(first, "_", "-")
	parts := strings.Split(first, "-")
	if len(parts) > 0 {
		language = strings.ToLower(parts[0])
	}
	if len(parts) > 1 {
		region = strings.ToUpper(parts[1])
	}
	return raw, language, region
}

func referrerDetails(referrer, requestHost string) (referrerHost, referrerPath string, internal bool) {
	referrer = strings.TrimSpace(referrer)
	if referrer == "" {
		return "", "", false
	}
	parsed, err := url.Parse(referrer)
	if err != nil {
		return "", "", false
	}
	referrerHost = cleanHost(parsed.Host)
	if referrerHost == "" {
		return "", "", false
	}
	internal = referrerHost == requestHost
	if !internal {
		return referrerHost, "", false
	}
	return referrerHost, sanitizePath(parsed.EscapedPath()), true
}

func utmParams(query url.Values) map[string]string {
	result := map[string]string{}
	for _, key := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term"} {
		if value := limitString(strings.TrimSpace(query.Get(key)), 120); value != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func limitString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
