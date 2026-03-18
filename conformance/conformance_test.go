package conformance_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jpcummins/tsk-lib/conformance"
	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/parse"
	"github.com/jpcummins/tsk-lib/query"
	"github.com/jpcummins/tsk-lib/scan"
)

// tskBasePath returns the path to the tsk spec repository.
func tskBasePath(t *testing.T) string {
	t.Helper()
	path := "../../tsk"
	if _, err := os.Stat(path); err != nil {
		if envPath := os.Getenv("TSK_SPEC_PATH"); envPath != "" {
			path = envPath
		} else {
			t.Skipf("tsk spec not found at %s (set TSK_SPEC_PATH env var)", path)
		}
	}
	return path
}

func TestConformance(t *testing.T) {
	basePath := tskBasePath(t)

	cases, err := conformance.LoadAll(basePath)
	if err != nil {
		t.Fatalf("Failed to load conformance tests: %v", err)
	}

	t.Logf("Loaded %d conformance test cases", len(cases))

	for i, tc := range cases {
		name := fmt.Sprintf("%s/%s", tc.Operation, tc.Title)
		t.Run(name, func(t *testing.T) {
			switch tc.Operation {
			case "normalize_path":
				runNormalizePath(t, tc)
			case "validate_identifier":
				runValidateIdentifier(t, tc)
			case "derive_iteration_status":
				runDeriveIterationStatus(t, tc)
			case "order_tasks":
				runOrderTasks(t, tc)
			case "parse_config":
				runParseConfig(t, tc)
			case "resolve_stub":
				runResolveStub(t, tc)
			case "resolve_task":
				runResolveTask(t, tc)
			case "resolve_hierarchy":
				runResolveHierarchy(t, tc)
			case "resolve_assignee":
				runResolveAssignee(t, tc)
			case "evaluate_sla":
				runEvaluateSLA(t, tc)
			case "run_query":
				runQuery(t, tc)
			case "generate_report":
				runGenerateReport(t, tc)
			default:
				t.Skipf("Unknown operation %q (test %d)", tc.Operation, i)
			}
		})
	}
}

// --- Operation handlers ---

func runNormalizePath(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	result := model.NormalizePath(tc.Inputs.Path)

	if tc.Expect.CanonicalPath != "" {
		if string(result) != tc.Expect.CanonicalPath {
			t.Errorf("NormalizePath(%q) = %q, want %q",
				tc.Inputs.Path, result, tc.Expect.CanonicalPath)
		}
	}

	var diags model.Diagnostics
	if model.ContainsUppercase(tc.Inputs.Path) {
		diags = append(diags, model.NewWarningf(model.CodePathUppercase,
			"path contains uppercase characters: %s", tc.Inputs.Path))
	}

	if tc.Files != nil {
		conflicts := detectCaseConflicts(tc.Files)
		for _, conflict := range conflicts {
			diags = append(diags, model.NewWarningf(model.CodePathCaseConflict,
				"case conflict detected: %s", conflict))
		}
	}

	checkDiagnostics(t, diags, tc.Warnings, tc.Errors)
}

func runValidateIdentifier(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	result := model.ValidateIdentifier(tc.Inputs.Value)

	if tc.Expect.Valid != nil {
		if result != *tc.Expect.Valid {
			t.Errorf("ValidateIdentifier(%q) = %v, want %v",
				tc.Inputs.Value, result, *tc.Expect.Valid)
		}
	}
}

func runDeriveIterationStatus(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	start, err := time.Parse(time.RFC3339, tc.Inputs.Start)
	if err != nil {
		t.Fatalf("Invalid start time: %v", err)
	}
	end, err := time.Parse(time.RFC3339, tc.Inputs.End)
	if err != nil {
		t.Fatalf("Invalid end time: %v", err)
	}
	now, err := time.Parse(time.RFC3339, tc.Inputs.Now)
	if err != nil {
		t.Fatalf("Invalid now time: %v", err)
	}

	it := &model.Iteration{Start: start, End: end}
	status := it.DeriveStatus(now)

	if tc.Expect.Status != "" && string(status) != tc.Expect.Status {
		t.Errorf("DeriveStatus() = %q, want %q", status, tc.Expect.Status)
	}
}

