package config

import (
	"testing"
	"time"
)

func clearEmbeddingEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"EMBEDDING_BASE_URL",
		"EMBEDDING_API_KEY",
		"EMBEDDING_MODEL",
		"EMBEDDING_VERSION",
		"EMBEDDING_DIMENSIONS",
		"EMBEDDING_INDEX_VERSION",
		"EMBEDDING_TIMEOUT",
		"EMBEDDING_MAX_RETRIES",
		"EMBEDDING_MAX_INPUT_RUNES",
		"EMBEDDING_INPUT_OVERLAP_RUNES",
		"EMBEDDING_SEND_DIMENSIONS",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadEmbeddingDisabledByDefault(t *testing.T) {
	clearEmbeddingEnvironment(t)

	embeddingConfig, err := LoadEmbedding()
	if err != nil {
		t.Fatal(err)
	}
	if embeddingConfig.Enabled {
		t.Fatal("embedding configuration should be disabled")
	}
}

func TestLoadEmbeddingEnvironment(t *testing.T) {
	clearEmbeddingEnvironment(t)
	t.Setenv("EMBEDDING_BASE_URL", "http://embedding.example/v1/")
	t.Setenv("EMBEDDING_API_KEY", "secret")
	t.Setenv("EMBEDDING_MODEL", "text-embedding")
	t.Setenv("EMBEDDING_VERSION", "2026-09")
	t.Setenv("EMBEDDING_DIMENSIONS", "768")
	t.Setenv("EMBEDDING_INDEX_VERSION", "v2")
	t.Setenv("EMBEDDING_TIMEOUT", "12s")
	t.Setenv("EMBEDDING_MAX_RETRIES", "4")
	t.Setenv("EMBEDDING_MAX_INPUT_RUNES", "8000")
	t.Setenv("EMBEDDING_INPUT_OVERLAP_RUNES", "200")
	t.Setenv("EMBEDDING_SEND_DIMENSIONS", "true")

	embeddingConfig, err := LoadEmbedding()
	if err != nil {
		t.Fatal(err)
	}
	if !embeddingConfig.Enabled ||
		embeddingConfig.BaseURL != "http://embedding.example/v1" ||
		embeddingConfig.APIKey != "secret" ||
		embeddingConfig.Model != "text-embedding" ||
		embeddingConfig.Version != "2026-09" ||
		embeddingConfig.Dimensions != 768 ||
		embeddingConfig.IndexVersion != "v2" ||
		embeddingConfig.Timeout != 12*time.Second ||
		embeddingConfig.MaxRetries != 4 ||
		embeddingConfig.MaxInputRunes != 8000 ||
		embeddingConfig.InputOverlapRunes != 200 ||
		!embeddingConfig.SendDimensions {
		t.Fatalf("unexpected embedding configuration: %#v", embeddingConfig)
	}
}

func TestLoadEmbeddingRejectsPartialOrInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
	}{
		{
			name: "dimensions without model",
			values: map[string]string{
				"EMBEDDING_DIMENSIONS": "3",
			},
		},
		{
			name: "model without dimensions",
			values: map[string]string{
				"EMBEDDING_MODEL": "test",
			},
		},
		{
			name: "invalid URL",
			values: map[string]string{
				"EMBEDDING_MODEL":      "test",
				"EMBEDDING_DIMENSIONS": "3",
				"EMBEDDING_BASE_URL":   "redis://localhost",
			},
		},
		{
			name: "URL with query parameters",
			values: map[string]string{
				"EMBEDDING_MODEL":      "test",
				"EMBEDDING_DIMENSIONS": "3",
				"EMBEDDING_BASE_URL":   "https://example.com/v1?key=value",
			},
		},
		{
			name: "overlap exceeds window",
			values: map[string]string{
				"EMBEDDING_MODEL":               "test",
				"EMBEDDING_DIMENSIONS":          "3",
				"EMBEDDING_MAX_INPUT_RUNES":     "10",
				"EMBEDDING_INPUT_OVERLAP_RUNES": "10",
			},
		},
		{
			name: "unsafe index version",
			values: map[string]string{
				"EMBEDDING_MODEL":         "test",
				"EMBEDDING_DIMENSIONS":    "3",
				"EMBEDDING_INDEX_VERSION": "v1 alias",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearEmbeddingEnvironment(t)
			for name, value := range test.values {
				t.Setenv(name, value)
			}
			if _, err := LoadEmbedding(); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
