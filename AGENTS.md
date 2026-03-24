# AGENTS.md

This file is the working guide for coding agents in `github.com/jpcummins/tsk-lib`.
It is based on the current Go module, source tree, and tests in this repository.

## Authority And Related Instruction Files

- No existing root `AGENTS.md` was present at the time of writing.
- No Cursor rules were found in `.cursor/rules/` or `.cursorrules`.
- No Copilot instructions were found in `.github/copilot-instructions.md`.
- Supplemental team-wide Go guidance is preserved in `GO_RULES.md`.
- When `GO_RULES.md` and this file overlap, follow `AGENTS.md` for repo-specific conventions and use `GO_RULES.md` as supporting guidance.
- Treat this file as the primary repository-specific agent instruction set.

## Project Snapshot

- Language: Go.
- Module path: `github.com/jpcummins/tsk-lib`.
- Go version in `go.mod`: `1.25.1`.
- This is a library-first repository; there is no `cmd/` entrypoint in the current tree.
- Key packages: `engine`, `model`, `scan`, `parse`, `query`, `sql`, `store`, `search`, and `conformance`.
- `engine` is the composition root; `model` holds core domain types and diagnostics.
- `scan` and `parse` turn repository files into resolved domain objects.
- `query`, `sql`, `store`, and `search` implement DSL execution, persistence, and text search.

## Working Assumptions For Agents

- Run commands from the repository root: `/home/jp/src/tsk-lib`.
- Prefer standard Go tooling over custom scripts; there are no Makefile targets today.
- Keep changes library-oriented unless the task explicitly introduces a new executable.
- Follow existing package boundaries rather than creating cross-package shortcuts.
- Prefer clarity and discoverability over clever structure or abstraction.
- Preserve the repository layout expected by the scanner: root `tsk.toml`, root `sla.toml`, `tasks/...`, `teams/<team>/team.toml`, and `teams/<team>/iterations/*.md`.

## Build Commands

- Build all packages: `go build ./...`
- Build one package: `go build ./parse`
- Rebuild without cached test results when validating behavior changes: `go test ./... -count=1`

## Lint And Formatting Commands

- Format check: `gofmt -l .`
- Apply formatting: `gofmt -w .`
- Static analysis: `go vet ./...`
- Dependency cleanup: `go mod tidy`
- Vulnerability scan if installed: `govulncheck ./...`
- Practical pre-PR validation in this repo: `gofmt -w . && go vet ./... && go test ./...`
- There is no `golangci-lint` config in the repository today, so do not assume it is required.

## Test Commands

- Run the full test suite: `go test ./...`
- Run one package's tests: `go test ./conformance`
- Run one named test: `go test ./conformance -run '^TestConformance$'`
- Run one subtest: `go test ./conformance -run 'TestConformance/normalize_path'`
- List tests in a package: `go test ./conformance -list .`
- Disable caching for focused debugging: `go test ./conformance -run 'TestConformance/resolve_task' -count=1`

## Test Environment Notes

- `conformance/conformance_test.go` looks for the tsk spec repository at `../../tsk` by default.
- If that path is unavailable, set `TSK_SPEC_PATH` to the spec checkout before running conformance tests.
- The current repository setup already allows `go test ./...` to pass, but agents should keep the spec path rule in mind for fresh environments.

## Code Style: General

- Use `gofmt` formatting and let Go's standard layout drive whitespace and indentation.
- Keep files ASCII unless the file already requires non-ASCII content.
- Match the current style of concise, descriptive doc comments on exported identifiers.
- Keep package comments only where they add value; several packages already open with one.
- Prefer small, focused files grouped by domain concern rather than giant utility dumps.
- Prefer clear, simple code over clever optimizations unless a hot path is proven.

## Code Style: Imports

- Use standard Go import grouping: standard library first, blank line, then third-party/internal imports.
- Use import aliases only when needed for clarity or name collision resolution.
- Current example: `sql` is imported as `tsql` in `engine` to avoid collision with `database/sql`.
- Blank imports should be rare and purposeful; current usage is the SQLite driver registration in `store`.

## Code Style: Types And Data Modeling

- Prefer strong domain types over raw strings when the repository already defines one.
- Use `model.CanonicalPath`, `model.Duration`, `model.StatusCategory`, and `model.Diagnostics` instead of ad hoc substitutes.
- Optional scalar fields are commonly modeled as pointers (`*time.Time`, `*float64`, `*Duration`).
- Pass small immutable structs by value when it keeps APIs simpler and avoids unnecessary indirection.
- Keep exported structs clean and spec-oriented; field comments are used when the meaning is not obvious.
- Preserve existing struct tags and field names exactly when they map to YAML, JSON, or TOML schema.

## Code Style: Interfaces And Constructors