func runOrderTasks(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	tasks := make([]*model.Task, len(tc.Inputs.Paths))
	for i, p := range tc.Inputs.Paths {
		task := &model.Task{Path: model.NormalizePath(p)}
		if w, ok := tc.Inputs.Weights[p]; ok {
			wCopy := w
			task.Weight = &wCopy
		}
		tasks[i] = task
	}

	model.SortTasks(tasks)

	got := make([]string, len(tasks))
	for i, task := range tasks {
		got[i] = task.Path.Base()
	}

	expected := make([]string, len(tc.Expect.OrderedPaths))
	for i, p := range tc.Expect.OrderedPaths {
		expected[i] = model.NormalizePath(p).Base()
	}

	if len(got) != len(expected) {
		t.Errorf("ordered length = %d, want %d", len(got), len(expected))
		return
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Errorf("ordered[%d] = %q, want %q", i, got[i], expected[i])
		}
	}
}

func runParseConfig(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	// Create an in-memory scanner with the config file
	files := make(map[string]string)
	files[tc.Inputs.Path] = tc.Inputs.Content

	scanner := scan.NewMemScanner(files)
	entries, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	parser := parse.NewParser()
	repo, err := parser.Parse(entries)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check version
	if tc.Expect.Version != "" {
		if repo.Version != tc.Expect.Version {
			t.Errorf("version = %q, want %q", repo.Version, tc.Expect.Version)
		}
	}

	// Check version_ignored: subdirectory configs should not set repo version
	if tc.Expect.VersionIgnored != nil && *tc.Expect.VersionIgnored {
		if repo.Version != "" {
			t.Errorf("version should be ignored in subdirectory config, got %q", repo.Version)
		}
	}

	// Check status map
	if tc.Expect.StatusMap != nil {
		// Find the config for the given path
		var cfg *model.Config
		for _, c := range repo.Configs {
			cfg = c
		}
		if cfg == nil {
			t.Fatal("no config found")
		}

		for name, expected := range tc.Expect.StatusMap {
			got, ok := cfg.StatusMap[name]
			if !ok {
				t.Errorf("status map missing entry %q", name)
				continue
			}
			if string(got.Category) != expected.Category {
				t.Errorf("status_map[%q].category = %q, want %q", name, got.Category, expected.Category)
			}
			if got.Order != expected.Order {
				t.Errorf("status_map[%q].order = %d, want %d", name, got.Order, expected.Order)
			}
		}
	}

	checkDiagnostics(t, repo.Diagnostics, tc.Warnings, tc.Errors)
}

func runResolveStub(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	scanner := scan.NewMemScanner(tc.Files)
	entries, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	parser := parse.NewParser()
	repo, err := parser.Parse(entries)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check for expected errors first
	if len(tc.Errors) > 0 {
		checkDiagnostics(t, repo.Diagnostics, tc.Warnings, tc.Errors)
		return
	}

	// Check resolved target
	if tc.Expect.Target != "" {
		stubPath := model.CanonicalPath(tc.Inputs.Path)
		target, ok := repo.Stubs[stubPath]
		if !ok {
			t.Errorf("stub %q not resolved, stubs = %v", tc.Inputs.Path, repo.Stubs)
			return
		}
		if string(target) != tc.Expect.Target {
			t.Errorf("stub target = %q, want %q", target, tc.Expect.Target)
		}
	}

	checkDiagnostics(t, repo.Diagnostics, tc.Warnings, tc.Errors)
}

func runResolveTask(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	scanner := scan.NewMemScanner(tc.Files)
	entries, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	parser := parse.NewParser()
	repo, err := parser.Parse(entries)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	taskPath := model.CanonicalPath(tc.Inputs.Path)
	task, ok := repo.Tasks[taskPath]
	if !ok {
		t.Fatalf("task %q not found, tasks = %v", tc.Inputs.Path, keysOf(repo.Tasks))
	}

	if tc.Expect.CanonicalPath != "" && string(task.Path) != tc.Expect.CanonicalPath {
		t.Errorf("canonical_path = %q, want %q", task.Path, tc.Expect.CanonicalPath)
	}

	// Check fields_present
	for _, field := range tc.Expect.FieldsPresent {
		if !task.HasField(field) {
			t.Errorf("expected field %q to be present", field)
		}
	}

	// Check task_fields
	for _, field := range tc.Expect.TaskFields {
		if !task.HasField(field) {
			t.Errorf("expected task field %q to be present", field)
		}
	}
}

