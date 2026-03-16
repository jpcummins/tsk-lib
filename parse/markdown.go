package parse

import (
	"strings"

	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/scan"
)

// parseTask converts a scanned task entry into a model.Task.
// This is Phase 1 parsing — no resolution (inheritance, labels, redirects) yet.
func parseTask(entry scan.Entry) (*model.Task, error) {
	fm, body, err := extractFrontMatter(entry.Content)
	if err != nil {
		return nil, err
	}

	// Compute canonical path: strip "tasks/" prefix, normalize.
	relPath := strings.TrimPrefix(entry.Path, "tasks/")
	isReadme := isReadmeFile(entry.Path)

	// For README.md files, the canonical path is the directory.
	if isReadme {
		relPath = strings.TrimSuffix(relPath, "/README.md")
		relPath = strings.TrimSuffix(relPath, "/readme.md")
	}

	canonical := model.NormalizePath(relPath)

	task := &model.Task{
		CanonicalPath: canonical,
		ParentPath:    canonical.Parent(),
		IsReadme:      isReadme,
		Body:          body,
	}

	if fm == nil {
		return task, nil
	}

	// Check if this is a redirect stub.
	if fm.RedirectTo != "" {
		task.IsStub = true
		task.RedirectTo = model.CanonicalPath(fm.RedirectTo)
		return task, nil
	}

	// Required fields.
	if fm.Date != nil {
		task.Date = *fm.Date
	}

	// Optional fields.
	task.Due = fm.Due
	task.Assignee = fm.Assignee
	task.Summary = fm.Summary
	task.Status = fm.Status
	task.UpdatedAt = fm.UpdatedAt
	task.Labels = fm.Labels
	task.Weight = fm.Weight

	// Dependencies.
	for _, dep := range fm.Dependencies {
		task.Dependencies = append(task.Dependencies, model.CanonicalPath(dep))
	}

	// Estimate.
	if fm.Estimate != "" {
		dur, err := model.ParseDuration(fm.Estimate)
		if err != nil {
			return nil, err
		}
		task.Estimate = &dur
	}

	// Status log.
	for _, entry := range fm.StatusLog {
		task.StatusLog = append(task.StatusLog, model.StatusLogEntry{
			Status: entry.Status,
			At:     entry.At,
		})
	}

	return task, nil
}

// parseIteration converts a scanned iteration entry into a model.Iteration.
func parseIteration(entry scan.Entry) (*model.Iteration, error) {
	fm, body, err := extractFrontMatter(entry.Content)
	if err != nil {
		return nil, err
	}

	iter := &model.Iteration{
		Body: body,
	}

	// Derive team from path: teams/<team>/iterations/<file>.md
	parts := strings.Split(entry.Path, "/")
	if len(parts) >= 2 {
		iter.Team = parts[1]
	}

	// Build canonical path for the iteration.
	iter.CanonicalPath = model.NormalizePath(entry.Path)

	if fm == nil {
		return iter, nil
	}

	iter.Name = fm.Name
	iter.Status = fm.Status

	if fm.Team != "" {
		iter.Team = fm.Team
	}

	if fm.Start != nil {
		iter.Start = *fm.Start
	}
	if fm.End != nil {
		iter.End = *fm.End
	}

	// Task references.
	for _, t := range fm.Tasks {
		iter.Tasks = append(iter.Tasks, model.CanonicalPath(t))
	}

	// Capacity.
	if fm.Capacity != "" {
		dur, err := model.ParseDuration(fm.Capacity)
		if err != nil {
			return nil, err
		}
		iter.Capacity = &dur
	}

	return iter, nil
}

// isReadmeFile returns true if the path ends with README.md (case-insensitive).
func isReadmeFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, "/readme.md") || lower == "readme.md"
}
