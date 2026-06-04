package main

import (
	"log"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/handlers"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/index"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	"github.com/gofiber/fiber/v2"
)

func main() {
	// Build the index once at startup. Search requests reuse this in-memory
	// structure instead of fetching/parsing the seed URLs every time.
	searchIndex := index.Build(config.SeedURLs)
	searchService := search.NewService(searchIndex)
	searchHandler := handlers.NewSearchHandler(searchService)

	app := fiber.New()

	app.Get("/search", searchHandler.Search)

	log.Fatal(app.Listen(":3000"))
}
