package store

// schema contains the DDL for the tsk SQLite database.
const schema = `
CREATE TABLE IF NOT EXISTS tasks (
	path TEXT PRIMARY KEY,
	parent TEXT,
	is_readme INTEGER NOT NULL DEFAULT 0,
	is_stub INTEGER NOT NULL DEFAULT 0,
	redirect_to TEXT,
	created_at TEXT,
	due TEXT,
	assignee TEXT,
	summary TEXT,
	estimate TEXT,
	status TEXT,
	status_category TEXT,
	updated_at TEXT,
	type TEXT,
	weight REAL,
	body TEXT
);

CREATE TABLE IF NOT EXISTS task_labels (
	task_path TEXT NOT NULL,
	value TEXT NOT NULL,
	FOREIGN KEY (task_path) REFERENCES tasks(path)
);

CREATE TABLE IF NOT EXISTS task_dependencies (
	task_path TEXT NOT NULL,
	value TEXT NOT NULL,
	FOREIGN KEY (task_path) REFERENCES tasks(path)
);

CREATE TABLE IF NOT EXISTS change_log (
	task_path TEXT NOT NULL,
	field TEXT NOT NULL,
	from_value TEXT,
	to_value TEXT,
	at TEXT NOT NULL,
	FOREIGN KEY (task_path) REFERENCES tasks(path)
);

CREATE TABLE IF NOT EXISTS iterations (
	id TEXT PRIMARY KEY,
	team TEXT NOT NULL,
	start_time TEXT NOT NULL,
	end_time TEXT NOT NULL,
	body TEXT
);

CREATE TABLE IF NOT EXISTS iteration_tasks (
	iteration_id TEXT NOT NULL,
	task_path TEXT NOT NULL,
	sort_order INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY (iteration_id) REFERENCES iterations(id)
);

CREATE TABLE IF NOT EXISTS teams (
	name TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS team_members (
	team_name TEXT NOT NULL,
	identifier TEXT NOT NULL,
	value TEXT NOT NULL,
	display_name TEXT,
	email TEXT,
	FOREIGN KEY (team_name) REFERENCES teams(name)
);

CREATE TABLE IF NOT EXISTS sla_rules (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	query TEXT NOT NULL,
	target TEXT NOT NULL,
	warn_at TEXT,
	start_event TEXT NOT NULL,
	stop_event TEXT NOT NULL,
	severity TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sla_results (
	rule_id TEXT NOT NULL,
	task_path TEXT NOT NULL,
	status TEXT NOT NULL,
	start_time TEXT,
	stop_time TEXT,
	target TEXT NOT NULL,
	elapsed TEXT NOT NULL,
	remaining TEXT NOT NULL,
	FOREIGN KEY (rule_id) REFERENCES sla_rules(id)
);

CREATE TABLE IF NOT EXISTS repository_meta (
	key TEXT PRIMARY KEY,
	value TEXT
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assignee);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent);
CREATE INDEX IF NOT EXISTS idx_tasks_category ON tasks(status_category);
CREATE INDEX IF NOT EXISTS idx_labels_task ON task_labels(task_path);
CREATE INDEX IF NOT EXISTS idx_labels_value ON task_labels(value);
CREATE INDEX IF NOT EXISTS idx_deps_task ON task_dependencies(task_path);
CREATE INDEX IF NOT EXISTS idx_iter_tasks ON iteration_tasks(iteration_id);
CREATE INDEX IF NOT EXISTS idx_iter_tasks_path ON iteration_tasks(task_path);
CREATE INDEX IF NOT EXISTS idx_sla_results_task ON sla_results(task_path);
`
