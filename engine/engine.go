// Package engine is the composition root that wires all subsystems together.
package engine

import (
	"time"

	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/parse"
	"github.com/jpcummins/tsk-lib/query"
	"github.com/jpcummins/tsk-lib/scan"
	"github.com/jpcummins/tsk-lib/search"
	tsql "github.com/jpcummins/tsk-lib/sql"
	"github.com/jpcummins/tsk-lib/store"
)

// Engine is the top-level orchestrator for tsk operations.
type Engine struct {
	scanner     scan.Scanner
	parser      parse.Parser
	store       store.Store
	compiler    tsql.Compiler
	qparser     query.Parser
	qvalidator  query.Validator
	searcher    *search.Searcher
	currentUser string
}

// Option configures an Engine.
type Option func(*Engine)

// WithCurrentUser sets the current user for me()/my_team() queries.
func WithCurrentUser(user string) Option {
	return func(e *Engine) {
		e.currentUser = user
	}
}

// New creates an Engine with injected dependencies.
func New(
	scanner scan.Scanner,
	parser parse.Parser,
	st store.Store,
	compiler tsql.Compiler,
	qparser query.Parser,
	qvalidator query.Validator,
	opts ...Option,
) *Engine {
	e := &Engine{
		scanner:    scanner,
		parser:     parser,
		store:      st,
		compiler:   compiler,
		qparser:    qparser,
		qvalidator: qvalidator,
		searcher:   search.NewSearcher(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// NewDefault creates an Engine with default implementations.
func NewDefault(dbPath string, opts ...Option) (*Engine, error) {
	sc := scan.NewFSScanner()
	p := parse.NewParser()
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	comp := tsql.NewCompiler()
	qp := query.NewParser()
	qv := query.NewValidator()

	return New(sc, p, st, comp, qp, qv, opts...), nil
}

// Index scans and parses a tsk repository, writing it to the store.
func (e *Engine) Index(root string) (*model.Repository, error) {
	entries, err := e.scanner.Scan(root)
	if err != nil {
		return nil, err
	}

	repo, err := e.parser.Parse(entries)
	if err != nil {
		return nil, err
	}

	if e.store != nil {
		if err := e.store.WriteRepository(repo); err != nil {
			return nil, err
		}
	}

	return repo, nil
}

// Query parses, validates, and executes a DSL query.
func (e *Engine) Query(dsl string) ([]*model.Task, model.Diagnostics, error) {
	expr, err := e.qparser.Parse(dsl)
	if err != nil {
		return nil, nil, err
	}

	diags := e.qvalidator.Validate(expr, nil)
	if diags.HasErrors() {
		return nil, diags, nil
	}

	sqlStr, params, err := e.compiler.Compile(expr, e.compileContext())
	if err != nil {
		return nil, diags, err
	}

	tasks, err := e.store.QueryTasks(sqlStr, params)
	if err != nil {
		return nil, diags, err
	}

	return tasks, diags, nil
}

// Search performs a fuzzy text search across tasks.
func (e *Engine) Search(queryStr string) ([]search.Match, error) {
	tasks, err := e.store.AllTasks()
	if err != nil {
		return nil, err
	}
	return e.searcher.SearchWithHighlights(tasks, queryStr), nil
}

// TaskByPath retrieves a single task by canonical path.
func (e *Engine) TaskByPath(path model.CanonicalPath) (*model.Task, error) {
	return e.store.TaskByPath(path)
}

// Close releases resources.
func (e *Engine) Close() error {
	if e.store != nil {
		return e.store.Close()
	}
	return nil
}

// --- CompileContext implementation ---

type engineContext struct {
	engine *Engine
}

func (e *Engine) compileContext() tsql.CompileContext {
	return &engineContext{engine: e}
}

func (c *engineContext) CurrentUser() string {
	return c.engine.currentUser
}

func (c *engineContext) CurrentUserAliases() []string {
	user := c.engine.currentUser
	if user == "" {
		return nil
	}

	seen := map[string]bool{user: true}
	aliases := []string{user}

	names, err := c.engine.store.AllTeamNames()
	if err != nil {
		return aliases
	}
	for _, name := range names {
		members, err := c.engine.store.TeamMembers(name)
		if err != nil {
			continue
		}
		for _, m := range members {
			if m.Identifier == user || m.Email == user {
				if !seen[m.Identifier] {
					seen[m.Identifier] = true
					aliases = append(aliases, m.Identifier)
				}
				if m.Email != "" && !seen[m.Email] {
					seen[m.Email] = true
					aliases = append(aliases, m.Email)
				}
			}
		}
	}
	return aliases
}

func (c *engineContext) CurrentUserTeams() []string {
	names, err := c.engine.store.AllTeamNames()
	if err != nil {
		return nil
	}
	var teams []string
	for _, name := range names {
		members, err := c.engine.store.TeamMembers(name)
		if err != nil {
			continue
		}
		for _, m := range members {
			if m.Identifier == c.engine.currentUser || m.Email == c.engine.currentUser {
				teams = append(teams, name)
				break
			}
		}
	}
	return teams
}

func (c *engineContext) TeamMembers(teamName string) []string {
	members, err := c.engine.store.TeamMembers(teamName)
	if err != nil {
		return nil
	}
	var ids []string
	for _, m := range members {
		ids = append(ids, m.Identifier)
		if m.Email != "" {
			ids = append(ids, m.Email)
		}
	}
	return ids
}

func (c *engineContext) ResolveDate(spec string) string {
	now := time.Now()
	resolved := query.ResolveDate(spec, now)
	if resolved != nil {
		return resolved.Format(time.RFC3339)
	}
	return spec
}
