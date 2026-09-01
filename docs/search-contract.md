# Search contract

The public search contract is version `v1`.

## Request

```text
GET /search?q=iron+man&mode=lexical&limit=10&offset=0&site=en.wikipedia.org&language=en
```

- `q` is required.
- `mode` defaults to `lexical`. Phase 3 supports only `lexical`.
- `limit` defaults to 10 and must be between 1 and 100.
- `offset` defaults to 0 and must be non-negative.
- `site` optionally filters results to an exact site value.
- `language` optionally filters results to an exact language value.
- Double quotes express an exact phrase, for example `"infinity stones"`.
- Query syntax is treated as text rather than raw Redis Search syntax.

## Response

```json
{
  "version": "v1",
  "query": "iron man",
  "mode": "lexical",
  "used_mode": "lexical",
  "took_ms": 1,
  "next_offset": null,
  "suggestions": [
    {
      "term": "spidr",
      "candidates": ["spider"]
    }
  ],
  "results": [
    {
      "page_id": "sha256:example",
      "url": "https://en.wikipedia.org/wiki/Iron_Man",
      "title": "Iron Man",
      "score": 12,
      "matched_sections": [
        {
          "section_id": "sha256:section-example",
          "heading": "Fictional character biography",
          "snippet": "Iron Man is a superhero appearing in American comic books...",
          "score": 12
        }
      ]
    }
  ]
}
```

Results are grouped by page and include up to three matching sections. The best section supplies the page's base score; additional matching sections add a capped bonus. `next_offset` is set when another grouped page is available.

When lexical search returns no results, Redis Search may provide spelling
suggestions. Suggestions are advisory: the API never rewrites or reruns the
user's query automatically, and the field is omitted when Redis has no
candidate.

Future semantic or hybrid modes must report the actual executed mode in `used_mode`; lexical search remains the default and fallback must never be silent.

## Relevance baseline

`testdata/search_relevance.json` contains the initial judged query set. Run
`go run ./cmd/relevance` to measure the retained in-memory implementation, or
`go run ./cmd/relevance -backend redis` to evaluate the live lexical backend.
Reports include top results, Recall@10, MRR, nDCG@10, p50/p95 latency,
zero-result rate, and backend metadata. The historical Phase 0 measurement is
checked in at `docs/relevance-baseline.json`; future field-weight and ranking
changes should compare Redis output with it.

`go run ./cmd/search-diagnostics` reports the alias, active physical index,
schema version, indexed section count, indexing errors and memory, a
content-derived corpus version, and whether page records and the search index
are currently in sync.
