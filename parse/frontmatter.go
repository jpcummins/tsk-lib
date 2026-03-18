package parse

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// frontMatter represents the raw YAML front matter of a task or iteration.
type frontMatter struct {
	// Task fields
	CreatedAt    *string       `yaml:"created_at"`
	Due          *string       `yaml:"due"`
	Assignee     string        `yaml:"assignee"`
	Dependencies []string      `yaml:"dependencies"`
	Summary      string        `yaml:"summary"`
	Estimate     string        `yaml:"estimate"`
	Status       string        `yaml:"status"`
	UpdatedAt    *string       `yaml:"updated_at"`
	ChangeLog    []changeEntry `yaml:"change_log"`
	Labels       []string      `yaml:"labels"`
	Type         string        `yaml:"type"`
	Weight       *float64      `yaml:"weight"`

	// Redirect stub
	RedirectTo string `yaml:"redirect_to"`

	// Iteration fields
	Start string   `yaml:"start"`
	End   string   `yaml:"end"`
	Tasks []string `yaml:"tasks"`
}

// changeEntry represents a raw change_log entry.
type changeEntry struct {
	Field string `yaml:"field"`
	From  string `yaml:"from"`
	To    string `yaml:"to"`
	At    string `yaml:"at"`
}

// extractFrontMatter splits a markdown file into front matter and body.
// Returns the parsed front matter and the body content.
func extractFrontMatter(data []byte) (*frontMatter, string, error) {
	// Don't trim — preserve the original content structure
	content := data

	// Must start with ---
	trimmed := bytes.TrimLeft(content, " \t")
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return &frontMatter{}, string(content), nil
	}

	// Handle bare "---" (empty front matter)
	stripped := bytes.TrimSpace(trimmed)
	if bytes.Equal(stripped, []byte("---")) {
		return &frontMatter{}, "", nil
	}

	// Skip the opening ---
	rest := trimmed[3:]

	// Skip optional newline after opening ---
	rest = skipNewline(rest)

	// Find closing --- on its own line
	closeIdx := findClosingFence(rest)
	if closeIdx < 0 {
		// The entire remaining content might be front matter
		// if there's a --- at the very end
		endTrimmed := bytes.TrimRight(rest, " \t\r\n")
		if bytes.HasSuffix(endTrimmed, []byte("---")) {
			// Check if --- is at beginning of line
			yamlContent := endTrimmed[:len(endTrimmed)-3]
			yamlContent = bytes.TrimRight(yamlContent, " \t\r\n")
			var fm frontMatter
			if err := yaml.Unmarshal(yamlContent, &fm); err != nil {
				return nil, "", fmt.Errorf("parsing front matter: %w", err)
			}
			return &fm, "", nil
		}
		return nil, "", fmt.Errorf("unclosed front matter")
	}

	yamlContent := rest[:closeIdx]
	remaining := rest[closeIdx:]

	// Skip the closing ---
	remaining = remaining[3:]

	// Skip the rest of the line (any trailing content on the --- line)
	if idx := bytes.IndexByte(remaining, '\n'); idx >= 0 {
		remaining = remaining[idx+1:]
	} else {
		remaining = nil
	}

	var fm frontMatter
	if len(bytes.TrimSpace(yamlContent)) > 0 {
		if err := yaml.Unmarshal(yamlContent, &fm); err != nil {
			return nil, "", fmt.Errorf("parsing front matter: %w", err)
		}
	}

	return &fm, string(remaining), nil
}

// findClosingFence finds the position of the closing --- fence.
// Returns the index in rest where "---" starts (at beginning of a line),
// or -1 if not found.
func findClosingFence(rest []byte) int {
	pos := 0
	for pos < len(rest) {
		lineStart := pos

		// Find end of this line
		eol := bytes.IndexByte(rest[pos:], '\n')
		var line []byte
		if eol >= 0 {
			line = rest[pos : pos+eol]
			pos = pos + eol + 1
		} else {
			line = rest[pos:]
			pos = len(rest)
		}

		// Check if this line is just ---
		trimmedLine := bytes.TrimRight(line, " \t\r")
		if bytes.Equal(trimmedLine, []byte("---")) {
			return lineStart
		}
	}
	return -1
}

func skipNewline(data []byte) []byte {
	if len(data) > 0 && data[0] == '\n' {
		return data[1:]
	}
	if len(data) > 1 && data[0] == '\r' && data[1] == '\n' {
		return data[2:]
	}
	return data
}
