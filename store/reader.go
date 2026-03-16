package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jpcummins/tsk-lib/model"
)

// TaskByPath returns a single task by canonical path.
func (s *SQLiteStore) TaskByPath(path model.CanonicalPath) (*model.Task, error) {
	row := s.db.QueryRow(`
		SELECT canonical_path, parent_path, date, due, assignee, summary,
			estimate_mins, status, status_category, updated_at, weight, body, is_readme
		FROM tasks WHERE canonical_path = ?`, string(path))

	task, err := scanTask(row)
	if err != nil {
		return nil, fmt.Errorf("reading task %s: %w", path, err)
	}

	if err := s.loadTaskRelations(task); err != nil {
		return nil, err
	}

	return task, nil
}

// AllTasks returns all tasks in the store.
func (s *SQLiteStore) AllTasks() ([]*model.Task, error) {
	rows, err := s.db.Query(`
		SELECT canonical_path, parent_path, date, due, assignee, summary,
			estimate_mins, status, status_category, updated_at, weight, body, is_readme
		FROM tasks ORDER BY canonical_path`)
	if err != nil {
		return nil, fmt.Errorf("querying all tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		task, err := scanTaskFromRows(rows)
		if err != nil {
			return nil, err
		}
		if err := s.loadTaskRelations(task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

// IterationsByTeam returns all iterations for a given team.
func (s *SQLiteStore) IterationsByTeam(team string) ([]*model.Iteration, error) {
	rows, err := s.db.Query(`
		SELECT canonical_path, name, team, start, end, status, status_category, capacity_mins
		FROM iterations WHERE team = ? ORDER BY start`,
		team)
	if err != nil {
		return nil, fmt.Errorf("querying iterations: %w", err)
	}
	defer rows.Close()

	var iterations []*model.Iteration
	for rows.Next() {
		iter, err := scanIteration(rows)
		if err != nil {
			return nil, err
		}

		// Load task references.
		taskRows, err := s.db.Query(`
			SELECT task_path FROM iteration_tasks
			WHERE iteration_path = ? ORDER BY position`,
			string(iter.CanonicalPath))
		if err != nil {
			return nil, err
		}
		for taskRows.Next() {
			var tp string
			if err := taskRows.Scan(&tp); err != nil {
				taskRows.Close()
				return nil, err
			}
			iter.Tasks = append(iter.Tasks, model.CanonicalPath(tp))
		}
		taskRows.Close()

		iterations = append(iterations, iter)
	}

	return iterations, rows.Err()
}

// TeamMembers returns the members of a given team.
func (s *SQLiteStore) TeamMembers(team string) ([]model.TeamMember, error) {
	rows, err := s.db.Query(`
		SELECT display, name, email FROM team_members WHERE team_name = ?`, team)
	if err != nil {
		return nil, fmt.Errorf("querying team members: %w", err)
	}
	defer rows.Close()

	var members []model.TeamMember
	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.Display, &m.Name, &m.Email); err != nil {
			return nil, err
		}
		members = append(members, m)
	}

	return members, rows.Err()
}

// AllTeamNames returns all team names in the store.
func (s *SQLiteStore) AllTeamNames() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM teams ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying teams: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}

	return names, rows.Err()
}

