package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
)

const maximumErrorBodyBytes = 4096

type OpenAIClient struct {
	httpClient     *http.Client
	endpoint       string
	apiKey         string
	model          string
	dimensions     int
	sendDimensions bool
	maxRetries     int
}

type embeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewOpenAIClient(embeddingConfig config.Embedding) *OpenAIClient {
	return &OpenAIClient{
		httpClient: &http.Client{
			Timeout: embeddingConfig.Timeout,
		},
		endpoint:       embeddingConfig.BaseURL + "/embeddings",
		apiKey:         embeddingConfig.APIKey,
		model:          embeddingConfig.Model,
		dimensions:     embeddingConfig.Dimensions,
		sendDimensions: embeddingConfig.SendDimensions,
		maxRetries:     embeddingConfig.MaxRetries,
	}
}

func (c *OpenAIClient) Embed(
	ctx context.Context,
	inputs []string,
) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("%w: no inputs", ErrInvalidResponse)
	}
	for _, input := range inputs {
		if input == "" {
			return nil, fmt.Errorf("%w: empty input", ErrInvalidResponse)
		}
	}

	requestBody := embeddingRequest{
		Model: c.model,
		Input: inputs,
	}
	if c.sendDimensions {
		requestBody.Dimensions = c.dimensions
	}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("encode embedding request: %w", err)
	}

	for attempt := 0; ; attempt++ {
		vectors, retry, err := c.request(ctx, encoded, len(inputs))
		if err == nil || !retry || attempt == c.maxRetries {
			return vectors, err
		}
		delay := time.Duration(1<<attempt) * 200 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *OpenAIClient) request(
	ctx context.Context,
	body []byte,
	inputCount int,
) ([][]float32, bool, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, false, fmt.Errorf("create embedding request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, false, err
		}
		return nil, true, fmt.Errorf("request embeddings: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(
			response.Body,
			maximumErrorBodyBytes,
		))
		return nil,
			response.StatusCode == http.StatusTooManyRequests ||
				response.StatusCode >= http.StatusInternalServerError,
			fmt.Errorf(
				"embedding endpoint returned %s: %s",
				response.Status,
				string(message),
			)
	}

	var decoded embeddingResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, false, fmt.Errorf(
			"%w: decode response: %v",
			ErrInvalidResponse,
			err,
		)
	}
	if decoded.Error.Message != "" {
		return nil, false, fmt.Errorf(
			"embedding endpoint error: %s",
			decoded.Error.Message,
		)
	}
	if len(decoded.Data) != inputCount {
		return nil, false, fmt.Errorf(
			"%w: received %d vectors for %d inputs",
			ErrInvalidResponse,
			len(decoded.Data),
			inputCount,
		)
	}

	vectors := make([][]float32, inputCount)
	for _, item := range decoded.Data {
		if item.Index < 0 || item.Index >= inputCount ||
			vectors[item.Index] != nil {
			return nil, false, fmt.Errorf(
				"%w: invalid response index %d",
				ErrInvalidResponse,
				item.Index,
			)
		}
		if len(item.Embedding) != c.dimensions {
			return nil, false, fmt.Errorf(
				"%w: vector has %d dimensions, expected %d",
				ErrInvalidResponse,
				len(item.Embedding),
				c.dimensions,
			)
		}
		vectors[item.Index] = item.Embedding
	}

	return vectors, false, nil
}
