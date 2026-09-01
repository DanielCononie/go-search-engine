package parser

import (
	"testing"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
)

func TestParseDocumentCreatesStructuredSections(t *testing.T) {
	fetchedAt := time.Date(2026, time.September, 1, 1, 2, 3, 0, time.UTC)
	html := `
		<html lang="en">
			<head>
				<title>Example page</title>
				<link rel="canonical" href="/example#ignored">
			</head>
			<body>
				<nav>Navigation noise</nav>
				<main>
					<p>Introductory text.</p>
					<h1 id="history">History</h1>
					<p>History text.</p>
					<h2>Early years</h2>
					<p>Early details.</p>
					<pre><code>go test ./...</code></pre>
					<h2>Early years</h2>
					<p>Later details.</p>
				</main>
			</body>
		</html>`

	parsed, err := ParseDocument(models.FetchResult{
		URL:        "https://Example.com/source#fragment",
		HTML:       html,
		StatusCode: 200,
		FetchedAt:  fetchedAt,
	})
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Page.CanonicalURL != "https://example.com/example" {
		t.Fatalf("canonical URL = %q", parsed.Page.CanonicalURL)
	}
	if parsed.Page.Site != "example.com" || parsed.Page.Language != "en" {
		t.Fatalf("site = %q, language = %q", parsed.Page.Site, parsed.Page.Language)
	}
	if parsed.Page.CrawledAt != fetchedAt.Unix() || parsed.Page.ContentHash == "" {
		t.Fatalf("page metadata = %#v", parsed.Page)
	}
	if len(parsed.Sections) != 4 {
		t.Fatalf("section count = %d, want 4", len(parsed.Sections))
	}
	if parsed.Sections[0].Heading != "Overview" {
		t.Fatalf("first heading = %q", parsed.Sections[0].Heading)
	}
	if parsed.Sections[2].HeadingPath != "History > Early years" {
		t.Fatalf("heading path = %q", parsed.Sections[2].HeadingPath)
	}
	if parsed.Sections[2].Text != "Early details. go test ./..." {
		t.Fatalf("code section text = %q", parsed.Sections[2].Text)
	}
	if parsed.Sections[2].ID == parsed.Sections[3].ID {
		t.Fatal("duplicate headings must have distinct section IDs")
	}
	if len(parsed.Page.SectionIDs) != len(parsed.Sections) {
		t.Fatalf("page section IDs = %d", len(parsed.Page.SectionIDs))
	}
}

func TestParseDocumentUsesArticleBeforeMain(t *testing.T) {
	html := `
		<html>
			<head><title>Priority</title></head>
			<body>
				<article><p>Article text.</p></article>
				<main><p>Main text.</p></main>
			</body>
		</html>`

	parsed, err := ParseDocument(models.FetchResult{
		URL:  "https://example.com/priority",
		HTML: html,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Sections) != 1 || parsed.Sections[0].Text != "Article text." {
		t.Fatalf("sections = %#v", parsed.Sections)
	}
}

func TestParseDocumentAllowsEmptyContent(t *testing.T) {
	parsed, err := ParseDocument(models.FetchResult{
		URL:  "https://example.com/empty",
		HTML: "<html><head><title>Empty</title></head><body></body></html>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Sections) != 0 || len(parsed.Page.SectionIDs) != 0 {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestParseDocumentIDsAreStable(t *testing.T) {
	result := models.FetchResult{
		URL:       "https://example.com/page#one",
		HTML:      "<main><h1>Heading</h1><p>Text.</p></main>",
		FetchedAt: time.Unix(100, 0),
	}
	first, err := ParseDocument(result)
	if err != nil {
		t.Fatal(err)
	}
	result.URL = "https://example.com/page#two"
	second, err := ParseDocument(result)
	if err != nil {
		t.Fatal(err)
	}

	if first.Page.ID != second.Page.ID || first.Sections[0].ID != second.Sections[0].ID {
		t.Fatal("fragment changes must not change document IDs")
	}
}
