package handlers

import (
	"strings"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	"github.com/gofiber/fiber/v2"
)

type SearchHandler struct {
	searchService *search.Service
}

// NewSearchHandler wires the HTTP layer to the search service instead of
// reaching into package-level search functions.
func NewSearchHandler(searchService *search.Service) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
	}
}

// Takes in a query string, returns a list of website url's ranked with scores, based on similarity/relevance.
func (h *SearchHandler) Search(c *fiber.Ctx) error {

	question := strings.TrimSpace(c.Query("q"))

	if question == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing query parameter q",
		})
	}

	results := h.searchService.Search(question)

	return c.JSON(fiber.Map{
		"query":   question,
		"results": results,
	})
}
