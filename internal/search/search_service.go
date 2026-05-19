package search

import (
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/crawler"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/parser"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/ranking"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/pkg/text"
)

func Search(question string) []models.SearchResult {
	queryTokens := text.ProcessText(question)
	fetchResults := crawler.FetchURLs(config.SeedURLs)

	pages := make([]models.Page, 0, len(fetchResults))
	for _, result := range fetchResults {
		if result.Err != nil {
			continue
		}

		page := parser.ParseHTML(result)
		if page.URL == "" {
			continue
		}

		pages = append(pages, page)
	}

	return ranking.RankPages(queryTokens, pages)
}
