package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jp/tsk-lib/model"
	"github.com/jp/tsk-lib/parse"
	"github.com/jp/tsk-lib/scan"
)

func TestWriteAndRead_MinimalTodo(t *testing.T) {
	repo := loadExample(t, "minimal-todo")

	st, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if err := st.WriteRepository(repo); err != nil {
		t.Fatalf("WriteRepository: %v", err)
	}

	// Read all tasks back.
	tasks, err := st.AllTasks()
	if err != nil {
		t.Fatalf("AllTasks: %v", err)
	}
	if len(tasks) != 6 {
		t.Errorf("expected 6 tasks, got %d", len(tasks))
	}

	// Read a specific task.
	task, err := st.TaskByPath("todo-app/add-item")
	if err != nil {
		t.Fatalf("TaskByPath: %v", err)
	}
	if task.Summary != "Add new todo items" {
		t.Errorf("summary = %q, want %q", task.Summary, "Add new todo items")
	}
	if task.Status != "in_progress" {
		t.Errorf("status = %q, want %q", task.Status, "in_progress")
	}
	if len(task.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(task.Dependencies))
	}
}

func TestWriteAndRead_ComplexShoppingCart(t *testing.T) {
	repo := loadExample(t, "complex-shopping-cart")

	st, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if err := st.WriteRepository(repo); err != nil {
		t.Fatalf("WriteRepository: %v", err)
	}

	// Tasks round-trip.
	tasks, err := st.AllTasks()
	if err != nil {
		t.Fatalf("AllTasks: %v", err)
	}
	if len(tasks) < 20 {
		t.Errorf("expected at least 20 tasks, got %d", len(tasks))
	}

	// Verify labels persisted.
	cartUI, err := st.TaskByPath("shopping-cart/frontend/cart-ui-shell")
	if err != nil {
		t.Fatalf("TaskByPath: %v", err)
	}
	if !hasLabel(cartUI.Labels, "capitalizable") {
		t.Errorf("cart-ui-shell should have 'capitalizable' label, got %v", cartUI.Labels)
	}

	// Verify status category.
	endpoints, err := st.TaskByPath("shopping-cart/backend/cart-service-endpoints")
	if err != nil {
		t.Fatalf("TaskByPath: %v", err)
	}
	if endpoints.StatusCategory != model.StatusCategoryInProgress {
		t.Errorf("status_category = %q, want %q",
			endpoints.StatusCategory, model.StatusCategoryInProgress)
	}

	// Verify iterations.
	iters, err := st.IterationsByTeam("frontend")
	if err != nil {
		t.Fatalf("IterationsByTeam: %v", err)
	}
	if len(iters) != 4 {
		t.Errorf("expected 4 frontend iterations, got %d", len(iters))
	}

	// Verify team members.
	members, err := st.TeamMembers("frontend")
	if err != nil {
		t.Fatalf("TeamMembers: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("expected 3 frontend members, got %d", len(members))
	}
}

func TestQueryTasks(t *testing.T) {
	repo := loadExample(t, "complex-shopping-cart")

	st, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if err := st.WriteRepository(repo); err != nil {
		t.Fatalf("WriteRepository: %v", err)
	}

	// Query tasks with status_category = 'done'.
	query := `
		SELECT canonical_path, parent_path, date, due, assignee, summary,
			estimate_mins, status, status_category, updated_at, weight, body, is_readme
		FROM tasks WHERE status_category = ?`

	tasks, err := st.QueryTasks(query, []any{"done"})
	if err != nil {
		t.Fatalf("QueryTasks: %v", err)
	}

	// All returned tasks should have done category.
	for _, task := range tasks {
		if task.StatusCategory != model.StatusCategoryDone {
			t.Errorf("task %s has category %q, expected done",
				task.CanonicalPath, task.StatusCategory)
		}
	}

	if len(tasks) == 0 {
		t.Error("expected at least one done task")
	}
}

// ── Helpers ───────────────────────────────────────────────────────

func loadExample(t *testing.T, name string) *model.Repository {
	t.Helper()
	root := findSpecExample(t, name)
	s := scan.NewFSScanner()
	entries, err := s.Scan(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	p := parse.NewParser()
	repo, err := p.Parse(entries)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return repo
}

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
