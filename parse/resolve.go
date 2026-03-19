package parse

import (
	"sort"
	"strings"

	"github.com/jpcummins/tsk-lib/model"
)

const maxRedirectDepth = 3

// resolveStubs resolves redirect stubs to their targets.
// Returns diagnostics for chain depth and invalid targets.
func resolveStubs(repo *model.Repository) {
	for path, task := range repo.Tasks {
		if !task.IsStub {
			continue
		}

		target, diag := resolveStubChain(repo, path, 0)
		if diag != nil {
			repo.Diagnostics = append(repo.Diagnostics, *diag)
			continue
		}

		repo.Stubs[path] = target
	}
}

// resolveStubChain follows a redirect chain up to maxRedirectDepth.
func resolveStubChain(repo *model.Repository, path model.CanonicalPath, depth int) (model.CanonicalPath, *model.Diagnostic) {
	if depth > maxRedirectDepth {
		d := model.NewErrorf(model.CodeRedirectChainTooDeep,
			"redirect chain exceeds max depth of %d at %s", maxRedirectDepth, path)
		return "", &d
	}

	task, ok := repo.Tasks[path]
	if !ok {
		return path, nil // not found, return as-is
	}

	if !task.IsStub {
		return path, nil // not a stub, we've reached the target
	}

	target := task.RedirectTo

	// Validate target is canonical (no extension)
	if strings.Contains(string(target), ".") {
		d := model.NewErrorf(model.CodeRedirectInvalidTarget,
			"redirect target is not a canonical path: %s", target)
		return "", &d
	}

	return resolveStubChain(repo, target, depth+1)
}

// resolveHierarchy handles directory/file precedence.
// When both a directory container (README.md) and a task file share the same
// canonical path, the directory container takes precedence.
func resolveHierarchy(repo *model.Repository) {
	// Group tasks by canonical path
	byPath := make(map[model.CanonicalPath][]*model.Task)
	for _, task := range repo.Tasks {
		byPath[task.Path] = append(byPath[task.Path], task)
	}

	for path, tasks := range byPath {
		if len(tasks) <= 1 {
			continue
		}

		// Find the README variant (directory container)
		var readme *model.Task
		for _, t := range tasks {
			if t.IsReadme {
				readme = t
				break
			}
		}

		if readme != nil {
			// Keep the README, remove others
			repo.Tasks[path] = readme
			repo.Diagnostics = append(repo.Diagnostics, model.NewWarningf(
				model.CodePathCaseConflict,
				"directory container takes precedence over task file at %s", path))
		}
	}
}

// resolveStatusCategories resolves status values to categories.
// Status must be one of the base categories: icebox, todo, in_progress, done.
func resolveStatusCategories(repo *model.Repository) {
	for _, task := range repo.Tasks {
		if task.IsStub || task.Status == "" {
			continue
		}

		switch model.StatusCategory(task.Status) {
		case model.StatusIcebox, model.StatusTodo, model.StatusInProgress, model.StatusDone:
			task.Category = model.StatusCategory(task.Status)
		}
	}
}

// AssigneeResult holds the resolved assignee type and value.
type AssigneeResult struct {
	Type  string // "person" or "team"
	Value string // resolved value
}

// ResolveAssignee resolves an assignee value to person or team type.
func ResolveAssignee(assignee string, teams map[string]*model.Team) (AssigneeResult, model.Diagnostics) {
	var diags model.Diagnostics

	// team:<name> format
	if strings.HasPrefix(assignee, "team:") {
		return AssigneeResult{Type: "team", Value: assignee}, nil
	}

	// Email address — resolve directly
	if strings.Contains(assignee, "@") {
		return AssigneeResult{Type: "person", Value: assignee}, nil
	}

	// Member identifier — look up across all teams
	type match struct {
		teamName string
		value    string
	}
	var matches []match

	// Sort team names for deterministic lexical ordering
	teamNames := make([]string, 0, len(teams))
	for name := range teams {
		teamNames = append(teamNames, name)
	}
	sort.Strings(teamNames)

	for _, teamName := range teamNames {
		team := teams[teamName]
		if member, ok := team.Members[assignee]; ok {
			matches = append(matches, match{teamName: teamName, value: member.Value})
		}
	}

	if len(matches) == 0 {
		diags = append(diags, model.NewWarningf(model.CodeAssigneeUnknownMember,
			"assignee %q not found in any team", assignee))
		return AssigneeResult{Type: "person", Value: assignee}, diags
	}

	if len(matches) > 1 {
		// Check if values actually differ
		allSame := true
		for _, m := range matches[1:] {
			if m.value != matches[0].value {
				allSame = false
				break
			}
		}
		if !allSame {
			diags = append(diags, model.NewWarningf(model.CodeAssigneeMemberConflict,
				"member %q found in multiple teams with different values", assignee))
		}
	}

	// Lexically first team takes priority (matches are already sorted)
	return AssigneeResult{Type: "person", Value: matches[0].value}, diags
}

// resolveDependencies resolves dependency paths through stubs.
func resolveDependencies(repo *model.Repository) {
	for _, task := range repo.Tasks {
		if task.IsStub {
			continue
		}
		for i, dep := range task.Dependencies {
			if target, ok := repo.Stubs[dep]; ok {
				task.Dependencies[i] = target
			}
		}
	}
}
