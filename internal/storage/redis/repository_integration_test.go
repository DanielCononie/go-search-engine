package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	searchdomain "github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/search"
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

func TestReplacePageIntegration(t *testing.T) {
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
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	page := documents.Page{
		ID:         "replace-" + suffix,
		Title:      "Replace example",
		SectionIDs: []string{"section-a-" + suffix, "section-b-" + suffix},
	}
	sectionA := documents.Section{
		ID:          page.SectionIDs[0],
		PageID:      page.ID,
		Heading:     "A",
		Text:        "Alpha",
		ContentHash: "sha256:alpha",
	}
	sectionB := documents.Section{
		ID:          page.SectionIDs[1],
		PageID:      page.ID,
		Heading:     "B",
		Text:        "Beta",
		ContentHash: "sha256:beta",
	}
	t.Cleanup(func() {
		_ = repository.DeletePage(context.Background(), page.ID)
		_ = repository.DeleteSection(context.Background(), sectionA.ID)
		_ = repository.DeleteSection(context.Background(), sectionB.ID)
		_ = repository.DeleteSection(context.Background(), "section-c-"+suffix)
		_ = client.Del(
			context.Background(),
			CrawlFailureKeyPrefix+page.ID,
		).Err()
	})

	if err := repository.ReplacePage(ctx, page, []documents.Section{sectionA, sectionB}); err != nil {
		t.Fatal(err)
	}
	if err := client.Do(
		ctx,
		"JSON.SET",
		SectionKeyPrefix+sectionA.ID,
		"$.embedding_model",
		`"keep-me"`,
	).Err(); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveCrawlFailure(ctx, documents.CrawlFailure{
		PageID: page.ID,
		URL:    "https://example.com",
		Error:  "temporary",
	}); err != nil {
		t.Fatal(err)
	}

	sectionC := documents.Section{
		ID:          "section-c-" + suffix,
		PageID:      page.ID,
		Heading:     "C",
		Text:        "Gamma",
		ContentHash: "sha256:gamma",
	}
	sectionA.Ordinal = 2
	page.SectionIDs = []string{sectionA.ID, sectionC.ID}
	if err := repository.ReplacePage(ctx, page, []documents.Section{sectionA, sectionC}); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.Section(ctx, sectionB.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("stale section error = %v, want ErrNotFound", err)
	}
	storedA, err := repository.Section(ctx, sectionA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedA.EmbeddingModel != "keep-me" {
		t.Fatalf("section embedding was not preserved: %#v", storedA)
	}
	if storedA.Ordinal != sectionA.Ordinal {
		t.Fatalf("section metadata was not updated: %#v", storedA)
	}
	if _, err := repository.Section(ctx, sectionC.ID); err != nil {
		t.Fatal(err)
	}
	exists, err := client.Exists(ctx, CrawlFailureKeyPrefix+page.ID).Result()
	if err != nil {
		t.Fatal(err)
	}
	if exists != 0 {
		t.Fatal("successful replacement must clear the crawl failure")
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
	diagnostics, err := manager.Diagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.Alias != indexAlias ||
		diagnostics.PhysicalIndex != indexName ||
		diagnostics.IndexedSections != 0 {
		t.Fatalf("index diagnostics = %#v", diagnostics)
	}
}

func TestSearchBackendIntegration(t *testing.T) {
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
	indexName := "test_search_sections_" + suffix
	aliasName := "test_search_sections_alias_" + suffix
	pagePrefix := "test:search:page:" + suffix + ":"
	sectionPrefix := "test:search:section:" + suffix + ":"
	failurePrefix := "test:search:failure:" + suffix + ":"
	indexManager := newIndexManager(
		client,
		indexName,
		aliasName,
		sectionPrefix,
	)
	if err := indexManager.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Do(context.Background(), "FT.ALIASDEL", aliasName).Err()
		_ = client.Do(context.Background(), "FT.DROPINDEX", indexName, "DD").Err()
	})

	repository := newRepository(
		client,
		pagePrefix,
		sectionPrefix,
		failurePrefix,
	)
	pages := []documents.Page{
		{
			ID:         "page-one",
			URL:        "https://example.com/one",
			Title:      "Exact phrase",
			Site:       "example.com",
			Language:   "en",
			SectionIDs: []string{"section-one"},
		},
		{
			ID:         "page-two",
			URL:        "https://other.example/two",
			Title:      "Separate terms",
			Site:       "other.example",
			Language:   "en",
			SectionIDs: []string{"section-two"},
		},
	}
	sections := []documents.Section{
		{
			ID:          "section-one",
			PageID:      "page-one",
			URL:         "https://example.com/one#details",
			PageTitle:   "Exact phrase",
			Site:        "example.com",
			Language:    "en",
			Heading:     "Details",
			Text:        "The infinity stones are a cosmicartifactkeyword.",
			ContentHash: "sha256:one",
		},
		{
			ID:          "section-two",
			PageID:      "page-two",
			URL:         "https://other.example/two#details",
			PageTitle:   "Separate terms",
			Site:        "other.example",
			Language:    "en",
			Heading:     "Details",
			Text:        "Infinity is separate from stones but both describe a cosmicartifactkeyword.",
			ContentHash: "sha256:two",
		},
	}
	for index := range pages {
		if err := repository.ReplacePage(ctx, pages[index], sections[index:index+1]); err != nil {
			t.Fatal(err)
		}
	}
	corpus, err := repository.CorpusDiagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if corpus.Pages != 2 || corpus.ExpectedSections != 2 || corpus.Version == "" {
		t.Fatalf("corpus diagnostics = %#v", corpus)
	}

	backend := newSearchBackend(client, aliasName)
	exact := waitForSearchResults(t, ctx, backend, searchdomain.Request{
		Query: `"infinity stones"`,
		Limit: 10,
	})
	if exact.Total != 1 || exact.Results[0].PageID != "page-one" {
		t.Fatalf("exact phrase results = %#v", exact)
	}

	filtered, err := backend.Search(ctx, searchdomain.Request{
		Query: "cosmicartifactkeyword",
		Site:  "other.example",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || filtered.Results[0].PageID != "page-two" {
		t.Fatalf("filtered results = %#v", filtered)
	}

	misspelled, err := backend.Search(ctx, searchdomain.Request{
		Query: "cosmicartifactkeywrd",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if misspelled.Total != 0 ||
		len(misspelled.Suggestions) != 1 ||
		len(misspelled.Suggestions[0].Candidates) == 0 ||
		misspelled.Suggestions[0].Candidates[0] != "cosmicartifactkeyword" {
		t.Fatalf("misspelled results = %#v", misspelled)
	}

	secondPage, err := backend.Search(ctx, searchdomain.Request{
		Query:  "cosmicartifactkeyword",
		Limit:  1,
		Offset: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if secondPage.Total != 2 || len(secondPage.Results) != 1 {
		t.Fatalf("paginated results = %#v", secondPage)
	}
}

func waitForSearchResults(
	t *testing.T,
	ctx context.Context,
	backend *SearchBackend,
	request searchdomain.Request,
) searchdomain.ResultPage {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		result, err := backend.Search(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if result.Total > 0 {
			return result
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for Redis Search indexing")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
