package search

import (
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/index"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/ranking"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/pkg/text"
)

type Service struct {
	index *index.Index
}

// NewService receives an already-built index so the request path can stay fast
// and avoid crawling during each search.
func NewService(idx *index.Index) *Service {
	return &Service{
		index: idx,
	}
}

func (s *Service) Search(question string) []models.SearchResult {
	queryTokens := text.ProcessText(question)
	// Use the inverted index to find candidate pages before scoring them.
	pages := s.index.PagesForTokens(queryTokens)

	return ranking.RankPages(queryTokens, pages)
}
