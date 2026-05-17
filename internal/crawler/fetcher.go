package crawler

import (
	"fmt"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
)

// Take a URL
// ↓
// Make an HTTP request
// ↓
// Get the raw HTML
// ↓
// Return it to the search service

func FetchURL(url string) (models.FetchResult, error) {

	return models.FetchResult{}, nil
}

func FetchURLs(urls []string) []models.FetchResult {
	fmt.Print(urls)
	return []models.FetchResult{}
}
