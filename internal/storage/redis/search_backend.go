package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	goredis "github.com/redis/go-redis/v9"
)

const (
	maxSectionHits        = 1000
	maxSectionsPerResult  = 3
	snippetLength         = 240
	additionalSectionRate = 0.05
	maxSectionBonusRate   = 0.25
)

type SearchBackend struct {
	client     *goredis.Client
	indexAlias string
}

var _ search.Backend = (*SearchBackend)(nil)

func NewSearchBackend(client *goredis.Client) *SearchBackend {
	return newSearchBackend(client, SectionIndexAlias)
}

func newSearchBackend(client *goredis.Client, indexAlias string) *SearchBackend {
	return &SearchBackend{
		client:     client,
		indexAlias: indexAlias,
	}
}

func (b *SearchBackend) Search(
	ctx context.Context,
	request search.Request,
) (search.ResultPage, error) {
	compiledQuery, err := search.CompileLexicalQuery(
		request.Query,
		request.Site,
		request.Language,
	)
	if err != nil {
		return search.ResultPage{}, err
	}

	searchResult, err := b.client.FTSearchWithArgs(
		ctx,
		b.indexAlias,
		compiledQuery,
		&goredis.FTSearchOptions{
			WithScores: true,
			Scorer:     "BM25",
			Return: []goredis.FTSearchReturn{
				{FieldName: "$.id", As: "id"},
				{FieldName: "$.page_id", As: "page_id"},
				{FieldName: "$.url", As: "url"},
				{FieldName: "$.page_title", As: "page_title"},
				{FieldName: "$.heading", As: "heading"},
				{FieldName: "$.text", As: "text"},
			},
			LimitOffset:    0,
			Limit:          maxSectionHits,
			Timeout:        2000,
			DialectVersion: 2,
		},
	).Result()
	if err != nil {
		return search.ResultPage{}, fmt.Errorf("search Redis sections: %w", err)
	}

	results, err := groupSearchDocuments(request.Query, searchResult.Docs)
	if err != nil {
		return search.ResultPage{}, err
	}
	total := len(results)
	start := min(request.Offset, total)
	end := min(start+request.Limit, total)

	return search.ResultPage{
		Results: results[start:end],
		Total:   total,
	}, nil
}

func groupSearchDocuments(
	query string,
	documents []goredis.Document,
) ([]models.SearchResult, error) {
	type pageGroup struct {
		result       models.SearchResult
		bestScore    float64
		sectionCount int
	}

	groups := map[string]*pageGroup{}
	for _, document := range documents {
		if document.Error != nil {
			return nil, fmt.Errorf("decode Redis search document %s: %w", document.ID, document.Error)
		}
		pageID := redisField(document.Fields, "page_id")
		if pageID == "" {
			return nil, fmt.Errorf("Redis search document %s has no page_id", document.ID)
		}
		score := 0.0
		if document.Score != nil {
			score = *document.Score
		}

		group := groups[pageID]
		if group == nil {
			group = &pageGroup{
				result: models.SearchResult{
					PageID: pageID,
					URL:    pageURL(redisField(document.Fields, "url")),
					Title:  redisField(document.Fields, "page_title"),
				},
				bestScore: score,
			}
			groups[pageID] = group
		}
		group.sectionCount++
		if len(group.result.MatchedSections) < maxSectionsPerResult {
			text := redisField(document.Fields, "text")
			group.result.MatchedSections = append(
				group.result.MatchedSections,
				models.MatchedSection{
					SectionID: redisField(document.Fields, "id"),
					Heading:   redisField(document.Fields, "heading"),
					Snippet:   searchSnippet(text, query),
					Score:     score,
				},
			)
		}
	}

	results := make([]models.SearchResult, 0, len(groups))
	for _, group := range groups {
		bonusRate := min(
			float64(group.sectionCount-1)*additionalSectionRate,
			maxSectionBonusRate,
		)
		group.result.Score = group.bestScore * (1 + bonusRate)
		results = append(results, group.result)
	}
	sort.Slice(results, func(i int, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].URL < results[j].URL
		}
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func redisField(fields map[string]string, name string) string {
	value := fields[name]
	if value == "" {
		return ""
	}

	var values []string
	if strings.HasPrefix(value, "[") && json.Unmarshal([]byte(value), &values) == nil {
		if len(values) > 0 {
			return values[0]
		}
		return ""
	}
	var decoded string
	if json.Unmarshal([]byte(value), &decoded) == nil {
		return decoded
	}

	return value
}

func pageURL(sectionURL string) string {
	parsed, err := url.Parse(sectionURL)
	if err != nil {
		return sectionURL
	}
	parsed.Fragment = ""
	return parsed.String()
}

func searchSnippet(text string, query string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= snippetLength {
		return text
	}

	lowerText := strings.ToLower(text)
	matchAt := -1
	for _, term := range queryTerms(query) {
		if index := strings.Index(lowerText, term); index >= 0 &&
			(matchAt == -1 || index < matchAt) {
			matchAt = index
		}
	}
	if matchAt == -1 {
		matchAt = 0
	}

	start := max(0, matchAt-snippetLength/3)
	end := min(len(text), start+snippetLength)
	start = runeBoundaryForward(text, start)
	end = runeBoundaryBackward(text, end)

	snippet := text[start:end]
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(text) {
		snippet += "…"
	}

	return snippet
}

func queryTerms(query string) []string {
	compiled, err := search.CompileLexicalQuery(query, "", "")
	if err != nil {
		return nil
	}
	compiled = strings.ReplaceAll(compiled, `"`, "")
	return strings.Fields(compiled)
}

func runeBoundaryForward(value string, index int) int {
	for index < len(value) && !isUTF8Start(value[index]) {
		index++
	}
	return index
}

func runeBoundaryBackward(value string, index int) int {
	for index > 0 && index < len(value) && !isUTF8Start(value[index]) {
		index--
	}
	return index
}

func isUTF8Start(value byte) bool {
	return value&0xC0 != 0x80
}
