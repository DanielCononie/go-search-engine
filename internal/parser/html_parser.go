package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/pkg/text"
	"github.com/PuerkitoBio/goquery"
)

type ParsedDocument struct {
	Page     documents.Page
	Sections []documents.Section
}

func ParseDocument(result models.FetchResult) (ParsedDocument, error) {
	if result.URL == "" {
		return ParsedDocument{}, errors.New("result URL is required")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(result.HTML))
	if err != nil {
		return ParsedDocument{}, fmt.Errorf("parse HTML: %w", err)
	}

	for _, selector := range []string{
		"script",
		"style",
		"noscript",
		"svg",
		"nav",
		"footer",
		"header",
		"aside",
		"form",
		"button",
		"input",
		"select",
		"textarea",
		"iframe",
		"canvas",
		".mw-editsection",
		".navbox",
		".infobox",
		".metadata",
		".reference",
		".reflist",
		".toc",
	} {
		doc.Find(selector).Remove()
	}

	canonicalURL, err := canonicalURL(doc, result.URL)
	if err != nil {
		return ParsedDocument{}, err
	}
	pageID := stableID(canonicalURL)
	title := normalizedText(doc.Find("title").First().Text())
	language, _ := doc.Find("html").First().Attr("lang")
	language = strings.TrimSpace(language)
	site := siteFromURL(canonicalURL)
	crawledAt := result.FetchedAt
	if crawledAt.IsZero() {
		crawledAt = time.Now()
	}
	statusCode := result.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}

	root := contentRoot(doc)
	sections := parseSections(root, pageID, canonicalURL, title, site, language, crawledAt)
	sectionIDs := make([]string, 0, len(sections))
	contentParts := make([]string, 0, len(sections)+1)
	contentParts = append(contentParts, title)
	for _, section := range sections {
		sectionIDs = append(sectionIDs, section.ID)
		contentParts = append(contentParts, section.Heading, section.Text)
	}

	return ParsedDocument{
		Page: documents.Page{
			ID:           pageID,
			URL:          result.URL,
			CanonicalURL: canonicalURL,
			Title:        title,
			Site:         site,
			Language:     language,
			ContentHash:  contentHash(strings.Join(contentParts, "\n")),
			CrawlStatus:  "ready",
			HTTPStatus:   statusCode,
			CrawledAt:    crawledAt.Unix(),
			SectionIDs:   sectionIDs,
		},
		Sections: sections,
	}, nil
}

func ParseHTML(result models.FetchResult) models.Page {
	parsed, err := ParseDocument(result)
	if err != nil {
		return models.Page{}
	}

	contentParts := make([]string, 0, len(parsed.Sections))
	for _, section := range parsed.Sections {
		contentParts = append(contentParts, section.Heading, section.Text)
	}
	content := normalizedText(strings.Join(contentParts, " "))

	return models.Page{
		URL:     parsed.Page.CanonicalURL,
		Title:   parsed.Page.Title,
		Content: content,
		Tokens:  text.ProcessText(parsed.Page.Title + " " + content),
	}
}

func canonicalURL(doc *goquery.Document, sourceURL string) (string, error) {
	selectedURL := sourceURL
	if canonical, exists := doc.Find(`link[rel="canonical"]`).First().Attr("href"); exists {
		selectedURL = canonical
	}

	base, err := url.Parse(sourceURL)
	if err != nil {
		return "", fmt.Errorf("parse source URL: %w", err)
	}
	parsed, err := url.Parse(selectedURL)
	if err != nil {
		return "", fmt.Errorf("parse canonical URL: %w", err)
	}
	parsed = base.ResolveReference(parsed)
	return normalizeURL(parsed)
}

func PageIdentity(rawURL string) (string, string, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("parse page URL: %w", err)
	}
	canonical, err := normalizeURL(parsed)
	if err != nil {
		return "", "", "", err
	}

	return canonical, stableID(canonical), siteFromURL(canonical), nil
}

func normalizeURL(parsed *url.URL) (string, error) {
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("URL must include a scheme and host")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""

	return parsed.String(), nil
}

