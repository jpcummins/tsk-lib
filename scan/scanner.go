package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Scanner walks a tsk repository and yields classified entries.
type Scanner interface {
	Scan(root string) ([]Entry, error)
}

// FSScanner is the default filesystem-based Scanner implementation.
type FSScanner struct{}

// NewFSScanner returns a new filesystem scanner.
func NewFSScanner() *FSScanner {
	return &FSScanner{}
}

// Scan walks the given root directory and returns classified entries.
// It reads each recognized file into memory for downstream parsing.
func (s *FSScanner) Scan(root string) ([]Entry, error) {
	var entries []Entry

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories (e.g., .git) but not hidden files at root.
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Compute the relative path with forward slashes.
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		kind, ok := classify(rel)
		if !ok {
			return nil // Unrecognized file, skip.
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		entries = append(entries, Entry{
			Path:    rel,
			Kind:    kind,
			Content: content,
		})

		return nil
	})

	return entries, err
}

// classify determines the EntryKind for a relative path.
// Returns false if the file is not a recognized tsk file.
func classify(rel string) (EntryKind, bool) {
	// Root-level files.
	switch rel {
	case ".config.toml":
		return EntryRootConfig, true
	case "sla.toml":
		return EntrySLA, true
	}

	// Tasks: anything under tasks/ that is a .md file.
	if strings.HasPrefix(rel, "tasks/") {
		if strings.HasSuffix(rel, ".config.toml") {
			return EntryProjectConfig, true
		}
		if strings.HasSuffix(rel, ".md") {
			return EntryTask, true
		}
		return 0, false
	}

	// Teams: team.toml or iterations.
	if strings.HasPrefix(rel, "teams/") {
		parts := strings.Split(rel, "/")

		// teams/<team>/team.toml
		if len(parts) == 3 && parts[2] == "team.toml" {
			return EntryTeamConfig, true
		}

		// teams/<team>/iterations/*.md
		if len(parts) == 4 && parts[2] == "iterations" && strings.HasSuffix(parts[3], ".md") {
			return EntryIteration, true
		}

		return 0, false
	}

	return 0, false
}
