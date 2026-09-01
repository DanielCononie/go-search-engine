package models

import "time"

type Page struct {
	URL     string
	Title   string
	Content string
	Tokens  []string
}

type SearchResult struct {
	PageID          string           `json:"page_id,omitempty"`
	URL             string           `json:"url"`
	Title           string           `json:"title"`
	Score           float64          `json:"score"`
	MatchedSections []MatchedSection `json:"matched_sections,omitempty"`
}

type MatchedSection struct {
	SectionID string  `json:"section_id"`
	Heading   string  `json:"heading"`
	Snippet   string  `json:"snippet"`
	Score     float64 `json:"score"`
}

type FetchResult struct {
	URL        string
	HTML       string
	StatusCode int
	FetchedAt  time.Time
	Err        error
}
