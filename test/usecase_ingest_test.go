package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	"github.com/danubiobwm/chatbot-rag-go/internal/usecase"
	"github.com/danubiobwm/chatbot-rag-go/test/mocks"
)

func TestIngestDocumentUseCase_SkipsDuplicates(t *testing.T) {
	ctx := context.Background()
	docsRepo := new(mocks.DocumentRepositoryMock)
	queue := new(mocks.MessageQueueMock)
	log := new(mocks.LoggerMock)
	log.On("Info", mock.Anything, mock.Anything).Return()

	existing := &domain.Document{ID: "doc-1", ContentHash: "abc123"}
	docsRepo.On("FindByHash", ctx, "abc123").Return(existing, nil)

	uc := usecase.NewIngestDocumentUseCase(docsRepo, nil, queue, log)

	result, err := uc.Execute(ctx, usecase.IngestInput{
		SourceKey: "docs/contrato.pdf", ContentHash: "abc123", Category: "juridico",
	})

	assert.NoError(t, err)
	assert.Equal(t, "doc-1", result.ID)
	docsRepo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	queue.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestIngestDocumentUseCase_NewDocumentIsSavedAndEnqueued(t *testing.T) {
	ctx := context.Background()
	docsRepo := new(mocks.DocumentRepositoryMock)
	queue := new(mocks.MessageQueueMock)
	log := new(mocks.LoggerMock)

	docsRepo.On("FindByHash", ctx, "newhash").Return(nil, nil)
	docsRepo.On("Save", ctx, mock.AnythingOfType("domain.Document")).Return(nil)
	queue.On("Publish", ctx, "extraction-queue", mock.Anything).Return(nil)

	uc := usecase.NewIngestDocumentUseCase(docsRepo, nil, queue, log)

	result, err := uc.Execute(ctx, usecase.IngestInput{
		SourceKey: "docs/politica-rh.pdf", ContentHash: "newhash", Category: "rh",
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.DocumentStatusPending, result.Status)
	docsRepo.AssertExpectations(t)
	queue.AssertExpectations(t)
}
