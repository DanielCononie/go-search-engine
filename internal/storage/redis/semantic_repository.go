package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage"
	goredis "github.com/redis/go-redis/v9"
)

const SemanticSectionKeyPrefix = "search:semantic-section:"

var saveSemanticSectionScript = goredis.NewScript(`
local encodedSource = redis.call("JSON.GET", KEYS[1])
if not encodedSource then
	return -1
end

local source = cjson.decode(encodedSource)
if source["content_hash"] ~= ARGV[1] then
	return 0
end

redis.call("JSON.SET", KEYS[2], "$", ARGV[2])
return 1
`)

type SemanticRepository struct {
	client         *goredis.Client
	sectionPrefix  string
	semanticPrefix string
}

type SemanticCorpusDiagnostics struct {
	IndexVersion   string `json:"index_version"`
	Profile        string `json:"profile"`
	SourceSections int    `json:"source_sections"`
	Records        int    `json:"records"`
	Ready          int    `json:"ready"`
	Failed         int    `json:"failed"`
	Stale          int    `json:"stale"`
	Missing        int    `json:"missing"`
	InSync         bool   `json:"in_sync"`
}

func NewSemanticRepository(
	client *goredis.Client,
	indexVersion string,
) *SemanticRepository {
	return newSemanticRepository(
		client,
		SectionKeyPrefix,
		SemanticSectionKeyPrefix+indexVersion+":",
	)
}

func newSemanticRepository(
	client *goredis.Client,
	sectionPrefix string,
	semanticPrefix string,
) *SemanticRepository {
	return &SemanticRepository{
		client:         client,
		sectionPrefix:  sectionPrefix,
		semanticPrefix: semanticPrefix,
	}
}

func (r *SemanticRepository) Sections(
	ctx context.Context,
) ([]documents.Section, error) {
	repository := newRepository(r.client, "", r.sectionPrefix, "")
	return repository.Sections(ctx)
}

func (r *SemanticRepository) SemanticSections(
	ctx context.Context,
) ([]documents.SemanticSection, error) {
	keys, err := r.scanKeys(ctx, r.semanticPrefix+"*")
	if err != nil {
		return nil, err
	}

	sections := make([]documents.SemanticSection, 0, len(keys))
	for _, key := range keys {
		var section documents.SemanticSection
		if err := r.loadJSON(ctx, key, &section); err != nil {
			return nil, err
		}
		sections = append(sections, section)
	}
	sort.Slice(sections, func(i int, j int) bool {
		return sections[i].SectionID < sections[j].SectionID
	})

	return sections, nil
}

func (r *SemanticRepository) SaveSemanticSection(
	ctx context.Context,
	section documents.SemanticSection,
) (bool, error) {
	if err := validateID("section", section.SectionID); err != nil {
		return false, err
	}
	if section.ID != section.SectionID ||
		section.SourceContentHash == "" ||
		section.EmbeddingModel == "" ||
		section.EmbeddingVersion == "" ||
		section.EmbeddingProfile == "" ||
		section.EmbeddingDimensions < 1 ||
		(section.EmbeddingStatus != "ready" &&
			section.EmbeddingStatus != "failed") ||
		(section.EmbeddingStatus == "ready" &&
			len(section.Embedding) != section.EmbeddingDimensions) ||
		(section.EmbeddingStatus == "failed" &&
			len(section.Embedding) != 0) {
		return false, errors.New("valid semantic section metadata is required")
	}
	encoded, err := json.Marshal(section)
	if err != nil {
		return false, fmt.Errorf(
			"encode semantic section %s: %w",
			section.SectionID,
			err,
		)
	}

	result, err := saveSemanticSectionScript.Run(
		ctx,
		r.client,
		[]string{
			r.sectionPrefix + section.SectionID,
			r.semanticPrefix + section.SectionID,
		},
		section.SourceContentHash,
		string(encoded),
	).Int()
	if err != nil {
		return false, fmt.Errorf(
			"save semantic section %s: %w",
			section.SectionID,
			err,
		)
	}
	if result < 0 {
		return false, storage.ErrNotFound
	}

	return result == 1, nil
}

func (r *SemanticRepository) DeleteSemanticSection(
	ctx context.Context,
	sectionID string,
) error {
	if err := validateID("section", sectionID); err != nil {
		return err
	}
	return r.client.Del(ctx, r.semanticPrefix+sectionID).Err()
}

func (r *SemanticRepository) Diagnostics(
	ctx context.Context,
	indexVersion string,
	profile string,
	dimensions int,
) (SemanticCorpusDiagnostics, error) {
	sections, err := r.Sections(ctx)
	if err != nil {
		return SemanticCorpusDiagnostics{}, err
	}
	semanticSections, err := r.SemanticSections(ctx)
	if err != nil {
		return SemanticCorpusDiagnostics{}, err
	}
	records := make(map[string]documents.SemanticSection, len(semanticSections))
	for _, section := range semanticSections {
		records[section.SectionID] = section
	}

	diagnostics := SemanticCorpusDiagnostics{
		IndexVersion:   indexVersion,
		Profile:        profile,
		SourceSections: len(sections),
		Records:        len(semanticSections),
	}
	for _, section := range sections {
		semanticSection, exists := records[section.ID]
		delete(records, section.ID)
		if !exists {
			diagnostics.Missing++
			continue
		}
		if semanticSection.SourceContentHash != section.ContentHash ||
			semanticSection.EmbeddingProfile != profile ||
			semanticSection.EmbeddingDimensions != dimensions {
			diagnostics.Stale++
			continue
		}
		switch semanticSection.EmbeddingStatus {
		case "ready":
			if len(semanticSection.Embedding) == dimensions {
				diagnostics.Ready++
			} else {
				diagnostics.Stale++
			}
		case "failed":
			diagnostics.Failed++
		default:
			diagnostics.Stale++
		}
	}
	diagnostics.Stale += len(records)
	diagnostics.InSync = diagnostics.Ready == diagnostics.SourceSections &&
		diagnostics.Records == diagnostics.SourceSections &&
		diagnostics.Failed == 0 &&
		diagnostics.Stale == 0 &&
		diagnostics.Missing == 0
	return diagnostics, nil
}

func (r *SemanticRepository) loadJSON(
	ctx context.Context,
	key string,
	destination any,
) error {
	value, err := r.client.Do(ctx, "JSON.GET", key).Text()
	if errors.Is(err, goredis.Nil) {
		return storage.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("load %s: %w", key, err)
	}
	if err := json.Unmarshal([]byte(value), destination); err != nil {
		return fmt.Errorf("decode %s: %w", key, err)
	}
	return nil
}

func (r *SemanticRepository) scanKeys(
	ctx context.Context,
	pattern string,
) ([]string, error) {
	keys := make([]string, 0)
	iterator := r.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iterator.Next(ctx) {
		keys = append(keys, iterator.Val())
	}
	if err := iterator.Err(); err != nil {
		return nil, fmt.Errorf("scan Redis keys matching %s: %w", pattern, err)
	}
	sort.Strings(keys)
	return keys, nil
}
