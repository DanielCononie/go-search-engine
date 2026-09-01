package main

import (
	"context"
	"log"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/embedding"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/handlers"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	redisstorage "github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage/redis"
	"github.com/gofiber/fiber/v2"
)

func main() {
	if err := config.LoadEnvironment(); err != nil {
		log.Fatal(err)
	}
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

	lexicalBackend := redisstorage.NewSearchBackend(redisClient)
	searchService := search.NewService(lexicalBackend)
	embeddingConfig, err := config.LoadEmbedding()
	if err != nil {
		log.Fatal(err)
	}
	if embeddingConfig.Enabled {
		profile := embedding.Profile(
			embeddingConfig.Model,
			embeddingConfig.Version,
			embeddingConfig.Dimensions,
		)
		semanticIndexes := redisstorage.NewSemanticIndexManager(
			redisClient,
			embeddingConfig.IndexVersion,
			embeddingConfig.Dimensions,
		)
		semanticContext, cancelSemantic := context.WithTimeout(
			context.Background(),
			15*time.Second,
		)
		semanticIndex, indexErr := semanticIndexes.Diagnostics(semanticContext)
		semanticCorpus, corpusErr := redisstorage.NewSemanticRepository(
			redisClient,
			embeddingConfig.IndexVersion,
		).Diagnostics(
			semanticContext,
			embeddingConfig.IndexVersion,
			profile,
			embeddingConfig.Dimensions,
		)
		cancelSemantic()
		if indexErr == nil &&
			corpusErr == nil &&
			semanticIndex.PhysicalIndex == semanticIndexes.TargetName() &&
			!semanticIndex.Indexing &&
			semanticIndex.IndexingFailures == 0 &&
			semanticIndex.IndexedSections == semanticCorpus.Ready &&
			semanticCorpus.InSync {
			searchService = search.NewServiceWithSemantic(
				lexicalBackend,
				redisstorage.NewSemanticBackend(
					redisClient,
					embedding.NewOpenAIClient(embeddingConfig),
					profile,
					embeddingConfig.Dimensions,
				),
			)
		} else {
			log.Print("semantic search unavailable; run cmd/embedder and restart the API")
		}
	}
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
