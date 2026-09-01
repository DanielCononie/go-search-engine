package search

import (
	"context"
	"errors"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/index"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/ranking"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/pkg/text"
)

const (
	ContractVersion = "v1"
	DefaultLimit    = 10
	MaxLimit        = 100
)

type Mode string

const ModeLexical Mode = "lexical"

var (
	ErrUnsupportedMode   = errors.New("unsupported search mode")
	ErrInvalidPagination = errors.New("invalid search pagination")
)

type Request struct {
	Query  string
	Mode   Mode
	Limit  int
	Offset int
}

type Response struct {
	Version    string                `json:"version"`
	Query      string                `json:"query"`
	Mode       Mode                  `json:"mode"`
	UsedMode   Mode                  `json:"used_mode"`
	TookMS     int64                 `json:"took_ms"`
	NextOffset *int                  `json:"next_offset"`
	Results    []models.SearchResult `json:"results"`
}

type ResultPage struct {
	Results []models.SearchResult
	Total   int
}

type Backend interface {
	Search(ctx context.Context, request Request) (ResultPage, error)
}

type Service struct {
	backend Backend
}

func NewService(backend Backend) *Service {
	return &Service{
		backend: backend,
	}
}

func (s *Service) Search(ctx context.Context, request Request) (Response, error) {
	if request.Mode == "" {
		request.Mode = ModeLexical
	}
	if request.Mode != ModeLexical {
		return Response{}, ErrUnsupportedMode
	}
	if request.Limit == 0 {
		request.Limit = DefaultLimit
	}
	if request.Limit < 1 || request.Limit > MaxLimit || request.Offset < 0 {
		return Response{}, ErrInvalidPagination
	}

	startedAt := time.Now()
	page, err := s.backend.Search(ctx, request)
	if err != nil {
		return Response{}, err
	}
	if page.Results == nil {
		page.Results = []models.SearchResult{}
	}

	var nextOffset *int
	if request.Offset+len(page.Results) < page.Total {
		next := request.Offset + len(page.Results)
		nextOffset = &next
	}

	return Response{
		Version:    ContractVersion,
		Query:      request.Query,
		Mode:       request.Mode,
		UsedMode:   request.Mode,
		TookMS:     time.Since(startedAt).Milliseconds(),
		NextOffset: nextOffset,
		Results:    page.Results,
	}, nil
}

type InMemoryBackend struct {
	index *index.Index
}

func NewInMemoryBackend(idx *index.Index) *InMemoryBackend {
	return &InMemoryBackend{index: idx}
}

func (b *InMemoryBackend) Search(_ context.Context, request Request) (ResultPage, error) {
	queryTokens := text.ProcessText(request.Query)
	pages := b.index.PagesForTokens(queryTokens)
	results := ranking.RankPages(queryTokens, pages)
	total := len(results)

	start := min(request.Offset, total)
	end := min(start+request.Limit, total)

	return ResultPage{
		Results: results[start:end],
		Total:   total,
	}, nil
}
