package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFSScanner_MinimalTodo(t *testing.T) {
	root := findSpecExample(t, "minimal-todo")
	scanner := NewFSScanner()
	entries, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan(%q) error: %v", root, err)
	}

	// Expected: .config.toml + 6 task files = 7 entries.
	// (README.md at root is not under tasks/ so it's skipped.)
	counts := countByKind(entries)

	if counts[EntryRootConfig] != 1 {
		t.Errorf("expected 1 root config, got %d", counts[EntryRootConfig])
	}
	if counts[EntryTask] != 6 {
		t.Errorf("expected 6 tasks, got %d", counts[EntryTask])
	}
	if counts[EntrySLA] != 0 {
		t.Errorf("expected 0 SLA files, got %d", counts[EntrySLA])
	}
}

func TestFSScanner_ComplexShoppingCart(t *testing.T) {
	root := findSpecExample(t, "complex-shopping-cart")
	scanner := NewFSScanner()
	entries, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan(%q) error: %v", root, err)
	}

	counts := countByKind(entries)

	// Verify each kind has expected entries.
	if counts[EntryRootConfig] != 1 {
		t.Errorf("expected 1 root config, got %d", counts[EntryRootConfig])
	}
	if counts[EntrySLA] != 1 {
		t.Errorf("expected 1 SLA file, got %d", counts[EntrySLA])
	}
	if counts[EntryProjectConfig] != 1 {
		t.Errorf("expected 1 project config, got %d", counts[EntryProjectConfig])
	}

	// Teams: frontend, backend, billing, checkout — but only frontend has team.toml.
	if counts[EntryTeamConfig] < 1 {
		t.Errorf("expected at least 1 team config, got %d", counts[EntryTeamConfig])
	}

	// 4 teams * 4 iterations = 16 iteration files.
	if counts[EntryIteration] != 16 {
		t.Errorf("expected 16 iterations, got %d", counts[EntryIteration])
	}

	// Tasks: shopping-cart (README + 3*backend + 4*frontend + 3*billing + 3*checkout)
	//        + incidents (3 READMEs + 3+3+3 subtasks)
	//        + support (3 tasks)
	// = 1 + 3 + 4 + 3 + 3 + 3 + 3 + 3 + 3 + 3 + 3 = many
	// Just check we got a reasonable number.
	if counts[EntryTask] < 20 {
		t.Errorf("expected at least 20 tasks, got %d", counts[EntryTask])
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		path string
		kind EntryKind
		ok   bool
	}{
		{".config.toml", EntryRootConfig, true},
		{"sla.toml", EntrySLA, true},
		{"tasks/todo-app/add-item.md", EntryTask, true},
		{"tasks/shopping-cart/README.md", EntryTask, true},
		{"tasks/shopping-cart/backend/.config.toml", EntryProjectConfig, true},
		{"teams/frontend/team.toml", EntryTeamConfig, true},
		{"teams/frontend/iterations/2026-03-17.md", EntryIteration, true},
		{"README.md", 0, false},        // Root README is not a tsk file.
		{"tasks/foo.txt", 0, false},    // Not a .md file.
		{"teams/foo/bar.md", 0, false}, // Not in iterations/.
		{"random/file.md", 0, false},   // Not under tasks/ or teams/.
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			kind, ok := classify(tt.path)
			if ok != tt.ok {
				t.Errorf("classify(%q) ok = %v, want %v", tt.path, ok, tt.ok)
			}
			if ok && kind != tt.kind {
				t.Errorf("classify(%q) kind = %v, want %v", tt.path, kind, tt.kind)
			}
		})
	}
}

// findSpecExample locates a tsk-spec example directory.
func findSpecExample(t *testing.T, name string) string {
	t.Helper()

	// Walk up from the working directory to find tsk-spec.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error: %v", err)
	}

	for {
		candidate := filepath.Join(dir, "..", "tsk-spec", "example", name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Skipf("tsk-spec example %q not found", name)
	return ""
}

func countByKind(entries []Entry) map[EntryKind]int {
	m := make(map[EntryKind]int)
	for _, e := range entries {
		m[e.Kind]++
	}
	return m
}
