package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/evaluation"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/index"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
)

func main() {
	fixturePath := flag.String(
		"fixture",
		"testdata/search_relevance.json",
		"path to the judged relevance fixture",
	)
	flag.Parse()

	judgments, err := loadJudgments(*fixturePath)
	if err != nil {
		exit(err)
	}

	indexStartedAt := time.Now()
	searchIndex := index.Build(config.SeedURLs)
	indexingDuration := time.Since(indexStartedAt)
	service := search.NewService(search.NewInMemoryBackend(searchIndex))

	results := make([][]models.SearchResult, 0, len(judgments))
	latencies := make([]time.Duration, 0, len(judgments))
	for _, judgment := range judgments {
		startedAt := time.Now()
		response, err := service.Search(context.Background(), search.Request{
			Query: judgment.Query,
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
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		exit(err)
	}
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
