package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jp/tsk-lib/model"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver.
)

// Store is the persistence interface for indexed tsk data.
type Store interface {
	Writer
	Reader
	QueryExecutor
	Close() error
}

// Writer ingests a fully resolved repository into the database.
type Writer interface {
	WriteRepository(repo *model.Repository) error
}

// Reader provides read access to stored entities.
type Reader interface {
	TaskByPath(path model.CanonicalPath) (*model.Task, error)
	AllTasks() ([]*model.Task, error)
	IterationsByTeam(team string) ([]*model.Iteration, error)
	TeamMembers(team string) ([]model.TeamMember, error)
	AllTeamNames() ([]string, error)
}

// QueryExecutor runs a compiled SQL query and returns matching tasks.
type QueryExecutor interface {
	QueryTasks(query string, params []any) ([]*model.Task, error)
}

// SQLiteStore is the default SQLite-backed Store implementation.
type SQLiteStore struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at the given path.
// Use ":memory:" for an in-memory database (useful for testing).
func Open(dsn string) (*SQLiteStore, error) {
	// For in-memory databases, use shared cache so all connections
	// within this pool see the same database.
	if dsn == ":memory:" {
		dsn = "file::memory:?cache=shared"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for concurrent reads.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	// Create schema — execute each statement individually since some
	// drivers don't support multi-statement exec.
	for _, stmt := range splitSQL(schemaSQL) {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("creating schema (%s): %w", truncate(stmt, 60), err)
		}
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for advanced use cases.
func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}

// splitSQL splits a multi-statement SQL string into individual statements.
func splitSQL(sql string) []string {
	var stmts []string
	for _, part := range strings.Split(sql, ";") {
		stmt := strings.TrimSpace(part)
		if stmt != "" && !strings.HasPrefix(stmt, "--") {
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}

// truncate shortens a string for error messages.
func truncate(s string, maxLen int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
