package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/crawler"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/ingest"
	redisstorage "github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage/redis"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := config.LoadEnvironment(); err != nil {
		exit(err)
	}
	redisConfig, err := config.LoadRedis()
	if err != nil {
		exit(err)
	}
	redisClient := redisstorage.NewClient(redisConfig)
	defer redisClient.Close()

	indexManager := redisstorage.NewIndexManager(redisClient)
	if err := indexManager.Ensure(ctx); err != nil {
		exit(err)
	}

	repository := redisstorage.NewRepository(redisClient)
	service := ingest.NewService(crawler.NewDefault(), repository)
	report, runErr := service.Run(ctx, config.SeedURLs)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		exit(err)
	}
	if runErr != nil {
		if errors.Is(runErr, ingest.ErrPartialFailure) {
			os.Exit(1)
		}
		exit(runErr)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
