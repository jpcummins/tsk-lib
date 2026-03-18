// Package store provides SQLite persistence for tsk repositories.
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jpcummins/tsk-lib/model"
	_ "modernc.org/sqlite"
)

// Store is the persistence interface for tsk repositories.
type Store interface {
	Writer
	Reader
	QueryExecutor
	Close() error
}

// Writer handles bulk write operations.
type Writer interface {
	WriteRepository(repo *model.Repository) error
}

// Reader handles read operations.
type Reader interface {
	TaskByPath(path model.CanonicalPath) (*model.Task, error)
	AllTasks() ([]*model.Task, error)
	IterationsByTeam(team string) ([]*model.Iteration, error)
	TeamMembers(teamName string) ([]model.TeamMember, error)
	AllTeamNames() ([]string, error)
}

// QueryExecutor executes compiled SQL queries.
type QueryExecutor interface {
	QueryTasks(query string, params []any) ([]*model.Task, error)
}

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at the given path.
func Open(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// WriteRepository writes a complete repository to the database in a single transaction.
func (s *SQLiteStore) WriteRepository(repo *model.Repository) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing data
	tables := []string{"sla_results", "sla_rules", "team_members", "teams",
		"iteration_tasks", "iterations", "change_log", "task_dependencies",
		"task_labels", "tasks", "repository_meta"}
	for _, table := range tables {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("clearing %s: %w", table, err)
		}
	}

	// Write version
	if repo.Version != "" {
		if _, err := tx.Exec("INSERT INTO repository_meta (key, value) VALUES ('version', ?)", repo.Version); err != nil {
			return err
		}
	}

	// Write tasks
	for _, task := range repo.Tasks {
		var createdAt, due, updatedAt, estimate *string
		if task.CreatedAt != nil {
			s := task.CreatedAt.Format(time.RFC3339)
			createdAt = &s
		}
		if task.Due != nil {
			s := task.Due.Format(time.RFC3339)
			due = &s
		}
		if task.UpdatedAt != nil {
			s := task.UpdatedAt.Format(time.RFC3339)
			updatedAt = &s
		}
		if task.Estimate != nil {
			s := task.Estimate.String()
			estimate = &s
		}

		_, err := tx.Exec(`INSERT INTO tasks (path, parent, is_readme, is_stub, redirect_to,
			created_at, due, assignee, summary, estimate, status, status_category,
			updated_at, type, weight, body) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			string(task.Path), string(task.Parent), task.IsReadme, task.IsStub,
			string(task.RedirectTo), createdAt, due, task.Assignee, task.Summary,
			estimate, task.Status, string(task.Category), updatedAt,
			task.Type, task.Weight, task.Body)
		if err != nil {
			return fmt.Errorf("writing task %s: %w", task.Path, err)
		}

		// Labels
		for _, label := range task.Labels {
			if _, err := tx.Exec("INSERT INTO task_labels (task_path, value) VALUES (?, ?)",
				string(task.Path), label); err != nil {
				return err
			}
		}

		// Dependencies
		for _, dep := range task.Dependencies {
			if _, err := tx.Exec("INSERT INTO task_dependencies (task_path, value) VALUES (?, ?)",
				string(task.Path), string(dep)); err != nil {
				return err
			}
		}

		// Change log
		for _, cl := range task.ChangeLog {
			if _, err := tx.Exec("INSERT INTO change_log (task_path, field, from_value, to_value, at) VALUES (?, ?, ?, ?, ?)",
				string(task.Path), cl.Field, cl.From, cl.To, cl.At.Format(time.RFC3339)); err != nil {
				return err
			}
		}
	}

	// Write teams
	for _, team := range repo.Teams {
		if _, err := tx.Exec("INSERT INTO teams (name) VALUES (?)", team.Name); err != nil {
			return err
		}
		for _, member := range team.Members {
			if _, err := tx.Exec("INSERT INTO team_members (team_name, identifier, value, display_name, email) VALUES (?, ?, ?, ?, ?)",
				team.Name, member.Identifier, member.Value, member.Name, member.Email); err != nil {
				return err
			}
		}
	}

	// Write iterations
	for _, iter := range repo.Iterations {
		if _, err := tx.Exec("INSERT INTO iterations (id, team, start_time, end_time, body) VALUES (?, ?, ?, ?, ?)",
			iter.ID, iter.Team, iter.Start.Format(time.RFC3339), iter.End.Format(time.RFC3339), iter.Body); err != nil {
			return err
		}
		for i, tp := range iter.Tasks {
			if _, err := tx.Exec("INSERT INTO iteration_tasks (iteration_id, task_path, sort_order) VALUES (?, ?, ?)",
				iter.ID, string(tp), i); err != nil {
				return err
			}
		}
	}

	// Write SLA rules
	for _, rule := range repo.SLARules {
		var warnAt *string
		if rule.WarnAt != nil {
			s := rule.WarnAt.String()
			warnAt = &s
		}
		if _, err := tx.Exec("INSERT INTO sla_rules (id, name, query, target, warn_at, start_event, stop_event, severity) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			rule.ID, rule.Name, rule.Query, rule.Target.String(), warnAt, rule.Start, rule.Stop, rule.Severity); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// TaskByPath retrieves a single task by its canonical path.
func (s *SQLiteStore) TaskByPath(path model.CanonicalPath) (*model.Task, error) {
	row := s.db.QueryRow("SELECT path, parent, is_readme, is_stub, redirect_to, created_at, due, assignee, summary, estimate, status, status_category, updated_at, type, weight, body FROM tasks WHERE path = ?", string(path))
	return scanTask(row)
}

// AllTasks retrieves all non-stub tasks.
func (s *SQLiteStore) AllTasks() ([]*model.Task, error) {
	rows, err := s.db.Query("SELECT path, parent, is_readme, is_stub, redirect_to, created_at, due, assignee, summary, estimate, status, status_category, updated_at, type, weight, body FROM tasks WHERE is_stub = 0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		task, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// IterationsByTeam retrieves iterations for a team.
func (s *SQLiteStore) IterationsByTeam(team string) ([]*model.Iteration, error) {
	rows, err := s.db.Query("SELECT id, team, start_time, end_time, body FROM iterations WHERE team = ?", team)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var iters []*model.Iteration
	for rows.Next() {
		var iter model.Iteration
		var startStr, endStr string
		if err := rows.Scan(&iter.ID, &iter.Team, &startStr, &endStr, &iter.Body); err != nil {
			return nil, err
		}
		iter.Start, _ = time.Parse(time.RFC3339, startStr)
		iter.End, _ = time.Parse(time.RFC3339, endStr)
		iters = append(iters, &iter)
	}
	return iters, rows.Err()
}

// TeamMembers retrieves members of a team.
func (s *SQLiteStore) TeamMembers(teamName string) ([]model.TeamMember, error) {
	rows, err := s.db.Query("SELECT identifier, value, display_name, email FROM team_members WHERE team_name = ?", teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.TeamMember
	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.Identifier, &m.Value, &m.Name, &m.Email); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// AllTeamNames retrieves all team names.
func (s *SQLiteStore) AllTeamNames() ([]string, error) {
	rows, err := s.db.Query("SELECT name FROM teams ORDER BY name")
	if err != nil {
		return nil, err
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
func (s *SQLiteStore) QueryTasks(queryStr string, params []any) ([]*model.Task, error) {
	rows, err := s.db.Query(queryStr, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Hydrate full task objects
	var tasks []*model.Task
	for _, path := range paths {
		task, err := s.TaskByPath(model.CanonicalPath(path))
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// --- Internal helpers ---

type scannable interface {
	Scan(dest ...any) error
}

func scanTask(row *sql.Row) (*model.Task, error) {
	var task model.Task
	var parent, redirectTo, createdAt, due, updatedAt, estimate, category sql.NullString
	var weight sql.NullFloat64

	err := row.Scan(&task.Path, &parent, &task.IsReadme, &task.IsStub, &redirectTo,
		&createdAt, &due, &task.Assignee, &task.Summary, &estimate,
		&task.Status, &category, &updatedAt, &task.Type, &weight, &task.Body)
	if err != nil {
		return nil, err
	}

	if parent.Valid {
		task.Parent = model.CanonicalPath(parent.String)
	}
	if redirectTo.Valid {
		task.RedirectTo = model.CanonicalPath(redirectTo.String)
	}
	if category.Valid {
		task.Category = model.StatusCategory(category.String)
	}
	if weight.Valid {
		task.Weight = &weight.Float64
	}
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			task.CreatedAt = &t
		}
	}
	if due.Valid {
		if t, err := time.Parse(time.RFC3339, due.String); err == nil {
			task.Due = &t
		}
	}
	if updatedAt.Valid {
		if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			task.UpdatedAt = &t
		}
	}
	if estimate.Valid {
		if d, err := model.ParseDuration(estimate.String); err == nil {
			task.Estimate = &d
		}
	}

	return &task, nil
}

func scanTaskRow(rows *sql.Rows) (*model.Task, error) {
	var task model.Task
	var parent, redirectTo, createdAt, due, updatedAt, estimate, category sql.NullString
	var weight sql.NullFloat64

	err := rows.Scan(&task.Path, &parent, &task.IsReadme, &task.IsStub, &redirectTo,
		&createdAt, &due, &task.Assignee, &task.Summary, &estimate,
		&task.Status, &category, &updatedAt, &task.Type, &weight, &task.Body)
	if err != nil {
		return nil, err
	}

	if parent.Valid {
		task.Parent = model.CanonicalPath(parent.String)
	}
	if redirectTo.Valid {
		task.RedirectTo = model.CanonicalPath(redirectTo.String)
	}
	if category.Valid {
		task.Category = model.StatusCategory(category.String)
	}
	if weight.Valid {
		task.Weight = &weight.Float64
	}
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			task.CreatedAt = &t
		}
	}
	if due.Valid {
		if t, err := time.Parse(time.RFC3339, due.String); err == nil {
			task.Due = &t
		}
	}
	if updatedAt.Valid {
		if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			task.UpdatedAt = &t
		}
	}
	if estimate.Valid {
		if d, err := model.ParseDuration(estimate.String); err == nil {
			task.Estimate = &d
		}
	}

	return &task, nil
}
