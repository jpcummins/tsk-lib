package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jp/tsk-lib/model"
	"github.com/jp/tsk-lib/scan"
)

func scanExample(t *testing.T, name string) []scan.Entry {
	t.Helper()
	root := findSpecExample(t, name)
	s := scan.NewFSScanner()
	entries, err := s.Scan(root)
	if err != nil {
		t.Fatalf("scan %s: %v", name, err)
	}
	return entries
}

func TestParse_MinimalTodo(t *testing.T) {
	entries := scanExample(t, "minimal-todo")
	parser := NewParser()
	repo, err := parser.Parse(entries)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(repo.Tasks) != 6 {
		t.Errorf("expected 6 tasks, got %d", len(repo.Tasks))
	}

	// Verify a specific task.
	task := findTask(repo, "todo-app/add-item")
	if task == nil {
		t.Fatal("task todo-app/add-item not found")
	}
	if task.Summary != "Add new todo items" {
		t.Errorf("summary = %q, want %q", task.Summary, "Add new todo items")
	}
	if task.Status != "in_progress" {
		t.Errorf("status = %q, want %q", task.Status, "in_progress")
	}
	if len(task.Dependencies) != 1 || task.Dependencies[0] != "todo-app/initialize-project" {
		t.Errorf("dependencies = %v, want [todo-app/initialize-project]", task.Dependencies)
	}
	if task.Estimate == nil || task.Estimate.Raw != "4h" {
		t.Errorf("estimate = %v, want 4h", task.Estimate)
	}

	// No teams, iterations, or SLA rules in minimal example.
	if len(repo.Teams) != 0 {
		t.Errorf("expected 0 teams, got %d", len(repo.Teams))
	}
	if len(repo.Iterations) != 0 {
		t.Errorf("expected 0 iterations, got %d", len(repo.Iterations))
	}
}

func TestParse_ComplexShoppingCart(t *testing.T) {
	entries := scanExample(t, "complex-shopping-cart")
	parser := NewParser()
	repo, err := parser.Parse(entries)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Should have many tasks.
	if len(repo.Tasks) < 20 {
		t.Errorf("expected at least 20 tasks, got %d", len(repo.Tasks))
	}

	// Should have 16 iterations (4 teams * 4 iterations).
	if len(repo.Iterations) != 16 {
		t.Errorf("expected 16 iterations, got %d", len(repo.Iterations))
	}

	// Should have at least 1 team config.
	if len(repo.Teams) < 1 {
		t.Errorf("expected at least 1 team, got %d", len(repo.Teams))
	}

	// Should have 2 SLA rules.
	if len(repo.SLARules) != 2 {
		t.Errorf("expected 2 SLA rules, got %d", len(repo.SLARules))
	}

	// Verify custom status mapping: backend tasks use "dev" -> "in_progress".
	endpoints := findTask(repo, "shopping-cart/backend/cart-service-endpoints")
	if endpoints == nil {
		t.Fatal("task shopping-cart/backend/cart-service-endpoints not found")
	}
	if endpoints.Status != "dev" {
		t.Errorf("status = %q, want %q", endpoints.Status, "dev")
	}
	if endpoints.StatusCategory != model.StatusCategoryInProgress {
		t.Errorf("status_category = %q, want %q",
			endpoints.StatusCategory, model.StatusCategoryInProgress)
	}

	// Verify label inheritance: shopping-cart/README.md has "capitalizable".
	// Subtasks should inherit it.
	cartUI := findTask(repo, "shopping-cart/frontend/cart-ui-shell")
	if cartUI == nil {
		t.Fatal("task shopping-cart/frontend/cart-ui-shell not found")
	}
	if !hasLabel(cartUI.Labels, "capitalizable") {
		t.Errorf("cart-ui-shell should inherit 'capitalizable' label, got %v", cartUI.Labels)
	}

	// Verify refund-flow has both "capitalizable" (inherited) and "not-capitalizable" (own).
	refund := findTask(repo, "shopping-cart/billing/refund-flow")
	if refund == nil {
		t.Fatal("task shopping-cart/billing/refund-flow not found")
	}
	if !hasLabel(refund.Labels, "capitalizable") {
		t.Errorf("refund-flow should have 'capitalizable', got %v", refund.Labels)
	}
	if !hasLabel(refund.Labels, "not-capitalizable") {
		t.Errorf("refund-flow should have 'not-capitalizable', got %v", refund.Labels)
	}

	// Verify iteration status category for frontend (uses custom "in_flight").
	var frontendIter *model.Iteration
	for _, iter := range repo.Iterations {
		if iter.Name == "Frontend Iteration 1" {
			frontendIter = iter
			break
		}
	}
	if frontendIter == nil {
		t.Fatal("Frontend Iteration 1 not found")
	}
	if frontendIter.Status != "in_flight" {
		t.Errorf("iteration status = %q, want %q", frontendIter.Status, "in_flight")
	}
	if frontendIter.StatusCategory != model.StatusCategoryInProgress {
		t.Errorf("iteration status_category = %q, want %q",
			frontendIter.StatusCategory, model.StatusCategoryInProgress)
	}

	// Verify team members parsed correctly.
	var frontendTeam *model.Team
	for _, team := range repo.Teams {
		if team.Name == "frontend" {
			frontendTeam = team
			break
		}
	}
	if frontendTeam == nil {
		t.Fatal("frontend team not found")
	}
	if len(frontendTeam.Members) != 3 {
		t.Errorf("expected 3 frontend members, got %d", len(frontendTeam.Members))
	}
	if frontendTeam.Members[0].Email != "alice@example.com" {
		t.Errorf("first member email = %q, want %q",
			frontendTeam.Members[0].Email, "alice@example.com")
	}
}

