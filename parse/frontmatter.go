// Package parse converts raw scanned entries into domain model objects.
// It handles YAML front matter, TOML configs, and multi-phase resolution
// (redirects, config inheritance, label union semantics).
package parse

import (
	"bytes"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// frontMatter is the raw YAML front matter from a Markdown file.
// Fields are parsed loosely — validation happens during model construction.
type frontMatter struct {
	Date         *time.Time       `yaml:"date"`
	Due          *time.Time       `yaml:"due"`
	UpdatedAt    *time.Time       `yaml:"updated_at"`
	Assignee     string           `yaml:"assignee"`
	Dependencies []string         `yaml:"dependencies"`
	Summary      string           `yaml:"summary"`
	Estimate     string           `yaml:"estimate"`
	Status       string           `yaml:"status"`
	Labels       []string         `yaml:"labels"`
	Weight       *int             `yaml:"weight"`
	StatusLog    []statusLogEntry `yaml:"status_log"`
	RedirectTo   string           `yaml:"redirect_to"`

	// Iteration-specific fields.
	Name     string     `yaml:"name"`
	Team     string     `yaml:"team"`
	Start    *time.Time `yaml:"start"`
	End      *time.Time `yaml:"end"`
	Capacity string     `yaml:"capacity"`
	Tasks    []string   `yaml:"tasks"`
}

type statusLogEntry struct {
	Status string    `yaml:"status"`
	At     time.Time `yaml:"at"`
}

var (
	fmDelimiter = []byte("---")
)

// extractFrontMatter splits a Markdown file into YAML front matter and body.
// Returns the parsed front matter and the body text after the closing ---.
func extractFrontMatter(content []byte) (*frontMatter, string, error) {
	content = bytes.TrimSpace(content)

	if !bytes.HasPrefix(content, fmDelimiter) {
		return nil, string(content), nil
	}

	// Find the closing delimiter.
	rest := content[len(fmDelimiter):]
	idx := bytes.Index(rest, fmDelimiter)
	if idx < 0 {
		return nil, "", fmt.Errorf("unclosed front matter delimiter")
	}

	yamlBytes := rest[:idx]
	body := rest[idx+len(fmDelimiter):]

	var fm frontMatter
	if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
		return nil, "", fmt.Errorf("parsing front matter YAML: %w", err)
	}

	return &fm, string(bytes.TrimSpace(body)), nil
}
