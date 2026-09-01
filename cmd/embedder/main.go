package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/embedding"
	redisstorage "github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage/redis"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := config.LoadEnvironment(); err != nil {
		exit(err)
	}
	embeddingConfig, err := config.LoadEmbedding()
	if err != nil {
		exit(err)
	}
	if !embeddingConfig.Enabled {
		exit(errors.New("embedding configuration is required"))
	}
	redisConfig, err := config.LoadRedis()
	if err != nil {
		exit(err)
	}
	redisClient := redisstorage.NewClient(redisConfig)
	defer redisClient.Close()

	indexManager := redisstorage.NewSemanticIndexManager(
		redisClient,
		embeddingConfig.IndexVersion,
		embeddingConfig.Dimensions,
	)
	if err := indexManager.Ensure(ctx); err != nil {
		exit(err)
	}
	repository := redisstorage.NewSemanticRepository(
		redisClient,
		embeddingConfig.IndexVersion,
	)
	service := embedding.NewService(
		repository,
		embedding.NewOpenAIClient(embeddingConfig),
		embeddingConfig,
	)
	report, runErr := service.Run(ctx)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		exit(err)
	}
	if runErr != nil {
		if errors.Is(runErr, embedding.ErrPartialFailure) {
			os.Exit(1)
		}
		exit(runErr)
	}
	profile := embedding.Profile(
		embeddingConfig.Model,
		embeddingConfig.Version,
		embeddingConfig.Dimensions,
	)
	corpus, err := repository.Diagnostics(
		ctx,
		embeddingConfig.IndexVersion,
		profile,
		embeddingConfig.Dimensions,
	)
	if err != nil {
		exit(err)
	}
	if !corpus.InSync {
		exit(errors.New(
			"semantic corpus changed during backfill; rerun cmd/embedder",
		))
	}
	if err := indexManager.WaitReady(ctx, corpus.Ready); err != nil {
		exit(err)
	}
	if err := indexManager.Activate(ctx); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
