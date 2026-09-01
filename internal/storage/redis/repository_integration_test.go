package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage"
	goredis "github.com/redis/go-redis/v9"
)

func TestRepositoryIntegration(t *testing.T) {
	address := os.Getenv("REDIS_INTEGRATION_ADDR")
	if address == "" {
		t.Skip("REDIS_INTEGRATION_ADDR is not set")
	}

	client := goredis.NewClient(&goredis.Options{Addr: address, Protocol: 2})
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repository := NewRepository(client)
	id := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	page := documents.Page{
		ID:           id,
		URL:          "https://example.com/page",
		CanonicalURL: "https://example.com/page",
		Title:        "Example",
		SectionIDs:   []string{id + "-section"},
	}

	t.Cleanup(func() {
		_ = repository.DeletePage(context.Background(), id)
	})

	if err := repository.SavePage(ctx, page); err != nil {
		t.Fatal(err)
	}

	stored, err := repository.Page(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != page.ID || stored.Title != page.Title {
		t.Fatalf("stored page = %#v", stored)
	}

	if err := repository.DeletePage(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Page(ctx, id); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}

	section := documents.Section{
		ID:        id + "-section",
		PageID:    id,
		PageTitle: "Example",
		Heading:   "Overview",
		Text:      "Example section text.",
	}
	t.Cleanup(func() {
		_ = repository.DeleteSection(context.Background(), section.ID)
	})

	if err := repository.SaveSection(ctx, section); err != nil {
		t.Fatal(err)
	}
	storedSection, err := repository.Section(ctx, section.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedSection.ID != section.ID || storedSection.PageID != page.ID {
		t.Fatalf("stored section = %#v", storedSection)
	}
}

func TestIndexManagerIntegration(t *testing.T) {
	address := os.Getenv("REDIS_INTEGRATION_ADDR")
	if address == "" {
		t.Skip("REDIS_INTEGRATION_ADDR is not set")
	}

	client := goredis.NewClient(&goredis.Options{Addr: address, Protocol: 2})
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	indexName := "search_sections_integration_" + suffix
	indexAlias := "search_sections_integration_current_" + suffix
	sectionPrefix := "search:integration:" + suffix + ":section:"
	manager := newIndexManager(client, indexName, indexAlias, sectionPrefix)

	t.Cleanup(func() {
		_ = client.Do(context.Background(), "FT.ALIASDEL", indexAlias).Err()
		_ = client.Do(context.Background(), "FT.DROPINDEX", indexName, "DD").Err()
	})

	if err := manager.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.Ensure(ctx); err != nil {
		t.Fatalf("second ensure failed: %v", err)
	}
	if err := manager.Check(ctx); err != nil {
		t.Fatal(err)
	}
}
