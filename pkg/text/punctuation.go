package text

import "strings"

var PUNCTUATION = map[rune]struct{}{
	'.': {},

	'!':  {},
	',':  {},
	'?':  {},
	';':  {},
	':':  {},
	'(':  {},
	')':  {},
	'[':  {},
	']':  {},
	'{':  {},
	'}':  {},
	'\\': {},
	'/':  {},
	'\'': {},
	'_':  {},
	'-':  {},
	'=':  {},
	'+':  {},
	'*':  {},
	'&':  {},
	'%':  {},
	'$':  {},
	'#':  {},
	'@':  {},
	'~':  {},
	'`':  {},
	'^':  {},
	'<':  {},
	'>':  {},
	'|':  {},
}

func RemovePunctuation(words string) string {

	var filtered strings.Builder
	filtered.Grow(len(words))

	for _, letter := range words {
		if _, ok := PUNCTUATION[letter]; ok {
			continue
		}

		filtered.WriteRune(letter)
	}

	return filtered.String()
}
