package handlers

import "github.com/gofiber/fiber/v2"

// Read query
// Validate input
// Call search service
// Return ranked results

// Takes in a query string, returns a list of website url's ranked with scores, based on similarity/relevance
func Search(c *fiber.Ctx) error {

	// Call search service which returns a list of sorted url's and scores

	var question string = c.Query("q")

	if question == "" {

	}

	return c.SendString("Hello World")
}