func TestParseFrontMatter(t *testing.T) {
	input := []byte(`---
date: 2026-03-14T09:30:00Z
status: up_next
summary: "Ship CLI MVP"
estimate: "12h"
labels: ["capitalizable", "mvp"]
---
Notes here...`)

	fm, body, err := extractFrontMatter(input)
	if err != nil {
		t.Fatalf("extractFrontMatter error: %v", err)
	}
	if fm.Status != "up_next" {
		t.Errorf("status = %q, want %q", fm.Status, "up_next")
	}
	if fm.Summary != "Ship CLI MVP" {
		t.Errorf("summary = %q, want %q", fm.Summary, "Ship CLI MVP")
	}
	if body != "Notes here..." {
		t.Errorf("body = %q, want %q", body, "Notes here...")
	}
	if len(fm.Labels) != 2 {
		t.Errorf("labels count = %d, want 2", len(fm.Labels))
	}
}

func TestParseTeamMember(t *testing.T) {
	m := parseTeamMember("Alice Smith <alice@example.com>")
	if m.Name != "Alice Smith" {
		t.Errorf("name = %q, want %q", m.Name, "Alice Smith")
	}
	if m.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", m.Email, "alice@example.com")
	}
}

func TestUnionLabels(t *testing.T) {
	tests := []struct {
		name   string
		parent []string
		child  []string
		want   []string
	}{
		{"empty", nil, nil, nil},
		{"parent only", []string{"a"}, nil, []string{"a"}},
		{"child only", nil, []string{"b"}, []string{"b"}},
		{"union", []string{"a"}, []string{"b"}, []string{"a", "b"}},
		{"dedup case insensitive", []string{"A"}, []string{"a"}, []string{"A"}},
		{"overlap", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unionLabels(tt.parent, tt.child)
			if len(got) != len(tt.want) {
				t.Errorf("unionLabels(%v, %v) = %v, want %v", tt.parent, tt.child, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("unionLabels(%v, %v)[%d] = %q, want %q",
						tt.parent, tt.child, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ── Helpers ───────────────────────────────────────────────────────

func findTask(repo *model.Repository, path string) *model.Task {
	for _, t := range repo.Tasks {
		if string(t.CanonicalPath) == path {
			return t
		}
	}
	return nil
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
