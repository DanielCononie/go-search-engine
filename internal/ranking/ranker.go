package ranking

import "github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"

// score = number of query terms found in page title/body
// title matches are worth more
// heading matches are worth more
// body matches are worth less

func RankPages(queryTokens []string, pages []models.Page) []models.SearchResult {
	return []models.SearchResult{}
}
