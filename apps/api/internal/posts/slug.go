package posts

import (
	"regexp"
	"strings"
)

// slugRegex mirrors the DB CHECK constraint:
//
//	^[a-z0-9]+(-[a-z0-9]+)*$
var slugRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// validSlug reports whether s is a non-empty, schema-conforming slug ≤200 chars.
func validSlug(s string) bool {
	if s == "" || len(s) > 200 {
		return false
	}
	return slugRegex.MatchString(s)
}

// nonAlphaNum matches any character that is NOT [a-z0-9]. Note: input is
// lowercased before this regex runs.
var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify lowercases s, replaces every run of non-[a-z0-9] characters with a
// single '-', and trims leading/trailing '-'. Output is truncated to 200 chars
// at a hyphen boundary when possible. May return "" if no usable chars.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 200 {
		s = s[:200]
		s = strings.TrimRight(s, "-")
		// If truncation created a partial trailing chunk, cut at the last '-'.
		if i := strings.LastIndex(s, "-"); i > 0 && len(s)-i < 4 {
			s = s[:i]
		}
	}
	return s
}
