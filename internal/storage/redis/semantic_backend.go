package redis

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/embedding"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
	goredis "github.com/redis/go-redis/v9"
)

type SemanticBackend struct {
	client     *goredis.Client
	embedder   embedding.Embedder
	indexAlias string
	profile    string
	dimensions int
}

var _ search.Backend = (*SemanticBackend)(nil)

func NewSemanticBackend(
	client *goredis.Client,
	embedder embedding.Embedder,
	profile string,
	dimensions int,
) *SemanticBackend {
	return newSemanticBackend(
		client,
		embedder,
		SemanticSectionIndexAlias,
		profile,
		dimensions,
	)
}

func newSemanticBackend(
	client *goredis.Client,
	embedder embedding.Embedder,
	indexAlias string,
	profile string,
	dimensions int,
) *SemanticBackend {
	return &SemanticBackend{
		client:     client,
		embedder:   embedder,
		indexAlias: indexAlias,
		profile:    profile,
		dimensions: dimensions,
	}
}

func (b *SemanticBackend) Search(
	ctx context.Context,
	request search.Request,
) (search.ResultPage, error) {
	vectors, err := b.embedder.Embed(ctx, []string{request.Query})
	if err != nil {
		return search.ResultPage{}, fmt.Errorf(
			"%w: embed semantic query: %v",
			search.ErrModeUnavailable,
			err,
		)
	}
	vector, err := embedding.Average(vectors, b.dimensions)
	if err != nil {
		return search.ResultPage{}, fmt.Errorf(
			"%w: embed semantic query: %v",
			search.ErrModeUnavailable,
			err,
		)
	}
	filter, err := search.CompileSemanticFilter(
		request.Site,
		request.Language,
		b.profile,
	)
	if err != nil {
		return search.ResultPage{}, err
	}
	query := fmt.Sprintf(
		"%s=>[KNN %d @embedding $vector AS vector_distance]",
		filter,
		maxSectionHits,
	)
	searchResult, err := b.client.FTSearchWithArgs(
		ctx,
		b.indexAlias,
		query,
		&goredis.FTSearchOptions{
			Return: []goredis.FTSearchReturn{
				{FieldName: "$.id", As: "id"},
				{FieldName: "$.page_id", As: "page_id"},
				{FieldName: "$.url", As: "url"},
				{FieldName: "$.page_title", As: "page_title"},
				{FieldName: "$.heading", As: "heading"},
				{FieldName: "$.text", As: "text"},
				{FieldName: "vector_distance"},
			},
			SortBy: []goredis.FTSearchSortBy{{
				FieldName: "vector_distance",
				Asc:       true,
			}},
			LimitOffset: 0,
			Limit:       maxSectionHits,
			Timeout:     2000,
			Params: map[string]interface{}{
				"vector": encodeFloat32Vector(vector),
			},
			DialectVersion: 2,
		},
	).Result()
	if err != nil {
		return search.ResultPage{}, fmt.Errorf("search semantic Redis sections: %w", err)
	}

	results, err := groupSemanticDocuments(searchResult.Docs)
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

func groupSemanticDocuments(
	documents []goredis.Document,
) ([]models.SearchResult, error) {
	type pageGroup struct {
		result    models.SearchResult
		bestScore float64
	}
	groups := make(map[string]*pageGroup)
	for _, document := range documents {
		if document.Error != nil {
			return nil, fmt.Errorf(
				"decode Redis semantic document %s: %w",
				document.ID,
				document.Error,
			)
		}
		pageID := redisField(document.Fields, "page_id")
		if pageID == "" {
			return nil, fmt.Errorf(
				"Redis semantic document %s has no page_id",
				document.ID,
			)
		}
		distance, err := strconv.ParseFloat(
			redisField(document.Fields, "vector_distance"),
			64,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"decode Redis semantic distance for %s: %w",
				document.ID,
				err,
			)
		}
		score := max(-1.0, min(1.0, 1-distance))
		group := groups[pageID]
		if group == nil {
			group = &pageGroup{
				result: models.SearchResult{
					PageID: pageID,
					URL:    pageURL(redisField(document.Fields, "url")),
					Title:  redisField(document.Fields, "page_title"),
					Score:  score,
				},
				bestScore: score,
			}
			groups[pageID] = group
		}
		if score > group.bestScore {
			group.bestScore = score
			group.result.Score = score
		}
		if len(group.result.MatchedSections) < maxSectionsPerResult {
			group.result.MatchedSections = append(
				group.result.MatchedSections,
				models.MatchedSection{
					SectionID: redisField(document.Fields, "id"),
					Heading:   redisField(document.Fields, "heading"),
					Snippet: searchSnippet(
						redisField(document.Fields, "text"),
						"",
					),
					Score: score,
				},
			)
		}
	}

	results := make([]models.SearchResult, 0, len(groups))
	for _, group := range groups {
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

func encodeFloat32Vector(vector []float32) []byte {
	encoded := make([]byte, len(vector)*4)
	for index, value := range vector {
		binary.NativeEndian.PutUint32(
			encoded[index*4:],
			math.Float32bits(value),
		)
	}
	return encoded
}
