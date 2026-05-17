package text

import "strings"

func Tokenize(words string) []string {
	return strings.Fields(words)
}
