package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	SemanticSectionIndexPrefix = "search_semantic_sections_"
	SemanticSectionIndexAlias  = "search_semantic_sections_current"
)

type SemanticIndexDiagnostics struct {
	Alias                 string  `json:"alias"`
	PhysicalIndex         string  `json:"physical_index"`
	SchemaVersion         string  `json:"schema_version"`
	SectionPrefix         string  `json:"section_prefix"`
	Dimensions            int     `json:"dimensions"`
	IndexedSections       int     `json:"indexed_sections"`
	Indexing              bool    `json:"indexing"`
	PercentIndexed        float64 `json:"percent_indexed"`
	IndexingFailures      int     `json:"indexing_failures"`
	LastIndexingError     string  `json:"last_indexing_error,omitempty"`
	LastIndexingErrorKey  string  `json:"last_indexing_error_key,omitempty"`
	IndexMemoryMegabytes  float64 `json:"index_memory_megabytes"`
	VectorMemoryMegabytes float64 `json:"vector_memory_megabytes"`
}

type SemanticIndexManager struct {
	client        *goredis.Client
	indexName     string
	indexAlias    string
	sectionPrefix string
	dimensions    int
}

func NewSemanticIndexManager(
	client *goredis.Client,
	indexVersion string,
	dimensions int,
) *SemanticIndexManager {
	return newSemanticIndexManager(
		client,
		SemanticSectionIndexPrefix+indexVersion,
		SemanticSectionIndexAlias,
		SemanticSectionKeyPrefix+indexVersion+":",
		dimensions,
	)
}

func newSemanticIndexManager(
	client *goredis.Client,
	indexName string,
	indexAlias string,
	sectionPrefix string,
	dimensions int,
) *SemanticIndexManager {
	return &SemanticIndexManager{
		client:        client,
		indexName:     indexName,
		indexAlias:    indexAlias,
		sectionPrefix: sectionPrefix,
		dimensions:    dimensions,
	}
}

func (m *SemanticIndexManager) Ensure(ctx context.Context) error {
	if err := m.ensureIndex(ctx); err != nil {
		return err
	}
	if err := m.ensureAlias(ctx); err != nil {
		return err
	}
	return nil
}

func (m *SemanticIndexManager) Check(ctx context.Context) error {
	if err := m.client.Do(ctx, "FT.INFO", m.indexAlias).Err(); err != nil {
		return fmt.Errorf(
			"check semantic Redis Search index alias %s: %w",
			m.indexAlias,
			err,
		)
	}
	return nil
}

func (m *SemanticIndexManager) TargetName() string {
	return m.indexName
}

func (m *SemanticIndexManager) Activate(ctx context.Context) error {
	if err := m.client.FTAliasUpdate(
		ctx,
		m.indexName,
		m.indexAlias,
	).Err(); err != nil {
		return fmt.Errorf(
			"activate semantic Redis Search index %s: %w",
			m.indexName,
			err,
		)
	}
	return nil
}

func (m *SemanticIndexManager) WaitReady(
	ctx context.Context,
	expectedSections int,
) error {
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		diagnostics, err := m.targetDiagnostics(ctx)
		if err != nil {
			return err
		}
		if diagnostics.IndexingFailures > 0 {
			return fmt.Errorf(
				"semantic index reports %d indexing failures: %s",
				diagnostics.IndexingFailures,
				diagnostics.LastIndexingError,
			)
		}
		if !diagnostics.Indexing &&
			diagnostics.IndexedSections == expectedSections {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf(
				"semantic index %s did not reach %d documents",
				m.indexName,
				expectedSections,
			)
		case <-ticker.C:
		}
	}
}

func (m *SemanticIndexManager) Diagnostics(
	ctx context.Context,
) (SemanticIndexDiagnostics, error) {
	return m.diagnostics(ctx, m.indexAlias)
}

func (m *SemanticIndexManager) targetDiagnostics(
	ctx context.Context,
) (SemanticIndexDiagnostics, error) {
	return m.diagnostics(ctx, m.indexName)
}

func (m *SemanticIndexManager) diagnostics(
	ctx context.Context,
	indexName string,
) (SemanticIndexDiagnostics, error) {
	info, err := m.client.FTInfo(ctx, indexName).Result()
	if err != nil {
		return SemanticIndexDiagnostics{}, fmt.Errorf(
			"inspect semantic Redis Search index %s: %w",
			indexName,
			err,
		)
	}

	return SemanticIndexDiagnostics{
		Alias:                 m.indexAlias,
		PhysicalIndex:         info.IndexName,
		SchemaVersion:         semanticIndexVersion(info.IndexName),
		SectionPrefix:         semanticSectionPrefix(info, m.sectionPrefix),
		Dimensions:            semanticDimensions(info, m.dimensions),
		IndexedSections:       info.NumDocs,
		Indexing:              info.Indexing != 0,
		PercentIndexed:        info.PercentIndexed,
		IndexingFailures:      info.IndexErrors.IndexingFailures,
		LastIndexingError:     info.IndexErrors.LastIndexingError,
		LastIndexingErrorKey:  info.IndexErrors.LastIndexingErrorKey,
		IndexMemoryMegabytes:  info.TotalIndexMemorySzMB,
		VectorMemoryMegabytes: info.VectorIndexSzMB,
	}, nil
}

