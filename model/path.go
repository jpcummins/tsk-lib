// Package model defines the core domain types for the tsk system.
// It has zero external dependencies — every other package imports from here.
package model

import (
	"regexp"
	"strings"
)

// CanonicalPath is the normalized, extensionless path relative to tasks/.
// It is the canonical identity for a task (Section 2).
type CanonicalPath string

var nonAlphanumDash = regexp.MustCompile(`[^a-z0-9\-]`)

// NormalizePath applies the canonical path normalization rules from Section 2.1:
//   - Normalize separators to /
//   - Trim leading/trailing /
//   - Lowercase
//   - Remove file extension
//   - Replace spaces with - in each segment
//   - Remove non-alphanumeric characters except - in each segment
//   - Preserve / between segments
func NormalizePath(raw string) CanonicalPath {
	// Normalize separators.
	p := strings.ReplaceAll(raw, "\\", "/")

	// Trim leading/trailing slashes.
	p = strings.Trim(p, "/")

	// Lowercase.
	p = strings.ToLower(p)

	// Remove file extension from the last segment.
	p = trimExtension(p)

	// Process each segment.
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		// Replace spaces with dashes.
		seg = strings.ReplaceAll(seg, " ", "-")
		// Remove non-alphanumeric characters except dash.
		seg = nonAlphanumDash.ReplaceAllString(seg, "")
		segments[i] = seg
	}

	return CanonicalPath(strings.Join(segments, "/"))
}

// trimExtension removes the file extension from the last segment.
// Only common tsk extensions are stripped (.md, .toml).
func trimExtension(p string) string {
	for _, ext := range []string{".md", ".toml"} {
		if strings.HasSuffix(p, ext) {
			return strings.TrimSuffix(p, ext)
		}
	}
	return p
}

// String returns the string representation of the canonical path.
func (cp CanonicalPath) String() string {
	return string(cp)
}

// Parent returns the parent path, or empty string if at root level.
func (cp CanonicalPath) Parent() CanonicalPath {
	s := string(cp)
	i := strings.LastIndex(s, "/")
	if i < 0 {
		return ""
	}
	return CanonicalPath(s[:i])
}

// Base returns the last segment of the path.
func (cp CanonicalPath) Base() string {
	s := string(cp)
	i := strings.LastIndex(s, "/")
	if i < 0 {
		return s
	}
	return s[i+1:]
}

// IsEmpty returns true if the path is empty.
func (cp CanonicalPath) IsEmpty() bool {
	return string(cp) == ""
}

// HasPrefix returns true if the path starts with the given prefix.
func (cp CanonicalPath) HasPrefix(prefix CanonicalPath) bool {
	s := string(cp)
	p := string(prefix)
	if len(p) == 0 {
		return true
	}
	return s == p || strings.HasPrefix(s, p+"/")
}
