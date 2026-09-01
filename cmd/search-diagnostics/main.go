package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	redisstorage "github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage/redis"
)

type diagnostics struct {
	Index  redisstorage.IndexDiagnostics  `json:"index"`
	Corpus redisstorage.CorpusDiagnostics `json:"corpus"`
	InSync bool                           `json:"in_sync"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	redisConfig, err := config.LoadRedis()
	if err != nil {
		exit(err)
	}
	client := redisstorage.NewClient(redisConfig)
	defer client.Close()

	index, err := redisstorage.NewIndexManager(client).Diagnostics(ctx)
	if err != nil {
		exit(err)
	}
	corpus, err := redisstorage.NewRepository(client).CorpusDiagnostics(ctx)
	if err != nil {
		exit(err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(diagnostics{
		Index:  index,
		Corpus: corpus,
		InSync: !index.Indexing &&
			index.IndexingFailures == 0 &&
			index.IndexedSections == corpus.ExpectedSections,
	}); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
