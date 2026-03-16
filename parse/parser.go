package parse

import (
	"fmt"

	"github.com/jp/tsk-lib/model"
	"github.com/jp/tsk-lib/scan"
)

// EntryProvider supplies raw filesystem entries for parsing.
// Satisfied by scan.Scanner or a test fixture.
type EntryProvider interface {
	Scan(root string) ([]scan.Entry, error)
}

// Parser converts scanned entries into a fully resolved Repository.
type Parser interface {
	Parse(entries []scan.Entry) (*model.Repository, error)
}

// DefaultParser is the standard Parser implementation.
type DefaultParser struct{}

// NewParser returns a new DefaultParser.
func NewParser() *DefaultParser {
	return &DefaultParser{}
}

// Parse takes raw scanned entries and produces a fully resolved Repository.
// Resolution phases:
//  1. Parse individual files (front matter, TOML, Markdown body)
//  2. Resolve redirects (stub chain resolution, max depth 3)
//  3. Build config inheritance (deep-merge, defaults, status mapping)
//  4. Compute effective labels (union semantics from parent to child)
func (p *DefaultParser) Parse(entries []scan.Entry) (*model.Repository, error) {
	repo := &model.Repository{}

	// ── Phase 1: Parse individual files ──────────────────────────
	taskMap := make(map[model.CanonicalPath]*model.Task)
	var iterations []*model.Iteration
	var configs []*model.Config
	var teams []*model.Team

	for _, entry := range entries {
		switch entry.Kind {
		case scan.EntryTask:
			task, err := parseTask(entry)
			if err != nil {
				repo.Warnings = append(repo.Warnings,
					fmt.Sprintf("skipping %s: %v", entry.Path, err))
				continue
			}
			// Section 2.2: if a directory and file share the same path,
			// directory container takes precedence. README (IsReadme) wins.
			if existing, ok := taskMap[task.CanonicalPath]; ok {
				if task.IsReadme {
					taskMap[task.CanonicalPath] = task
					repo.Warnings = append(repo.Warnings,
						fmt.Sprintf("duplicate path %q: README takes precedence over %q",
							task.CanonicalPath, existing.CanonicalPath))
				} else {
					repo.Warnings = append(repo.Warnings,
						fmt.Sprintf("duplicate path %q: keeping existing entry",
							task.CanonicalPath))
				}
			} else {
				taskMap[task.CanonicalPath] = task
			}

		case scan.EntryRootConfig, scan.EntryProjectConfig:
			cfg, err := parseConfig(entry)
			if err != nil {
				repo.Warnings = append(repo.Warnings,
					fmt.Sprintf("skipping config %s: %v", entry.Path, err))
				continue
			}
			configs = append(configs, cfg)

		case scan.EntryTeamConfig:
			team, err := parseTeamConfig(entry)
			if err != nil {
				repo.Warnings = append(repo.Warnings,
					fmt.Sprintf("skipping team %s: %v", entry.Path, err))
				continue
			}
			teams = append(teams, team)

		case scan.EntryIteration:
			iter, err := parseIteration(entry)
			if err != nil {
				repo.Warnings = append(repo.Warnings,
					fmt.Sprintf("skipping iteration %s: %v", entry.Path, err))
				continue
			}
			iterations = append(iterations, iter)

		case scan.EntrySLA:
			rules, err := parseSLARules(entry)
			if err != nil {
				repo.Warnings = append(repo.Warnings,
					fmt.Sprintf("skipping sla.toml: %v", err))
				continue
			}
			repo.SLARules = rules
		}
	}

	// ── Phase 2: Resolve redirects ────────────────────────────────
	resolved, stubs, redirectWarnings, err := resolveRedirects(taskMap)
	if err != nil {
		return nil, fmt.Errorf("resolving redirects: %w", err)
	}
	repo.Stubs = stubs
	repo.Warnings = append(repo.Warnings, redirectWarnings...)

	// ── Phase 3: Config inheritance + status resolution ───────────
	resolveInheritance(resolved, iterations, configs, teams)

	// ── Phase 4: Label union semantics ────────────────────────────
	resolveLabels(resolved)

	// ── Assemble final repository ─────────────────────────────────
	for _, task := range resolved {
		repo.Tasks = append(repo.Tasks, task)
	}
	repo.Iterations = iterations
	repo.Teams = teams
	repo.Configs = configs

	return repo, nil
}
