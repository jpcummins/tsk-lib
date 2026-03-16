// Package store provides SQLite persistence for indexed tsk data.
// It defines the Store interface and a default implementation backed by
// modernc.org/sqlite (pure Go, no CGo).
package store

// schemaSQL contains the DDL for the tsk SQLite database.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS tasks (
    canonical_path  TEXT PRIMARY KEY,
    parent_path     TEXT NOT NULL DEFAULT '',
    date            TEXT NOT NULL DEFAULT '',
    due             TEXT,
    assignee        TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    estimate_mins   INTEGER,
    status          TEXT NOT NULL DEFAULT '',
    status_category TEXT NOT NULL DEFAULT '',
    updated_at      TEXT,
    weight          INTEGER,
    body            TEXT NOT NULL DEFAULT '',
    is_readme       INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS task_dependencies (
    task_path       TEXT NOT NULL REFERENCES tasks(canonical_path),
    dependency_path TEXT NOT NULL,
    PRIMARY KEY (task_path, dependency_path)
);

CREATE TABLE IF NOT EXISTS task_labels (
    task_path TEXT NOT NULL REFERENCES tasks(canonical_path),
    label     TEXT NOT NULL,
    PRIMARY KEY (task_path, label)
);

CREATE TABLE IF NOT EXISTS status_log (
    task_path TEXT NOT NULL REFERENCES tasks(canonical_path),
    status    TEXT NOT NULL,
    at        TEXT NOT NULL,
    PRIMARY KEY (task_path, at)
);

CREATE TABLE IF NOT EXISTS iterations (
    canonical_path  TEXT PRIMARY KEY,
    name            TEXT NOT NULL DEFAULT '',
    team            TEXT NOT NULL DEFAULT '',
    start           TEXT NOT NULL DEFAULT '',
    end             TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT '',
    status_category TEXT NOT NULL DEFAULT '',
    capacity_mins   INTEGER
);

CREATE TABLE IF NOT EXISTS iteration_tasks (
    iteration_path TEXT NOT NULL REFERENCES iterations(canonical_path),
    task_path      TEXT NOT NULL,
    position       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (iteration_path, task_path)
);

CREATE TABLE IF NOT EXISTS teams (
    name TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS team_members (
    team_name TEXT NOT NULL REFERENCES teams(name),
    display   TEXT NOT NULL DEFAULT '',
    name      TEXT NOT NULL DEFAULT '',
    email     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (team_name, email)
);

CREATE TABLE IF NOT EXISTS sla_rules (
    id       TEXT PRIMARY KEY,
    name     TEXT NOT NULL DEFAULT '',
    query    TEXT NOT NULL DEFAULT '',
    target_mins INTEGER NOT NULL DEFAULT 0,
    start    TEXT NOT NULL DEFAULT '',
    stop     TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT ''
);

-- Indexes for common query patterns.
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_status_category ON tasks(status_category);
CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assignee);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_path);
CREATE INDEX IF NOT EXISTS idx_task_labels_label ON task_labels(label);
CREATE INDEX IF NOT EXISTS idx_task_deps_dep ON task_dependencies(dependency_path);
CREATE INDEX IF NOT EXISTS idx_iteration_tasks_task ON iteration_tasks(task_path);
CREATE INDEX IF NOT EXISTS idx_iterations_team ON iterations(team);
`
