package parser

import (
	"strings"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/models"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/pkg/text"
	"github.com/PuerkitoBio/goquery"
)

// TODO add extraction priority (if large amount of article and main, check)
func ParseHTML(result models.FetchResult) models.Page {

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(result.HTML))

	if err != nil {
		return models.Page{}
	}

	removeList := []string{
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
	}

	for _, element := range removeList {
		doc.Find(element).Remove()
	}

	title := doc.Find("title").Text()
	content := doc.Find("article, main, h1, h2, h3, p, li, pre, code").Text()
	content = strings.Join(strings.Fields(content), " ")
	tokens := text.ProcessText(title + " " + content)

	return models.Page{
		URL:     result.URL,
		Title:   title,
		Content: content,
		Tokens:  tokens,
	}
}
