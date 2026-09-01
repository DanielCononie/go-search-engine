package search

import (
	"errors"
	"strings"
	"testing"
)

func TestCompileLexicalQuery(t *testing.T) {
	query, err := CompileLexicalQuery(
		`Iron-Man "Infinity Stones" @text:{injected}`,
		"EN.Wikipedia.org",
		"en-US",
	)
	if err != nil {
		t.Fatal(err)
	}

	expected := `iron man "infinity stones" text injected @site:{en\.wikipedia\.org} @language:{en\-us}`
	if query != expected {
		t.Fatalf("query = %q, want %q", query, expected)
	}
}

func TestCompileLexicalQueryRejectsMalformedInput(t *testing.T) {
	testCases := []struct {
		name     string
		query    string
		site     string
		language string
	}{
		{name: "unmatched quote", query: `"infinity stones`},
		{name: "empty", query: `@#$`},
		{name: "too long", query: strings.Repeat("a", MaxQueryRunes+1)},
		{name: "too many terms", query: strings.Repeat("term ", MaxQueryTerms+1)},
		{name: "invalid site", query: "iron", site: "site} @text:{*"},
		{name: "invalid language", query: "iron", language: "en|fr"},
		{name: "punctuation-only site", query: "iron", site: "..."},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := CompileLexicalQuery(
				testCase.query,
				testCase.site,
				testCase.language,
			)
			if !errors.Is(err, ErrInvalidQuery) {
				t.Fatalf("error = %v, want ErrInvalidQuery", err)
			}
		})
	}
}
