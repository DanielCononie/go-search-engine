package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/embedding"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/evaluation"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/index"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	redisstorage "github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage/redis"
)

func main() {
	if err := config.LoadEnvironment(); err != nil {
		exit(err)
	}
	fixturePath := flag.String(
		"fixture",
		"testdata/search_relevance.json",
		"path to the judged relevance fixture",
	)
	backendName := flag.String(
		"backend",
		"memory",
		"search backend to evaluate: memory, redis, or semantic",
	)
	flag.Parse()

	judgments, err := loadJudgments(*fixturePath)
	if err != nil {
		exit(err)
	}

	service, mode, indexingDuration, metadata, closeBackend, err := newSearchService(*backendName)
	if err != nil {
		exit(err)
	}
	defer closeBackend()

	results := make([][]models.SearchResult, 0, len(judgments))
	latencies := make([]time.Duration, 0, len(judgments))
	for _, judgment := range judgments {
		startedAt := time.Now()
		response, err := service.Search(context.Background(), search.Request{
			Query: judgment.Query,
			Mode:  mode,
			Limit: 10,
		})
		latencies = append(latencies, time.Since(startedAt))
		if err != nil {
			exit(err)
		}
		results = append(results, response.Results)
	}

	report, err := evaluation.BuildReport(
		time.Now(),
		indexingDuration,
		judgments,
		results,
		latencies,
	)
	if err != nil {
		exit(err)
	}
	report.Backend = *backendName
	report.IndexAlias = metadata.indexAlias
	report.PhysicalIndex = metadata.physicalIndex
	report.CorpusVersion = metadata.corpusVersion
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		exit(err)
	}
}

func newSearchService(
	backendName string,
) (*search.Service, search.Mode, time.Duration, backendMetadata, func(), error) {
	switch backendName {
	case "memory":
		indexStartedAt := time.Now()
		searchIndex := index.Build(config.SeedURLs)
		return search.NewService(search.NewInMemoryBackend(searchIndex)),
			search.ModeLexical,
			time.Since(indexStartedAt),
			backendMetadata{},
			func() {},
			nil
	case "redis":
		redisConfig, err := config.LoadRedis()
		if err != nil {
			return nil, "", 0, backendMetadata{}, nil, err
		}
		client := redisstorage.NewClient(redisConfig)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		index, err := redisstorage.NewIndexManager(client).Diagnostics(ctx)
		if err != nil {
			_ = client.Close()
			return nil, "", 0, backendMetadata{}, nil, err
		}
		corpus, err := redisstorage.NewRepository(client).CorpusDiagnostics(ctx)
		if err != nil {
			_ = client.Close()
			return nil, "", 0, backendMetadata{}, nil, err
		}
		return search.NewService(redisstorage.NewSearchBackend(client)),
			search.ModeLexical,
			0,
			backendMetadata{
				indexAlias:    index.Alias,
				physicalIndex: index.PhysicalIndex,
				corpusVersion: corpus.Version,
			},
			func() { _ = client.Close() },
			nil
	case "semantic":
		return newSemanticSearchService()
	default:
		return nil, "", 0, backendMetadata{}, nil, fmt.Errorf(
			"unsupported relevance backend %q: use memory, redis, or semantic",
			backendName,
		)
	}
}

func newSemanticSearchService() (
	*search.Service,
	search.Mode,
	time.Duration,
	backendMetadata,
	func(),
	error,
) {
	embeddingConfig, err := config.LoadEmbedding()
	if err != nil {
		return nil, "", 0, backendMetadata{}, nil, err
	}
	if !embeddingConfig.Enabled {
		return nil, "", 0, backendMetadata{}, nil, fmt.Errorf(
			"embedding configuration is required for semantic relevance evaluation",
		)
	}
	redisConfig, err := config.LoadRedis()
	if err != nil {
		return nil, "", 0, backendMetadata{}, nil, err
	}
	client := redisstorage.NewClient(redisConfig)
	closeClient := func() { _ = client.Close() }
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	profile := embedding.Profile(
		embeddingConfig.Model,
		embeddingConfig.Version,
		embeddingConfig.Dimensions,
	)
	manager := redisstorage.NewSemanticIndexManager(
		client,
		embeddingConfig.IndexVersion,
		embeddingConfig.Dimensions,
	)
	indexDiagnostics, err := manager.Diagnostics(ctx)
	if err != nil {
		closeClient()
		return nil, "", 0, backendMetadata{}, nil, err
	}
	semanticDiagnostics, err := redisstorage.NewSemanticRepository(
		client,
		embeddingConfig.IndexVersion,
	).Diagnostics(
		ctx,
		embeddingConfig.IndexVersion,
		profile,
		embeddingConfig.Dimensions,
	)
	if err != nil {
		closeClient()
		return nil, "", 0, backendMetadata{}, nil, err
	}
	if indexDiagnostics.PhysicalIndex != manager.TargetName() ||
		indexDiagnostics.Indexing ||
		indexDiagnostics.IndexingFailures != 0 ||
		indexDiagnostics.IndexedSections != semanticDiagnostics.Ready ||
		!semanticDiagnostics.InSync {
		closeClient()
		return nil, "", 0, backendMetadata{}, nil, fmt.Errorf(
			"semantic index is not active and synchronized; run cmd/embedder",
		)
	}
	corpus, err := redisstorage.NewRepository(client).CorpusDiagnostics(ctx)
	if err != nil {
		closeClient()
		return nil, "", 0, backendMetadata{}, nil, err
	}
	semanticBackend := redisstorage.NewSemanticBackend(
		client,
		embedding.NewOpenAIClient(embeddingConfig),
		profile,
		embeddingConfig.Dimensions,
	)
	return search.NewServiceWithSemantic(
			redisstorage.NewSearchBackend(client),
			semanticBackend,
		), search.ModeSemantic, 0, backendMetadata{
			indexAlias:    indexDiagnostics.Alias,
			physicalIndex: indexDiagnostics.PhysicalIndex,
			corpusVersion: corpus.Version,
		}, closeClient, nil
}

type backendMetadata struct {
	indexAlias    string
	physicalIndex string
	corpusVersion string
}

func loadJudgments(path string) ([]evaluation.Judgment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read relevance fixture: %w", err)
	}

	var judgments []evaluation.Judgment
	if err := json.Unmarshal(data, &judgments); err != nil {
		return nil, fmt.Errorf("decode relevance fixture: %w", err)
	}
	if len(judgments) == 0 {
		return nil, fmt.Errorf("relevance fixture is empty")
	}

	return judgments, nil
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
