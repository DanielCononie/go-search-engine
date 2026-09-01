package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/embedding"
	redisstorage "github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage/redis"
)

type diagnostics struct {
	Index             redisstorage.IndexDiagnostics           `json:"index"`
	Corpus            redisstorage.CorpusDiagnostics          `json:"corpus"`
	InSync            bool                                    `json:"in_sync"`
	SemanticIndex     *redisstorage.SemanticIndexDiagnostics  `json:"semantic_index,omitempty"`
	SemanticCorpus    *redisstorage.SemanticCorpusDiagnostics `json:"semantic_corpus,omitempty"`
	SemanticAvailable bool                                    `json:"semantic_available"`
	SemanticError     string                                  `json:"semantic_error,omitempty"`
}

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

	output := diagnostics{
		Index:  index,
		Corpus: corpus,
		InSync: !index.Indexing &&
			index.IndexingFailures == 0 &&
			index.IndexedSections == corpus.ExpectedSections,
	}
	embeddingConfig, err := config.LoadEmbedding()
	if err != nil {
		exit(err)
	}
	if embeddingConfig.Enabled {
		profile := embedding.Profile(
			embeddingConfig.Model,
			embeddingConfig.Version,
			embeddingConfig.Dimensions,
		)
		semanticIndexes := redisstorage.NewSemanticIndexManager(
			client,
			embeddingConfig.IndexVersion,
			embeddingConfig.Dimensions,
		)
		semanticIndex, indexErr := semanticIndexes.Diagnostics(ctx)
		semanticCorpus, corpusErr := redisstorage.NewSemanticRepository(
			client,
			embeddingConfig.IndexVersion,
		).Diagnostics(
			ctx,
			embeddingConfig.IndexVersion,
			profile,
			embeddingConfig.Dimensions,
		)
		if indexErr != nil {
			output.SemanticError = indexErr.Error()
		} else if corpusErr != nil {
			output.SemanticError = corpusErr.Error()
		} else {
			output.SemanticIndex = &semanticIndex
			output.SemanticCorpus = &semanticCorpus
			output.SemanticAvailable =
				semanticIndex.PhysicalIndex == semanticIndexes.TargetName() &&
					!semanticIndex.Indexing &&
					semanticIndex.IndexingFailures == 0 &&
					semanticIndex.IndexedSections == semanticCorpus.Ready &&
					semanticCorpus.InSync
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
