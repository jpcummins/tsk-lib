// Package search provides fuzzy text search across tsk tasks.
// It searches all task fields (path, summary, labels, assignee,
// status, body) and returns results with match highlights.
package search

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jpcummins/tsk-lib/model"
)

// Range is a half-open byte range [Start, End) within a string.
type Range struct {
	Start int
	End   int
}

// Highlight marks where a match was found within a specific task field.
type Highlight struct {
	Field     string  // "path", "summary", "body", "labels", "assignee", "status"
	Text      string  // Full text of the field
	Positions []Range // Matched byte ranges within Text
}

// Match represents a single fuzzy search result.
type Match struct {
	Task       *model.Task
	Score      int
	Highlights []Highlight
}

// Field weights for scoring. Higher = more important.
const (
	weightPath     = 100
	weightSummary  = 80
	weightLabels   = 60
	weightAssignee = 50
	weightStatus   = 40
	weightBody     = 10
)

// corpusField is one searchable field of a task.
type corpusField struct {
	name   string
	text   string
	lower  string // pre-lowercased for matching
	weight int
}

// taskEntry is the pre-built searchable representation of a task.
type taskEntry struct {
	task   *model.Task
	fields []corpusField
}

// Searcher performs fuzzy text search across indexed tasks.
type Searcher struct {
	entries []taskEntry
}

// New builds a Searcher from a set of tasks.
// The tasks slice is not copied; callers must not mutate it after passing.
func New(tasks []*model.Task) *Searcher {
	entries := make([]taskEntry, len(tasks))
	for i, t := range tasks {
		entries[i] = buildEntry(t)
	}
	return &Searcher{entries: entries}
}

func buildEntry(t *model.Task) taskEntry {
	var fields []corpusField

	addField := func(name, text string, weight int) {
		if text != "" {
			fields = append(fields, corpusField{
				name:   name,
				text:   text,
				lower:  strings.ToLower(text),
				weight: weight,
			})
		}
	}

	addField("path", string(t.CanonicalPath), weightPath)
	addField("summary", t.Summary, weightSummary)

	if len(t.Labels) > 0 {
		addField("labels", strings.Join(t.Labels, ", "), weightLabels)
	}

	addField("assignee", t.Assignee, weightAssignee)

	status := t.Status
	if status == "" && t.StatusCategory != "" {
		status = string(t.StatusCategory)
	}
	addField("status", status, weightStatus)

	addField("body", t.Body, weightBody)

	return taskEntry{task: t, fields: fields}
}

// Search performs fuzzy matching and returns up to limit results,
// sorted by score descending. The query is split into whitespace-
// delimited tokens. Each token is matched case-insensitively against
// all fields of every task. A task must match at least one token to
// be included.
func (s *Searcher) Search(query string, limit int) []Match {
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil
	}

	var matches []Match

	for i := range s.entries {
		entry := &s.entries[i]
		m := matchEntry(entry, tokens)
		if m.Score > 0 {
			matches = append(matches, m)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	return matches
}

// tokenize splits the query into lowercase tokens.
func tokenize(query string) []string {
	parts := strings.Fields(query)
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(p)
		if p != "" {
			tokens = append(tokens, p)
		}
	}
	return tokens
}

