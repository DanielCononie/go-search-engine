package redis

import (
	"context"
	"strings"
	"testing"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	goredis "github.com/redis/go-redis/v9"
)

func TestReplacePageValidatesSectionIDs(t *testing.T) {
	repository := NewRepository(goredis.NewClient(&goredis.Options{
		Addr: "127.0.0.1:1",
	}))
	t.Cleanup(func() {
		_ = repository.client.Close()
	})

	err := repository.ReplacePage(
		context.Background(),
		documents.Page{ID: "page", SectionIDs: []string{"expected"}},
		[]documents.Section{{ID: "different", PageID: "page"}},
	)
	if err == nil || !strings.Contains(err.Error(), "section IDs") {
		t.Fatalf("error = %v", err)
	}
}
