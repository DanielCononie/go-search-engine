package redis

import "testing"

func TestCorpusVersionIsDeterministic(t *testing.T) {
	first := corpusVersion([]string{"page-a:hash-a", "page-b:hash-b"})
	second := corpusVersion([]string{"page-a:hash-a", "page-b:hash-b"})
	if first != second {
		t.Fatalf("versions differ: %q and %q", first, second)
	}
	if first == corpusVersion([]string{"page-a:hash-changed", "page-b:hash-b"}) {
		t.Fatal("content hash changes must change the corpus version")
	}
}

func TestIndexSchemaVersion(t *testing.T) {
	if got := indexSchemaVersion("search_sections_v2"); got != "v2" {
		t.Fatalf("schema version = %q, want v2", got)
	}
	if got := indexSchemaVersion("custom_index"); got != "" {
		t.Fatalf("custom schema version = %q, want empty", got)
	}
}