// matchEntry scores a single task against the query tokens.
func matchEntry(entry *taskEntry, tokens []string) Match {
	var highlights []Highlight
	totalScore := 0
	tokensMatched := 0

	for _, token := range tokens {
		tokenFound := false

		for fi := range entry.fields {
			field := &entry.fields[fi]
			positions := findAllOccurrences(field.lower, token)
			if len(positions) == 0 {
				continue
			}

			tokenFound = true

			// Score: field weight * number of occurrences, with
			// diminishing returns for repeated matches in same field.
			fieldScore := field.weight
			if len(positions) > 1 {
				fieldScore += field.weight * (len(positions) - 1) / 4
			}
			totalScore += fieldScore

			// Map byte ranges from lowercase string back to original.
			// Since ToLower preserves byte offsets for ASCII and most
			// UTF-8, positions from the lowered string map directly
			// to the original. We use the original text in highlights.
			origPositions := mapPositionsToOriginal(field.text, field.lower, positions)

			// Merge into existing highlight for this field, or create new.
			merged := false
			for hi := range highlights {
				if highlights[hi].Field == field.name {
					highlights[hi].Positions = append(highlights[hi].Positions, origPositions...)
					merged = true
					break
				}
			}
			if !merged {
				highlights = append(highlights, Highlight{
					Field:     field.name,
					Text:      field.text,
					Positions: origPositions,
				})
			}
		}

		if tokenFound {
			tokensMatched++
		}
	}

	// Bonus for matching multiple tokens (all tokens = big bonus).
	if tokensMatched > 1 {
		totalScore += tokensMatched * 50
	}
	if tokensMatched == len(tokens) {
		totalScore += 100
	}

	// Sort positions within each highlight.
	for hi := range highlights {
		sort.Slice(highlights[hi].Positions, func(i, j int) bool {
			return highlights[hi].Positions[i].Start < highlights[hi].Positions[j].Start
		})
		highlights[hi].Positions = mergeOverlapping(highlights[hi].Positions)
	}

	// Sort highlights by field weight (highest first).
	sort.Slice(highlights, func(i, j int) bool {
		return fieldWeight(highlights[i].Field) > fieldWeight(highlights[j].Field)
	})

	return Match{
		Task:       entry.task,
		Score:      totalScore,
		Highlights: highlights,
	}
}

// findAllOccurrences returns byte ranges of all non-overlapping occurrences
// of needle in haystack. Both must be lowercase.
func findAllOccurrences(haystack, needle string) []Range {
	if needle == "" || haystack == "" {
		return nil
	}

	var results []Range
	needleLen := len(needle)
	start := 0

	for {
		idx := strings.Index(haystack[start:], needle)
		if idx < 0 {
			break
		}
		absStart := start + idx
		results = append(results, Range{
			Start: absStart,
			End:   absStart + needleLen,
		})
		start = absStart + needleLen
	}

	return results
}

// mapPositionsToOriginal maps byte positions found in a lowercased string
// back to the original string. For most text this is a 1:1 mapping, but
// some Unicode characters change byte length when lowercased.
func mapPositionsToOriginal(original, lowered string, positions []Range) []Range {
	if len(original) == len(lowered) {
		// Fast path: same byte length, positions map directly.
		return positions
	}

	// Slow path: build a byte-offset mapping.
	// Walk both strings rune by rune to build the mapping.
	origRunes := []rune(original)
	result := make([]Range, 0, len(positions))

	// Build cumulative byte offset maps.
	origByteOffsets := make([]int, len(origRunes)+1)
	lowByteOffsets := make([]int, len(origRunes)+1)

	origOff := 0
	lowOff := 0
	for i, r := range origRunes {
		origByteOffsets[i] = origOff
		lowByteOffsets[i] = lowOff
		origOff += utf8.RuneLen(r)
		lr := []rune(strings.ToLower(string(r)))
		for _, c := range lr {
			lowOff += utf8.RuneLen(c)
		}
	}
	origByteOffsets[len(origRunes)] = origOff
	lowByteOffsets[len(origRunes)] = lowOff

	for _, pos := range positions {
		// Find rune index for start and end in lowered string.
		startRune := -1
		endRune := -1
		for i := 0; i <= len(origRunes); i++ {
			if lowByteOffsets[i] == pos.Start && startRune == -1 {
				startRune = i
			}
			if lowByteOffsets[i] == pos.End {
				endRune = i
				break
			}
		}
		if startRune >= 0 && endRune >= 0 {
			result = append(result, Range{
				Start: origByteOffsets[startRune],
				End:   origByteOffsets[endRune],
			})
		}
	}

	return result
}

// mergeOverlapping merges overlapping or adjacent ranges.
// Input must be sorted by Start.
func mergeOverlapping(ranges []Range) []Range {
	if len(ranges) <= 1 {
		return ranges
	}

	merged := []Range{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.Start <= last.End {
			if r.End > last.End {
				last.End = r.End
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

func fieldWeight(name string) int {
	switch name {
	case "path":
		return weightPath
	case "summary":
		return weightSummary
	case "labels":
		return weightLabels
	case "assignee":
		return weightAssignee
	case "status":
		return weightStatus
	case "body":
		return weightBody
	default:
		return 0
	}
}
