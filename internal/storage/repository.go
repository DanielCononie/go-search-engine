package storage

import (
	"context"
	"errors"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
)

var ErrNotFound = errors.New("document not found")

type PageRepository interface {
	SavePage(ctx context.Context, page documents.Page) error
	Page(ctx context.Context, id string) (documents.Page, error)
	DeletePage(ctx context.Context, id string) error
}

type SectionRepository interface {
	SaveSection(ctx context.Context, section documents.Section) error
	Section(ctx context.Context, id string) (documents.Section, error)
	DeleteSection(ctx context.Context, id string) error
}

type IndexManager interface {
	Ensure(ctx context.Context) error
	Check(ctx context.Context) error
}