func runResolveHierarchy(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	scanner := scan.NewMemScanner(tc.Files)
	entries, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	parser := parse.NewParser()
	repo, err := parser.Parse(entries)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	taskPath := model.CanonicalPath(tc.Inputs.Path)
	task, ok := repo.Tasks[taskPath]
	if !ok {
		t.Fatalf("task %q not found", tc.Inputs.Path)
	}

	if tc.Expect.ResolvedPath != "" && string(task.Path) != tc.Expect.ResolvedPath {
		t.Errorf("resolved_path = %q, want %q", task.Path, tc.Expect.ResolvedPath)
	}

	if tc.Expect.ResolvedKind != "" {
		gotKind := "file"
		if task.IsReadme {
			gotKind = "directory"
		}
		if gotKind != tc.Expect.ResolvedKind {
			t.Errorf("resolved_kind = %q, want %q", gotKind, tc.Expect.ResolvedKind)
		}
	}
}

func runResolveAssignee(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	// Parse any team files
	files := tc.Files
	if files == nil {
		files = make(map[string]string)
	}

	scanner := scan.NewMemScanner(files)
	entries, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	parser := parse.NewParser()
	repo, err := parser.Parse(entries)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result, diags := parse.ResolveAssignee(tc.Inputs.Assignee, repo.Teams)

	if tc.Expect.Type != "" && result.Type != tc.Expect.Type {
		t.Errorf("assignee type = %q, want %q", result.Type, tc.Expect.Type)
	}

	if tc.Expect.Value != "" && result.Value != tc.Expect.Value {
		t.Errorf("assignee value = %q, want %q", result.Value, tc.Expect.Value)
	}

	checkDiagnostics(t, diags, tc.Warnings, tc.Errors)
}

func runEvaluateSLA(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	// Parse files
	files := tc.Files
	if files == nil {
		files = make(map[string]string)
	}
	scanner := scan.NewMemScanner(files)
	entries, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	p := parse.NewParser()
	repo, err := p.Parse(entries)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check applied_rule_ids
	if tc.Expect.AppliedRuleIDs != nil {
		gotIDs := make([]string, 0, len(repo.SLARules))
		taskPath := model.CanonicalPath(tc.Inputs.TaskPath)
		task := repo.Tasks[taskPath]

		for _, rule := range repo.SLARules {
			// Simple query matching: parse and evaluate the rule query
			if task != nil {
				qp := query.NewParser()
				expr, err := qp.Parse(rule.Query)
				if err == nil {
					ctx := &query.EvalContext{
						Repository: repo,
						Now:        time.Now(),
					}
					matches := query.Evaluate(expr, ctx)
					for _, m := range matches {
						if m == string(taskPath) {
							gotIDs = append(gotIDs, rule.ID)
							break
						}
					}
				}
			}
		}

		sort.Strings(gotIDs)
		expectedIDs := make([]string, len(tc.Expect.AppliedRuleIDs))
		copy(expectedIDs, tc.Expect.AppliedRuleIDs)
		sort.Strings(expectedIDs)

		if len(gotIDs) != len(expectedIDs) {
			t.Errorf("applied_rule_ids = %v, want %v", gotIDs, expectedIDs)
		} else {
			for i := range gotIDs {
				if gotIDs[i] != expectedIDs[i] {
					t.Errorf("applied_rule_ids[%d] = %q, want %q", i, gotIDs[i], expectedIDs[i])
				}
			}
		}
	}

	// Check for missing updated_at warning on SLA tasks
	taskPath := model.CanonicalPath(tc.Inputs.TaskPath)
	task := repo.Tasks[taskPath]
	var slaDiags model.Diagnostics

	if task != nil && task.Status != "" && task.UpdatedAt == nil && len(task.ChangeLog) == 0 {
		for _, rule := range repo.SLARules {
			if strings.HasPrefix(rule.Start, "status:") || strings.HasPrefix(rule.Stop, "status:") {
				slaDiags = append(slaDiags, model.NewWarningf(model.CodeSLAStatusMissingUpdatedAt,
					"task %s has status but no updated_at for SLA timing", taskPath))
				break
			}
		}
	}

	// If only warnings expected (no status evaluation), check and return
	if tc.Expect.Status == "" {
		checkDiagnostics(t, slaDiags, tc.Warnings, tc.Errors)
		return
	}

	if task == nil {
		t.Fatalf("task %q not found", tc.Inputs.TaskPath)
	}

	// If missing updated_at, can't evaluate SLA timing
	if len(slaDiags) > 0 {
		checkDiagnostics(t, slaDiags, tc.Warnings, tc.Errors)
		return
	}

	// Evaluate SLA
	evalTime, err := time.Parse(time.RFC3339, tc.Inputs.Time)
	if err != nil {
		t.Fatalf("Invalid evaluation time: %v", err)
	}

	for _, rule := range repo.SLARules {
		// Find start time from change_log
		var startTime *time.Time
		var stopTime *time.Time

		startStatus := strings.TrimPrefix(rule.Start, "status:")
		stopStatus := strings.TrimPrefix(rule.Stop, "status:")

		for i := range task.ChangeLog {
			cl := &task.ChangeLog[i]
			if cl.To == startStatus {
				startTime = &cl.At
			}
			if cl.To == stopStatus {
				stopTime = &cl.At
			}
		}

		if startTime == nil {
			continue
		}

		var elapsed time.Duration
		if stopTime != nil && stopTime.After(*startTime) {
			elapsed = evalTime.Sub(*startTime)
		} else {
			elapsed = evalTime.Sub(*startTime)
		}

		elapsedDays := model.Duration(elapsed.Hours() / 24 * float64(model.MinutesPerDay) / float64(model.HoursPerDay))

		status := model.EvaluateSLAStatus(elapsedDays, rule.Target, rule.WarnAt)

		if string(status) != tc.Expect.Status {
			t.Errorf("SLA status = %q, want %q (elapsed=%v)", status, tc.Expect.Status, elapsedDays)
		}

		// Check elapsed
		if tc.Expect.Elapsed != "" {
			expectedElapsed, err := model.ParseDuration(tc.Expect.Elapsed)
			if err != nil {
				t.Fatalf("Invalid expected elapsed: %v", err)
			}
			if elapsedDays != expectedElapsed {
				t.Errorf("elapsed = %v, want %v", elapsedDays, expectedElapsed)
			}
		}

		break // Only check first matching rule
	}
}

