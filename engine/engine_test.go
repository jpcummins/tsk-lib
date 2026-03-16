package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jpcummins/tsk-lib/model"
)

func TestEngine_Index_MinimalTodo(t *testing.T) {
	root := findSpecExample(t, "minimal-todo")
	e, err := NewDefault(":memory:")
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	defer e.Close()

	repo, err := e.Index(root)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if len(repo.Tasks) != 6 {
		t.Errorf("expected 6 tasks, got %d", len(repo.Tasks))
	}
}

func TestEngine_Index_ComplexShoppingCart(t *testing.T) {
	root := findSpecExample(t, "complex-shopping-cart")
	e, err := NewDefault(":memory:", WithCurrentUser("alice@example.com"))
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	defer e.Close()

	repo, err := e.Index(root)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if len(repo.Tasks) < 20 {
		t.Errorf("expected at least 20 tasks, got %d", len(repo.Tasks))
	}
	if len(repo.Iterations) != 16 {
		t.Errorf("expected 16 iterations, got %d", len(repo.Iterations))
	}
	if len(repo.SLARules) != 2 {
		t.Errorf("expected 2 SLA rules, got %d", len(repo.SLARules))
	}
}

// TestEngine_Search_SpecQueries runs spec example queries against real indexed data.
func TestEngine_Search_SpecQueries(t *testing.T) {
	root := findSpecExample(t, "complex-shopping-cart")
	e, err := NewDefault(":memory:", WithCurrentUser("alice@example.com"))
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	defer e.Close()

	if _, err := e.Index(root); err != nil {
		t.Fatalf("Index: %v", err)
	}

	tests := []struct {
		name     string
		query    string
		minCount int // Minimum expected results.
		check    func(t *testing.T, tasks []*model.Task)
	}{
		{
			name:     "security tasks",
			query:    `summary ~ "security" AND status.category != done`,
			minCount: 0,
		},
		{
			name:     "team:backend assignee",
			query:    `assignee = "team:backend" AND status.category != done`,
			minCount: 1,
			check: func(t *testing.T, tasks []*model.Task) {
				for _, task := range tasks {
					if task.Assignee != "team:backend" {
						t.Errorf("task %s has assignee %q, expected team:backend",
							task.CanonicalPath, task.Assignee)
					}
				}
			},
		},
		{
			name:     "missing assignee and open",
			query:    `missing(assignee) AND status.category != done`,
			minCount: 1,
			check: func(t *testing.T, tasks []*model.Task) {
				for _, task := range tasks {
					if task.Assignee != "" {
						t.Errorf("task %s has assignee %q, expected empty",
							task.CanonicalPath, task.Assignee)
					}
				}
			},
		},
		{
			name:     "capitalizable tasks",
			query:    `has(labels, "capitalizable")`,
			minCount: 10,
			check: func(t *testing.T, tasks []*model.Task) {
				for _, task := range tasks {
					if !hasLabel(task.Labels, "capitalizable") {
						t.Errorf("task %s missing 'capitalizable' label, has %v",
							task.CanonicalPath, task.Labels)
					}
				}
			},
		},
		{
			name:     "capitalizable excluding opt-outs",
			query:    `has(labels, "capitalizable") AND NOT has(labels, "not-capitalizable")`,
			minCount: 10,
			check: func(t *testing.T, tasks []*model.Task) {
				for _, task := range tasks {
					if hasLabel(task.Labels, "not-capitalizable") {
						t.Errorf("task %s should be excluded (has not-capitalizable)",
							task.CanonicalPath)
					}
				}
			},
		},
		{
			name:     "done tasks",
			query:    `status.category = done`,
			minCount: 1,
			check: func(t *testing.T, tasks []*model.Task) {
				for _, task := range tasks {
					if task.StatusCategory != model.StatusCategoryDone {
						t.Errorf("task %s has category %q, expected done",
							task.CanonicalPath, task.StatusCategory)
					}
				}
			},
		},
		{
			name:     "in_progress tasks with missing estimate",
			query:    `status.category = in_progress AND missing(estimate)`,
			minCount: 0,
		},
		{
			name:     "support bugs",
			query:    `path ~ "support/" AND status.category != done`,
			minCount: 2,
		},
		{
			name:     "incident tasks",
			query:    `summary ~ "incident"`,
			minCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := e.Search(tt.query)
			if err != nil {
				t.Fatalf("Search(%q): %v", tt.query, err)
			}

			if len(tasks) < tt.minCount {
				t.Errorf("expected at least %d results, got %d", tt.minCount, len(tasks))
				for _, task := range tasks {
					t.Logf("  - %s (status=%s, category=%s)",
						task.CanonicalPath, task.Status, task.StatusCategory)
				}
			}

			if tt.check != nil {
				tt.check(t, tasks)
			}

			t.Logf("query: %s -> %d results", tt.query, len(tasks))
		})
	}
}

// TestEngine_Search_Iteration tests queries involving iteration fields.
func TestEngine_Search_Iteration(t *testing.T) {
	root := findSpecExample(t, "complex-shopping-cart")
	e, err := NewDefault(":memory:")
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	defer e.Close()

	if _, err := e.Index(root); err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Find tasks in in_progress iterations.
	tasks, err := e.Search(`iteration.status = in_progress`)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(tasks) < 1 {
		t.Errorf("expected at least 1 task in in_progress iterations, got %d", len(tasks))
	}

	t.Logf("tasks in in_progress iterations: %d", len(tasks))
	for _, task := range tasks {
		t.Logf("  - %s", task.CanonicalPath)
	}
}

// ── Helpers ───────────────────────────────────────────────────────

func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

func findSpecExample(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd(): %v", err)
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
