package parse

import (
	"fmt"

	"github.com/jpcummins/tsk-lib/model"
)

const maxRedirectDepth = 3

// resolveRedirects resolves redirect stubs in the task set.
// Stubs are removed from the task map and placed in a separate list.
// Dependency paths that point to stubs are rewritten to the resolved target.
// Returns resolved tasks, stubs, and any warnings.
func resolveRedirects(tasks map[model.CanonicalPath]*model.Task) (
	resolved map[model.CanonicalPath]*model.Task,
	stubs []*model.Task,
	warnings []string,
	err error,
) {
	resolved = make(map[model.CanonicalPath]*model.Task, len(tasks))

	// First pass: separate stubs from real tasks.
	stubMap := make(map[model.CanonicalPath]*model.Task)
	for path, task := range tasks {
		if task.IsStub {
			stubMap[path] = task
			stubs = append(stubs, task)
		} else {
			resolved[path] = task
		}
	}

	// Build a resolution cache: stub path -> final resolved canonical path.
	resolveCache := make(map[model.CanonicalPath]model.CanonicalPath)
	for stubPath := range stubMap {
		target, resolveErr := resolveChain(stubPath, stubMap, maxRedirectDepth)
		if resolveErr != nil {
			err = resolveErr
			return
		}
		resolveCache[stubPath] = target
	}

	// Second pass: rewrite dependency paths that reference stubs.
	for _, task := range resolved {
		for i, dep := range task.Dependencies {
			if target, ok := resolveCache[dep]; ok {
				task.Dependencies[i] = target
				warnings = append(warnings,
					fmt.Sprintf("dependency %q in %q resolved via stub to %q",
						dep, task.CanonicalPath, target))
			}
		}
	}

	return
}

// resolveChain follows a redirect chain up to maxDepth.
func resolveChain(
	path model.CanonicalPath,
	stubs map[model.CanonicalPath]*model.Task,
	maxDepth int,
) (model.CanonicalPath, error) {
	visited := make(map[model.CanonicalPath]bool)
	current := path

	for depth := 0; depth <= maxDepth; depth++ {
		stub, isStub := stubs[current]
		if !isStub {
			return current, nil
		}

		if visited[current] {
			return "", fmt.Errorf("redirect cycle detected at %q", current)
		}
		visited[current] = true

		current = stub.RedirectTo
	}

	return "", fmt.Errorf("redirect chain from %q exceeds max depth %d", path, maxDepth)
}
