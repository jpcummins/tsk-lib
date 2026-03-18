package model

import (
	"path/filepath"
	"regexp"
	"strings"
)

// CanonicalPath is the normalized, extensionless path relative to tasks/.
// It serves as the primary identity for tasks.
type CanonicalPath string

var nonAlphanumHyphenUnderscore = regexp.MustCompile(`[^a-z0-9_-]`)

// NormalizePath converts a raw filesystem path into a canonical path per spec 1.2.1.
// The input may optionally include a "tasks/" prefix, which is stripped.
//
// Steps:
//  1. Normalize separators to /
//  2. Trim leading/trailing /
//  3. Strip "tasks/" prefix if present
//  4. Lowercase (README.md special-cased)
//  5. Remove file extension
//  6. Replace spaces with - within each segment
//  7. Replace non-alphanumeric chars (except -) with -
//  8. Collapse multiple hyphens and trim leading/trailing hyphens
//  9. Preserve / between segments
func NormalizePath(raw string) CanonicalPath {
	// Step 1: normalize separators
	p := filepath.ToSlash(raw)

	// Step 2: trim leading/trailing /
	p = strings.Trim(p, "/")

	if p == "" {
		return ""
	}

	// Step 3: strip "tasks/" prefix if present
	p = strings.TrimPrefix(p, "tasks/")

	segments := strings.Split(p, "/")
	result := make([]string, 0, len(segments))

	for _, seg := range segments {
		// Step 4: lowercase (README.md handling)
		lower := strings.ToLower(seg)

		// README.md resolves to parent — skip this segment
		if lower == "readme.md" || lower == "readme" {
			continue
		}

		// Step 5: remove file extension
		if ext := filepath.Ext(lower); ext != "" {
			lower = strings.TrimSuffix(lower, ext)
		}

		// Step 6: replace spaces with -
		lower = strings.ReplaceAll(lower, " ", "-")

		// Step 7: replace non-alphanumeric (except - and _) with -
		lower = nonAlphanumHyphenUnderscore.ReplaceAllString(lower, "-")

		// Step 8: collapse multiple hyphens and trim
		lower = collapseHyphens(lower)

		if lower != "" {
			result = append(result, lower)
		}
	}

	return CanonicalPath(strings.Join(result, "/"))
}

// collapseHyphens replaces runs of multiple hyphens with a single hyphen
// and trims leading/trailing hyphens.
func collapseHyphens(s string) string {
	var b strings.Builder
	prevHyphen := false
	for _, c := range s {
		if c == '-' {
			if !prevHyphen {
				b.WriteRune(c)
			}
			prevHyphen = true
		} else {
			b.WriteRune(c)
			prevHyphen = false
		}
	}
	return strings.Trim(b.String(), "-")
}

// String returns the string representation of the canonical path.
func (p CanonicalPath) String() string {
	return string(p)
}

// Parent returns the parent path, or empty if at root level.
func (p CanonicalPath) Parent() CanonicalPath {
	s := string(p)
	i := strings.LastIndex(s, "/")
	if i < 0 {
		return ""
	}
	return CanonicalPath(s[:i])
}

// Base returns the last segment of the path.
func (p CanonicalPath) Base() string {
	s := string(p)
	i := strings.LastIndex(s, "/")
	if i < 0 {
		return s
	}
	return s[i+1:]
}

// HasPrefix returns true if p starts with prefix (as a path prefix, not string prefix).
func (p CanonicalPath) HasPrefix(prefix CanonicalPath) bool {
	if prefix == "" {
		return true
	}
	ps := string(p)
	prs := string(prefix)
	if ps == prs {
		return true
	}
	return strings.HasPrefix(ps, prs+"/")
}

// IsEmpty returns true if the path is empty.
func (p CanonicalPath) IsEmpty() bool {
	return p == ""
}

// Depth returns the number of segments in the path (0 for empty).
func (p CanonicalPath) Depth() int {
	if p == "" {
		return 0
	}
	return strings.Count(string(p), "/") + 1
}

// ContainsUppercase checks if a raw path (before normalization) contains
// uppercase characters (excluding README.md and the tasks/ prefix).
// Used for PATH_UPPERCASE warnings.
func ContainsUppercase(raw string) bool {
	p := filepath.ToSlash(raw)
	p = strings.Trim(p, "/")
	p = strings.TrimPrefix(p, "tasks/")
	segments := strings.Split(p, "/")
	for _, seg := range segments {
		lower := strings.ToLower(seg)
		if lower == "readme.md" {
			continue
		}
		if seg != lower {
			return true
		}
	}
	return false
}
