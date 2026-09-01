package redis

import (
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestSemanticIndexVersion(t *testing.T) {
	if got := semanticIndexVersion("search_semantic_sections_v2"); got != "v2" {
		t.Fatalf("version = %q", got)
	}
	if got := semanticIndexVersion("other_v2"); got != "" {
		t.Fatalf("version = %q", got)
	}
}

func TestSemanticIndexManagerValidatesVersionedSchema(t *testing.T) {
	manager := &SemanticIndexManager{
		indexName:     "search_semantic_sections_v1",
		sectionPrefix: "search:semantic-section:v1:",
		dimensions:    3,
	}
	info := goredis.FTInfoResult{
		IndexDefinition: goredis.IndexDefinition{
			Prefixes: []string{"search:semantic-section:v1:"},
		},
		Attributes: []goredis.FTAttribute{{
			Attribute:      "embedding",
			Type:           "VECTOR",
			Algorithm:      "FLAT",
			DataType:       "FLOAT32",
			Dim:            3,
			DistanceMetric: "COSINE",
		}},
	}
	if err := manager.validateIndex(info); err != nil {
		t.Fatal(err)
	}
	info.Attributes[0].Dim = 4
	if err := manager.validateIndex(info); err == nil {
		t.Fatal("expected incompatible dimensions error")
	}
}
