package store

import (
	"database/sql"
	"fmt"

	"github.com/jpcummins/tsk-lib/model"
)

// WriteRepository persists a fully resolved repository into SQLite.
// It clears existing data and writes everything in a single transaction.
func (s *SQLiteStore) WriteRepository(repo *model.Repository) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing data.
	if err := clearTables(tx); err != nil {
		return err
	}

	// Write tasks.
	for _, task := range repo.Tasks {
		if err := writeTask(tx, task); err != nil {
			return fmt.Errorf("writing task %s: %w", task.CanonicalPath, err)
		}
	}

	// Write iterations.
	for _, iter := range repo.Iterations {
		if err := writeIteration(tx, iter); err != nil {
			return fmt.Errorf("writing iteration %s: %w", iter.CanonicalPath, err)
		}
	}

	// Write teams.
	for _, team := range repo.Teams {
		if err := writeTeam(tx, team); err != nil {
			return fmt.Errorf("writing team %s: %w", team.Name, err)
		}
	}

	// Write SLA rules.
	for _, rule := range repo.SLARules {
		if err := writeSLARule(tx, rule); err != nil {
			return fmt.Errorf("writing SLA rule %s: %w", rule.ID, err)
		}
	}

	return tx.Commit()
}

func clearTables(tx *sql.Tx) error {
	tables := []string{
		"task_labels", "task_dependencies", "status_log",
		"iteration_tasks", "iterations",
		"team_members", "teams",
		"sla_rules", "tasks",
	}
	for _, table := range tables {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("clearing %s: %w", table, err)
		}
	}
	return nil
}

func writeTask(tx *sql.Tx, task *model.Task) error {
	var estimateMins *int
	if task.Estimate != nil {
		v := task.Estimate.Minutes
		estimateMins = &v
	}

	var due, updatedAt *string
	if task.Due != nil {
		s := task.Due.Format("2006-01-02T15:04:05Z07:00")
		due = &s
	}
	if task.UpdatedAt != nil {
		s := task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		updatedAt = &s
	}

	dateStr := ""
	if !task.Date.IsZero() {
		dateStr = task.Date.Format("2006-01-02T15:04:05Z07:00")
	}

	_, err := tx.Exec(`
		INSERT INTO tasks (canonical_path, parent_path, date, due, assignee,
			summary, estimate_mins, status, status_category, updated_at,
			weight, body, is_readme)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(task.CanonicalPath),
		string(task.ParentPath),
		dateStr,
		due,
		task.Assignee,
		task.Summary,
		estimateMins,
		task.Status,
		string(task.StatusCategory),
		updatedAt,
		task.Weight,
		task.Body,
		task.IsReadme,
	)
	if err != nil {
		return err
	}

	// Write dependencies.
	for _, dep := range task.Dependencies {
		_, err := tx.Exec(`
			INSERT INTO task_dependencies (task_path, dependency_path)
			VALUES (?, ?)`,
			string(task.CanonicalPath), string(dep))
		if err != nil {
			return fmt.Errorf("writing dependency: %w", err)
		}
	}

	// Write labels.
	for _, label := range task.Labels {
		_, err := tx.Exec(`
			INSERT INTO task_labels (task_path, label) VALUES (?, ?)`,
			string(task.CanonicalPath), label)
		if err != nil {
			return fmt.Errorf("writing label: %w", err)
		}
	}

	// Write status log.
	for _, entry := range task.StatusLog {
		_, err := tx.Exec(`
			INSERT INTO status_log (task_path, status, at) VALUES (?, ?, ?)`,
			string(task.CanonicalPath), entry.Status,
			entry.At.Format("2006-01-02T15:04:05Z07:00"))
		if err != nil {
			return fmt.Errorf("writing status log: %w", err)
		}
	}

	return nil
}

func writeIteration(tx *sql.Tx, iter *model.Iteration) error {
	var capacityMins *int
	if iter.Capacity != nil {
		v := iter.Capacity.Minutes
		capacityMins = &v
	}

	_, err := tx.Exec(`
		INSERT INTO iterations (canonical_path, name, team, start, end,
			status, status_category, capacity_mins)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		string(iter.CanonicalPath),
		iter.Name,
		iter.Team,
		iter.Start.Format("2006-01-02T15:04:05Z07:00"),
		iter.End.Format("2006-01-02T15:04:05Z07:00"),
		iter.Status,
		string(iter.StatusCategory),
		capacityMins,
	)
	if err != nil {
		return err
	}

	// Write task references with position.
	for i, taskPath := range iter.Tasks {
		_, err := tx.Exec(`
			INSERT INTO iteration_tasks (iteration_path, task_path, position)
			VALUES (?, ?, ?)`,
			string(iter.CanonicalPath), string(taskPath), i)
		if err != nil {
			return fmt.Errorf("writing iteration task: %w", err)
		}
	}

	return nil
}

func writeTeam(tx *sql.Tx, team *model.Team) error {
	_, err := tx.Exec(`INSERT INTO teams (name) VALUES (?)`, team.Name)
	if err != nil {
		return err
	}

	for _, member := range team.Members {
		_, err := tx.Exec(`
			INSERT INTO team_members (team_name, display, name, email)
			VALUES (?, ?, ?, ?)`,
			team.Name, member.Display, member.Name, member.Email)
		if err != nil {
			return fmt.Errorf("writing team member: %w", err)
		}
	}

	return nil
}

func writeSLARule(tx *sql.Tx, rule *model.SLARule) error {
	_, err := tx.Exec(`
		INSERT INTO sla_rules (id, name, query, target_mins, start, stop, severity)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Name, rule.Query, rule.Target.Minutes,
		rule.Start, rule.Stop, rule.Severity)
	return err
}
