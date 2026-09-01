package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	"github.com/gofiber/fiber/v2"
)

type stubSearchBackend struct {
	page search.ResultPage
}

func (b stubSearchBackend) Search(_ context.Context, _ search.Request) (search.ResultPage, error) {
	return b.page, nil
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

func TestSearchHandlerRejectsInvalidOptions(t *testing.T) {
	service := search.NewService(stubSearchBackend{})
	app := fiber.New()
	app.Get("/search", NewSearchHandler(service).Search)

	for _, path := range []string{
		"/search",
		"/search?q=example&mode=semantic",
		"/search?q=example&limit=101",
		"/search?q=example&offset=-1",
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
