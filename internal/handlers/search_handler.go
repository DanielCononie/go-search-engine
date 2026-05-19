package handlers

import (
	"strings"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	"github.com/gofiber/fiber/v2"
)

// Read query
// Validate input
// Call search service
// Return ranked results

// Takes in a query string, returns a list of website url's ranked with scores, based on similarity/relevance
func Search(c *fiber.Ctx) error {

	// Call search service which returns a list of sorted url's and scores

	question := strings.TrimSpace(c.Query("q"))

	if question == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing query parameter q",
		})
	}

	results := search.Search(question)

	return c.JSON(fiber.Map{
		"query":   question,
		"results": results,
	})
}
