package redis

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage"
	goredis "github.com/redis/go-redis/v9"
)

const (
	PageKeyPrefix         = "search:page:"
	SectionKeyPrefix      = "search:section:"
	CrawlFailureKeyPrefix = "search:crawl-failure:"
)

var replacePageScript = goredis.NewScript(`
local oldPage = redis.call("JSON.GET", KEYS[1])
local newSectionIDs = {}

for keyIndex = 3, #KEYS do
	local sectionID = string.sub(KEYS[keyIndex], string.len(ARGV[1]) + 1)
	newSectionIDs[sectionID] = true
end

if oldPage then
	local decodedPage = cjson.decode(oldPage)
	if type(decodedPage["section_ids"]) == "table" then
		for _, sectionID in ipairs(decodedPage["section_ids"]) do
			if not newSectionIDs[sectionID] then
				redis.call("DEL", ARGV[1] .. sectionID)
			end
		end
	end
end

for keyIndex = 3, #KEYS do
	local encodedSection = ARGV[keyIndex]
	local oldSection = redis.call("JSON.GET", KEYS[keyIndex])
	local shouldWrite = not oldSection
	if oldSection then
		local decodedOld = cjson.decode(oldSection)
		local decodedNew = cjson.decode(encodedSection)
		shouldWrite = decodedOld["content_hash"] ~= decodedNew["content_hash"]
		if not shouldWrite then
			local metadataFields = {
				"url",
				"page_title",
				"heading",
				"heading_path",
				"ordinal",
				"token_count",
				"language",
				"site"
			}
			for _, field in ipairs(metadataFields) do
				if decodedOld[field] ~= decodedNew[field] then
					shouldWrite = true
					break
				end
			end
			if shouldWrite then
				local embeddingFields = {
					"embedding",
					"embedding_model",
					"embedding_dim",
					"embedding_updated_at"
				}
				for _, field in ipairs(embeddingFields) do
					if decodedOld[field] ~= nil then
						decodedNew[field] = decodedOld[field]
					end
				end
				encodedSection = cjson.encode(decodedNew)
			end
		end
	end
	if shouldWrite then
		redis.call("JSON.SET", KEYS[keyIndex], "$", encodedSection)
	end
end

redis.call("JSON.SET", KEYS[1], "$", ARGV[2])
redis.call("DEL", KEYS[2])
return 1
`)

type Repository struct {
	client             *goredis.Client
	pagePrefix         string
	sectionPrefix      string
	crawlFailurePrefix string
}

type CorpusDiagnostics struct {
	Version              string `json:"version"`
	Pages                int    `json:"pages"`
	ExpectedSections     int    `json:"expected_sections"`
	LatestCrawlTimestamp int64  `json:"latest_crawl_timestamp"`
}

var _ storage.PageRepository = (*Repository)(nil)
var _ storage.SectionRepository = (*Repository)(nil)

func NewRepository(client *goredis.Client) *Repository {
	return newRepository(
		client,
		PageKeyPrefix,
		SectionKeyPrefix,
		CrawlFailureKeyPrefix,
	)
}

func newRepository(
	client *goredis.Client,
	pagePrefix string,
	sectionPrefix string,
	crawlFailurePrefix string,
) *Repository {
	return &Repository{
		client:             client,
		pagePrefix:         pagePrefix,
		sectionPrefix:      sectionPrefix,
		crawlFailurePrefix: crawlFailurePrefix,
	}
}

func (r *Repository) SavePage(ctx context.Context, page documents.Page) error {
	if page.ID == "" {
		return errors.New("page ID is required")
	}

	return r.saveJSON(ctx, r.pagePrefix+page.ID, page)
}

func (r *Repository) Page(ctx context.Context, id string) (documents.Page, error) {
	if err := validateID("page", id); err != nil {
		return documents.Page{}, err
	}

	var page documents.Page
	if err := r.loadJSON(ctx, r.pagePrefix+id, &page); err != nil {
		return documents.Page{}, err
	}

	return page, nil
}

func (r *Repository) DeletePage(ctx context.Context, id string) error {
	if err := validateID("page", id); err != nil {
		return err
	}

	return r.client.Del(ctx, r.pagePrefix+id).Err()
}

