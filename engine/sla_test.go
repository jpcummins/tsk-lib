package engine

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/parse"
	"github.com/jpcummins/tsk-lib/query"
	"github.com/jpcummins/tsk-lib/scan"
	tsql "github.com/jpcummins/tsk-lib/sql"
	"github.com/jpcummins/tsk-lib/store"
)

// fixedNow is the reference time used in all SLA tests.
var fixedNow = time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

// mustParseDuration is a test helper that panics on invalid duration strings.
func mustParseDuration(s string) model.Duration {
	d, err := model.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

// --- evaluateSLA unit tests ---

func TestEvaluateSLA_NoRules(t *testing.T) {
	repo := model.NewRepository()
	results := evaluateSLA(repo, fixedNow)
	if len(results) != 0 {
		t.Errorf("evaluateSLA() with no rules = %d results, want 0", len(results))
	}
}

func TestEvaluateSLA_NoMatchingTasks(t *testing.T) {
	repo := model.NewRepository()
	repo.SLARules = []*model.SLARule{
		{
			ID:     "r1",
			Query:  `task.type = "bug"`,
			Target: mustParseDuration("5d"),
			Start:  "status:todo",
			Stop:   "status:done",
		},
	}
	// Task does not match the rule query (wrong type).
	task := &model.Task{
		Path:   "tasks/my-task",
		Type:   "feature",
		Status: "todo",
	}
	repo.Tasks[task.Path] = task

	results := evaluateSLA(repo, fixedNow)
	if len(results) != 0 {
		t.Errorf("evaluateSLA() with no matching tasks = %d results, want 0", len(results))
	}
}

func TestEvaluateSLA_NoStartEvent(t *testing.T) {
	repo := model.NewRepository()
	repo.SLARules = []*model.SLARule{
		{
			ID:     "r1",
			Query:  `task.type = "bug"`,
			Target: mustParseDuration("5d"),
			Start:  "status:in_progress",
			Stop:   "status:done",
		},
	}
	// Task matches the rule query but has never entered in_progress.
	task := &model.Task{
		Path:   "tasks/my-task",
		Type:   "bug",
		Status: "todo",
	}
	repo.Tasks[task.Path] = task

	results := evaluateSLA(repo, fixedNow)
	if len(results) != 0 {
		t.Errorf("evaluateSLA() with no start event = %d results, want 0", len(results))
	}
}

func TestEvaluateSLA_StatusOK(t *testing.T) {
	repo := model.NewRepository()
	target := mustParseDuration("5d")
	repo.SLARules = []*model.SLARule{
		{
			ID:     "r1",
			Query:  `task.type = "bug"`,
			Target: target,
			Start:  "status:in_progress",
			Stop:   "status:done",
		},
	}

	// Started 2 work-days ago (2 * 8h = 16 wall-clock hours). fixedNow is Jan 10.
	startTime := fixedNow.Add(-16 * time.Hour)
	task := &model.Task{
		Path:   "tasks/my-task",
		Type:   "bug",
		Status: "in_progress",
		ChangeLog: []model.FieldChange{
			{Field: "status", To: "in_progress", At: startTime},
		},
	}
	repo.Tasks[task.Path] = task

	results := evaluateSLA(repo, fixedNow)
	if len(results) != 1 {
		t.Fatalf("evaluateSLA() = %d results, want 1", len(results))
	}
	r := results[0]
	if r.RuleID != "r1" {
		t.Errorf("RuleID = %q, want %q", r.RuleID, "r1")
	}
	if r.TaskPath != task.Path {
		t.Errorf("TaskPath = %q, want %q", r.TaskPath, task.Path)
	}
	if r.Status != model.SLAOk {
		t.Errorf("Status = %q, want %q", r.Status, model.SLAOk)
	}
	if r.Target != target {
		t.Errorf("Target = %v, want %v", r.Target, target)
	}
	// Elapsed should be 2d.
	expectedElapsed := mustParseDuration("2d")
	if r.Elapsed != expectedElapsed {
		t.Errorf("Elapsed = %v, want %v", r.Elapsed, expectedElapsed)
	}
	expectedRemaining := target - expectedElapsed
	if r.Remaining != expectedRemaining {
		t.Errorf("Remaining = %v, want %v", r.Remaining, expectedRemaining)
	}
}

func TestEvaluateSLA_StatusAtRisk(t *testing.T) {
	warnAt := mustParseDuration("3d")
	target := mustParseDuration("5d")
	repo := model.NewRepository()
	repo.SLARules = []*model.SLARule{
		{
			ID:     "r1",
			Query:  `task.type = "bug"`,
			Target: target,
			WarnAt: &warnAt,
			Start:  "status:in_progress",
			Stop:   "status:done",
		},
	}

	// Started 4 work-days ago (4 * 8h = 32 wall-clock hours). Elapsed > warn_at but <= target.
	startTime := fixedNow.Add(-32 * time.Hour)
	task := &model.Task{
		Path:   "tasks/my-task",
		Type:   "bug",
		Status: "in_progress",
		ChangeLog: []model.FieldChange{
			{Field: "status", To: "in_progress", At: startTime},
		},
	}
	repo.Tasks[task.Path] = task

	results := evaluateSLA(repo, fixedNow)
	if len(results) != 1 {
		t.Fatalf("evaluateSLA() = %d results, want 1", len(results))
	}
	if results[0].Status != model.SLAAtRisk {
		t.Errorf("Status = %q, want %q", results[0].Status, model.SLAAtRisk)
	}
}

func TestEvaluateSLA_StatusBreached(t *testing.T) {
	target := mustParseDuration("5d")
	repo := model.NewRepository()
	repo.SLARules = []*model.SLARule{
		{
			ID:     "r1",
			Query:  `task.type = "bug"`,
			Target: target,
			Start:  "status:in_progress",
			Stop:   "status:done",
		},
	}

	// Started 6 work-days ago (6 * 8h = 48 wall-clock hours). Elapsed > target.
	startTime := fixedNow.Add(-48 * time.Hour)
	task := &model.Task{
		Path:   "tasks/my-task",
		Type:   "bug",
		Status: "in_progress",
		ChangeLog: []model.FieldChange{
			{Field: "status", To: "in_progress", At: startTime},
		},
	}
	repo.Tasks[task.Path] = task

	results := evaluateSLA(repo, fixedNow)
	if len(results) != 1 {
		t.Fatalf("evaluateSLA() = %d results, want 1", len(results))
	}
	if results[0].Status != model.SLABreached {
		t.Errorf("Status = %q, want %q", results[0].Status, model.SLABreached)
	}
}

func TestEvaluateSLA_UpdatedAtFallback(t *testing.T) {
	// When no ChangeLog entry exists for the start status but the task's current
	// status matches, fall back to UpdatedAt.
	target := mustParseDuration("5d")
	repo := model.NewRepository()
	repo.SLARules = []*model.SLARule{
		{
			ID:     "r1",
			Query:  `task.type = "bug"`,
			Target: target,
			Start:  "status:in_progress",
			Stop:   "status:done",
		},
	}

	updatedAt := fixedNow.Add(-8 * time.Hour) // 1 work-day ago
	task := &model.Task{
		Path:      "tasks/my-task",
		Type:      "bug",
		Status:    "in_progress",
		UpdatedAt: &updatedAt,
		// No ChangeLog
	}
	repo.Tasks[task.Path] = task

	results := evaluateSLA(repo, fixedNow)
	if len(results) != 1 {
		t.Fatalf("evaluateSLA() = %d results, want 1", len(results))
	}
	expectedElapsed := mustParseDuration("1d")
	if results[0].Elapsed != expectedElapsed {
		t.Errorf("Elapsed = %v (want %v) — UpdatedAt fallback not working", results[0].Elapsed, expectedElapsed)
	}
}

func TestEvaluateSLA_MultipleRules(t *testing.T) {
	repo := model.NewRepository()
	repo.SLARules = []*model.SLARule{
		{
			ID:     "r1",
			Query:  `task.type = "bug"`,
			Target: mustParseDuration("5d"),
			Start:  "status:in_progress",
			Stop:   "status:done",
		},
		{
			ID:     "r2",
			Query:  `task.type = "bug"`,
			Target: mustParseDuration("10d"),
			Start:  "status:todo",
			Stop:   "status:done",
		},
	}

	startInProgress := fixedNow.Add(-8 * time.Hour)
	startTodo := fixedNow.Add(-16 * time.Hour)
	task := &model.Task{
		Path:   "tasks/my-task",
		Type:   "bug",
		Status: "in_progress",
		ChangeLog: []model.FieldChange{
			{Field: "status", To: "todo", At: startTodo},
			{Field: "status", To: "in_progress", At: startInProgress},
		},
	}
	repo.Tasks[task.Path] = task

	results := evaluateSLA(repo, fixedNow)
	if len(results) != 2 {
		t.Fatalf("evaluateSLA() = %d results, want 2", len(results))
	}

	// Sort by rule ID for deterministic comparison.
	sort.Slice(results, func(i, j int) bool { return results[i].RuleID < results[j].RuleID })

	if results[0].RuleID != "r1" || results[0].Elapsed != mustParseDuration("1d") {
		t.Errorf("r1: Elapsed = %v, want 1d", results[0].Elapsed)
	}
	if results[1].RuleID != "r2" || results[1].Elapsed != mustParseDuration("2d") {
		t.Errorf("r2: Elapsed = %v, want 2d", results[1].Elapsed)
	}
}

func TestEvaluateSLA_StubsSkipped(t *testing.T) {
	repo := model.NewRepository()
	repo.SLARules = []*model.SLARule{
		{
			ID:     "r1",
			Query:  `task.type = "bug"`,
			Target: mustParseDuration("5d"),
			Start:  "status:in_progress",
			Stop:   "status:done",
		},
	}

	startTime := fixedNow.Add(-8 * time.Hour)
	stub := &model.Task{
		Path:   "tasks/stub-task",
		Type:   "bug",
		IsStub: true,
		ChangeLog: []model.FieldChange{
			{Field: "status", To: "in_progress", At: startTime},
		},
	}
	repo.Tasks[stub.Path] = stub

	results := evaluateSLA(repo, fixedNow)
	if len(results) != 0 {
		t.Errorf("evaluateSLA() should skip stubs, got %d results", len(results))
	}
}

// --- containsSLAFields unit tests ---

func TestContainsSLAFields(t *testing.T) {
	qp := query.NewParser()

	cases := []struct {
		dsl  string
		want bool
	}{
		{`task.status = "todo"`, false},
		{`sla.status = "at_risk"`, true},
		{`task.assignee = "alice" AND sla.elapsed > "3d"`, true},
		{`task.type = "bug" AND task.status = "in_progress"`, false},
		{`NOT sla.status = "breached"`, true},
	}

	for _, tc := range cases {
		expr, err := qp.Parse(tc.dsl)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", tc.dsl, err)
		}
		got := containsSLAFields(expr)
		if got != tc.want {
			t.Errorf("containsSLAFields(%q) = %v, want %v", tc.dsl, got, tc.want)
		}
	}
}

