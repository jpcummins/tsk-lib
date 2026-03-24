package scan

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Scanner walks a tsk repository and classifies files.
type Scanner interface {
	Scan(ctx context.Context, root string) ([]Entry, error)
}

// FSScanner implements Scanner by walking the real filesystem.
type FSScanner struct{}

// NewFSScanner creates a new filesystem scanner.
func NewFSScanner() *FSScanner {
	return &FSScanner{}
}

// Scan walks the root directory and returns classified entries.
func (s *FSScanner) Scan(ctx context.Context, root string) ([]Entry, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var entries []Entry

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			return err
		}

		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		// Only process files
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		kind, ok := classify(rel)
		if !ok {
			return nil // skip unrecognized files
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		entries = append(entries, Entry{
			Kind:    kind,
			Path:    rel,
			Content: content,
		})

		return nil
	})

	return entries, err
}

// MemScanner implements Scanner using an in-memory filesystem.
// Used for testing with conformance test fixtures.
type MemScanner struct {
	Files map[string]string
}

// NewMemScanner creates a scanner from a map of path -> content.
func NewMemScanner(files map[string]string) *MemScanner {
	return &MemScanner{Files: files}
}

// Scan classifies all files in the in-memory filesystem.
func (s *MemScanner) Scan(ctx context.Context, _ string) ([]Entry, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var entries []Entry

	for path, content := range s.Files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path = filepath.ToSlash(path)

		kind, ok := classify(path)
		if !ok {
			continue
		}

		entries = append(entries, Entry{
			Kind:    kind,
			Path:    path,
			Content: []byte(content),
		})
	}

	return entries, nil
}

// classify determines the EntryKind for a given relative path.
func classify(rel string) (EntryKind, bool) {
	parts := strings.Split(rel, "/")

	// Root tsk.toml
	if rel == "tsk.toml" {
		return EntryRootConfig, true
	}

	// Root sla.toml
	if rel == "sla.toml" {
		return EntrySLA, true
	}

	// tasks/ hierarchy
	if len(parts) >= 2 && parts[0] == "tasks" {
		name := parts[len(parts)-1]

		// .md files are tasks
		if strings.HasSuffix(name, ".md") {
			return EntryTask, true
		}

		return EntryKind(0), false
	}

	// teams/ hierarchy
	if len(parts) >= 2 && parts[0] == "teams" {
		name := parts[len(parts)-1]

		// teams/<team>/team.toml
		if len(parts) == 3 && name == "team.toml" {
			return EntryTeamConfig, true
		}

		// teams/<team>/iterations/*.md
		if len(parts) == 4 && parts[2] == "iterations" && strings.HasSuffix(name, ".md") {
			return EntryIteration, true
		}

		return EntryKind(0), false
	}

	return EntryKind(0), false
}
