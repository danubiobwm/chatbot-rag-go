package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	"github.com/danubiobwm/chatbot-rag-go/pkg/logger"
)

// LLMSegmenter usa um LLM para segmentação semântica, mas NUNCA deixa o
// pipeline travar: se o LLM falhar ou devolver algo que não é JSON válido,
// cai automaticamente no fallback determinístico (split por tamanho fixo).
//
// Isso é o padrão Decorator: FixedSizeSegmenter sozinho já satisfaz
// domain.Segmenter; LLMSegmenter "decora" esse comportamento por cima,
// sem o usecase saber que existe um fallback.
type LLMSegmenter struct {
	llm      domain.LLMClient
	fallback domain.Segmenter
	log      logger.Logger
}

func NewLLMSegmenter(llm domain.LLMClient, fallback domain.Segmenter, log logger.Logger) *LLMSegmenter {
	return &LLMSegmenter{llm: llm, fallback: fallback, log: log}
}

type llmSegmentResponse struct {
	Chunks []string `json:"chunks"`
}

func (s *LLMSegmenter) Segment(ctx context.Context, documentID string, rawText string) ([]domain.Chunk, error) {
	prompt := fmt.Sprintf(
		"Divida o texto abaixo em blocos semanticamente coerentes. "+
			"Responda APENAS com JSON no formato {\"chunks\": [\"...\", \"...\"]}.\n\nTexto:\n%s",
		rawText,
	)

	raw, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		s.log.Error("llm segmentation failed, using fallback", "documentID", documentID, "error", err)
		return s.fallback.Segment(ctx, documentID, rawText)
	}

	var parsed llmSegmentResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || len(parsed.Chunks) == 0 {
		s.log.Error("llm segmentation returned invalid json, using fallback", "documentID", documentID)
		return s.fallback.Segment(ctx, documentID, rawText)
	}

	chunks := make([]domain.Chunk, 0, len(parsed.Chunks))
	for i, content := range parsed.Chunks {
		chunks = append(chunks, domain.Chunk{
			ID:         uuid.NewString(),
			DocumentID: documentID,
			Content:    content,
			Order:      i,
			Metadata:   map[string]string{"segmentation": "llm"},
		})
	}
	return chunks, nil
}

// FixedSizeSegmenter é o fallback determinístico: divide por tamanho fixo
// com overlap. Simples, previsível, nunca falha por causa de uma API externa.
type FixedSizeSegmenter struct {
	ChunkSize int
	Overlap   int
}

func NewFixedSizeSegmenter(chunkSize, overlap int) *FixedSizeSegmenter {
	return &FixedSizeSegmenter{ChunkSize: chunkSize, Overlap: overlap}
}

func (s *FixedSizeSegmenter) Segment(_ context.Context, documentID string, rawText string) ([]domain.Chunk, error) {
	words := strings.Fields(rawText)
	if len(words) == 0 {
		return nil, fmt.Errorf("empty text for document %s", documentID)
	}

	var chunks []domain.Chunk
	step := s.ChunkSize - s.Overlap
	if step <= 0 {
		step = s.ChunkSize
	}

	order := 0
	for start := 0; start < len(words); start += step {
		end := start + s.ChunkSize
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, domain.Chunk{
			ID:         uuid.NewString(),
			DocumentID: documentID,
			Content:    strings.Join(words[start:end], " "),
			Order:      order,
			Metadata:   map[string]string{"segmentation": "fixed-size-fallback"},
		})
		order++
		if end == len(words) {
			break
		}
	}
	return chunks, nil
}
