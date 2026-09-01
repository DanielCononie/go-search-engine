package main

import (
	"context"
	"log"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/handlers"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/index"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	redisstorage "github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage/redis"
	"github.com/gofiber/fiber/v2"
)

func main() {
	redisConfig, err := config.LoadRedis()
	if err != nil {
		log.Fatal(err)
	}

	redisClient := redisstorage.NewClient(redisConfig)
	defer redisClient.Close()

	indexManager := redisstorage.NewIndexManager(redisClient)
	setupContext, cancelSetup := context.WithTimeout(context.Background(), 15*time.Second)
	err = indexManager.Ensure(setupContext)
	cancelSetup()
	if err != nil {
		log.Fatal(err)
	}

	// Build the index once at startup. Search requests reuse this in-memory
	// structure instead of fetching/parsing the seed URLs every time.
	searchIndex := index.Build(config.SeedURLs)
	searchBackend := search.NewInMemoryBackend(searchIndex)
	searchService := search.NewService(searchBackend)
	searchHandler := handlers.NewSearchHandler(searchService)
	healthChecker := redisstorage.NewHealthChecker(redisClient)
	readinessChecker := redisstorage.NewReadinessChecker(healthChecker, indexManager)
	healthHandler := handlers.NewHealthHandler(readinessChecker)

	app := fiber.New()

	app.Get("/search", searchHandler.Search)
	app.Get("/health/live", healthHandler.Live)
	app.Get("/health/ready", healthHandler.Ready)

	if err := app.Listen(":3000"); err != nil {
		log.Print(err)
	}
}
