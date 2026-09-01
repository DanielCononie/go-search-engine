package redis

import (
	"encoding/binary"
	"math"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestGroupSemanticDocumentsUsesCosineSimilarity(t *testing.T) {
	results, err := groupSemanticDocuments([]goredis.Document{
		{
			ID: "section-2",
			Fields: map[string]string{
				"id":              "section-2",
				"page_id":         "page-2",
				"url":             "https://example.com/two#part",
				"page_title":      "Two",
				"heading":         "Part",
				"text":            "second result",
				"vector_distance": "0.4",
			},
		},
		{
			ID: "section-1",
			Fields: map[string]string{
				"id":              "section-1",
				"page_id":         "page-1",
				"url":             "https://example.com/one#part",
				"page_title":      "One",
				"heading":         "Part",
				"text":            "first result",
				"vector_distance": "0.1",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 ||
		results[0].PageID != "page-1" ||
		math.Abs(results[0].Score-0.9) > 0.0001 ||
		results[0].URL != "https://example.com/one" ||
		results[0].MatchedSections[0].SectionID != "section-1" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestEncodeFloat32VectorUsesNativeEndian(t *testing.T) {
	encoded := encodeFloat32Vector([]float32{1.5, -2})
	if len(encoded) != 8 ||
		math.Float32frombits(binary.NativeEndian.Uint32(encoded[:4])) != 1.5 ||
		math.Float32frombits(binary.NativeEndian.Uint32(encoded[4:])) != -2 {
		t.Fatalf("unexpected encoded vector: %#v", encoded)
	}
}
