# tsk-lib

Go library for parsing, indexing, and querying [tsk](https://github.com/jpcummins/tsk-spec) task repositories.

## Features

- **Parse** markdown task files with YAML front matter
- **Index** task repositories into an in-memory SQLite database
- **Query** using the tsk DSL (domain-specific language)
- **Fuzzy search** across all task content with match highlighting
- **Resolve** redirects, inheritance, labels, and team membership
- **SLA evaluation** based on configurable rules

## Installation

```bash
go get github.com/jpcummins/tsk-lib
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/jpcummins/tsk-lib/engine"
    "github.com/jpcummins/tsk-lib/search"
)

func main() {
    // Create an engine
    eng, _ := engine.NewDefault(":memory:")
    defer eng.Close()

    // Index a repository
    repo, _ := eng.Index("/path/to/tsk-repo")
    fmt.Printf("Indexed %d tasks\n", len(repo.Tasks))

    // DSL query
    tasks, _ := eng.Search(`status.category = in_progress`)
    fmt.Printf("Found %d in-progress tasks\n", len(tasks))

    // Fuzzy search
    searcher := search.New(repo.Tasks)
    matches := searcher.Search("payment timeout", 10)
    for _, m := range matches {
        fmt.Printf("[score=%d] %s\n", m.Score, m.Task.CanonicalPath)
    }
}
```

## Architecture

### Packages

- **`engine`** - High-level API for indexing and querying
- **`store`** - SQLite persistence layer
- **`parse`** - Markdown + YAML parser
- **`scan`** - Filesystem scanner with redirect resolution
- **`query`** - DSL parser and SQL compiler
- **`search`** - Fuzzy text search with highlighting
- **`model`** - Core data types (Task, Iteration, Team, SLA)

### Engine Options

```go
eng, _ := engine.NewDefault(":memory:",
    engine.WithCurrentUser("alice@example.com"),
)
```

### Query Language

The DSL supports:
- Field predicates: `status = "todo"`, `due < date("tomorrow")`
- Boolean logic: `AND`, `OR`, `NOT`, parentheses
- Functions: `me()`, `my_team()`, `team(name)`, `has(labels, "bug")`
- Operators: `=`, `!=`, `<`, `>`, `~` (contains), `IN`

See [tsk-spec](https://github.com/jpcummins/tsk-spec) section 12.1 for full reference.

### Fuzzy Search

```go
searcher := search.New(repo.Tasks)
matches := searcher.Search("cart checkout", 100)

for _, m := range matches {
    fmt.Printf("Task: %s (score=%d)\n", m.Task.CanonicalPath, m.Score)
    for _, hl := range m.Highlights {
        fmt.Printf("  Matched in %s: %d positions\n", hl.Field, len(hl.Positions))
    }
}
```

Fuzzy search:
- Tokenizes queries by whitespace
- Case-insensitive substring matching
- Weighted by field (path > summary > labels > body)
- Returns highlight positions for rendering

## Data Model

### Task

```go
type Task struct {
    CanonicalPath  CanonicalPath
    Status         string
    StatusCategory StatusCategory // todo, in_progress, done
    Assignee       string
    Due            *time.Time
    Date           time.Time
    Estimate       *Duration
    Summary        string
    Body           string
    Labels         []string
    Dependencies   []CanonicalPath
    // ... more fields
}
```

### Iteration

```go
type Iteration struct {
    Team           string
    CanonicalPath  CanonicalPath
    Status         string
    StatusCategory StatusCategory
    Start          time.Time
    End            time.Time
    Tasks          []CanonicalPath
}
```

### Team

```go
type Team struct {
    Name       string
    Members    []string
    StatusMap  StatusMap
}
```

## Requirements

- Go 1.25+
- modernc.org/sqlite (pure-go SQLite)

## Related Projects

- [tsk-spec](https://github.com/jpcummins/tsk-spec) - Formal specification
- [tsk-cli](https://github.com/jpcummins/tsk-cli) - Terminal UI

## License

MIT License - see [LICENSE](LICENSE)

## Author

J.P. Cummins <jcummins@hey.com>
