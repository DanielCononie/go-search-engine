package embedding

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage"
)

type serviceRepository struct {
	sections         []documents.Section
	semanticSections []documents.SemanticSection
	saved            []documents.SemanticSection
	deleted          []string
	supersedeSaves   bool
	saveErr          error
}

func (r *serviceRepository) Sections(
	context.Context,
) ([]documents.Section, error) {
	return r.sections, nil
}

func (r *serviceRepository) SemanticSections(
	context.Context,
) ([]documents.SemanticSection, error) {
	return r.semanticSections, nil
}

func (r *serviceRepository) SaveSemanticSection(
	_ context.Context,
	section documents.SemanticSection,
) (bool, error) {
	r.saved = append(r.saved, section)
	if r.saveErr != nil {
		return false, r.saveErr
	}
	return !r.supersedeSaves, nil
}

func (r *serviceRepository) DeleteSemanticSection(
	_ context.Context,
	sectionID string,
) error {
	r.deleted = append(r.deleted, sectionID)
	return nil
}

type serviceEmbedder struct {
	inputs [][]string
	err    error
}

func (e *serviceEmbedder) Embed(
	_ context.Context,
	inputs []string,
) ([][]float32, error) {
	e.inputs = append(e.inputs, inputs)
	if e.err != nil {
		return nil, e.err
	}
	vectors := make([][]float32, len(inputs))
	for index := range inputs {
		vectors[index] = []float32{1, 0}
	}
	return vectors, nil
}

func TestServiceSkipsCurrentAndEmbedsStaleSections(t *testing.T) {
	embeddingConfig := config.Embedding{
		Model:             "model",
		Version:           "v1",
		Dimensions:        2,
		MaxInputRunes:     100,
		InputOverlapRunes: 10,
	}
	profile := Profile("model", "v1", 2)
	repository := &serviceRepository{
		sections: []documents.Section{
			{
				ID:          "current",
				ContentHash: "hash-1",
			},
			{
				ID:          "stale",
				PageTitle:   "Page",
				Heading:     "Heading",
				Text:        "Body",
				ContentHash: "hash-2",
			},
		},
		semanticSections: []documents.SemanticSection{
			{
				SectionID:           "current",
				SourceContentHash:   "hash-1",
				EmbeddingStatus:     "ready",
				EmbeddingProfile:    profile,
				EmbeddingDimensions: 2,
				Embedding:           []float32{1, 0},
			},
			{
				SectionID:         "stale",
				SourceContentHash: "old-hash",
				EmbeddingAttempts: 3,
			},
		},
	}
	embedder := &serviceEmbedder{}
	service := NewService(repository, embedder, embeddingConfig)
	service.now = func() time.Time {
		return time.Unix(100, 0)
	}

	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 2 || report.Current != 1 || report.Updated != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if !reflect.DeepEqual(embedder.inputs, [][]string{{
		"Page\n\nHeading\n\nBody",
	}}) {
		t.Fatalf("inputs = %#v", embedder.inputs)
	}
	if len(repository.saved) != 1 ||
		repository.saved[0].SourceContentHash != "hash-2" ||
		repository.saved[0].EmbeddingUpdatedAt != 100 ||
		repository.saved[0].EmbeddingAttempts != 4 {
		t.Fatalf("saved embeddings = %#v", repository.saved)
	}
}

func TestServiceRecordsEmbeddingFailures(t *testing.T) {
	repository := &serviceRepository{
		sections: []documents.Section{{
			ID:          "section",
			Text:        "Body",
			ContentHash: "hash",
		}},
	}
	embedder := &serviceEmbedder{err: errors.New("provider unavailable")}
	service := NewService(repository, embedder, config.Embedding{
		Model:             "model",
		Version:           "v1",
		Dimensions:        2,
		MaxInputRunes:     100,
		InputOverlapRunes: 10,
	})

	report, err := service.Run(context.Background())
	if !errors.Is(err, ErrPartialFailure) {
		t.Fatalf("error = %v", err)
	}
	if report.Failed != 1 || len(repository.saved) != 1 ||
		repository.saved[0].EmbeddingStatus != "failed" {
		t.Fatalf("report = %#v, saved = %#v", report, repository.saved)
	}
}

func TestServiceDeletesSemanticRecordsForRemovedSections(t *testing.T) {
	repository := &serviceRepository{
		semanticSections: []documents.SemanticSection{{
			SectionID: "removed",
		}},
	}
	service := NewService(repository, &serviceEmbedder{}, config.Embedding{
		Model:             "model",
		Version:           "v1",
		Dimensions:        2,
		MaxInputRunes:     100,
		InputOverlapRunes: 10,
	})

	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Deleted != 1 ||
		!reflect.DeepEqual(repository.deleted, []string{"removed"}) {
		t.Fatalf("report = %#v, deleted = %#v", report, repository.deleted)
	}
}

func TestServiceAveragesLongSectionWindows(t *testing.T) {
	repository := &serviceRepository{
		sections: []documents.Section{{
			ID:          "section",
			Text:        "one two three four five six",
			ContentHash: "hash",
		}},
	}
	embedder := &serviceEmbedder{}
	service := NewService(repository, embedder, config.Embedding{
		Model:             "model",
		Version:           "v1",
		Dimensions:        2,
		MaxInputRunes:     14,
		InputOverlapRunes: 4,
	})

	if _, err := service.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(embedder.inputs) != 1 || len(embedder.inputs[0]) != 3 {
		t.Fatalf("inputs = %#v", embedder.inputs)
	}
}

func TestServiceReportsContentChangesAsSuperseded(t *testing.T) {
	repository := &serviceRepository{
		sections: []documents.Section{{
			ID:          "section",
			Text:        "body",
			ContentHash: "old",
		}},
		supersedeSaves: true,
	}
	service := NewService(repository, &serviceEmbedder{}, config.Embedding{
		Model:             "model",
		Version:           "v1",
		Dimensions:        2,
		MaxInputRunes:     100,
		InputOverlapRunes: 10,
	})

	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Superseded != 1 || report.Updated != 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestServiceReportsRemovedSourceAsSuperseded(t *testing.T) {
	repository := &serviceRepository{
		sections: []documents.Section{{
			ID:          "section",
			Text:        "body",
			ContentHash: "old",
		}},
		saveErr: storage.ErrNotFound,
	}
	service := NewService(repository, &serviceEmbedder{}, config.Embedding{
		Model:             "model",
		Version:           "v1",
		Dimensions:        2,
		MaxInputRunes:     100,
		InputOverlapRunes: 10,
	})

	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Superseded != 1 || report.Updated != 0 || report.Failed != 0 {
		t.Fatalf("report = %#v", report)
	}
}
