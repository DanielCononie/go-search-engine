package models

type Page struct {
	URL     string
	Title   string
	Content string
	Tokens  []string
}

type SearchResult struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Score int    `json:"score"`
}

type FetchResult struct {
	URL  string
	HTML string
	Err  error
}
