package handlers

import (
	"errors"
	"strconv"
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

	mode := search.Mode(c.Query("mode", string(search.ModeLexical)))
	limit, err := parseBoundedInt(c.Query("limit"), search.DefaultLimit, 1, search.MaxLimit)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "limit must be an integer between 1 and 100",
		})
	}
	offset, err := parseBoundedInt(c.Query("offset"), 0, 0, int(^uint(0)>>1))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "offset must be a non-negative integer",
		})
	}

	response, err := h.searchService.Search(c.UserContext(), search.Request{
		Query:    question,
		Mode:     mode,
		Limit:    limit,
		Offset:   offset,
		Site:     c.Query("site"),
		Language: c.Query("language"),
	})
	if errors.Is(err, search.ErrUnsupportedMode) ||
		errors.Is(err, search.ErrInvalidPagination) ||
		errors.Is(err, search.ErrInvalidQuery) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "search failed",
		})
	}

	return c.JSON(response)
}

func parseBoundedInt(raw string, fallback int, minimum int, maximum int) (int, error) {
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, strconv.ErrSyntax
	}

	return value, nil
}
