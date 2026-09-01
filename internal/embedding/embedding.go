package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
)

var ErrInvalidResponse = errors.New("invalid embedding response")

type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

func Profile(model string, version string, dimensions int) string {
	hash := sha256.Sum256([]byte(
		model + "\x00" + version + "\x00" + fmt.Sprintf("%d", dimensions),
	))
	return hex.EncodeToString(hash[:])
}

func Windows(value string, maximumRunes int, overlapRunes int) []string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= maximumRunes {
		if value == "" {
			return nil
		}
		return []string{value}
	}

	windows := make([]string, 0, len(runes)/maximumRunes+1)
	for start := 0; start < len(runes); {
		end := min(start+maximumRunes, len(runes))
		if end < len(runes) {
			for split := end; split > start+maximumRunes/2; split-- {
				if unicode.IsSpace(runes[split-1]) {
					end = split
					break
				}
			}
		}
		window := strings.TrimSpace(string(runes[start:end]))
		if window != "" {
			windows = append(windows, window)
		}
		if end == len(runes) {
			break
		}
		next := max(start+1, end-overlapRunes)
		for next > start+1 && !unicode.IsSpace(runes[next-1]) {
			next--
		}
		for next < end && unicode.IsSpace(runes[next]) {
			next++
		}
		start = next
	}

	return windows
}

func Average(vectors [][]float32, dimensions int) ([]float32, error) {
	if len(vectors) == 0 {
		return nil, fmt.Errorf("%w: no vectors", ErrInvalidResponse)
	}
	sums := make([]float64, dimensions)
	for _, vector := range vectors {
		if len(vector) != dimensions {
			return nil, fmt.Errorf(
				"%w: vector has %d dimensions, expected %d",
				ErrInvalidResponse,
				len(vector),
				dimensions,
			)
		}
		for index, value := range vector {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return nil, fmt.Errorf(
					"%w: vector contains a non-finite value",
					ErrInvalidResponse,
				)
			}
			sums[index] += float64(value)
			if math.IsInf(sums[index], 0) {
				return nil, fmt.Errorf(
					"%w: vector values overflow",
					ErrInvalidResponse,
				)
			}
		}
	}

	average := make([]float32, dimensions)
	scale := 1 / float64(len(vectors))
	var magnitudeSquared float64
	for index, sum := range sums {
		value := sum * scale
		magnitudeSquared += value * value
		average[index] = float32(value)
	}
	if magnitudeSquared == 0 || math.IsInf(magnitudeSquared, 0) {
		return nil, fmt.Errorf("%w: vector has invalid magnitude", ErrInvalidResponse)
	}
	magnitude := math.Sqrt(magnitudeSquared)
	for index := range average {
		average[index] = float32(float64(average[index]) / magnitude)
	}

	return average, nil
}
