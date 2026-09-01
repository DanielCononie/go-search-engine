package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
)

func TestOpenAIClientEmbedsInputsInRequestOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		var body embeddingRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "test-model" ||
			body.Dimensions != 2 ||
			!reflect.DeepEqual(body.Input, []string{"first", "second"}) {
			t.Fatalf("unexpected request: %#v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"data": [
				{"index": 1, "embedding": [0, 1]},
				{"index": 0, "embedding": [1, 0]}
			]
		}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(config.Embedding{
		BaseURL:        server.URL + "/v1",
		APIKey:         "secret",
		Model:          "test-model",
		Dimensions:     2,
		Timeout:        time.Second,
		SendDimensions: true,
	})
	vectors, err := client.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(vectors, [][]float32{{1, 0}, {0, 1}}) {
		t.Fatalf("vectors = %#v", vectors)
	}
}

func TestOpenAIClientRetriesTransientErrors(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		if attempts.Add(1) == 1 {
			http.Error(writer, "retry", http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write([]byte(
			`{"data":[{"index":0,"embedding":[1,0]}]}`,
		))
	}))
	defer server.Close()

	client := NewOpenAIClient(config.Embedding{
		BaseURL:    server.URL,
		Model:      "test",
		Dimensions: 2,
		Timeout:    time.Second,
		MaxRetries: 1,
	})
	if _, err := client.Embed(context.Background(), []string{"input"}); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d", attempts.Load())
	}
}

func TestOpenAIClientRejectsWrongDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, _ = writer.Write([]byte(
			`{"data":[{"index":0,"embedding":[1]}]}`,
		))
	}))
	defer server.Close()

	client := NewOpenAIClient(config.Embedding{
		BaseURL:    server.URL,
		Model:      "test",
		Dimensions: 2,
		Timeout:    time.Second,
	})
	if _, err := client.Embed(context.Background(), []string{"input"}); err == nil {
		t.Fatal("expected dimension error")
	}
}