// --- Integration test: Index → Query with sla.* fields ---

// newTestEngine creates a fully wired in-memory engine with a fixed clock.
func newTestEngine(t *testing.T, now time.Time) *Engine {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	return New(
		nil, // scanner — not used in integration tests (we call Index indirectly)
		parse.NewParser(),
		st,
		tsql.NewCompiler(),
		query.NewParser(),
		query.NewValidator(),
		WithNow(func() time.Time { return now }),
	)
}

// indexFiles uses the engine's parser and store directly (bypassing the
// filesystem scanner) to index a set of in-memory files.
func indexFiles(t *testing.T, e *Engine, files map[string]string) *model.Repository {
	t.Helper()
	ctx := context.Background()

	scanner := scan.NewMemScanner(files)
	entries, err := scanner.Scan(ctx, "")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	repo, err := e.parser.Parse(entries)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	repo.SLAResults = evaluateSLA(repo, e.nowTime())

	if err := e.store.WriteRepository(ctx, repo); err != nil {
		t.Fatalf("WriteRepository: %v", err)
	}

	return repo
}

func TestIntegration_SLAQuery_Breached(t *testing.T) {
	// fixedNow = Jan 10. Task moved to in_progress on Jan 2 (8 work-days elapsed).
	// SLA target is 5d → breached.
	startTime := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	files := map[string]string{
		"tsk.toml": `version = "1"`,
		"sla.toml": `
[[rule]]
id       = "fast-bugs"
name     = "Bugs in 5 days"
query    = "task.type = \"bug\""
target   = "5d"
start    = "status:in_progress"
stop     = "status:done"
severity = "high"
`,
		"tasks/bug-123.md": `---
type: bug
status: in_progress
change_log:
  - field: status
    to: in_progress
    at: ` + startTime.Format(time.RFC3339) + `
---
A bug task.
`,
		"tasks/feature-456.md": `---
type: feature
status: in_progress
change_log:
  - field: status
    to: in_progress
    at: ` + startTime.Format(time.RFC3339) + `
---
A feature (should not match the SLA rule).
`,
	}

	e := newTestEngine(t, fixedNow)
	indexFiles(t, e, files)

	tasks, diags, err := e.Query(context.Background(), `sla.status = "breached"`)
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if diags.HasErrors() {
		t.Fatalf("Query() diagnostics: %v", diags)
	}
	if len(tasks) != 1 {
		t.Fatalf("Query() = %d tasks, want 1", len(tasks))
	}
	if string(tasks[0].Path) != "bug-123" {
		t.Errorf("Query() task path = %q, want %q", tasks[0].Path, "bug-123")
	}
}

