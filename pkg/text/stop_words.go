package text

var STOP_WORDS = map[string]struct{}{
	"a":     {},
	"an":    {},
	"and":   {},
	"are":   {},
	"as":    {},
	"at":    {},
	"be":    {},
	"but":   {},
	"by":    {},
	"for":   {},
	"if":    {},
	"in":    {},
	"into":  {},
	"is":    {},
	"it":    {},
	"no":    {},
	"not":   {},
	"of":    {},
	"on":    {},
	"or":    {},
	"such":  {},
	"that":  {},
	"the":   {},
	"their": {},
	"then":  {},
	"there": {},
	"these": {},
	"they":  {},
	"this":  {},
	"to":    {},
	"was":   {},
	"will":  {},
	"with":  {},
}

func RemoveStopWords(tokens []string) []string {

	filtered := make([]string, 0, len(tokens))

	for _, word := range tokens {
		if _, ok := STOP_WORDS[word]; ok {
			continue
		}

		filtered = append(filtered, word)
	}

	return filtered
}
