package crawler

import (
	"fmt"
	"io"
	"sync"
	"time"

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

	// create custom client
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return models.FetchResult{}, err
	}

	req.Header.Set("User-Agent", "go-search-engine/0.1 (learning project; contact: danielcononie1278@gmail.com)")

	res, err := client.Do(req)
	if err != nil {
		return models.FetchResult{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return models.FetchResult{}, fmt.Errorf("unexpected status code: %d", res.StatusCode)
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
	var wg sync.WaitGroup

	resultsChan := make(chan models.FetchResult, len(urls))

	for _, url := range urls {
		wg.Add(1)

		go func(url string) {
			defer wg.Done()

			res, err := FetchURL(url)
			if err != nil {
				// add error result to the channel
				resultsChan <- models.FetchResult{
					URL: url,
					Err: err,
				}
				return
			}

			// add the result to the channel
			resultsChan <- res
		}(url)
	}

	wg.Wait()
	close(resultsChan)

	results := make([]models.FetchResult, 0, len(urls))

	// add results from channel to slice
	for result := range resultsChan {
		results = append(results, result)
	}

	return results
}