- Follow the existing interface-plus-default-implementation pattern.
- Consumer-facing interfaces are small and focused, for example `scan.Scanner`, `parse.Parser`, `query.Validator`, and `store.Reader`.
- Constructors are named `NewX` and usually return concrete default implementations.
- Prefer accepting interfaces and returning concrete structs where it matches existing package design.
- Dependency injection is preferred at the `engine` layer instead of hidden globals.
- Options use the functional options pattern (`type Option func(*Engine)`).

## Code Style: Packages

- Package names should be short, lowercase, and specific to a single domain.
- Avoid introducing generic package names like `util`, `utils`, or `common`.
- If new code should be private to this module and not imported by downstream consumers, prefer an `internal` package.
- Do not reorganize existing public packages into `internal` without an explicit compatibility decision.

## Code Style: Naming

- Exported names use Go's usual PascalCase.
- Unexported helpers use lowerCamelCase.
- Use specific domain names instead of generic `data`, `info`, or `util`.
- Favor names that reflect the tsk spec domain: `Task`, `Iteration`, `SLARule`, `ChangeLog`, `StatusMap`.
- Parser internals may use short local names (`p`, `qp`, `tc`) when the scope is tight and conventional.

## Code Style: Errors And Diagnostics

- Return Go `error` values for operational failures: I/O, SQL, parsing failures, invalid input that blocks progress.
- Wrap lower-level errors with context using `fmt.Errorf("...: %w", err)`.
- Do not compare wrapped errors with `==`; use `errors.Is` and `errors.As` when inspecting error chains.
- Use structured diagnostics for spec-level warnings and recoverable validation findings.
- Prefer `model.NewError`, `model.NewErrorf`, `model.NewWarning`, and `model.NewWarningf` for those cases.
- If a function can continue and report multiple issues, accumulate `model.Diagnostics` instead of failing fast.
- Do not introduce panics for normal error handling; there is no panic-based control flow in the current code.

## Code Style: Control Flow And Helpers

- Fail fast on unrecoverable setup errors, then keep the happy path straight.
- Use helper methods like `t.Helper()` in tests and small private helpers in production code where repetition would obscure intent.
- Prefer early returns over unnecessary `else` blocks.
- When building slices or maps, preallocate when the size is known or easy to estimate.
- Use `strings.Builder` for incremental string assembly when building larger strings such as SQL.
- Use `sync.Pool` only for proven hot-path allocation pressure, not as a default optimization.

## Code Style: APIs And Context

- Prefer idiomatic `(value, error)` return signatures and avoid naked returns.
- For I/O, long-running work, or cancelable operations, pass `context.Context` as the first parameter.
- If you introduce concurrent work that shares cancellation or error propagation, prefer `errgroup.Group` over ad hoc goroutine management.
- Validate untrusted input at API boundaries before persisting, parsing deeply, or executing queries derived from it.

## Code Style: Parsing And Spec Behavior

- Preserve spec behavior over convenience; many packages encode exact tsk semantics.
- Be careful when changing path normalization, hierarchy resolution, config inheritance, assignee resolution, or SLA logic.
- Keep query parsing, validation, SQL compilation, and evaluation behavior aligned.
- If adding a new query field or function, update all affected layers, not just one package.
- Maintain compatibility with the repository layout rules encoded in `scan.classify`.

## Code Style: Storage And SQL

- Keep SQL parameterized; current compiler and store code avoid string interpolation for values.
- Use `[]any` for SQL parameters, matching current interfaces.
- Hydrate domain objects through existing scan helpers instead of duplicating row parsing logic.
- Preserve transaction boundaries in bulk writes; `WriteRepository` intentionally writes in one transaction.

## Testing Conventions

- Add or update tests whenever behavior changes, especially for parser, query, and normalization logic.
- Prefer package-level tests for pure library behavior and conformance cases for spec-driven behavior.
- In tests, use `t.Fatalf` for setup failures and `t.Errorf` for field-by-field mismatches that should continue.
- Prefer table-driven tests for logic with multiple inputs, outputs, or edge cases.
- Reuse in-memory scanners and small fixtures to avoid unnecessary filesystem setup.
- When adding spec-facing behavior, extend conformance coverage if possible.

## Change Review Checklist For Agents

- Did you run `gofmt -w .` or `gofmt` on touched Go files?
- Did you run `go vet ./...` after structural changes?
- Did you run `go test ./...` or the narrowest relevant package/test command?
- If you changed dependencies, did you run `go mod tidy`?
- If you changed query semantics, did you consider parser, validator, evaluator, and SQL compiler together?
- If you changed repository parsing, did you preserve diagnostics and canonical path behavior?
- If you changed schema-facing structs, did you preserve YAML/JSON/TOML tags and optional field handling?

## When Unsure

- Choose the smallest change that fits the existing architecture.
- Prefer consistency with nearby code over introducing a new pattern.
- If a proposed abstraction is not already justified by repeated use, keep the code direct.
- Treat this repository as a spec-oriented library where correctness and traceable behavior matter more than cleverness.
