package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
)

type stubFetcher struct {
	results []models.FetchResult
}

func (f stubFetcher) FetchAll(_ context.Context, _ []string) []models.FetchResult {
	return f.results
}

type stubRepository struct {
	pages    []documents.Page
	sections [][]documents.Section
	failures []documents.CrawlFailure
	err      error
}

func (r *stubRepository) ReplacePage(
	_ context.Context,
	page documents.Page,
	sections []documents.Section,
) error {
	if r.err != nil {
		return r.err
	}
	r.pages = append(r.pages, page)
	r.sections = append(r.sections, sections)
	return nil
}

func (r *stubRepository) SaveCrawlFailure(
	_ context.Context,
	failure documents.CrawlFailure,
) error {
	r.failures = append(r.failures, failure)
	return nil
}

func TestServiceIndexesStructuredDocuments(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(stubFetcher{results: []models.FetchResult{{
		URL:        "https://example.com/page",
		HTML:       "<html><title>Page</title><main><h1>Heading</h1><p>Text.</p></main></html>",
		StatusCode: 200,
		FetchedAt:  time.Unix(100, 0),
	}}}, repository)

	report, err := service.Run(context.Background(), []string{"https://example.com/page"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Indexed != 1 || report.Failed != 0 {
		t.Fatalf("report = %#v", report)
	}
	if len(repository.pages) != 1 || len(repository.sections[0]) != 1 {
		t.Fatalf("repository = %#v", repository)
	}
}

func TestServicePersistsCrawlFailures(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(stubFetcher{results: []models.FetchResult{{
		URL:        "https://example.com/page",
		StatusCode: 503,
		FetchedAt:  time.Unix(100, 0),
		Err:        errors.New("unavailable"),
	}}}, repository)

	report, err := service.Run(context.Background(), []string{"https://example.com/page"})
	if !errors.Is(err, ErrPartialFailure) {
		t.Fatalf("error = %v, want ErrPartialFailure", err)
	}
	if report.Failed != 1 || len(repository.failures) != 1 {
		t.Fatalf("report = %#v, failures = %#v", report, repository.failures)
	}
	if repository.failures[0].HTTPStatus != 503 {
		t.Fatalf("failure = %#v", repository.failures[0])
	}
}
