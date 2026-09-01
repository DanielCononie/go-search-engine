package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultEmbeddingBaseURL      = "https://api.openai.com/v1"
	defaultEmbeddingIndexVersion = "v1"
	defaultEmbeddingTimeout      = 30 * time.Second
	defaultEmbeddingMaxRetries   = 2
	defaultEmbeddingMaxRunes     = 12000
	defaultEmbeddingOverlapRunes = 500
)

type Embedding struct {
	Enabled           bool
	BaseURL           string
	APIKey            string
	Model             string
	Version           string
	Dimensions        int
	IndexVersion      string
	Timeout           time.Duration
	MaxRetries        int
	MaxInputRunes     int
	InputOverlapRunes int
	SendDimensions    bool
}

func LoadEmbedding() (Embedding, error) {
	model := strings.TrimSpace(os.Getenv("EMBEDDING_MODEL"))
	if model == "" {
		if os.Getenv("EMBEDDING_DIMENSIONS") != "" ||
			os.Getenv("EMBEDDING_VERSION") != "" {
			return Embedding{}, fmt.Errorf(
				"EMBEDDING_MODEL is required when embedding configuration is present",
			)
		}
		return Embedding{}, nil
	}

	dimensions, err := intFromEnvironment("EMBEDDING_DIMENSIONS", 0)
	if err != nil {
		return Embedding{}, err
	}
	if dimensions < 1 {
		return Embedding{}, fmt.Errorf("EMBEDDING_DIMENSIONS must be positive")
	}
	timeout, err := durationFromEnvironment(
		"EMBEDDING_TIMEOUT",
		defaultEmbeddingTimeout,
	)
	if err != nil {
		return Embedding{}, err
	}
	maxRetries, err := intFromEnvironment(
		"EMBEDDING_MAX_RETRIES",
		defaultEmbeddingMaxRetries,
	)
	if err != nil {
		return Embedding{}, err
	}
	maxInputRunes, err := intFromEnvironment(
		"EMBEDDING_MAX_INPUT_RUNES",
		defaultEmbeddingMaxRunes,
	)
	if err != nil {
		return Embedding{}, err
	}
	overlapRunes, err := intFromEnvironment(
		"EMBEDDING_INPUT_OVERLAP_RUNES",
		defaultEmbeddingOverlapRunes,
	)
	if err != nil {
		return Embedding{}, err
	}
	sendDimensions, err := boolFromEnvironment(
		"EMBEDDING_SEND_DIMENSIONS",
		false,
	)
	if err != nil {
		return Embedding{}, err
	}
	if maxRetries < 0 {
		return Embedding{}, fmt.Errorf("EMBEDDING_MAX_RETRIES must be non-negative")
	}
	if maxInputRunes < 1 {
		return Embedding{}, fmt.Errorf("EMBEDDING_MAX_INPUT_RUNES must be positive")
	}
	if overlapRunes < 0 || overlapRunes >= maxInputRunes {
		return Embedding{}, fmt.Errorf(
			"EMBEDDING_INPUT_OVERLAP_RUNES must be non-negative and smaller than EMBEDDING_MAX_INPUT_RUNES",
		)
	}

	baseURL := strings.TrimSpace(os.Getenv("EMBEDDING_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultEmbeddingBaseURL
	}
	parsedURL, err := url.Parse(baseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" ||
		(parsedURL.Scheme != "http" && parsedURL.Scheme != "https") ||
		parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return Embedding{}, fmt.Errorf("EMBEDDING_BASE_URL must be an absolute HTTP URL")
	}

	version := strings.TrimSpace(os.Getenv("EMBEDDING_VERSION"))
	if version == "" {
		version = model
	}
	indexVersion := strings.TrimSpace(os.Getenv("EMBEDDING_INDEX_VERSION"))
	if indexVersion == "" {
		indexVersion = defaultEmbeddingIndexVersion
	}
	for _, character := range indexVersion {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') &&
			character != '-' && character != '_' {
			return Embedding{}, fmt.Errorf(
				"EMBEDDING_INDEX_VERSION may contain only letters, numbers, hyphens, and underscores",
			)
		}
	}

	return Embedding{
		Enabled:           true,
		BaseURL:           strings.TrimRight(baseURL, "/"),
		APIKey:            os.Getenv("EMBEDDING_API_KEY"),
		Model:             model,
		Version:           version,
		Dimensions:        dimensions,
		IndexVersion:      indexVersion,
		Timeout:           timeout,
		MaxRetries:        maxRetries,
		MaxInputRunes:     maxInputRunes,
		InputOverlapRunes: overlapRunes,
		SendDimensions:    sendDimensions,
	}, nil
}
