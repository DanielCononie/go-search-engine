package evaluation

import (
	"testing"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
)

func TestBuildReport(t *testing.T) {
	report, err := BuildReport(
		time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
		1500*time.Millisecond,
		[]Judgment{
			{Query: "first", RelevantURLs: []string{"relevant-a", "relevant-b"}},
			{Query: "second", RelevantURLs: []string{"relevant-c"}},
		},
		[][]models.SearchResult{
			{{URL: "irrelevant"}, {URL: "relevant-a"}},
			{},
		},
		[]time.Duration{2 * time.Millisecond, 4 * time.Millisecond},
	)
	if err != nil {
		t.Fatal(err)
	}

	if report.QueryCount != 2 {
		t.Fatalf("query count = %d, want 2", report.QueryCount)
	}
	if report.RecallAt10 != 0.25 {
		t.Fatalf("Recall@10 = %f, want 0.25", report.RecallAt10)
	}
	if report.MeanReciprocal != 0.25 {
		t.Fatalf("MRR = %f, want 0.25", report.MeanReciprocal)
	}
	if report.ZeroResultRate != 0.5 {
		t.Fatalf("zero-result rate = %f, want 0.5", report.ZeroResultRate)
	}
	if report.P50LatencyMS != 2 || report.P95LatencyMS != 4 {
		t.Fatalf(
			"latencies = p50 %f, p95 %f; want 2, 4",
			report.P50LatencyMS,
			report.P95LatencyMS,
		)
	}
}

func TestBuildReportRejectsMismatchedMeasurements(t *testing.T) {
	_, err := BuildReport(
		time.Now(),
		0,
		[]Judgment{{Query: "example"}},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected mismatched measurement error")
	}
}
