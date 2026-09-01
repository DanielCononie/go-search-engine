package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/parser"
)

var ErrPartialFailure = errors.New("one or more pages failed to index")

type Fetcher interface {
	FetchAll(ctx context.Context, urls []string) []models.FetchResult
}

type Repository interface {
	ReplacePage(
		ctx context.Context,
		page documents.Page,
		sections []documents.Section,
	) error
	SaveCrawlFailure(ctx context.Context, failure documents.CrawlFailure) error
}

type Failure struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type Report struct {
	Requested int       `json:"requested"`
	Indexed   int       `json:"indexed"`
	Failed    int       `json:"failed"`
	Failures  []Failure `json:"failures,omitempty"`
}

type Service struct {
	fetcher    Fetcher
	repository Repository
}

func NewService(fetcher Fetcher, repository Repository) *Service {
	return &Service{
		fetcher:    fetcher,
		repository: repository,
	}
}

func (s *Service) Run(ctx context.Context, urls []string) (Report, error) {
	report := Report{Requested: len(urls)}
	results := s.fetcher.FetchAll(ctx, urls)
	for _, result := range results {
		if result.Err != nil {
			s.recordFailure(ctx, &report, result, result.Err)
			continue
		}

		parsed, err := parser.ParseDocument(result)
		if err != nil {
			s.recordFailure(ctx, &report, result, err)
			continue
		}
		if err := s.repository.ReplacePage(ctx, parsed.Page, parsed.Sections); err != nil {
			s.recordFailure(ctx, &report, result, err)
			continue
		}
		report.Indexed++
	}

	if report.Failed > 0 {
		return report, ErrPartialFailure
	}

	return report, nil
}

func (s *Service) recordFailure(
	ctx context.Context,
	report *Report,
	result models.FetchResult,
	cause error,
) {
	report.Failed++
	report.Failures = append(report.Failures, Failure{
		URL:   result.URL,
		Error: cause.Error(),
	})

	canonicalURL, pageID, _, err := parser.PageIdentity(result.URL)
	if err != nil {
		return
	}
	attemptedAt := result.FetchedAt
	if attemptedAt.IsZero() {
		attemptedAt = time.Now()
	}
	failure := documents.CrawlFailure{
		PageID:      pageID,
		URL:         canonicalURL,
		HTTPStatus:  result.StatusCode,
		AttemptedAt: attemptedAt.Unix(),
		Error:       cause.Error(),
	}
	if err := s.repository.SaveCrawlFailure(ctx, failure); err != nil {
		report.Failures = append(report.Failures, Failure{
			URL: canonicalURL,
			Error: fmt.Sprintf(
				"persist crawl failure: %v",
				err,
			),
		})
	}
}