// QueryTasks executes a compiled SQL query and returns matching tasks.
func (s *SQLiteStore) QueryTasks(query string, params []any) ([]*model.Task, error) {
	rows, err := s.db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		task, err := scanTaskFromRows(rows)
		if err != nil {
			return nil, err
		}
		if err := s.loadTaskRelations(task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

// loadTaskRelations populates a task's dependencies, labels, and status log.
func (s *SQLiteStore) loadTaskRelations(task *model.Task) error {
	// Dependencies.
	depRows, err := s.db.Query(`
		SELECT dependency_path FROM task_dependencies WHERE task_path = ?`,
		string(task.CanonicalPath))
	if err != nil {
		return err
	}
	defer depRows.Close()
	for depRows.Next() {
		var dep string
		if err := depRows.Scan(&dep); err != nil {
			return err
		}
		task.Dependencies = append(task.Dependencies, model.CanonicalPath(dep))
	}

	// Labels.
	labelRows, err := s.db.Query(`
		SELECT label FROM task_labels WHERE task_path = ?`,
		string(task.CanonicalPath))
	if err != nil {
		return err
	}
	defer labelRows.Close()
	for labelRows.Next() {
		var label string
		if err := labelRows.Scan(&label); err != nil {
			return err
		}
		task.Labels = append(task.Labels, label)
	}

	// Status log.
	logRows, err := s.db.Query(`
		SELECT status, at FROM status_log WHERE task_path = ? ORDER BY at`,
		string(task.CanonicalPath))
	if err != nil {
		return err
	}
	defer logRows.Close()
	for logRows.Next() {
		var entry model.StatusLogEntry
		var atStr string
		if err := logRows.Scan(&entry.Status, &atStr); err != nil {
			return err
		}
		entry.At, _ = time.Parse(time.RFC3339, atStr)
		task.StatusLog = append(task.StatusLog, entry)
	}

	return nil
}

// scanTask scans a single task row from a *sql.Row.
func scanTask(row *sql.Row) (*model.Task, error) {
	task := &model.Task{}
	var (
		cp, parent, dateStr, assignee, summary, status, statusCat, body string
		due, updatedAt                                                  *string
		estimateMins                                                    *int
		weight                                                          *int
		isReadme                                                        bool
	)

	err := row.Scan(&cp, &parent, &dateStr, &due, &assignee, &summary,
		&estimateMins, &status, &statusCat, &updatedAt, &weight, &body, &isReadme)
	if err != nil {
		return nil, err
	}

	return populateTask(task, cp, parent, dateStr, due, assignee, summary,
		estimateMins, status, statusCat, updatedAt, weight, body, isReadme), nil
}

// scanTaskFromRows scans a single task from *sql.Rows.
func scanTaskFromRows(rows *sql.Rows) (*model.Task, error) {
	task := &model.Task{}
	var (
		cp, parent, dateStr, assignee, summary, status, statusCat, body string
		due, updatedAt                                                  *string
		estimateMins                                                    *int
		weight                                                          *int
		isReadme                                                        bool
	)

	err := rows.Scan(&cp, &parent, &dateStr, &due, &assignee, &summary,
		&estimateMins, &status, &statusCat, &updatedAt, &weight, &body, &isReadme)
	if err != nil {
		return nil, err
	}

	return populateTask(task, cp, parent, dateStr, due, assignee, summary,
		estimateMins, status, statusCat, updatedAt, weight, body, isReadme), nil
}

func populateTask(task *model.Task,
	cp, parent, dateStr string, due *string, assignee, summary string,
	estimateMins *int, status, statusCat string, updatedAt *string,
	weight *int, body string, isReadme bool,
) *model.Task {
	task.CanonicalPath = model.CanonicalPath(cp)
	task.ParentPath = model.CanonicalPath(parent)
	task.Date, _ = time.Parse(time.RFC3339, dateStr)
	task.Assignee = assignee
	task.Summary = summary
	task.Status = status
	task.StatusCategory = model.StatusCategory(statusCat)
	task.Body = body
	task.IsReadme = isReadme
	task.Weight = weight

	if due != nil {
		t, _ := time.Parse(time.RFC3339, *due)
		task.Due = &t
	}
	if updatedAt != nil {
		t, _ := time.Parse(time.RFC3339, *updatedAt)
		task.UpdatedAt = &t
	}
	if estimateMins != nil {
		task.Estimate = &model.Duration{Minutes: *estimateMins}
	}

	return task
}

// scanIteration scans an iteration row.
func scanIteration(rows *sql.Rows) (*model.Iteration, error) {
	iter := &model.Iteration{}
	var (
		cp, name, team, startStr, endStr, status, statusCat string
		capacityMins                                        *int
	)

	err := rows.Scan(&cp, &name, &team, &startStr, &endStr,
		&status, &statusCat, &capacityMins)
	if err != nil {
		return nil, err
	}

	iter.CanonicalPath = model.CanonicalPath(cp)
	iter.Name = name
	iter.Team = team
	iter.Start, _ = time.Parse(time.RFC3339, startStr)
	iter.End, _ = time.Parse(time.RFC3339, endStr)
	iter.Status = status
	iter.StatusCategory = model.StatusCategory(statusCat)

	if capacityMins != nil {
		iter.Capacity = &model.Duration{Minutes: *capacityMins}
	}

	return iter, nil
}
