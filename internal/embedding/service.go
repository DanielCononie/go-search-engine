package embedding

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/documents"
	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/storage"
)

var ErrPartialFailure = errors.New("some sections could not be embedded")

type Repository interface {
	Sections(ctx context.Context) ([]documents.Section, error)
	SemanticSections(ctx context.Context) ([]documents.SemanticSection, error)
	SaveSemanticSection(
		ctx context.Context,
		section documents.SemanticSection,
	) (bool, error)
	DeleteSemanticSection(ctx context.Context, sectionID string) error
}

type Report struct {
	GeneratedAt time.Time `json:"generated_at"`
	Profile     string    `json:"profile"`
	Model       string    `json:"model"`
	Version     string    `json:"version"`
	Dimensions  int       `json:"dimensions"`
	Total       int       `json:"total_sections"`
	Current     int       `json:"current_sections"`
	Updated     int       `json:"updated_sections"`
	Superseded  int       `json:"superseded_sections"`
	Deleted     int       `json:"deleted_sections"`
	Failed      int       `json:"failed_sections"`
	Failures    []Failure `json:"failures,omitempty"`
}

type Failure struct {
	SectionID string `json:"section_id"`
	Error     string `json:"error"`
}

type Service struct {
	repository Repository
	embedder   Embedder
	config     config.Embedding
	profile    string
	now        func() time.Time
}

func NewService(
	repository Repository,
	embedder Embedder,
	embeddingConfig config.Embedding,
) *Service {
	return &Service{
		repository: repository,
		embedder:   embedder,
		config:     embeddingConfig,
		profile: Profile(
			embeddingConfig.Model,
			embeddingConfig.Version,
			embeddingConfig.Dimensions,
		),
		now: time.Now,
	}
}

func (s *Service) Run(ctx context.Context) (Report, error) {
	sections, err := s.repository.Sections(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("list sections for embedding: %w", err)
	}
	semanticSections, err := s.repository.SemanticSections(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("list semantic sections: %w", err)
	}
	existing := make(map[string]documents.SemanticSection, len(semanticSections))
	for _, section := range semanticSections {
		existing[section.SectionID] = section
	}

	report := Report{
		GeneratedAt: s.now().UTC(),
		Profile:     s.profile,
		Model:       s.config.Model,
		Version:     s.config.Version,
		Dimensions:  s.config.Dimensions,
		Total:       len(sections),
	}
	for _, section := range sections {
		semanticSection, exists := existing[section.ID]
		delete(existing, section.ID)
		if exists && currentEmbedding(
			semanticSection,
			section,
			s.profile,
			s.config.Dimensions,
		) {
			report.Current++
			continue
		}
		attempts := 0
		if exists {
			attempts = semanticSection.EmbeddingAttempts
		}
		vector, embeddingErr := s.embedSection(ctx, section)
		if embeddingErr != nil {
			saved, saveErr := s.repository.SaveSemanticSection(
				ctx,
				s.semanticSection(
					section,
					"failed",
					nil,
					embeddingErr.Error(),
					attempts+1,
				),
			)
			if errors.Is(saveErr, storage.ErrNotFound) {
				report.Superseded++
				continue
			}
			if saveErr != nil {
				return report, saveErr
			}
			if !saved {
				report.Superseded++
				continue
			}
			report.Failed++
			report.Failures = append(report.Failures, Failure{
				SectionID: section.ID,
				Error:     embeddingErr.Error(),
			})
			continue
		}
		saved, saveErr := s.repository.SaveSemanticSection(
			ctx,
			s.semanticSection(
				section,
				"ready",
				vector,
				"",
				attempts+1,
			),
		)
		if errors.Is(saveErr, storage.ErrNotFound) {
			report.Superseded++
			continue
		}
		if saveErr != nil {
			return report, saveErr
		}
		if saved {
			report.Updated++
		} else {
			report.Superseded++
		}
	}
	for sectionID := range existing {
		if err := s.repository.DeleteSemanticSection(ctx, sectionID); err != nil {
			return report, err
		}
		report.Deleted++
	}
	if report.Failed > 0 {
		return report, ErrPartialFailure
	}

	return report, nil
}

func (s *Service) embedSection(
	ctx context.Context,
	section documents.Section,
) ([]float32, error) {
	windows := Windows(
		sectionEmbeddingInput(section),
		s.config.MaxInputRunes,
		s.config.InputOverlapRunes,
	)
	if len(windows) == 0 {
		return nil, errors.New("section has no embeddable content")
	}
	vectors, err := s.embedder.Embed(ctx, windows)
	if err != nil {
		return nil, err
	}
	vector, err := Average(vectors, s.config.Dimensions)
	if err != nil {
		return nil, err
	}
	return vector, nil
}

func currentEmbedding(
	semanticSection documents.SemanticSection,
	sourceSection documents.Section,
	profile string,
	dimensions int,
) bool {
	return semanticSection.EmbeddingStatus == "ready" &&
		semanticSection.EmbeddingProfile == profile &&
		semanticSection.SourceContentHash == sourceSection.ContentHash &&
		semanticSection.EmbeddingDimensions == dimensions &&
		len(semanticSection.Embedding) == dimensions
}

func (s *Service) semanticSection(
	section documents.Section,
	status string,
	vector []float32,
	lastError string,
	attempts int,
) documents.SemanticSection {
	return documents.SemanticSection{
		ID:                  section.ID,
		SectionID:           section.ID,
		PageID:              section.PageID,
		URL:                 section.URL,
		PageTitle:           section.PageTitle,
		Heading:             section.Heading,
		HeadingPath:         section.HeadingPath,
		Ordinal:             section.Ordinal,
		Text:                section.Text,
		Language:            section.Language,
		Site:                section.Site,
		CrawledAt:           section.CrawledAt,
		SourceContentHash:   section.ContentHash,
		EmbeddingModel:      s.config.Model,
		EmbeddingVersion:    s.config.Version,
		EmbeddingProfile:    s.profile,
		EmbeddingDimensions: s.config.Dimensions,
		EmbeddingUpdatedAt:  s.now().Unix(),
		EmbeddingStatus:     status,
		EmbeddingAttempts:   attempts,
		EmbeddingLastError:  lastError,
		Embedding:           vector,
	}
}

func sectionEmbeddingInput(section documents.Section) string {
	parts := []string{
		section.PageTitle,
		section.HeadingPath,
		section.Heading,
		section.Text,
	}
	nonEmpty := parts[:0]
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}
