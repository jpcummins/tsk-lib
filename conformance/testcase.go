// Package conformance provides a test runner for the tsk conformance test suite.
// It loads language-agnostic TOML test cases and validates implementations against them.
package conformance

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// TestFile represents a TOML file containing multiple test cases.
type TestFile struct {
	Test []TestCase `toml:"test"`
}

// TestCase represents a single conformance test case.
type TestCase struct {
	SpecRef   string            `toml:"spec_ref"`
	Title     string            `toml:"title"`
	Operation string            `toml:"operation"`
	Files     map[string]string `toml:"files"`
	Inputs    Inputs            `toml:"inputs"`
	Expect    Expect            `toml:"expect"`
	Warnings  []DiagExpect      `toml:"warnings"`
	Errors    []DiagExpect      `toml:"errors"`
}

// Inputs holds operation-specific input parameters.
type Inputs struct {
	// normalize_path, resolve_stub, resolve_task, resolve_hierarchy
	Path string `toml:"path"`

	// validate_identifier
	Value string `toml:"value"`
	Kind  string `toml:"kind"`

	// run_query
	Query     string `toml:"query"`
	Namespace string `toml:"namespace"`

	// order_tasks
	Paths   []string           `toml:"paths"`
	Weights map[string]float64 `toml:"weights"`

	// evaluate_sla
	TaskPath string `toml:"task_path"`
	Time     string `toml:"time"`

	// derive_iteration_status
	Start string `toml:"start"`
	End   string `toml:"end"`
	Now   string `toml:"now"`

	// parse_config
	Content string `toml:"content"`

	// resolve_assignee
	Assignee string `toml:"assignee"`

	// generate_report
	Scope string `toml:"scope"`

	// run_query context
	SLAResults []SLAResultInput `toml:"sla_results"`
}

// SLAResultInput provides SLA context for queries that reference sla.* fields.
type SLAResultInput struct {
	RuleID    string `toml:"rule_id"`
	TaskPath  string `toml:"task_path"`
	Status    string `toml:"status"`
	Elapsed   string `toml:"elapsed"`
	Remaining string `toml:"remaining"`
	Target    string `toml:"target"`
}

// Expect holds expected outputs for a test case.
type Expect struct {
	// normalize_path, resolve_task
	CanonicalPath string `toml:"canonical_path"`

	// resolve_task
	FrontMatterFormat string   `toml:"front_matter_format"`
	FieldsPresent     []string `toml:"fields_present"`
	TaskFields        []string `toml:"task_fields"`

	// validate_identifier
	Valid *bool `toml:"valid"`

	// run_query
	Matches []string `toml:"matches"`

	// resolve_stub
	Target     string `toml:"target"`
	ChainDepth *int   `toml:"chain_depth"`

	// resolve_hierarchy
	ResolvedKind string `toml:"resolved_kind"`
	ResolvedPath string `toml:"resolved_path"`

	// order_tasks
	OrderedPaths []string `toml:"ordered_paths"`

	// evaluate_sla
	Status         string   `toml:"status"`
	Elapsed        string   `toml:"elapsed"`
	Remaining      string   `toml:"remaining"`
	AppliedRuleIDs []string `toml:"applied_rule_ids"`

	// parse_config
	Version        string `toml:"version"`
	VersionIgnored *bool  `toml:"version_ignored"`

	// resolve_assignee
	Type  string `toml:"type"`
	Value string `toml:"value"`

	// generate_report
	Nodes        []string            `toml:"nodes"`
	VirtualNodes []string            `toml:"virtual_nodes"`
	Teams        []string            `toml:"teams"`
	TasksByTeam  map[string][]string `toml:"tasks_by_team"`
}

// DiagExpect represents an expected diagnostic (warning or error).
type DiagExpect struct {
	Code            string `toml:"code"`
	Level           string `toml:"level"`
	MessageContains string `toml:"message_contains"`
}

// Index represents the test index file.
type Index struct {
	Files []string `toml:"files"`
}

// LoadIndex loads the test index from the given path.
func LoadIndex(indexPath string) (*Index, error) {
	var idx Index
	_, err := toml.DecodeFile(indexPath, &idx)
	if err != nil {
		return nil, err
	}
	return &idx, nil
}

// LoadTestFile loads test cases from a TOML file.
func LoadTestFile(path string) ([]TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tf TestFile
	if err := toml.Unmarshal(data, &tf); err != nil {
		return nil, err
	}
	return tf.Test, nil
}

// LoadAll loads all test cases referenced by the index file.
// The basePath is the directory containing the index.toml
// (typically ../tsk relative to the library).
func LoadAll(basePath string) ([]TestCase, error) {
	indexPath := filepath.Join(basePath, "tests", "index.toml")
	idx, err := LoadIndex(indexPath)
	if err != nil {
		return nil, err
	}

	var all []TestCase
	for _, file := range idx.Files {
		path := filepath.Join(basePath, file)
		cases, err := LoadTestFile(path)
		if err != nil {
			return nil, err
		}
		all = append(all, cases...)
	}
	return all, nil
}