func (r *Repository) ReplacePage(
	ctx context.Context,
	page documents.Page,
	sections []documents.Section,
) error {
	if err := validateID("page", page.ID); err != nil {
		return err
	}
	if len(page.SectionIDs) != len(sections) {
		return errors.New("page section IDs must match replacement sections")
	}

	keys := make([]string, 0, len(sections)+2)
	keys = append(
		keys,
		r.pagePrefix+page.ID,
		r.crawlFailurePrefix+page.ID,
	)
	arguments := make([]any, 0, len(sections)+2)
	arguments = append(arguments, r.sectionPrefix)

	encodedPage, err := json.Marshal(page)
	if err != nil {
		return fmt.Errorf("encode page %s: %w", page.ID, err)
	}
	arguments = append(arguments, string(encodedPage))
	for index, section := range sections {
		if err := validateID("section", section.ID); err != nil {
			return err
		}
		if page.SectionIDs[index] != section.ID {
			return errors.New("page section IDs must match replacement sections")
		}
		if section.PageID != page.ID {
			return fmt.Errorf("section %s does not belong to page %s", section.ID, page.ID)
		}

		encodedSection, err := json.Marshal(section)
		if err != nil {
			return fmt.Errorf("encode section %s: %w", section.ID, err)
		}
		keys = append(keys, r.sectionPrefix+section.ID)
		arguments = append(arguments, string(encodedSection))
	}

	if err := replacePageScript.Run(ctx, r.client, keys, arguments...).Err(); err != nil {
		return fmt.Errorf("replace page %s: %w", page.ID, err)
	}

	return nil
}

func (r *Repository) SaveCrawlFailure(
	ctx context.Context,
	failure documents.CrawlFailure,
) error {
	if err := validateID("page", failure.PageID); err != nil {
		return err
	}

	return r.saveJSON(ctx, r.crawlFailurePrefix+failure.PageID, failure)
}

func (r *Repository) CorpusDiagnostics(
	ctx context.Context,
) (CorpusDiagnostics, error) {
	keys, err := r.scanKeys(ctx, r.pagePrefix+"*")
	if err != nil {
		return CorpusDiagnostics{}, err
	}

	versions := make([]string, 0, len(keys))
	diagnostics := CorpusDiagnostics{Pages: len(keys)}
	for _, key := range keys {
		var page documents.Page
		if err := r.loadJSON(ctx, key, &page); err != nil {
			return CorpusDiagnostics{}, err
		}
		diagnostics.ExpectedSections += len(page.SectionIDs)
		diagnostics.LatestCrawlTimestamp = max(
			diagnostics.LatestCrawlTimestamp,
			page.CrawledAt,
		)
		versions = append(versions, page.ID+":"+page.ContentHash)
	}
	sort.Strings(versions)
	diagnostics.Version = corpusVersion(versions)

	return diagnostics, nil
}

func (r *Repository) SaveSection(ctx context.Context, section documents.Section) error {
	if section.ID == "" {
		return errors.New("section ID is required")
	}

	return r.saveJSON(ctx, r.sectionPrefix+section.ID, section)
}

func (r *Repository) Section(ctx context.Context, id string) (documents.Section, error) {
	if err := validateID("section", id); err != nil {
		return documents.Section{}, err
	}

	var section documents.Section
	if err := r.loadJSON(ctx, r.sectionPrefix+id, &section); err != nil {
		return documents.Section{}, err
	}

	return section, nil
}

func (r *Repository) DeleteSection(ctx context.Context, id string) error {
	if err := validateID("section", id); err != nil {
		return err
	}

	return r.client.Del(ctx, r.sectionPrefix+id).Err()
}

func validateID(documentType string, id string) error {
	if id == "" {
		return fmt.Errorf("%s ID is required", documentType)
	}

	return nil
}

func (r *Repository) saveJSON(ctx context.Context, key string, document any) error {
	encoded, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}

	if err := r.client.Do(ctx, "JSON.SET", key, "$", string(encoded)).Err(); err != nil {
		return fmt.Errorf("save %s: %w", key, err)
	}

	return nil
}

func (r *Repository) loadJSON(ctx context.Context, key string, destination any) error {
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

func (r *Repository) scanKeys(ctx context.Context, pattern string) ([]string, error) {
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

func corpusVersion(pageVersions []string) string {
	hash := sha256.New()
	for _, version := range pageVersions {
		_, _ = hash.Write([]byte(version))
		_, _ = hash.Write([]byte{0})
	}

	return fmt.Sprintf("sha256:%x", hash.Sum(nil))
}
