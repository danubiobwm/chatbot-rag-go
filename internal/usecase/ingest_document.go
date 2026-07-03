package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	"github.com/danubiobwm/chatbot-rag-go/pkg/logger"
)

// IngestDocumentUseCase recebe um documento novo, evita reprocessar
// duplicados (pelo hash de conteúdo) e dispara o pipeline de extração.
//
// Cada dependência é uma interface (domain.*), não um cliente AWS.
// Isso é Dependency Injection clássico: o usecase recebe o que precisa
// pronto, não cria nada internamente (Single Responsibility + DIP).
type IngestDocumentUseCase struct {
	docs      domain.DocumentRepository
	extractor domain.TextExtractor
	queue     domain.MessageQueue
	log       logger.Logger
}

func NewIngestDocumentUseCase(
	docs domain.DocumentRepository,
	extractor domain.TextExtractor,
	queue domain.MessageQueue,
	log logger.Logger,
) *IngestDocumentUseCase {
	return &IngestDocumentUseCase{docs: docs, extractor: extractor, queue: queue, log: log}
}

type IngestInput struct {
	SourceKey   string
	ContentHash string
	Category    string
}

// Execute aplica a regra: "mesmo arquivo já processado? não reprocessa."
func (uc *IngestDocumentUseCase) Execute(ctx context.Context, in IngestInput) (*domain.Document, error) {
	existing, err := uc.docs.FindByHash(ctx, in.ContentHash)
	if err != nil {
		return nil, fmt.Errorf("checking duplicate: %w", err)
	}
	if existing != nil {
		uc.log.Info("document already processed, skipping", "hash", in.ContentHash)
		return existing, nil
	}

	doc := domain.Document{
		ID:          uuid.NewString(),
		SourceKey:   in.SourceKey,
		ContentHash: in.ContentHash,
		Category:    in.Category,
		Status:      domain.DocumentStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := uc.docs.Save(ctx, doc); err != nil {
		return nil, fmt.Errorf("saving document: %w", err)
	}

	payload := []byte(doc.ID)
	if err := uc.queue.Publish(ctx, "extraction-queue", payload); err != nil {
		// Falha ao publicar não derruba a ingestão já salva — fica
		// como PENDING e pode ser reprocessada/alertada.
		uc.log.Error("failed to enqueue extraction", "documentID", doc.ID, "error", err)
		return &doc, fmt.Errorf("enqueue extraction: %w", err)
	}

	return &doc, nil
}
