package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
)

const (
	defaultConcurrency     = 4
	defaultRequestTimeout  = 10 * time.Second
	defaultMaxResponseSize = 5 << 20
	defaultMaxRetries      = 2
	defaultRetryBackoff    = 250 * time.Millisecond
	defaultHostInterval    = 100 * time.Millisecond
	defaultUserAgent       = "go-search-engine/0.1 (learning project; contact: danielcononie1278@gmail.com)"
)

var ErrResponseTooLarge = errors.New("response exceeds configured size limit")

type Options struct {
	Concurrency     int
	RequestTimeout  time.Duration
	MaxResponseSize int64
	MaxRetries      int
	RetryBackoff    time.Duration
	HostInterval    time.Duration
	UserAgent       string
}

func DefaultOptions() Options {
	return Options{
		Concurrency:     defaultConcurrency,
		RequestTimeout:  defaultRequestTimeout,
		MaxResponseSize: defaultMaxResponseSize,
		MaxRetries:      defaultMaxRetries,
		RetryBackoff:    defaultRetryBackoff,
		HostInterval:    defaultHostInterval,
		UserAgent:       defaultUserAgent,
	}
}

type Crawler struct {
	options Options
	client  *http.Client

	hostMutex    sync.Mutex
	nextHostCall map[string]time.Time
}

func New(options Options) (*Crawler, error) {
	if options.Concurrency < 1 {
		return nil, errors.New("crawler concurrency must be positive")
	}
	if options.RequestTimeout <= 0 {
		return nil, errors.New("crawler request timeout must be positive")
	}
	if options.MaxResponseSize < 1 {
		return nil, errors.New("crawler response size limit must be positive")
	}
	if options.MaxRetries < 0 {
		return nil, errors.New("crawler retries must be non-negative")
	}
	if options.RetryBackoff < 0 || options.HostInterval < 0 {
		return nil, errors.New("crawler intervals must be non-negative")
	}
	if options.UserAgent == "" {
		return nil, errors.New("crawler user agent is required")
	}

	return &Crawler{
		options: options,
		client: &http.Client{
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("stopped after 10 redirects")
				}
				return nil
			},
		},
		nextHostCall: map[string]time.Time{},
	}, nil
}

func NewDefault() *Crawler {
	crawler, err := New(DefaultOptions())
	if err != nil {
		panic(err)
	}

	return crawler
}

func (c *Crawler) Fetch(ctx context.Context, rawURL string) models.FetchResult {
	var lastResult models.FetchResult
	var lastErr error
	for attempt := 0; attempt <= c.options.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := wait(ctx, c.options.RetryBackoff*time.Duration(attempt)); err != nil {
				return models.FetchResult{URL: rawURL, FetchedAt: time.Now(), Err: err}
			}
		}
		if err := c.waitForHost(ctx, rawURL); err != nil {
			return models.FetchResult{URL: rawURL, FetchedAt: time.Now(), Err: err}
		}

		lastResult, lastErr = c.fetchOnce(ctx, rawURL)
		if lastErr == nil || !retryable(lastResult.StatusCode, lastErr) {
			lastResult.Err = lastErr
			return lastResult
		}
	}

	lastResult.Err = lastErr
	return lastResult
}

func (c *Crawler) FetchAll(ctx context.Context, urls []string) []models.FetchResult {
	type job struct {
		index int
		url   string
	}

	results := make([]models.FetchResult, len(urls))
	jobs := make(chan job)
	var workers sync.WaitGroup
	workerCount := min(c.options.Concurrency, len(urls))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for next := range jobs {
				results[next.index] = c.Fetch(ctx, next.url)
			}
		}()
	}

	for index, rawURL := range urls {
		select {
		case jobs <- job{index: index, url: rawURL}:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			for remaining := index; remaining < len(urls); remaining++ {
				if results[remaining].URL == "" {
					results[remaining] = models.FetchResult{
						URL:       urls[remaining],
						FetchedAt: time.Now(),
						Err:       ctx.Err(),
					}
				}
			}
			return results
		}
	}
	close(jobs)
	workers.Wait()

	return results
}

func (c *Crawler) fetchOnce(ctx context.Context, rawURL string) (models.FetchResult, error) {
	requestContext, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, rawURL, nil)
	if err != nil {
		return models.FetchResult{URL: rawURL, FetchedAt: time.Now()}, err
	}
	request.Header.Set("User-Agent", c.options.UserAgent)

	response, err := c.client.Do(request)
	if err != nil {
		return models.FetchResult{URL: rawURL, FetchedAt: time.Now()}, err
	}
	defer response.Body.Close()

	result := models.FetchResult{
		URL:        response.Request.URL.String(),
		StatusCode: response.StatusCode,
		FetchedAt:  time.Now(),
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, c.options.MaxResponseSize+1))
	if err != nil {
		return result, err
	}
	if int64(len(body)) > c.options.MaxResponseSize {
		return result, ErrResponseTooLarge
	}
	result.HTML = string(body)

	return result, nil
}

func (c *Crawler) waitForHost(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	host := strings.ToLower(parsed.Hostname())

	c.hostMutex.Lock()
	scheduledAt := time.Now()
	if nextCall := c.nextHostCall[host]; nextCall.After(scheduledAt) {
		scheduledAt = nextCall
	}
	c.nextHostCall[host] = scheduledAt.Add(c.options.HostInterval)
	c.hostMutex.Unlock()

	return wait(ctx, time.Until(scheduledAt))
}

func retryable(statusCode int, err error) bool {
	if err == nil || errors.Is(err, ErrResponseTooLarge) {
		return false
	}

	return statusCode == 0 ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}

func wait(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func FetchURL(rawURL string) (models.FetchResult, error) {
	result := NewDefault().Fetch(context.Background(), rawURL)
	return result, result.Err
}

func FetchURLs(urls []string) []models.FetchResult {
	return NewDefault().FetchAll(context.Background(), urls)
}
