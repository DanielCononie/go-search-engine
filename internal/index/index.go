package index

import (
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/crawler"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/parser"
)

type Posting struct {
	PageID int
	Count  int
}

// Index keeps the parsed corpus in memory so search requests do not need to
// fetch and parse every URL each time.
type Index struct {
	Pages   []models.Page
	ByToken map[string][]Posting
	BuiltAt time.Time
}

func New() *Index {
	return &Index{
		Pages:   []models.Page{},
		ByToken: map[string][]Posting{},
		BuiltAt: time.Now(),
	}
}

func Build(urls []string) *Index {
	idx := New()
	fetchResults := crawler.FetchURLs(urls)

	// Build the searchable corpus once from the configured seed URLs.
	for _, result := range fetchResults {
		if result.Err != nil {
			continue
		}

		page := parser.ParseHTML(result)
		if page.URL == "" {
			continue
		}

		idx.AddPage(page)
	}

	return idx
}

func (idx *Index) AddPage(page models.Page) {
	pageID := len(idx.Pages)
	idx.Pages = append(idx.Pages, page)

	// Collapse repeated tokens into counts before adding postings, so each
	// token points to this page only once.
	tokenCounts := map[string]int{}
	for _, token := range page.Tokens {
		tokenCounts[token]++
	}

	for token, count := range tokenCounts {
		idx.ByToken[token] = append(idx.ByToken[token], Posting{
			PageID: pageID,
			Count:  count,
		})
	}

}

func (idx *Index) PagesForTokens(tokens []string) []models.Page {
	seenPageIDs := map[int]struct{}{}
	pages := []models.Page{}

	// Gather only pages that contain at least one query token. Ranking can then
	// work on this smaller candidate set instead of scanning the whole corpus.
	for _, token := range tokens {
		for _, posting := range idx.ByToken[token] {
			if _, ok := seenPageIDs[posting.PageID]; ok {
				continue
			}

			seenPageIDs[posting.PageID] = struct{}{}
			pages = append(pages, idx.Pages[posting.PageID])
		}
	}

	return pages
}
