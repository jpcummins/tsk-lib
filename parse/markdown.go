package parse

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/scan"
)

// parseTask parses a task from a scanned entry.
func parseTask(entry scan.Entry) (*model.Task, error) {
	fm, body, err := extractFrontMatter(entry.Content)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", entry.Path, err)
	}

	// Compute canonical path
	// entry.Path is relative to root, e.g., "tasks/launch/setup.md"
	canonPath := model.NormalizePath(entry.Path)

	// Determine if this is a README
	filename := filepath.Base(entry.Path)
	isReadme := strings.EqualFold(filename, "readme.md")

	task := &model.Task{
		Path:     canonPath,
		Parent:   canonPath.Parent(),
		IsReadme: isReadme,
		Body:     body,
	}

	// Check for redirect stub
	if fm.RedirectTo != "" {
		task.IsStub = true
		task.RedirectTo = model.CanonicalPath(fm.RedirectTo)
		return task, nil
	}

	// Parse fields
	task.Summary = fm.Summary
	task.Assignee = fm.Assignee
	task.Status = fm.Status
	task.Type = fm.Type
	task.Weight = fm.Weight

	if fm.CreatedAt != nil {
		t, err := time.Parse(time.RFC3339, *fm.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at in %s: %w", entry.Path, err)
		}
		task.CreatedAt = &t
	}

	if fm.Due != nil {
		t, err := time.Parse(time.RFC3339, *fm.Due)
		if err != nil {
			return nil, fmt.Errorf("parsing due in %s: %w", entry.Path, err)
		}
		task.Due = &t
	}

	if fm.UpdatedAt != nil {
		t, err := time.Parse(time.RFC3339, *fm.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing updated_at in %s: %w", entry.Path, err)
		}
		task.UpdatedAt = &t
	}

	if fm.Estimate != "" {
		d, err := model.ParseDuration(fm.Estimate)
		if err != nil {
			return nil, fmt.Errorf("parsing estimate in %s: %w", entry.Path, err)
		}
		task.Estimate = &d
	}

	// Dependencies
	for _, dep := range fm.Dependencies {
		task.Dependencies = append(task.Dependencies, model.CanonicalPath(dep))
	}

	// Labels
	task.Labels = fm.Labels

	// Change log
	for _, cl := range fm.ChangeLog {
		at, err := time.Parse(time.RFC3339, cl.At)
		if err != nil {
			return nil, fmt.Errorf("parsing change_log.at in %s: %w", entry.Path, err)
		}
		task.ChangeLog = append(task.ChangeLog, model.FieldChange{
			Field: cl.Field,
			From:  cl.From,
			To:    cl.To,
			At:    at,
		})
	}

	return task, nil
}

// parseIteration parses an iteration from a scanned entry.
func parseIteration(entry scan.Entry) (*model.Iteration, error) {
	fm, body, err := extractFrontMatter(entry.Content)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", entry.Path, err)
	}

	// Extract team and ID from path: teams/<team>/iterations/<file>.md
	parts := strings.Split(entry.Path, "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid iteration path: %s", entry.Path)
	}

	teamName := parts[1]
	filename := strings.TrimSuffix(parts[3], ".md")
	id := teamName + "/" + strings.ToLower(filename)

	start, err := time.Parse(time.RFC3339, fm.Start)
	if err != nil {
		return nil, fmt.Errorf("parsing iteration start in %s: %w", entry.Path, err)
	}

	end, err := time.Parse(time.RFC3339, fm.End)
	if err != nil {
		return nil, fmt.Errorf("parsing iteration end in %s: %w", entry.Path, err)
	}

	tasks := make([]model.CanonicalPath, len(fm.Tasks))
	for i, t := range fm.Tasks {
		tasks[i] = model.CanonicalPath(t)
	}

	return &model.Iteration{
		ID:    id,
		Team:  teamName,
		Start: start,
		End:   end,
		Tasks: tasks,
		Body:  body,
	}, nil
}