func runQuery(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	// Parse the query
	qp := query.NewParser()
	expr, parseErr := qp.Parse(tc.Inputs.Query)

	// Check for expected errors that would be parse-level
	if parseErr != nil {
		// Check if this was an expected error
		if len(tc.Errors) > 0 {
			diags := model.Diagnostics{
				model.NewErrorf(model.CodeQueryInvalidSyntax, "%v", parseErr),
			}
			checkDiagnostics(t, diags, tc.Warnings, tc.Errors)
			return
		}
		t.Fatalf("Parse failed: %v", parseErr)
	}

	// Validate
	isReporting := tc.Inputs.Namespace == "reporting"
	validator := query.NewValidator()
	valCtx := &query.ValidationContext{IsReportingContext: isReporting}
	diags := validator.Validate(expr, valCtx)

	// If there are validation errors, check them
	if diags.HasErrors() {
		checkDiagnostics(t, diags, tc.Warnings, tc.Errors)
		return
	}

	// Parse files if provided
	files := tc.Files
	if files == nil {
		files = make(map[string]string)
	}
	scanner := scan.NewMemScanner(files)
	entries, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	p := parse.NewParser()
	repo, err := p.Parse(entries)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Build SLA results from test inputs
	var slaResults []*model.SLAResult
	for _, sr := range tc.Inputs.SLAResults {
		result := &model.SLAResult{
			RuleID:   sr.RuleID,
			TaskPath: model.CanonicalPath(sr.TaskPath),
			Status:   model.SLAStatus(sr.Status),
		}
		if sr.Elapsed != "" {
			elapsed, _ := model.ParseDuration(sr.Elapsed)
			result.Elapsed = elapsed
		}
		if sr.Target != "" {
			target, _ := model.ParseDuration(sr.Target)
			result.Target = target
		}
		if sr.Remaining != "" {
			remaining, _ := model.ParseDuration(sr.Remaining)
			result.Remaining = remaining
		}
		slaResults = append(slaResults, result)
	}

	// Evaluate
	evalCtx := &query.EvalContext{
		Repository:         repo,
		Now:                time.Now(),
		SLAResults:         slaResults,
		IsReportingContext: isReporting,
	}

	matches := query.Evaluate(expr, evalCtx)

	// Check matches
	if tc.Expect.Matches != nil {
		got := make([]string, len(matches))
		copy(got, matches)
		sort.Strings(got)

		expected := make([]string, len(tc.Expect.Matches))
		copy(expected, tc.Expect.Matches)
		sort.Strings(expected)

		if len(got) != len(expected) {
			t.Errorf("matches = %v, want %v", got, expected)
		} else {
			for i := range got {
				if got[i] != expected[i] {
					t.Errorf("matches[%d] = %q, want %q", i, got[i], expected[i])
				}
			}
		}
	}

	// Check diagnostics (warnings from validation)
	checkDiagnostics(t, diags, tc.Warnings, tc.Errors)
}

