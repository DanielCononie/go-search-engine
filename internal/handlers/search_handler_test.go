package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	"github.com/gofiber/fiber/v2"
)

type stubSearchBackend struct {
	page search.ResultPage
	err  error
}

func (b stubSearchBackend) Search(_ context.Context, _ search.Request) (search.ResultPage, error) {
	return b.page, b.err
}

type recordingSearchBackend struct {
	request search.Request
}

func (b *recordingSearchBackend) Search(
	_ context.Context,
	request search.Request,
) (search.ResultPage, error) {
	b.request = request
	return search.ResultPage{}, nil
}

func TestSearchHandlerContract(t *testing.T) {
	service := search.NewService(stubSearchBackend{
		page: search.ResultPage{
			Results: []models.SearchResult{{
				URL:   "https://example.com",
				Title: "Example",
				Score: 2,
			}},
			Total: 1,
		},
	})
	app := fiber.New()
	app.Get("/search", NewSearchHandler(service).Search)

	response, err := app.Test(httptest.NewRequest("GET", "/search?q=example", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}

	var body search.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Version != search.ContractVersion || body.Mode != search.ModeLexical {
		t.Fatalf("response = %#v", body)
	}
}

func TestSearchHandlerReturnsEmptyResults(t *testing.T) {
	service := search.NewService(stubSearchBackend{
		page: search.ResultPage{
			Suggestions: []search.SpellingSuggestion{{
				Term:       "unkown",
				Candidates: []string{"unknown"},
			}},
		},
	})
	app := fiber.New()
	app.Get("/search", NewSearchHandler(service).Search)

	response, err := app.Test(httptest.NewRequest("GET", "/search?q=unknown", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}

	var body search.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Results == nil || len(body.Results) != 0 {
		t.Fatalf("results = %#v, want an empty array", body.Results)
	}
	if len(body.Suggestions) != 1 ||
		body.Suggestions[0].Candidates[0] != "unknown" {
		t.Fatalf("suggestions = %#v", body.Suggestions)
	}
}

func TestSearchHandlerRejectsInvalidOptions(t *testing.T) {
	service := search.NewService(stubSearchBackend{})
	app := fiber.New()
	app.Get("/search", NewSearchHandler(service).Search)

	for _, path := range []string{
		"/search",
		"/search?q=example&mode=semantic",
		"/search?q=example&limit=101",
		"/search?q=example&offset=-1",
		"/search?q=%22unmatched",
		"/search?q=example&site=example.com%7D%20%40text%3A%7B*",
	} {
		response, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("%s status = %d, want %d", path, response.StatusCode, fiber.StatusBadRequest)
		}
	}
}

func TestSearchHandlerPassesFilters(t *testing.T) {
	backend := &recordingSearchBackend{}
	service := search.NewService(backend)
	app := fiber.New()
	app.Get("/search", NewSearchHandler(service).Search)

	response, err := app.Test(httptest.NewRequest(
		"GET",
		"/search?q=example&site=example.com&language=en",
		nil,
	))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if backend.request.Site != "example.com" ||
		backend.request.Language != "en" {
		t.Fatalf("request = %#v", backend.request)
	}
}

func TestSearchHandlerPassesExactPhrase(t *testing.T) {
	backend := &recordingSearchBackend{}
	service := search.NewService(backend)
	app := fiber.New()
	app.Get("/search", NewSearchHandler(service).Search)

	response, err := app.Test(httptest.NewRequest(
		"GET",
		"/search?q=%22infinity+stones%22",
		nil,
	))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if backend.request.Query != `"infinity stones"` {
		t.Fatalf("query = %q", backend.request.Query)
	}
}

func TestSearchHandlerHandlesBackendFailure(t *testing.T) {
	service := search.NewService(stubSearchBackend{
		err: errors.New("Redis unavailable"),
	})
	app := fiber.New()
	app.Get("/search", NewSearchHandler(service).Search)

	response, err := app.Test(httptest.NewRequest("GET", "/search?q=example", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			response.StatusCode,
			fiber.StatusInternalServerError,
		)
	}
}
