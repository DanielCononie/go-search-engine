package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage"
	goredis "github.com/redis/go-redis/v9"
)

const (
	SectionIndexV1    = "search_sections_v1"
	SectionIndexAlias = "search_sections_current"
)

type IndexManager struct {
	client        *goredis.Client
	indexName     string
	indexAlias    string
	sectionPrefix string
}

var _ storage.IndexManager = (*IndexManager)(nil)

func NewIndexManager(client *goredis.Client) *IndexManager {
	return newIndexManager(client, SectionIndexV1, SectionIndexAlias, SectionKeyPrefix)
}

func newIndexManager(
	client *goredis.Client,
	indexName string,
	indexAlias string,
	sectionPrefix string,
) *IndexManager {
	return &IndexManager{
		client:        client,
		indexName:     indexName,
		indexAlias:    indexAlias,
		sectionPrefix: sectionPrefix,
	}
}

func (m *IndexManager) Ensure(ctx context.Context) error {
	if err := m.ensureIndex(ctx); err != nil {
		return err
	}
	if err := m.ensureAlias(ctx); err != nil {
		return err
	}

	return nil
}

func (m *IndexManager) Check(ctx context.Context) error {
	if err := m.client.Do(ctx, "FT.INFO", m.indexAlias).Err(); err != nil {
		return fmt.Errorf("check Redis Search index alias %s: %w", m.indexAlias, err)
	}

	return nil
}

func (m *IndexManager) ensureIndex(ctx context.Context) error {
	err := m.client.Do(ctx, "FT.INFO", m.indexName).Err()
	if err == nil {
		return nil
	}
	if !isUnknownIndexError(err) {
		return fmt.Errorf("inspect Redis Search index %s: %w", m.indexName, err)
	}

	arguments := []any{
		"FT.CREATE", m.indexName,
		"ON", "JSON",
		"PREFIX", "1", m.sectionPrefix,
		"SCHEMA",
		"$.page_title", "AS", "page_title", "TEXT", "WEIGHT", "5.0",
		"$.heading", "AS", "heading", "TEXT", "WEIGHT", "3.0",
		"$.heading_path", "AS", "heading_path", "TEXT", "WEIGHT", "2.0",
		"$.text", "AS", "text", "TEXT",
		"$.page_id", "AS", "page_id", "TAG",
		"$.site", "AS", "site", "TAG",
		"$.language", "AS", "language", "TAG",
		"$.ordinal", "AS", "ordinal", "NUMERIC", "SORTABLE",
		"$.crawled_at", "AS", "crawled_at", "NUMERIC", "SORTABLE",
	}
	if err := m.client.Do(ctx, arguments...).Err(); err != nil {
		return fmt.Errorf("create Redis Search index %s: %w", m.indexName, err)
	}

	return nil
}

func (m *IndexManager) ensureAlias(ctx context.Context) error {
	err := m.client.Do(ctx, "FT.INFO", m.indexAlias).Err()
	if err == nil {
		return nil
	}
	if !isUnknownIndexError(err) {
		return fmt.Errorf("inspect Redis Search index alias %s: %w", m.indexAlias, err)
	}

	if err := m.client.Do(ctx, "FT.ALIASADD", m.indexAlias, m.indexName).Err(); err != nil {
		return fmt.Errorf("create Redis Search index alias %s: %w", m.indexAlias, err)
	}

	return nil
}

func isUnknownIndexError(err error) bool {
	if err == nil || errors.Is(err, goredis.Nil) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unknown index") ||
		strings.Contains(message, "no such index")
}
