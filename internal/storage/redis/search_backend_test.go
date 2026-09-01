package redis

import (
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestGroupSearchDocuments(t *testing.T) {
	documents := []goredis.Document{
		searchDocument("first", 10, map[string]string{
			"id":         "section-1",
			"page_id":    "page-1",
			"url":        "https://example.com/page#first",
			"page_title": "Example",
			"heading":    "First",
			"text":       "Iron Man armor text.",
		}),
		searchDocument("second", 5, map[string]string{
			"id":         "section-2",
			"page_id":    "page-1",
			"url":        "https://example.com/page#second",
			"page_title": "Example",
			"heading":    "Second",
			"text":       "More armor text.",
		}),
		searchDocument("other", 9, map[string]string{
			"id":         "section-3",
			"page_id":    "page-2",
			"url":        "https://example.com/other",
			"page_title": "Other",
			"heading":    "Other",
			"text":       "Other Iron Man text.",
		}),
	}

	results, err := groupSearchDocuments("iron man", documents)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}
	if results[0].PageID != "page-1" || results[0].Score != 10.5 {
		t.Fatalf("first result = %#v", results[0])
	}
	if results[0].URL != "https://example.com/page" {
		t.Fatalf("page URL = %q", results[0].URL)
	}
	if len(results[0].MatchedSections) != 2 {
		t.Fatalf("matched sections = %#v", results[0].MatchedSections)
	}
}

func TestRedisFieldDecodesJSONValues(t *testing.T) {
	if value := redisField(map[string]string{"value": `["example"]`}, "value"); value != "example" {
		t.Fatalf("value = %q", value)
	}
	if value := redisField(map[string]string{"value": `"example"`}, "value"); value != "example" {
		t.Fatalf("value = %q", value)
	}
}

func TestSearchSnippetCentersOnMatchingText(t *testing.T) {
	text := strings.Repeat("prefix ", 60) + "Infinity Stones are here. " + strings.Repeat("suffix ", 60)
	snippet := searchSnippet(text, `"infinity stones"`)

	if !strings.Contains(snippet, "Infinity Stones") {
		t.Fatalf("snippet = %q", snippet)
	}
	if len(snippet) > snippetLength+6 {
		t.Fatalf("snippet length = %d", len(snippet))
	}
}

func TestMapSpellingSuggestionsCapsAndDeduplicatesCandidates(t *testing.T) {
	suggestions := mapSpellingSuggestions([]goredis.SpellCheckResult{{
		Term: "spidr",
		Suggestions: []goredis.SpellCheckSuggestion{
			{Suggestion: "spider"},
			{Suggestion: "spider"},
			{Suggestion: "spire"},
			{Suggestion: "spied"},
			{Suggestion: "spade"},
		},
	}})

	if len(suggestions) != 1 {
		t.Fatalf("suggestions = %#v", suggestions)
	}
	if got := suggestions[0].Candidates; len(got) != 3 ||
		got[0] != "spider" ||
		got[1] != "spire" ||
		got[2] != "spied" {
		t.Fatalf("candidates = %#v", got)
	}
}

func searchDocument(
	id string,
	score float64,
	fields map[string]string,
) goredis.Document {
	return goredis.Document{
		ID:     id,
		Score:  &score,
		Fields: fields,
	}
}
