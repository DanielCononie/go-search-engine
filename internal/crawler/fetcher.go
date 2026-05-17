package crawler

import (
	"fmt"
	"io"

	"net/http"

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

	// Make an HTTP request
	// Get the raw HTML

	// Return it to the search service

	res, err := http.Get(url)

	if err != nil {
		return models.FetchResult{}, err
	}

	parsed, err := io.ReadAll(res.Body)

	if err != nil {
		return models.FetchResult{}, err
	}

	return models.FetchResult{
		URL:  url,
		HTML: string(parsed),
	}, nil

}

func FetchURLs(urls []string) []models.FetchResult {
	fmt.Print(urls)
	return []models.FetchResult{}
}