func TestIntegration_SLAQuery_AtRisk(t *testing.T) {
	// Target 5d, warn_at 3d. Task started 4 work-days ago → at_risk.
	startTime := fixedNow.Add(-32 * time.Hour) // 4 * 8h

	files := map[string]string{
		"tsk.toml": `version = "1"`,
		"sla.toml": `
[[rule]]
id       = "warn-bugs"
name     = "Warn at 3d"
query    = "task.type = \"bug\""
target   = "5d"
warn_at  = "3d"
start    = "status:in_progress"
stop     = "status:done"
severity = "medium"
`,
		"tasks/bug-at-risk.md": `---
type: bug
status: in_progress
change_log:
  - field: status
    to: in_progress
    at: ` + startTime.Format(time.RFC3339) + `
---
`,
	}

	e := newTestEngine(t, fixedNow)
	indexFiles(t, e, files)

	tasks, diags, err := e.Query(context.Background(), `sla.status = "at_risk"`)
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if diags.HasErrors() {
		t.Fatalf("Query() diagnostics: %v", diags)
	}
	if len(tasks) != 1 {
		t.Fatalf("Query() = %d tasks, want 1", len(tasks))
	}
	if string(tasks[0].Path) != "bug-at-risk" {
		t.Errorf("Query() task path = %q, want %q", tasks[0].Path, "bug-at-risk")
	}
}

