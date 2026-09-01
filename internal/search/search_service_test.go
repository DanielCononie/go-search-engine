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

func TestServiceSearchReportsUnavailableSemanticMode(t *testing.T) {
	service := NewService(stubBackend{})

	_, err := service.Search(context.Background(), Request{
		Query: "example",
		Mode:  ModeSemantic,
	})
	if !errors.Is(err, ErrModeUnavailable) {
		t.Fatalf("error = %v, want ErrModeUnavailable", err)
	}
}

func TestServiceSearchRoutesSemanticMode(t *testing.T) {
	semantic := &recordingBackend{}
	service := NewServiceWithSemantic(stubBackend{}, semantic)

	response, err := service.Search(context.Background(), Request{
		Query: "a hero who uses powered armor",
		Mode:  ModeSemantic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if semantic.request.Mode != ModeSemantic ||
		response.Mode != ModeSemantic ||
		response.UsedMode != ModeSemantic {
		t.Fatalf("request = %#v, response = %#v", semantic.request, response)
	}
}

func TestServiceSearchRejectsUnknownMode(t *testing.T) {
	service := NewService(stubBackend{})
	_, err := service.Search(context.Background(), Request{
		Query: "example",
		Mode:  Mode("hybrid"),
	})
	if !errors.Is(err, ErrUnsupportedMode) {
		t.Fatalf("error = %v, want ErrUnsupportedMode", err)
	}
}

type recordingBackend struct {
	request Request
}

func (b *recordingBackend) Search(
	_ context.Context,
	request Request,
) (ResultPage, error) {
	b.request = request
	return ResultPage{}, nil
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
	for _, path := range []string{
		"../../testdata/search_relevance.json",
		"../../testdata/semantic_relevance.json",
	} {
		data, err := os.ReadFile(path)
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
			t.Fatalf("%s must contain at least one query", path)
		}

		for _, testCase := range cases {
			if testCase.Query == "" {
				t.Fatalf("%s has an empty relevance query", path)
			}
			if len(testCase.RelevantURLs) == 0 {
				t.Fatalf(
					"%s query %q has no relevant URLs",
					path,
					testCase.Query,
				)
			}
		}
	}
}
