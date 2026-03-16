package parse

import (
	"sort"
	"strings"

	"github.com/jpcummins/tsk-lib/model"
)

// resolveLabels applies union semantics for label inheritance (Section 5, Labels).
// A task's effective labels = union of all ancestor labels + its own labels.
// Labels propagate through the directory hierarchy, even through implicit
// directory nodes (directories without README.md).
func resolveLabels(tasks map[model.CanonicalPath]*model.Task) {
	// Process tasks in path-depth order so shorter paths (ancestors) are resolved first.
	ordered := make([]model.CanonicalPath, 0, len(tasks))
	for path := range tasks {
		ordered = append(ordered, path)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return depthOf(ordered[i]) < depthOf(ordered[j])
	})

	// For each task, walk up the ancestor chain to collect inherited labels.
	// Since we process in depth order, all ancestors are already resolved.
	for _, path := range ordered {
		task := tasks[path]
		ancestorLabels := collectAncestorLabels(path, tasks)
		task.Labels = unionLabels(ancestorLabels, task.Labels)
	}
}

// collectAncestorLabels walks up the path hierarchy and returns the merged
// labels from the nearest ancestor task(s). This handles implicit directory
// nodes by continuing to walk up until we find an ancestor with a task entry.
func collectAncestorLabels(path model.CanonicalPath, tasks map[model.CanonicalPath]*model.Task) []string {
	var labels []string
	current := path.Parent()

	for !current.IsEmpty() {
		if ancestor, ok := tasks[current]; ok {
			// Found an ancestor task — its labels are already resolved
			// (we process in depth order), so just return them.
			labels = ancestor.Labels
			break
		}
		current = current.Parent()
	}

	return labels
}

// depthOf returns the nesting depth of a canonical path (number of / separators).
func depthOf(path model.CanonicalPath) int {
	s := string(path)
	if s == "" {
		return 0
	}
	return strings.Count(s, "/") + 1
}

// unionLabels merges two label slices with union semantics.
// Case-insensitive deduplication (preserving the first occurrence's casing).
func unionLabels(parent, child []string) []string {
	if len(parent) == 0 && len(child) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(parent)+len(child))
	var result []string

	for _, l := range parent {
		key := strings.ToLower(l)
		if !seen[key] {
			seen[key] = true
			result = append(result, l)
		}
	}

	for _, l := range child {
		key := strings.ToLower(l)
		if !seen[key] {
			seen[key] = true
			result = append(result, l)
		}
	}

	return result
}
