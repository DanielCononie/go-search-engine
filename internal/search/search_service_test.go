package search

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
)

type stubBackend struct {
	page ResultPage
	err  error
}

func (b stubBackend) Search(_ context.Context, _ Request) (ResultPage, error) {
	return b.page, b.err
}

func TestServiceSearchUsesLexicalDefaults(t *testing.T) {
	service := NewService(stubBackend{
		page: ResultPage{
			Results: []models.SearchResult{{URL: "https://example.com", Score: 1}},
			Total:   2,
			Suggestions: []SpellingSuggestion{{
				Term:       "exmple",
				Candidates: []string{"example"},
			}},
		},
	})

	response, err := service.Search(context.Background(), Request{Query: "example"})
	if err != nil {
		t.Fatal(err)
	}

	if response.Version != ContractVersion {
		t.Fatalf("version = %q, want %q", response.Version, ContractVersion)
	}
	if response.Mode != ModeLexical || response.UsedMode != ModeLexical {
		t.Fatalf("mode = %q, used mode = %q", response.Mode, response.UsedMode)
	}
	if response.NextOffset == nil || *response.NextOffset != 1 {
		t.Fatalf("next offset = %v, want 1", response.NextOffset)
	}
	if len(response.Suggestions) != 1 ||
		response.Suggestions[0].Candidates[0] != "example" {
		t.Fatalf("suggestions = %#v", response.Suggestions)
	}
}

func TestServiceSearchRejectsUnsupportedMode(t *testing.T) {
	service := NewService(stubBackend{})

	_, err := service.Search(context.Background(), Request{
		Query: "example",
		Mode:  Mode("semantic"),
	})
	if !errors.Is(err, ErrUnsupportedMode) {
		t.Fatalf("error = %v, want ErrUnsupportedMode", err)
	}
}

func TestServiceSearchRejectsInvalidPagination(t *testing.T) {
	service := NewService(stubBackend{})

	for _, request := range []Request{
		{Query: "example", Limit: -1},
		{Query: "example", Limit: MaxLimit + 1},
		{Query: "example", Offset: -1},
	} {
		if _, err := service.Search(context.Background(), request); !errors.Is(err, ErrInvalidPagination) {
			t.Fatalf("error = %v, want ErrInvalidPagination", err)
		}
	}
}

func TestRelevanceFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/search_relevance.json")
	if err != nil {
		t.Fatal(err)
	}

	var cases []struct {
		Query        string   `json:"query"`
		RelevantURLs []string `json:"relevant_urls"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("relevance fixture must contain at least one query")
	}

	for _, testCase := range cases {
		if testCase.Query == "" {
			t.Fatal("relevance query must not be empty")
		}
		if len(testCase.RelevantURLs) == 0 {
			t.Fatalf("query %q has no relevant URLs", testCase.Query)
		}
	}
}
