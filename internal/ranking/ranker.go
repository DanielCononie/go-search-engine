package ranking

import (
	"sort"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
)

// score = number of query terms found in page

func RankPages(queryTokens []string, pages []models.Page) []models.SearchResult {

	querySet := map[string]struct{}{}

	for _, token := range queryTokens {
		querySet[token] = struct{}{}
	}

	results := []models.SearchResult{}
	// construct search result, then sort by score (future state possibly insert effectively to avoid sorting)
	for _, page := range pages {

		score := 0
		for _, pageToken := range page.Tokens {
			if _, ok := querySet[pageToken]; ok {
				score++
			}
		}

		if score == 0 {
			continue
		}

		results = append(results, models.SearchResult{
			URL:   page.URL,
			Title: page.Title,
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}
