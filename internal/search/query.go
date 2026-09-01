package search

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxQueryRunes = 256
	MaxQueryTerms = 20
)

func CompileLexicalQuery(query string, site string, language string) (string, error) {
	if utf8.RuneCountInString(query) > MaxQueryRunes {
		return "", fmt.Errorf("%w: query exceeds %d characters", ErrInvalidQuery, MaxQueryRunes)
	}

	segments, err := querySegments(query)
	if err != nil {
		return "", err
	}
	clauses := make([]string, 0, len(segments)+2)
	termCount := 0
	for _, segment := range segments {
		words := safeWords(segment.text)
		if len(words) == 0 {
			continue
		}
		termCount += len(words)
		if termCount > MaxQueryTerms {
			return "", fmt.Errorf("%w: query exceeds %d terms", ErrInvalidQuery, MaxQueryTerms)
		}
		if segment.phrase {
			clauses = append(clauses, `"`+strings.Join(words, " ")+`"`)
			continue
		}
		clauses = append(clauses, words...)
	}
	if termCount == 0 {
		return "", fmt.Errorf("%w: query has no searchable terms", ErrInvalidQuery)
	}

	filters, err := compileFilters(site, language)
	if err != nil {
		return "", err
	}
	clauses = append(clauses, filters...)

	return strings.Join(clauses, " "), nil
}

func CompileSemanticFilter(
	site string,
	language string,
	profile string,
) (string, error) {
	filters, err := compileFilters(site, language)
	if err != nil {
		return "", err
	}
	safeProfile, err := safeFilter(profile, "")
	if err != nil {
		return "", fmt.Errorf("%w: invalid embedding profile", ErrInvalidQuery)
	}
	filters = append(
		filters,
		"@embedding_status:{ready}",
		"@embedding_profile:{"+safeProfile+"}",
	)
	return "(" + strings.Join(filters, " ") + ")", nil
}

func ValidateSemanticQuery(query string, site string, language string) error {
	if utf8.RuneCountInString(query) > MaxQueryRunes {
		return fmt.Errorf(
			"%w: query exceeds %d characters",
			ErrInvalidQuery,
			MaxQueryRunes,
		)
	}
	if len(safeWords(query)) == 0 {
		return fmt.Errorf("%w: query has no searchable terms", ErrInvalidQuery)
	}
	_, err := compileFilters(site, language)
	return err
}

func compileFilters(site string, language string) ([]string, error) {
	filters := make([]string, 0, 2)
	if site != "" {
		value, err := safeFilter(site, ".-")
		if err != nil {
			return nil, fmt.Errorf("%w: invalid site filter", ErrInvalidQuery)
		}
		filters = append(filters, "@site:{"+value+"}")
	}
	if language != "" {
		value, err := safeFilter(language, "-")
		if err != nil {
			return nil, fmt.Errorf("%w: invalid language filter", ErrInvalidQuery)
		}
		filters = append(filters, "@language:{"+value+"}")
	}
	return filters, nil
}

type querySegment struct {
	text   string
	phrase bool
}

func querySegments(query string) ([]querySegment, error) {
	segments := []querySegment{}
	var current strings.Builder
	inPhrase := false
	flush := func() {
		if current.Len() == 0 {
			return
		}
		segments = append(segments, querySegment{
			text:   current.String(),
			phrase: inPhrase,
		})
		current.Reset()
	}

	for _, character := range query {
		if character == '"' {
			flush()
			inPhrase = !inPhrase
			continue
		}
		current.WriteRune(character)
	}
	if inPhrase {
		return nil, fmt.Errorf("%w: unmatched quote", ErrInvalidQuery)
	}
	flush()

	return segments, nil
}

func safeWords(value string) []string {
	var normalized strings.Builder
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			normalized.WriteRune(character)
			continue
		}
		normalized.WriteRune(' ')
	}

	return strings.Fields(normalized.String())
}

func safeFilter(value string, allowedPunctuation string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", ErrInvalidQuery
	}
	var escaped strings.Builder
	hasWordCharacter := false
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			hasWordCharacter = true
			escaped.WriteRune(character)
			continue
		}
		if strings.ContainsRune(allowedPunctuation, character) {
			escaped.WriteRune('\\')
			escaped.WriteRune(character)
			continue
		}
		return "", ErrInvalidQuery
	}
	if !hasWordCharacter {
		return "", ErrInvalidQuery
	}

	return escaped.String(), nil
}
