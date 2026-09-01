package crawler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testOptions() Options {
	options := DefaultOptions()
	options.HostInterval = 0
	options.RetryBackoff = 0
	return options
}

func TestFetchAllPreservesInputOrderAndBoundsConcurrency(t *testing.T) {
	var active atomic.Int64
	var maximum atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		_, _ = response.Write([]byte(request.URL.Path))
	}))
	defer server.Close()

	options := testOptions()
	options.Concurrency = 2
	crawler, err := New(options)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{server.URL + "/one", server.URL + "/two", server.URL + "/three"}
	results := crawler.FetchAll(context.Background(), urls)
	expectedBodies := []string{"/one", "/two", "/three"}

	if maximum.Load() > 2 {
		t.Fatalf("maximum concurrency = %d, want at most 2", maximum.Load())
	}
	for index, result := range results {
		if result.URL != urls[index] || result.HTML != expectedBodies[index] {
			t.Fatalf("result %d = %#v", index, result)
		}
	}
}

func TestFetchRetriesServerFailure(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			response.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = response.Write([]byte("ok"))
	}))
	defer server.Close()

	options := testOptions()
	options.MaxRetries = 1
	crawler, err := New(options)
	if err != nil {
		t.Fatal(err)
	}

	result := crawler.Fetch(context.Background(), server.URL)
	if result.Err != nil || result.HTML != "ok" || requests.Load() != 2 {
		t.Fatalf("result = %#v, requests = %d", result, requests.Load())
	}
}

func TestFetchRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte("too large"))
	}))
	defer server.Close()

	options := testOptions()
	options.MaxResponseSize = 3
	crawler, err := New(options)
	if err != nil {
		t.Fatal(err)
	}

	result := crawler.Fetch(context.Background(), server.URL)
	if !errors.Is(result.Err, ErrResponseTooLarge) {
		t.Fatalf("error = %v, want ErrResponseTooLarge", result.Err)
	}
}

func TestFetchHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = response.Write([]byte("late"))
	}))
	defer server.Close()

	options := testOptions()
	crawler, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result := crawler.Fetch(ctx, server.URL)
	if !errors.Is(result.Err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", result.Err)
	}
}
