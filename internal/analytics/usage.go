package analytics

import (
	"sort"
	"strconv"
	"time"
)

const UsageEventName = "usage.http_request"

type UsageEvent struct {
	SchemaVersion    string            `json:"schema_version"`
	Event            string            `json:"event"`
	Timestamp        time.Time         `json:"timestamp"`
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	Host             string            `json:"host,omitempty"`
	Scheme           string            `json:"scheme,omitempty"`
	Status           int               `json:"status"`
	DurationMS       int64             `json:"duration_ms"`
	RequestBytes     int64             `json:"request_bytes,omitempty"`
	ResponseBytes    int64             `json:"response_bytes,omitempty"`
	AuthSurface      string            `json:"auth_surface"`
	Authenticated    bool              `json:"authenticated"`
	ClientHash       string            `json:"client_hash,omitempty"`
	ClientIPVersion  string            `json:"client_ip_version,omitempty"`
	ClientIPPrefix   string            `json:"client_ip_prefix,omitempty"`
	UserAgent        string            `json:"user_agent,omitempty"`
	Browser          string            `json:"browser,omitempty"`
	BrowserMajor     string            `json:"browser_major,omitempty"`
	OperatingSystem  string            `json:"operating_system,omitempty"`
	OSMajor          string            `json:"os_major,omitempty"`
	DeviceClass      string            `json:"device_class,omitempty"`
	Bot              bool              `json:"bot,omitempty"`
	AcceptLanguage   string            `json:"accept_language,omitempty"`
	Language         string            `json:"language,omitempty"`
	Region           string            `json:"region,omitempty"`
	ReferrerHost     string            `json:"referrer_host,omitempty"`
	ReferrerPath     string            `json:"referrer_path,omitempty"`
	ReferrerInternal bool              `json:"referrer_internal,omitempty"`
	UTM              map[string]string `json:"utm,omitempty"`
}

type Count struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type UsageRequestStats struct {
	Total       int `json:"total"`
	Success     int `json:"success"`
	Redirect    int `json:"redirect"`
	ClientError int `json:"client_error"`
	ServerError int `json:"server_error"`
}

type UsageStats struct {
	GeneratedAt        time.Time         `json:"generated_at"`
	Since              time.Time         `json:"since"`
	Until              time.Time         `json:"until"`
	EventCount         int               `json:"event_count"`
	Truncated          bool              `json:"truncated"`
	Requests           UsageRequestStats `json:"requests"`
	ByPath             []Count           `json:"by_path"`
	ByMethod           []Count           `json:"by_method"`
	ByStatus           []Count           `json:"by_status"`
	ByAuthSurface      []Count           `json:"by_auth_surface"`
	ByUserAgent        []Count           `json:"by_user_agent"`
	ByBrowser          []Count           `json:"by_browser,omitempty"`
	ByOperatingSystem  []Count           `json:"by_operating_system,omitempty"`
	ByDeviceClass      []Count           `json:"by_device_class,omitempty"`
	ByLanguage         []Count           `json:"by_language,omitempty"`
	ByRegion           []Count           `json:"by_region,omitempty"`
	ByReferrerHost     []Count           `json:"by_referrer_host,omitempty"`
	ByUTMSource        []Count           `json:"by_utm_source,omitempty"`
	ByUTMCampaign      []Count           `json:"by_utm_campaign,omitempty"`
	ByClientIPPrefix   []Count           `json:"by_client_ip_prefix,omitempty"`
	Daily              []Count           `json:"daily"`
	UniqueClientHashes int               `json:"unique_client_hashes,omitempty"`
}

func NewUsageEvent(now time.Time) UsageEvent {
	return UsageEvent{
		SchemaVersion: SchemaVersion,
		Event:         UsageEventName,
		Timestamp:     now.UTC(),
	}
}

func SummarizeUsageEvents(events []UsageEvent, since, until time.Time, truncated bool) UsageStats {
	stats := UsageStats{
		GeneratedAt: time.Now().UTC(),
		Since:       since.UTC(),
		Until:       until.UTC(),
		Truncated:   truncated,
	}
	paths := map[string]int{}
	methods := map[string]int{}
	statuses := map[string]int{}
	authSurfaces := map[string]int{}
	userAgents := map[string]int{}
	browsers := map[string]int{}
	operatingSystems := map[string]int{}
	deviceClasses := map[string]int{}
	languages := map[string]int{}
	regions := map[string]int{}
	referrerHosts := map[string]int{}
	utmSources := map[string]int{}
	utmCampaigns := map[string]int{}
	clientIPPrefixes := map[string]int{}
	daily := map[string]int{}
	clientHashes := map[string]struct{}{}

	for _, event := range events {
		if !since.IsZero() && event.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && event.Timestamp.After(until) {
			continue
		}
		stats.EventCount++
		stats.Requests.Total++
		switch {
		case event.Status >= 500:
			stats.Requests.ServerError++
		case event.Status >= 400:
			stats.Requests.ClientError++
		case event.Status >= 300:
			stats.Requests.Redirect++
		default:
			stats.Requests.Success++
		}
		increment(paths, event.Path)
		increment(methods, event.Method)
		increment(statuses, strconv.Itoa(event.Status))
		increment(authSurfaces, event.AuthSurface)
		increment(userAgents, event.UserAgent)
		increment(browsers, event.Browser)
		increment(operatingSystems, event.OperatingSystem)
		increment(deviceClasses, event.DeviceClass)
		increment(languages, event.Language)
		increment(regions, event.Region)
		increment(referrerHosts, event.ReferrerHost)
		if event.UTM != nil {
			increment(utmSources, event.UTM["utm_source"])
			increment(utmCampaigns, event.UTM["utm_campaign"])
		}
		increment(clientIPPrefixes, event.ClientIPPrefix)
		if !event.Timestamp.IsZero() {
			increment(daily, event.Timestamp.UTC().Format("2006-01-02"))
		}
		if event.ClientHash != "" {
			clientHashes[event.ClientHash] = struct{}{}
		}
	}

	stats.ByPath = sortedCounts(paths)
	stats.ByMethod = sortedCounts(methods)
	stats.ByStatus = sortedCounts(statuses)
	stats.ByAuthSurface = sortedCounts(authSurfaces)
	stats.ByUserAgent = sortedCounts(userAgents)
	stats.ByBrowser = sortedCounts(browsers)
	stats.ByOperatingSystem = sortedCounts(operatingSystems)
	stats.ByDeviceClass = sortedCounts(deviceClasses)
	stats.ByLanguage = sortedCounts(languages)
	stats.ByRegion = sortedCounts(regions)
	stats.ByReferrerHost = sortedCounts(referrerHosts)
	stats.ByUTMSource = sortedCounts(utmSources)
	stats.ByUTMCampaign = sortedCounts(utmCampaigns)
	stats.ByClientIPPrefix = sortedCounts(clientIPPrefixes)
	stats.Daily = sortedCounts(daily)
	stats.UniqueClientHashes = len(clientHashes)
	return stats
}

func increment(counts map[string]int, key string) {
	if key == "" {
		key = "unknown"
	}
	counts[key]++
}

func sortedCounts(counts map[string]int) []Count {
	result := make([]Count, 0, len(counts))
	for key, count := range counts {
		result = append(result, Count{Key: key, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count == result[j].Count {
			return result[i].Key < result[j].Key
		}
		return result[i].Count > result[j].Count
	})
	return result
}
