package main

import (
	"fmt"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/crawler"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/pkg/text"
)

func TestPipeline() {
	word := "The BIG brown cat JuMps over the lazy dog but it likes to eat fish"
	fmt.Println(len(text.ProcessQuestionText(word)))
}

func main() {
	res, err := crawler.FetchURL("https://en.wikipedia.org/wiki/Hulk")

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(res)
}
