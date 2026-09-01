package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage"
	goredis "github.com/redis/go-redis/v9"
)

const (
	PageKeyPrefix    = "search:page:"
	SectionKeyPrefix = "search:section:"
)

type Repository struct {
	client *goredis.Client
}

var _ storage.PageRepository = (*Repository)(nil)
var _ storage.SectionRepository = (*Repository)(nil)

func NewRepository(client *goredis.Client) *Repository {
	return &Repository{client: client}
}

func (r *Repository) SavePage(ctx context.Context, page documents.Page) error {
	if page.ID == "" {
		return errors.New("page ID is required")
	}

	return r.saveJSON(ctx, PageKeyPrefix+page.ID, page)
}

func (r *Repository) Page(ctx context.Context, id string) (documents.Page, error) {
	if err := validateID("page", id); err != nil {
		return documents.Page{}, err
	}

	var page documents.Page
	if err := r.loadJSON(ctx, PageKeyPrefix+id, &page); err != nil {
		return documents.Page{}, err
	}

	return page, nil
}

func (r *Repository) DeletePage(ctx context.Context, id string) error {
	if err := validateID("page", id); err != nil {
		return err
	}

	return r.client.Del(ctx, PageKeyPrefix+id).Err()
}

func (r *Repository) SaveSection(ctx context.Context, section documents.Section) error {
	if section.ID == "" {
		return errors.New("section ID is required")
	}

	return r.saveJSON(ctx, SectionKeyPrefix+section.ID, section)
}

func (r *Repository) Section(ctx context.Context, id string) (documents.Section, error) {
	if err := validateID("section", id); err != nil {
		return documents.Section{}, err
	}

	var section documents.Section
	if err := r.loadJSON(ctx, SectionKeyPrefix+id, &section); err != nil {
		return documents.Section{}, err
	}

	return section, nil
}

func (r *Repository) DeleteSection(ctx context.Context, id string) error {
	if err := validateID("section", id); err != nil {
		return err
	}

	return r.client.Del(ctx, SectionKeyPrefix+id).Err()
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
