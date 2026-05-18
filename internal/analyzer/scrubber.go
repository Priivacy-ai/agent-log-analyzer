package analyzer

import (
	"bytes"
	"regexp"
	"strings"
)

type secretRule struct {
	name string
	re   *regexp.Regexp
}

var secretRules = []secretRule{
	{"api_key", regexp.MustCompile(`(?i)(api[_-]?key|token|secret)["'=:\s]+([A-Za-z0-9_\-]{24,})`)},
	{"anthropic_key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{20,}`)},
	{"openai_key", regexp.MustCompile(`sk-[A-Za-z0-9]{32,}`)},
	{"github_token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{30,}`)},
	{"npm_token", regexp.MustCompile(`npm_[A-Za-z0-9]{36,}`)},
	{"aws_access_key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"google_api_key", regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`)},
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`)},
	{"ssh_private_key", regexp.MustCompile(`-----BEGIN OPENSSH PRIVATE KEY-----[\s\S]*?-----END OPENSSH PRIVATE KEY-----`)},
	{"private_key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`)},
	{"database_url", regexp.MustCompile(`(?i)(postgres|postgresql|mysql|mongodb|redis)://[^/\s:@]+:[^@\s]+@[^"\s]+`)},
	{"url_credential", regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^/\s:@]+:[^@\s]+@`)},
	{"cookie", regexp.MustCompile(`(?i)(cookie|set-cookie)["'=:\s]+[^"\n;]+=[^"\n;]+`)},
	{"email", regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)},
}

func Scrub(input []byte) ([]byte, map[string]int) {
	output := bytes.Clone(input)
	counts := map[string]int{}
	for _, rule := range secretRules {
		matches := rule.re.FindAll(output, -1)
		if len(matches) == 0 {
			continue
		}
		counts[rule.name] += len(matches)
		marker := "[REDACTED-" + strings.ToUpper(strings.ReplaceAll(rule.name, "_", "-")) + "]"
		output = rule.re.ReplaceAll(output, []byte(marker))
	}
	return output, counts
}
