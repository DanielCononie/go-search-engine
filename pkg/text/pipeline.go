package text

func ProcessQuestionText(question string) []string {

	question = Lowercase(question)
	question = RemovePunctuation(question)
	tokens := Tokenize(question)
	tokens = RemoveStopWords(tokens)

	return tokens
}
