package parse

import (
	"fmt"
	"strings"

	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/scan"
)

// Parser resolves scanned entries into a fully resolved Repository.
type Parser interface {
	Parse(entries []scan.Entry) (*model.Repository, error)
}

// DefaultParser implements the multi-phase resolution pipeline.
type DefaultParser struct{}

// NewParser creates a new DefaultParser.
func NewParser() *DefaultParser {
	return &DefaultParser{}
}

// Parse executes the 4-phase resolution pipeline:
// Phase 1: Individual file parsing
// Phase 2: Redirect resolution
// Phase 3: Config inheritance + status resolution
// Phase 4: Assignee resolution
func (p *DefaultParser) Parse(entries []scan.Entry) (*model.Repository, error) {
	repo := model.NewRepository()

	// Phase 1: Parse individual files
	if err := p.parseFiles(entries, repo); err != nil {
		return nil, fmt.Errorf("phase 1 (parse): %w", err)
	}

	// Phase 2: Resolve redirects
	resolveStubs(repo)

	// Resolve dependencies through stubs
	resolveDependencies(repo)

	// Phase 3: Config inheritance + status resolution
	resolveStatusCategories(repo)

	// Resolve hierarchy (directory vs file precedence)
	resolveHierarchy(repo)

	// Phase 4: Path warnings
	p.checkPathWarnings(entries, repo)

	return repo, nil
}

// parseFiles handles Phase 1: individual file parsing.
func (p *DefaultParser) parseFiles(entries []scan.Entry, repo *model.Repository) error {
	// Collect tasks, potentially multiple per path
	tasksByPath := make(map[model.CanonicalPath][]*model.Task)

	for _, entry := range entries {
		switch entry.Kind {
		case scan.EntryTask:
			task, err := parseTask(entry)
			if err != nil {
				return err
			}
			tasksByPath[task.Path] = append(tasksByPath[task.Path], task)

		case scan.EntryRootConfig:
			cfg, err := parseConfig(entry.Content, "")
			if err != nil {
				return fmt.Errorf("parsing root config: %w", err)
			}
			repo.Version = cfg.Version

		case scan.EntryTeamConfig:
			// Path like "teams/backend/team.toml" -> team name "backend"
			parts := strings.Split(entry.Path, "/")
			if len(parts) < 3 {
				continue
			}
			teamName := parts[1]
			team, err := parseTeamConfig(entry.Content, teamName)
			if err != nil {
				return fmt.Errorf("parsing team %s: %w", entry.Path, err)
			}
			repo.Teams[teamName] = team

		case scan.EntryIteration:
			iter, err := parseIteration(entry)
			if err != nil {
				return fmt.Errorf("parsing iteration %s: %w", entry.Path, err)
			}
			repo.Iterations = append(repo.Iterations, iter)

		case scan.EntrySLA:
			rules, err := parseSLAFile(entry.Content)
			if err != nil {
				return fmt.Errorf("parsing SLA rules: %w", err)
			}
			repo.SLARules = append(repo.SLARules, rules...)
		}
	}

	// Resolve path conflicts: README.md (directory) takes precedence over leaf files
	for path, tasks := range tasksByPath {
		if len(tasks) == 1 {
			repo.Tasks[path] = tasks[0]
			continue
		}

		// Multiple tasks at the same path: prefer the README (directory container)
		var readme *model.Task
		for _, t := range tasks {
			if t.IsReadme {
				readme = t
				break
			}
		}

		if readme != nil {
			repo.Tasks[path] = readme
			repo.Diagnostics = append(repo.Diagnostics, model.NewWarningf(
				model.CodePathCaseConflict,
				"directory container takes precedence over task file at %s", path))
		} else {
			// No README; use first one (arbitrary)
			repo.Tasks[path] = tasks[0]
		}
	}

	return nil
}

// checkPathWarnings emits PATH_UPPERCASE warnings for paths with uppercase chars.
func (p *DefaultParser) checkPathWarnings(entries []scan.Entry, repo *model.Repository) {
	for _, entry := range entries {
		if entry.Kind != scan.EntryTask {
			continue
		}
		if model.ContainsUppercase(entry.Path) {
			repo.Diagnostics = append(repo.Diagnostics, model.NewWarningf(
				model.CodePathUppercase,
				"path contains uppercase characters: %s", entry.Path))
		}
	}
}