func semanticIndexVersion(indexName string) string {
	if !strings.HasPrefix(indexName, SemanticSectionIndexPrefix) {
		return ""
	}
	return strings.TrimPrefix(indexName, SemanticSectionIndexPrefix)
}

func (m *SemanticIndexManager) ensureIndex(ctx context.Context) error {
	info, err := m.client.FTInfo(ctx, m.indexName).Result()
	if err == nil {
		return m.validateIndex(info)
	}
	if !isUnknownIndexError(err) {
		return fmt.Errorf(
			"inspect semantic Redis Search index %s: %w",
			m.indexName,
			err,
		)
	}

	arguments := []any{
		"FT.CREATE", m.indexName,
		"ON", "JSON",
		"PREFIX", "1", m.sectionPrefix,
		"SCHEMA",
		"$.page_id", "AS", "page_id", "TAG",
		"$.site", "AS", "site", "TAG",
		"$.language", "AS", "language", "TAG",
		"$.embedding_profile", "AS", "embedding_profile", "TAG",
		"$.embedding_status", "AS", "embedding_status", "TAG",
		"$.ordinal", "AS", "ordinal", "NUMERIC", "SORTABLE",
		"$.crawled_at", "AS", "crawled_at", "NUMERIC", "SORTABLE",
		"$.embedding", "AS", "embedding",
		"VECTOR", "FLAT", "6",
		"TYPE", "FLOAT32",
		"DIM", strconv.Itoa(m.dimensions),
		"DISTANCE_METRIC", "COSINE",
	}
	if err := m.client.Do(ctx, arguments...).Err(); err != nil {
		return fmt.Errorf(
			"create semantic Redis Search index %s: %w",
			m.indexName,
			err,
		)
	}
	return nil
}

func (m *SemanticIndexManager) validateIndex(
	info goredis.FTInfoResult,
) error {
	if semanticSectionPrefix(info, "") != m.sectionPrefix ||
		semanticDimensions(info, 0) != m.dimensions {
		return fmt.Errorf(
			"semantic index %s does not match its configured prefix and dimensions; change EMBEDDING_INDEX_VERSION",
			m.indexName,
		)
	}
	for _, attribute := range info.Attributes {
		if attribute.Attribute != "embedding" {
			continue
		}
		if !strings.EqualFold(attribute.Type, "VECTOR") ||
			!strings.EqualFold(attribute.Algorithm, "FLAT") ||
			!strings.EqualFold(attribute.DataType, "FLOAT32") ||
			!strings.EqualFold(attribute.DistanceMetric, "COSINE") {
			return fmt.Errorf(
				"semantic index %s has an incompatible vector schema; change EMBEDDING_INDEX_VERSION",
				m.indexName,
			)
		}
		return nil
	}
	return fmt.Errorf(
		"semantic index %s has no embedding vector field; change EMBEDDING_INDEX_VERSION",
		m.indexName,
	)
}

func semanticSectionPrefix(
	info goredis.FTInfoResult,
	fallback string,
) string {
	if len(info.IndexDefinition.Prefixes) == 0 {
		return fallback
	}
	return info.IndexDefinition.Prefixes[0]
}

func semanticDimensions(
	info goredis.FTInfoResult,
	fallback int,
) int {
	for _, attribute := range info.Attributes {
		if attribute.Attribute == "embedding" &&
			strings.EqualFold(attribute.Type, "VECTOR") {
			return attribute.Dim
		}
	}
	return fallback
}

func (m *SemanticIndexManager) ensureAlias(ctx context.Context) error {
	err := m.client.Do(ctx, "FT.INFO", m.indexAlias).Err()
	if err == nil {
		return nil
	}
	if !isUnknownIndexError(err) {
		return fmt.Errorf(
			"inspect semantic Redis Search index alias %s: %w",
			m.indexAlias,
			err,
		)
	}
	if err := m.client.FTAliasAdd(
		ctx,
		m.indexName,
		m.indexAlias,
	).Err(); err != nil {
		return fmt.Errorf(
			"create semantic Redis Search index alias %s: %w",
			m.indexAlias,
			err,
		)
	}
	return nil
}