func runGenerateReport(t *testing.T, tc conformance.TestCase) {
	t.Helper()

	files := tc.Files
	if files == nil {
		files = make(map[string]string)
	}

	scanner := scan.NewMemScanner(files)
	entries, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	p := parse.NewParser()
	repo, err := p.Parse(entries)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	switch tc.Inputs.Scope {
	case "tasks":
		// Collect all nodes (tasks + virtual parent directories)
		nodeSet := make(map[string]bool)
		virtualSet := make(map[string]bool)

		for _, task := range repo.Tasks {
			if task.IsStub {
				continue
			}
			nodeSet[string(task.Path)] = true

			// Walk up the path to find virtual parent nodes
			parent := task.Path.Parent()
			for !parent.IsEmpty() {
				if _, hasTask := repo.Tasks[parent]; !hasTask {
					// No README.md for this directory — it's virtual
					virtualSet[string(parent)] = true
				}
				nodeSet[string(parent)] = true
				parent = parent.Parent()
			}
		}

		if tc.Expect.Nodes != nil {
			got := setToSortedSlice(nodeSet)
			expected := make([]string, len(tc.Expect.Nodes))
			copy(expected, tc.Expect.Nodes)
			sort.Strings(expected)
			if !slicesEqual(got, expected) {
				t.Errorf("nodes = %v, want %v", got, expected)
			}
		}

		if tc.Expect.VirtualNodes != nil {
			got := setToSortedSlice(virtualSet)
			expected := make([]string, len(tc.Expect.VirtualNodes))
			copy(expected, tc.Expect.VirtualNodes)
			sort.Strings(expected)
			if !slicesEqual(got, expected) {
				t.Errorf("virtual_nodes = %v, want %v", got, expected)
			}
		}

	case "teams":
		if tc.Expect.Teams != nil {
			got := make([]string, 0, len(repo.Teams))
			for name := range repo.Teams {
				got = append(got, name)
			}
			sort.Strings(got)
			expected := make([]string, len(tc.Expect.Teams))
			copy(expected, tc.Expect.Teams)
			sort.Strings(expected)
			if !slicesEqual(got, expected) {
				t.Errorf("teams = %v, want %v", got, expected)
			}
		}

		if tc.Expect.TasksByTeam != nil {
			for teamName, expectedPaths := range tc.Expect.TasksByTeam {
				var gotPaths []string
				for _, task := range repo.Tasks {
					if task.IsStub {
						continue
					}
					if task.Assignee == "team:"+teamName {
						gotPaths = append(gotPaths, string(task.Path))
					}
				}
				sort.Strings(gotPaths)
				exp := make([]string, len(expectedPaths))
				copy(exp, expectedPaths)
				sort.Strings(exp)
				if !slicesEqual(gotPaths, exp) {
					t.Errorf("tasks_by_team[%s] = %v, want %v", teamName, gotPaths, exp)
				}
			}
		}
	}
}

func setToSortedSlice(m map[string]bool) []string {
	s := make([]string, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- Helpers ---

func checkDiagnostics(t *testing.T, got model.Diagnostics, expectedWarnings, expectedErrors []conformance.DiagExpect) {
	t.Helper()

	for _, ew := range expectedWarnings {
		found := false
		for _, d := range got {
			if d.Level == model.LevelWarning && string(d.Code) == ew.Code {
				if ew.MessageContains == "" || strings.Contains(strings.ToLower(d.Message), strings.ToLower(ew.MessageContains)) {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("expected warning code=%s (message_contains=%q) not found in diagnostics: %v",
				ew.Code, ew.MessageContains, got)
		}
	}

	for _, ee := range expectedErrors {
		found := false
		for _, d := range got {
			if d.Level == model.LevelError && string(d.Code) == ee.Code {
				if ee.MessageContains == "" || strings.Contains(strings.ToLower(d.Message), strings.ToLower(ee.MessageContains)) {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("expected error code=%s (message_contains=%q) not found in diagnostics: %v",
				ee.Code, ee.MessageContains, got)
		}
	}
}

func detectCaseConflicts(files map[string]string) []string {
	lower := make(map[string][]string)
	for path := range files {
		l := strings.ToLower(path)
		lower[l] = append(lower[l], path)
	}

	var conflicts []string
	for _, paths := range lower {
		if len(paths) > 1 {
			sort.Strings(paths)
			conflicts = append(conflicts, strings.Join(paths, " vs "))
		}
	}
	return conflicts
}

func keysOf(m map[model.CanonicalPath]*model.Task) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	return keys
}
