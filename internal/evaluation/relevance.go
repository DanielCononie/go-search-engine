package evaluation

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
)

type Judgment struct {
	Query        string   `json:"query"`
	RelevantURLs []string `json:"relevant_urls"`
	Notes        string   `json:"notes,omitempty"`
}

type QueryMeasurement struct {
	Query          string                `json:"query"`
	RecallAt10     float64               `json:"recall_at_10"`
	ReciprocalRank float64               `json:"reciprocal_rank"`
	TookMS         float64               `json:"took_ms"`
	Results        []models.SearchResult `json:"results"`
}

type Report struct {
	GeneratedAt    time.Time          `json:"generated_at"`
	IndexingMS     float64            `json:"indexing_ms"`
	QueryCount     int                `json:"query_count"`
	RecallAt10     float64            `json:"recall_at_10"`
	MeanReciprocal float64            `json:"mean_reciprocal_rank"`
	ZeroResultRate float64            `json:"zero_result_rate"`
	P50LatencyMS   float64            `json:"p50_latency_ms"`
	P95LatencyMS   float64            `json:"p95_latency_ms"`
	Queries        []QueryMeasurement `json:"queries"`
}

func BuildReport(
	generatedAt time.Time,
	indexingDuration time.Duration,
	judgments []Judgment,
	results [][]models.SearchResult,
	latencies []time.Duration,
) (Report, error) {
	report := Report{
		GeneratedAt: generatedAt.UTC(),
		IndexingMS:  milliseconds(indexingDuration),
		QueryCount:  len(judgments),
		Queries:     make([]QueryMeasurement, 0, len(judgments)),
	}
	if len(results) != len(judgments) || len(latencies) != len(judgments) {
		return Report{}, fmt.Errorf(
			"measurement counts must match judgments: judgments=%d results=%d latencies=%d",
			len(judgments),
			len(results),
			len(latencies),
		)
	}
	if len(judgments) == 0 {
		return report, nil
	}

	sortedLatencies := append([]time.Duration(nil), latencies...)
	sort.Slice(sortedLatencies, func(i int, j int) bool {
		return sortedLatencies[i] < sortedLatencies[j]
	})

	zeroResultCount := 0
	for queryIndex, judgment := range judgments {
		queryResults := results[queryIndex]
		recall, reciprocalRank := relevanceMetrics(judgment.RelevantURLs, queryResults)
		if len(queryResults) == 0 {
			zeroResultCount++
		}

		report.RecallAt10 += recall
		report.MeanReciprocal += reciprocalRank
		report.Queries = append(report.Queries, QueryMeasurement{
			Query:          judgment.Query,
			RecallAt10:     recall,
			ReciprocalRank: reciprocalRank,
			TookMS:         milliseconds(latencies[queryIndex]),
			Results:        queryResults,
		})
	}

	queryCount := float64(len(judgments))
	report.RecallAt10 /= queryCount
	report.MeanReciprocal /= queryCount
	report.ZeroResultRate = float64(zeroResultCount) / queryCount
	report.P50LatencyMS = milliseconds(percentile(sortedLatencies, 0.50))
	report.P95LatencyMS = milliseconds(percentile(sortedLatencies, 0.95))

	return report, nil
}

func relevanceMetrics(relevantURLs []string, results []models.SearchResult) (float64, float64) {
	relevant := make(map[string]struct{}, len(relevantURLs))
	for _, url := range relevantURLs {
		relevant[url] = struct{}{}
	}

	hits := 0
	reciprocalRank := 0.0
	for resultIndex, result := range results {
		if _, ok := relevant[result.URL]; !ok {
			continue
		}

		hits++
		if reciprocalRank == 0 {
			reciprocalRank = 1 / float64(resultIndex+1)
		}
	}
	if len(relevant) == 0 {
		return 0, reciprocalRank
	}

	return float64(hits) / float64(len(relevant)), reciprocalRank
}

func percentile(sorted []time.Duration, quantile float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}

	index := int(math.Ceil(float64(len(sorted))*quantile)) - 1
	index = max(0, min(index, len(sorted)-1))
	return sorted[index]
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}
