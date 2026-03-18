package model

import (
	"sort"
	"strings"
)

// SortTasks sorts tasks by weight (lower first), then by filename (lexicographic).
// Tasks without weight are sorted after weighted tasks within the same parent,
// following the spec: weight overrides default lexicographic-by-filename ordering.
func SortTasks(tasks []*Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]

		// If both have weights, sort by weight
		if a.Weight != nil && b.Weight != nil {
			if *a.Weight != *b.Weight {
				return *a.Weight < *b.Weight
			}
		}

		// Weighted tasks come before unweighted
		if a.Weight != nil && b.Weight == nil {
			return true
		}
		if a.Weight == nil && b.Weight != nil {
			return false
		}

		// Default: lexicographic by base filename
		return strings.ToLower(a.Path.Base()) < strings.ToLower(b.Path.Base())
	})
}

// SortTasksByFilename sorts tasks by their base filename lexicographically.
func SortTasksByFilename(tasks []*Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return strings.ToLower(tasks[i].Path.Base()) < strings.ToLower(tasks[j].Path.Base())
	})
}
