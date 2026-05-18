package analyzer

import (
	"bytes"
	"regexp"
)

type secretRule struct {
	name string
	re   *regexp.Regexp
}

var secretRules = []secretRule{
	{"api_key", regexp.MustCompile(`(?i)(api[_-]?key|token|secret)["'=:\s]+([A-Za-z0-9_\-]{24,})`)},
	{"anthropic_key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{20,}`)},
	{"openai_key", regexp.MustCompile(`sk-[A-Za-z0-9]{32,}`)},
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`)},
	{"private_key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`)},
	{"url_credential", regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^/\s:@]+:[^@\s]+@`)},
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
		output = rule.re.ReplaceAll(output, []byte("[REDACTED_"+rule.name+"]"))
	}
	return output, counts
}
