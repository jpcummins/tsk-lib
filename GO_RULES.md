---
description: Definitive guidelines for writing clear, simple, performant, and maintainable Go code, adhering to Google's style and modern best practices.
globs: **/*
---
# Go Best Practices

This guide outlines our team's definitive Go coding standards, emphasizing clarity, simplicity, performance, and maintainability. Adhere to these principles to ensure a consistent, high-quality codebase that aligns with Google's Go Style Guide.

## 1. Code Organization & Structure

**Prioritize clarity and discoverability.** Structure your projects logically, making it easy for new team members to understand the codebase.

### Package Naming & Purpose
Packages must have short, lowercase names that clearly describe their single purpose. Avoid generic names like `util` or `common`.

## 2. Error Handling

**Handle errors explicitly and provide actionable context.** Never ignore errors. Use `errors.Is` and `errors.As` for robust error inspection.

### Wrap Errors with Context
Always wrap errors to add context, making debugging easier and preserving the error chain. Use `%w` with `fmt.Errorf` to enable `errors.Is` and `errors.As`.

### Use `errors.Is` and `errors.As` for Inspection
For checking specific error types or values, use `errors.Is` for direct comparison and `errors.As` for unwrapping to a specific error type. This works correctly with wrapped errors.

## 3. Performance Considerations

**Minimize allocations and GC pressure.** Go's GC is efficient, but excessive allocations will always be a bottleneck in hot paths.

### Pre-allocate Slices and Maps
Always pre-allocate slices and maps with known or estimated capacities to avoid repeated reallocations and garbage generation.

### Use `strings.Builder` for String Concatenation
Avoid `+` or `fmt.Sprintf` in loops for building strings. `strings.Builder` minimizes allocations by writing to a single underlying buffer.

### Leverage `sync.Pool` for Reusable Objects
For frequently created and discarded objects, use `sync.Pool` only when profiling or proven hot paths justify the added complexity.

### Pass Small Structs by Value
For small, immutable structs, passing by value can avoid heap allocations and improve cache locality, as determined by escape analysis.

## 4. Concurrency & Context

**Manage goroutines and resource lifetimes effectively.** Always use `context.Context` for cancellation and timeouts.

### Use `context.Context` for Cancellation and Timeouts
Pass `context.Context` as the first argument to functions that perform I/O or long-running operations. This enables graceful shutdown and resource management.

### Coordinate Goroutines with `errgroup.Group`
For managing multiple goroutines that need to complete or be cancelled together, use `golang.org/x/sync/errgroup`.

## 5. API Design

**Design clear, intuitive, and idiomatic APIs.** Follow Go's conventions for function signatures and return values.

### Idiomatic Function Signatures
Prefer `(value, error)` return patterns. Avoid naked returns and ensure clarity in parameter order.

### Accept Interfaces, Return Structs
Accept interfaces at package boundaries when it improves flexibility, and return concrete types when callers benefit from discoverable behavior.

## 6. Testing Approaches

**Write comprehensive, fast, and maintainable tests.** Prioritize table-driven tests for clarity and coverage.

### Table-Driven Tests
Use table-driven tests for functions with multiple inputs and expected outputs. This reduces boilerplate, improves readability, and makes it easy to add new test cases.

## 7. Security Best Practices

**Integrate security from the start.** Always validate inputs and handle sensitive data carefully.

### Input Validation
Never trust user input. Validate all inputs at the API boundary to prevent injection attacks and malformed data from reaching deeper layers.

### Secure Dependency Management
Regularly audit and update dependencies to mitigate known vulnerabilities. Use `go mod tidy` to clean up unused modules and `govulncheck` to identify issues.
