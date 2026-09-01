package documents

type Page struct {
	ID           string   `json:"id"`
	URL          string   `json:"url"`
	CanonicalURL string   `json:"canonical_url"`
	Title        string   `json:"title"`
	Site         string   `json:"site"`
	Language     string   `json:"language"`
	ContentHash  string   `json:"content_hash"`
	CrawlStatus  string   `json:"crawl_status"`
	HTTPStatus   int      `json:"http_status"`
	CrawledAt    int64    `json:"crawled_at"`
	SectionIDs   []string `json:"section_ids"`
}

type Section struct {
	ID               string    `json:"id"`
	PageID           string    `json:"page_id"`
	URL              string    `json:"url"`
	PageTitle        string    `json:"page_title"`
	Heading          string    `json:"heading"`
	HeadingPath      string    `json:"heading_path"`
	Ordinal          int       `json:"ordinal"`
	Text             string    `json:"text"`
	TokenCount       int       `json:"token_count"`
	ContentHash      string    `json:"content_hash"`
	Language         string    `json:"language"`
	Site             string    `json:"site"`
	CrawledAt        int64     `json:"crawled_at"`
	EmbeddingModel   string    `json:"embedding_model,omitempty"`
	EmbeddingVersion string    `json:"embedding_version,omitempty"`
	Embedding        []float32 `json:"embedding,omitempty"`
}

type CrawlFailure struct {
	PageID      string `json:"page_id"`
	URL         string `json:"url"`
	HTTPStatus  int    `json:"http_status"`
	AttemptedAt int64  `json:"attempted_at"`
	Error       string `json:"error"`
}
