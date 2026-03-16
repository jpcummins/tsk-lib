// Package engine is the top-level orchestrator for the tsk system.
// It wires together scanning, parsing, storage, query parsing, and
// SQL compilation. This is the primary public API for consumers.
package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/parse"
	"github.com/jpcummins/tsk-lib/query"
	"github.com/jpcummins/tsk-lib/scan"
	tsql "github.com/jpcummins/tsk-lib/sql"
	"github.com/jpcummins/tsk-lib/store"
)

// Engine is the main entry point for tsk operations.
// It holds references to all subsystem interfaces.
type Engine struct {
	scanner  scan.Scanner
	parser   parse.Parser
	store    store.Store
	compiler tsql.Compiler
	qparser  query.Parser
	qvalid   query.Validator

	// Runtime context for query compilation.
	currentUser string
}

// Option configures an Engine.
type Option func(*Engine)

// WithCurrentUser sets the current user identity for me() resolution.
func WithCurrentUser(user string) Option {
	return func(e *Engine) {
		e.currentUser = user
	}
}

// New creates a new Engine with the given dependencies.
func New(
	scanner scan.Scanner,
	parser parse.Parser,
	st store.Store,
	compiler tsql.Compiler,
	qparser query.Parser,
	qvalid query.Validator,
	opts ...Option,
) *Engine {
	e := &Engine{
		scanner:  scanner,
		parser:   parser,
		store:    st,
		compiler: compiler,
		qparser:  qparser,
		qvalid:   qvalid,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// NewDefault creates an Engine with all default implementations.
// The dbPath is the SQLite database path (use ":memory:" for testing).
func NewDefault(dbPath string, opts ...Option) (*Engine, error) {
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	e := New(
		scan.NewFSScanner(),
		parse.NewParser(),
		st,
		tsql.NewCompiler(),
		query.NewParser(),
		query.NewValidator(),
		opts...,
	)
	return e, nil
}

// Close releases engine resources (primarily the database connection).
func (e *Engine) Close() error {
	return e.store.Close()
}

// Index scans the filesystem at root, parses all entries, and writes
// the resolved repository to the database.
func (e *Engine) Index(root string) (*model.Repository, error) {
	entries, err := e.scanner.Scan(root)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", root, err)
	}

	repo, err := e.parser.Parse(entries)
	if err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}

	repo.Root = root

	if err := e.store.WriteRepository(repo); err != nil {
		return nil, fmt.Errorf("writing to store: %w", err)
	}

	return repo, nil
}

// Search parses a DSL query string and returns matching tasks.
func (e *Engine) Search(queryStr string) ([]*model.Task, error) {
	ast, err := e.qparser.Parse(queryStr)
	if err != nil {
		return nil, fmt.Errorf("parsing query: %w", err)
	}

	if err := e.qvalid.Validate(ast); err != nil {
		return nil, fmt.Errorf("validating query: %w", err)
	}

	ctx := &engineContext{engine: e}

	sqlStr, params, err := e.compiler.Compile(ast, ctx)
	if err != nil {
		return nil, fmt.Errorf("compiling query: %w", err)
	}

	tasks, err := e.store.QueryTasks(sqlStr, params)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}

	return tasks, nil
}

// engineContext implements sql.CompileContext using the engine's store
// and runtime configuration.
type engineContext struct {
	engine *Engine
}

func (c *engineContext) CurrentUser() string {
	return c.engine.currentUser
}

func (c *engineContext) CurrentUserTeams() []string {
	if c.engine.currentUser == "" {
		return nil
	}

	teamNames, err := c.engine.store.AllTeamNames()
	if err != nil {
		return nil
	}

	var teams []string
	for _, teamName := range teamNames {
		members, err := c.engine.store.TeamMembers(teamName)
		if err != nil {
			continue
		}
		for _, m := range members {
			if m.Email == c.engine.currentUser {
				teams = append(teams, teamName)
				break
			}
		}
	}
	return teams
}

func (c *engineContext) TeamMembers(team string) ([]string, error) {
	members, err := c.engine.store.TeamMembers(team)
	if err != nil {
		return nil, err
	}
	var emails []string
	for _, m := range members {
		emails = append(emails, m.Email)
	}
	return emails, nil
}

func (c *engineContext) ResolveDate(token string) (time.Time, error) {
	now := time.Now().UTC()

	switch strings.ToLower(token) {
	case "today":
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
	case "yesterday":
		y, m, d := now.AddDate(0, 0, -1).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
	case "tomorrow":
		y, m, d := now.AddDate(0, 0, 1).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
	default:
		return time.Parse(time.RFC3339, token)
	}
}
