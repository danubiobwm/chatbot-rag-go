package usecase

import (
	"context"
	"fmt"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	"github.com/danubiobwm/chatbot-rag-go/pkg/logger"
)

// ProcessDocumentUseCase executa o pipeline: extrai texto -> segmenta em
// chunks -> gera embeddings -> indexa no vector store. É o "worker" que
// roda assincronamente depois que a fila de extração recebe uma mensagem.
type ProcessDocumentUseCase struct {
	docs      domain.DocumentRepository
	chunks    domain.ChunkRepository
	extractor domain.TextExtractor
	segmenter domain.Segmenter
	embedder  domain.Embedder
	store     domain.VectorStore
	notifier  domain.Notifier
	log       logger.Logger
}

func NewProcessDocumentUseCase(
	docs domain.DocumentRepository,
	chunks domain.ChunkRepository,
	extractor domain.TextExtractor,
	segmenter domain.Segmenter,
	embedder domain.Embedder,
	store domain.VectorStore,
	notifier domain.Notifier,
	log logger.Logger,
) *ProcessDocumentUseCase {
	return &ProcessDocumentUseCase{
		docs: docs, chunks: chunks, extractor: extractor,
		segmenter: segmenter, embedder: embedder, store: store,
		notifier: notifier, log: log,
	}
}

func (uc *ProcessDocumentUseCase) Execute(ctx context.Context, doc domain.Document) error {
	if err := uc.docs.UpdateStatus(ctx, doc.ID, domain.DocumentStatusExtracting); err != nil {
		return fmt.Errorf("update status extracting: %w", err)
	}

	rawText, err := uc.extractor.Extract(ctx, doc)
	if err != nil {
		uc.fail(ctx, doc.ID, "extraction failed", err)
		return err
	}

	if err := uc.docs.UpdateStatus(ctx, doc.ID, domain.DocumentStatusSegmenting); err != nil {
		return fmt.Errorf("update status segmenting: %w", err)
	}

	chunks, err := uc.segmenter.Segment(ctx, doc.ID, rawText)
	if err != nil {
		// O Segmenter já deve aplicar fallback determinístico internamente.
		// Se chegou erro aqui, é falha grave — não tem mais rede de segurança.
		uc.fail(ctx, doc.ID, "segmentation failed even with fallback", err)
		return err
	}

	if err := uc.docs.UpdateStatus(ctx, doc.ID, domain.DocumentStatusEmbedding); err != nil {
		return fmt.Errorf("update status embedding: %w", err)
	}

	for i := range chunks {
		emb, err := uc.embedder.Embed(ctx, chunks[i].Content)
		if err != nil {
			uc.fail(ctx, doc.ID, fmt.Sprintf("embedding failed on chunk %d", i), err)
			return err
		}
		chunks[i].Embedding = emb
	}

	if err := uc.chunks.SaveBatch(ctx, chunks); err != nil {
		uc.fail(ctx, doc.ID, "saving chunks failed", err)
		return err
	}

	if err := uc.store.IndexChunks(ctx, chunks); err != nil {
		uc.fail(ctx, doc.ID, "indexing in vector store failed", err)
		return err
	}

	return uc.docs.UpdateStatus(ctx, doc.ID, domain.DocumentStatusDone)
}

func (uc *ProcessDocumentUseCase) fail(ctx context.Context, docID string, reason string, cause error) {
	uc.log.Error(reason, "documentID", docID, "error", cause)
	_ = uc.docs.UpdateStatus(ctx, docID, domain.DocumentStatusFailed)
	if uc.notifier != nil {
		_ = uc.notifier.Notify(ctx, "Falha no pipeline de processamento",
			fmt.Sprintf("doc=%s motivo=%s erro=%v", docID, reason, cause))
	}
}
