// Package search provides fuzzy text search across task fields.
package search

import (
	"sort"
	"strings"

	"github.com/jpcummins/tsk-lib/model"
)

// fieldWeight assigns scoring weights to different task fields.
var fieldWeight = map[string]int{
	"path":     100,
	"summary":  80,
	"labels":   60,
	"assignee": 50,
	"status":   40,
	"type":     35,
	"body":     10,
}

// Range represents a byte-offset range [Start, End) within a string.
type Range struct {
	Start int
	End   int
}

// Highlight records all match positions within a single field.
type Highlight struct {
	Field     string  // field name (e.g. "path", "body")
	Text      string  // full text of the field
	Positions []Range // matched byte ranges
}

// Match represents a single fuzzy search result with scoring and highlights.
type Match struct {
	Task       *model.Task
	Score      float64
	Highlights []Highlight // ordered by field weight (highest first)
}

// Result represents a single search result with scoring info (no highlights).
type Result struct {
	Task  *model.Task
	Score float64
}

// Searcher performs fuzzy text search across tasks.
type Searcher struct{}

// NewSearcher creates a new Searcher.
func NewSearcher() *Searcher {
	return &Searcher{}
}

// Search performs a fuzzy search across all tasks in the repository.
// Returns Results without highlight information.
func (s *Searcher) Search(tasks []*model.Task, queryStr string) []Result {
	tokens := tokenize(queryStr)
	if len(tokens) == 0 {
		return nil
	}

	var results []Result
	for _, task := range tasks {
		if task.IsStub {
			continue
		}
		score := scoreTask(task, tokens)
		if score > 0 {
			results = append(results, Result{Task: task, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// SearchWithHighlights performs a fuzzy search and returns Matches with
// per-field highlight positions for rendering.
func (s *Searcher) SearchWithHighlights(tasks []*model.Task, queryStr string) []Match {
	tokens := tokenize(queryStr)
	if len(tokens) == 0 {
		return nil
	}

	var matches []Match
	for _, task := range tasks {
		if task.IsStub {
			continue
		}
		score, highlights := scoreTaskWithHighlights(task, tokens)
		if score > 0 {
			matches = append(matches, Match{
				Task:       task,
				Score:      score,
				Highlights: highlights,
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}

// taskField describes a searchable field on a task.
type taskField struct {
	name   string
	value  string
	weight int
}

func getTaskFields(task *model.Task) []taskField {
	fields := []taskField{
		{"path", string(task.Path), fieldWeight["path"]},
		{"summary", task.Summary, fieldWeight["summary"]},
		{"assignee", task.Assignee, fieldWeight["assignee"]},
		{"status", task.Status, fieldWeight["status"]},
		{"type", task.Type, fieldWeight["type"]},
		{"body", task.Body, fieldWeight["body"]},
	}
	for _, label := range task.Labels {
		fields = append(fields, taskField{"labels", label, fieldWeight["labels"]})
	}
	return fields
}

func scoreTaskWithHighlights(task *model.Task, tokens []string) (float64, []Highlight) {
	fields := getTaskFields(task)

	var total float64
	matchedTokens := 0

	// Track highlights per field (by name+text to handle multiple label fields).
	type hlKey struct {
		name string
		text string
	}
	hlMap := make(map[hlKey]*Highlight)

	for _, token := range tokens {
		tokenScore := 0.0

		for _, f := range fields {
			positions := findAllPositions(f.value, token)
			if len(positions) == 0 {
				continue
			}

			// Score: base weight + diminishing returns for extra occurrences
			score := float64(f.weight)
			for i := 1; i < len(positions); i++ {
				score += float64(f.weight) * 0.3
			}
			tokenScore += score

			// Collect highlights
			key := hlKey{f.name, f.value}
			hl, ok := hlMap[key]
			if !ok {
				hl = &Highlight{Field: f.name, Text: f.value}
				hlMap[key] = hl
			}
			hl.Positions = append(hl.Positions, positions...)
		}

		if tokenScore > 0 {
			matchedTokens++
		}
		total += tokenScore
	}

	if matchedTokens == len(tokens) && len(tokens) > 1 {
		total *= 1.5
	}

	if total == 0 {
		return 0, nil
	}

	// Collect highlights sorted by field weight (highest first).
	highlights := make([]Highlight, 0, len(hlMap))
	for _, hl := range hlMap {
		highlights = append(highlights, *hl)
	}
	sort.Slice(highlights, func(i, j int) bool {
		wi := fieldWeight[highlights[i].Field]
		wj := fieldWeight[highlights[j].Field]
		if wi != wj {
			return wi > wj
		}
		return highlights[i].Field < highlights[j].Field
	})

	return total, highlights
}

// findAllPositions returns byte-offset ranges of all case-insensitive
// occurrences of token in text.
func findAllPositions(text, token string) []Range {
	if text == "" || token == "" {
		return nil
	}

	lower := strings.ToLower(text)
	tokenLower := strings.ToLower(token)
	tokenLen := len(tokenLower)

	var positions []Range
	start := 0
	for {
		idx := strings.Index(lower[start:], tokenLower)
		if idx < 0 {
			break
		}
		absStart := start + idx
		positions = append(positions, Range{Start: absStart, End: absStart + tokenLen})
		start = absStart + tokenLen
	}
	return positions
}

func scoreTask(task *model.Task, tokens []string) float64 {
	var total float64
	matchedTokens := 0

	for _, token := range tokens {
		tokenScore := 0.0

		// Check each field
		tokenScore += scoreField(string(task.Path), token, fieldWeight["path"])
		tokenScore += scoreField(task.Summary, token, fieldWeight["summary"])
		tokenScore += scoreField(task.Assignee, token, fieldWeight["assignee"])
		tokenScore += scoreField(task.Status, token, fieldWeight["status"])
		tokenScore += scoreField(task.Type, token, fieldWeight["type"])
		tokenScore += scoreField(task.Body, token, fieldWeight["body"])

		for _, label := range task.Labels {
			tokenScore += scoreField(label, token, fieldWeight["labels"])
		}

		if tokenScore > 0 {
			matchedTokens++
		}
		total += tokenScore
	}

	// Bonus for matching all tokens
	if matchedTokens == len(tokens) && len(tokens) > 1 {
		total *= 1.5
	}

	return total
}

func scoreField(fieldValue, token string, weight int) float64 {
	if fieldValue == "" || token == "" {
		return 0
	}

	lower := strings.ToLower(fieldValue)
	tokenLower := strings.ToLower(token)

	count := strings.Count(lower, tokenLower)
	if count == 0 {
		return 0
	}

	// Diminishing returns for multiple occurrences
	score := float64(weight)
	for i := 1; i < count; i++ {
		score += float64(weight) * 0.3
	}

	return score
}

func tokenize(query string) []string {
	parts := strings.Fields(strings.ToLower(query))
	var tokens []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tokens = append(tokens, p)
		}
	}
	return tokens
}