func TestIntegration_SLAQuery_NoSLAData(t *testing.T) {
	// A repository with no SLA rules; querying sla.* fields should return no
	// results without error (the sla_results table is simply empty).
	files := map[string]string{
		"tsk.toml": `version = "1"`,
		"tasks/my-task.md": `---
type: bug
status: in_progress
---
`,
	}

	e := newTestEngine(t, fixedNow)
	indexFiles(t, e, files)

	tasks, diags, err := e.Query(context.Background(), `sla.status = "breached"`)
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if diags.HasErrors() {
		t.Fatalf("Query() diagnostics: %v", diags)
	}
	if len(tasks) != 0 {
		t.Errorf("Query() = %d tasks, want 0", len(tasks))
	}
}

func TestIntegration_SLAQuery_CombinedWithTaskFields(t *testing.T) {
	// Query combining sla.* and task.* predicates.
	startTime := fixedNow.Add(-48 * time.Hour) // 6 work-days → breached

	files := map[string]string{
		"tsk.toml": `version = "1"`,
		"sla.toml": `
[[rule]]
id       = "r1"
name     = "Rule 1"
query    = "task.type = \"bug\""
target   = "5d"
start    = "status:in_progress"
stop     = "status:done"
severity = "high"
`,
		"tasks/bug-alice.md": `---
type: bug
assignee: alice
status: in_progress
change_log:
  - field: status
    to: in_progress
    at: ` + startTime.Format(time.RFC3339) + `
---
`,
		"tasks/bug-bob.md": `---
type: bug
assignee: bob
status: in_progress
change_log:
  - field: status
    to: in_progress
    at: ` + startTime.Format(time.RFC3339) + `
---
`,
	}

	e := newTestEngine(t, fixedNow)
	indexFiles(t, e, files)

	// Only Alice's breached bug.
	tasks, diags, err := e.Query(context.Background(), `sla.status = "breached" AND task.assignee = "alice"`)
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if diags.HasErrors() {
		t.Fatalf("Query() diagnostics: %v", diags)
	}
	if len(tasks) != 1 {
		t.Fatalf("Query() = %d tasks, want 1", len(tasks))
	}
	if string(tasks[0].Path) != "bug-alice" {
		t.Errorf("Query() task path = %q, want %q", tasks[0].Path, "bug-alice")
	}
}

func TestIntegration_NonSLAQuery_Unaffected(t *testing.T) {
	// Regular task queries must continue to work exactly as before.
	files := map[string]string{
		"tsk.toml": `version = "1"`,
		"tasks/todo-task.md": `---
status: todo
---
`,
		"tasks/done-task.md": `---
status: done
---
`,
	}

	e := newTestEngine(t, fixedNow)
	indexFiles(t, e, files)

	tasks, diags, err := e.Query(context.Background(), `task.status = "todo"`)
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if diags.HasErrors() {
		t.Fatalf("Query() diagnostics: %v", diags)
	}
	if len(tasks) != 1 {
		t.Fatalf("Query() = %d tasks, want 1", len(tasks))
	}
	if string(tasks[0].Path) != "todo-task" {
		t.Errorf("Query() task path = %q, want %q", tasks[0].Path, "todo-task")
	}
}
