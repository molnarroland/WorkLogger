package core

import (
	"regexp"
	"strings"
)

// RE2 has no lookarounds, so the (?<![A-Za-z0-9]) … (?![A-Za-z0-9]) boundaries are checked by hand.
var keyAt = regexp.MustCompile(`^([A-Z][A-Z0-9_]{1,9})-\d+`)

// Prefixes that look like issue keys but never are.
var keyDeny = map[string]bool{
	"UTF": true, "ISO": true, "SHA": true, "RFC": true, "HTTP": true, "TLS": true, "SSL": true, "PSR": true, "CVE": true,
	"ES": true, "RS": true, "HS": true, "GPT": true, "MP": true, "PHP": true, "IPV": true, "X11": true, "COVID": true,
}

func isASCIIAlnum(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// FindKey finds the first issue key. Keys whose project prefix is known win over unknown ones.
func FindKey(text string, known map[string]bool) (string, bool) {
	fallback := ""
	for i := 0; i < len(text); i++ {
		if i > 0 && isASCIIAlnum(text[i-1]) {
			continue
		}
		m := keyAt.FindStringSubmatchIndex(text[i:])
		if m == nil {
			continue
		}
		end := i + m[1]
		if end < len(text) && isASCIIAlnum(text[end]) {
			continue
		}
		prefix, key := text[i+m[2]:i+m[3]], text[i:end]
		if known[prefix] {
			return key, true
		}
		if fallback == "" && !keyDeny[prefix] {
			fallback = key
		}
	}
	return fallback, fallback != ""
}

// NormalizeKey trims and upper-cases; empty means no issue.
func NormalizeKey(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.ToUpper(strings.TrimSpace(*s))
	if t == "" {
		return nil
	}
	return &t
}

func NormalizeKeyString(s string) *string { return NormalizeKey(&s) }

func KeyPrefix(key string) string { return strings.SplitN(key, "-", 2)[0] }

var leadingKey = regexp.MustCompile(`^\s*\[?[A-Z][A-Z0-9_]{1,9}-\d+\]?\s*[:\-–]?\s*`)

// StripLeadingKey: "ABC-12: Fix login" → "Fix login".
func StripLeadingKey(subject string) string { return leadingKey.ReplaceAllString(subject, "") }