func contentRoot(doc *goquery.Document) *goquery.Selection {
	for _, selector := range []string{"article", "main", "body"} {
		root := doc.Find(selector).First()
		if root.Length() > 0 && normalizedText(root.Text()) != "" {
			return root
		}
	}

	return doc.Selection
}

func parseSections(
	root *goquery.Selection,
	pageID string,
	canonicalURL string,
	pageTitle string,
	site string,
	language string,
	crawledAt time.Time,
) []documents.Section {
	type sectionDraft struct {
		heading     string
		headingPath string
		anchor      string
		text        []string
	}

	headingPath := make([]string, 6)
	current := sectionDraft{heading: "Overview", headingPath: "Overview"}
	drafts := []sectionDraft{}
	flush := func() {
		body := normalizedText(strings.Join(current.text, " "))
		if body == "" {
			return
		}
		current.text = []string{body}
		drafts = append(drafts, current)
	}

	root.Find("h1,h2,h3,h4,h5,h6,p,li,pre,code").Each(func(_ int, selection *goquery.Selection) {
		tag := goquery.NodeName(selection)
		if strings.HasPrefix(tag, "h") && len(tag) == 2 {
			heading := normalizedText(selection.Text())
			if heading == "" {
				return
			}

			flush()
			level, err := strconv.Atoi(tag[1:])
			if err != nil {
				return
			}
			for index := level - 1; index < len(headingPath); index++ {
				headingPath[index] = ""
			}
			headingPath[level-1] = heading
			current = sectionDraft{
				heading:     heading,
				headingPath: joinedHeadingPath(headingPath),
				anchor:      headingAnchor(selection, heading),
			}
			return
		}
		if hasContentBlockAncestor(selection, root) {
			return
		}

		block := normalizedText(selection.Text())
		if block != "" {
			current.text = append(current.text, block)
		}
	})
	flush()

	occurrences := map[string]int{}
	sections := make([]documents.Section, 0, len(drafts))
	for ordinal, draft := range drafts {
		occurrenceKey := draft.headingPath
		occurrences[occurrenceKey]++
		sectionID := stableID(
			pageID + "|" + occurrenceKey + "|" + strconv.Itoa(occurrences[occurrenceKey]),
		)
		sectionURL := canonicalURL
		if draft.anchor != "" {
			sectionURL += "#" + url.PathEscape(draft.anchor)
		}
		body := draft.text[0]
		sections = append(sections, documents.Section{
			ID:          sectionID,
			PageID:      pageID,
			URL:         sectionURL,
			PageTitle:   pageTitle,
			Heading:     draft.heading,
			HeadingPath: draft.headingPath,
			Ordinal:     ordinal,
			Text:        body,
			TokenCount:  len(strings.Fields(body)),
			ContentHash: contentHash(
				strings.Join([]string{pageTitle, draft.headingPath, body}, "\n"),
			),
			Language:  language,
			Site:      site,
			CrawledAt: crawledAt.Unix(),
		})
	}

	return sections
}

func hasContentBlockAncestor(selection *goquery.Selection, root *goquery.Selection) bool {
	for parent := selection.Parent(); parent.Length() > 0; parent = parent.Parent() {
		if parent.Get(0) == root.Get(0) {
			return false
		}
		switch goquery.NodeName(parent) {
		case "p", "li", "pre":
			return true
		}
	}

	return false
}

func headingAnchor(selection *goquery.Selection, heading string) string {
	if id, exists := selection.Attr("id"); exists && id != "" {
		return id
	}
	if id, exists := selection.Find("[id]").First().Attr("id"); exists && id != "" {
		return id
	}

	return slug(heading)
}

func joinedHeadingPath(headings []string) string {
	path := make([]string, 0, len(headings))
	for _, heading := range headings {
		if heading != "" {
			path = append(path, heading)
		}
	}

	return strings.Join(path, " > ")
}

func siteFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return strings.ToLower(parsed.Hostname())
}

func stableID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func contentHash(value string) string {
	return "sha256:" + stableID(value)
}

func normalizedText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func slug(value string) string {
	words := strings.Fields(strings.ToLower(value))
	return strings.Join(words, "-")
}
