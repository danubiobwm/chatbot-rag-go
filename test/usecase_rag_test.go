package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	"github.com/danubiobwm/chatbot-rag-go/internal/usecase"
	"github.com/danubiobwm/chatbot-rag-go/test/mocks"
)

func TestRAGQuery_CacheHit(t *testing.T) {
	ctx := context.Background()
	cache := new(mocks.ResponseCacheMock)
	log := new(mocks.LoggerMock)

	msg := domain.Message{Text: "qual o prazo de férias?", Sender: domain.Sender{Channel: "slack", UserID: "U1"}}

	cache.On("Get", ctx, mock.AnythingOfType("string")).Return("resposta em cache", true)
	log.On("Info", mock.Anything, mock.Anything).Return()

	uc := usecase.NewRAGQueryUseCase(nil, nil, nil, nil, cache, log)
	result, err := uc.Execute(ctx, msg, "slack:U1")

	assert.NoError(t, err)
	assert.Equal(t, "resposta em cache", result.Answer)
	assert.True(t, result.FromCache)
	cache.AssertExpectations(t)
}

func TestRAGQuery_HappyPath(t *testing.T) {
	ctx := context.Background()
	sessions := new(mocks.SessionRepositoryMock)
	embedder := new(mocks.EmbedderMock)
	store := new(mocks.VectorStoreMock)
	llm := new(mocks.LLMClientMock)
	cache := new(mocks.ResponseCacheMock)
	log := new(mocks.LoggerMock)

	question := "qual o prazo de férias?"
	answer := "O prazo é 30 dias corridos."
	embedding := []float32{0.1, 0.2, 0.3}
	chunks := []domain.RetrievedChunk{{Chunk: domain.Chunk{Content: "férias: 30 dias"}, Score: 0.9}}

	cache.On("Get", ctx, mock.AnythingOfType("string")).Return("", false)
	cache.On("Set", ctx, mock.AnythingOfType("string"), answer).Return()
	sessions.On("Get", ctx, "slack:U1").Return(nil, nil)
	sessions.On("Save", ctx, mock.AnythingOfType("domain.Session")).Return(nil)
	embedder.On("Embed", ctx, question).Return(embedding, nil)
	store.On("Search", ctx, embedding, 4, (map[string]string)(nil)).Return(chunks, nil)
	llm.On("Generate", ctx, mock.AnythingOfType("string")).Return(answer, nil)

	uc := usecase.NewRAGQueryUseCase(sessions, embedder, store, llm, cache, log)
	result, err := uc.Execute(ctx, domain.Message{Text: question, Sender: domain.Sender{Channel: "slack", UserID: "U1"}}, "slack:U1")

	assert.NoError(t, err)
	assert.Equal(t, answer, result.Answer)
	assert.False(t, result.FromCache)
	assert.Equal(t, chunks, result.SourceChunks)
	sessions.AssertExpectations(t)
	embedder.AssertExpectations(t)
	store.AssertExpectations(t)
	llm.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRAGQuery_EmbedderError(t *testing.T) {
	ctx := context.Background()
	sessions := new(mocks.SessionRepositoryMock)
	embedder := new(mocks.EmbedderMock)
	cache := new(mocks.ResponseCacheMock)
	log := new(mocks.LoggerMock)

	cache.On("Get", ctx, mock.AnythingOfType("string")).Return("", false)
	sessions.On("Get", ctx, "slack:U1").Return(nil, nil)
	embedder.On("Embed", ctx, mock.AnythingOfType("string")).Return(nil, errors.New("bedrock unavailable"))

	uc := usecase.NewRAGQueryUseCase(sessions, embedder, nil, nil, cache, log)
	_, err := uc.Execute(ctx, domain.Message{Text: "pergunta", Sender: domain.Sender{Channel: "slack", UserID: "U1"}}, "slack:U1")

	assert.ErrorContains(t, err, "embedding query")
}

func TestRAGQuery_VectorStoreError(t *testing.T) {
	ctx := context.Background()
	sessions := new(mocks.SessionRepositoryMock)
	embedder := new(mocks.EmbedderMock)
	store := new(mocks.VectorStoreMock)
	cache := new(mocks.ResponseCacheMock)
	log := new(mocks.LoggerMock)

	embedding := []float32{0.1}

	cache.On("Get", ctx, mock.AnythingOfType("string")).Return("", false)
	sessions.On("Get", ctx, "slack:U1").Return(nil, nil)
	embedder.On("Embed", ctx, mock.AnythingOfType("string")).Return(embedding, nil)
	store.On("Search", ctx, embedding, 4, (map[string]string)(nil)).Return(nil, errors.New("opensearch down"))

	uc := usecase.NewRAGQueryUseCase(sessions, embedder, store, nil, cache, log)
	_, err := uc.Execute(ctx, domain.Message{Text: "pergunta", Sender: domain.Sender{Channel: "slack", UserID: "U1"}}, "slack:U1")

	assert.ErrorContains(t, err, "searching vector store")
}

func TestRAGQuery_LLMError(t *testing.T) {
	ctx := context.Background()
	sessions := new(mocks.SessionRepositoryMock)
	embedder := new(mocks.EmbedderMock)
	store := new(mocks.VectorStoreMock)
	llm := new(mocks.LLMClientMock)
	cache := new(mocks.ResponseCacheMock)
	log := new(mocks.LoggerMock)

	embedding := []float32{0.1}

	cache.On("Get", ctx, mock.AnythingOfType("string")).Return("", false)
	sessions.On("Get", ctx, "slack:U1").Return(nil, nil)
	embedder.On("Embed", ctx, mock.AnythingOfType("string")).Return(embedding, nil)
	store.On("Search", ctx, embedding, 4, (map[string]string)(nil)).Return([]domain.RetrievedChunk{}, nil)
	llm.On("Generate", ctx, mock.AnythingOfType("string")).Return("", errors.New("llm timeout"))

	uc := usecase.NewRAGQueryUseCase(sessions, embedder, store, llm, cache, log)
	_, err := uc.Execute(ctx, domain.Message{Text: "pergunta", Sender: domain.Sender{Channel: "slack", UserID: "U1"}}, "slack:U1")

	assert.ErrorContains(t, err, "generating answer")
}

func TestRAGQuery_ExpiredSession_CreatesNew(t *testing.T) {
	ctx := context.Background()
	sessions := new(mocks.SessionRepositoryMock)
	embedder := new(mocks.EmbedderMock)
	store := new(mocks.VectorStoreMock)
	llm := new(mocks.LLMClientMock)
	cache := new(mocks.ResponseCacheMock)
	log := new(mocks.LoggerMock)

	expired := &domain.Session{
		SessionID: "slack:U1",
		Sender:    domain.Sender{Channel: "slack", UserID: "U1"},
		History:   []domain.ConversationTurn{{Question: "old", Answer: "old", Timestamp: time.Now()}},
		ExpiresAt: time.Now().Add(-1 * time.Minute), // expirada
	}
	embedding := []float32{0.1}
	answer := "nova resposta"

	cache.On("Get", ctx, mock.AnythingOfType("string")).Return("", false)
	cache.On("Set", ctx, mock.AnythingOfType("string"), answer).Return()
	sessions.On("Get", ctx, "slack:U1").Return(expired, nil)
	sessions.On("Save", ctx, mock.MatchedBy(func(s domain.Session) bool {
		return len(s.History) == 1 // apenas a nova pergunta, histórico antigo descartado
	})).Return(nil)
	embedder.On("Embed", ctx, mock.AnythingOfType("string")).Return(embedding, nil)
	store.On("Search", ctx, embedding, 4, (map[string]string)(nil)).Return([]domain.RetrievedChunk{}, nil)
	llm.On("Generate", ctx, mock.AnythingOfType("string")).Return(answer, nil)

	uc := usecase.NewRAGQueryUseCase(sessions, embedder, store, llm, cache, log)
	result, err := uc.Execute(ctx, domain.Message{Text: "nova pergunta", Sender: domain.Sender{Channel: "slack", UserID: "U1"}}, "slack:U1")

	assert.NoError(t, err)
	assert.Equal(t, answer, result.Answer)
	sessions.AssertExpectations(t)
}

func TestRAGQuery_ExistingSession_HistoryInPrompt(t *testing.T) {
	ctx := context.Background()
	sessions := new(mocks.SessionRepositoryMock)
	embedder := new(mocks.EmbedderMock)
	store := new(mocks.VectorStoreMock)
	llm := new(mocks.LLMClientMock)
	cache := new(mocks.ResponseCacheMock)
	log := new(mocks.LoggerMock)

	existing := &domain.Session{
		SessionID: "slack:U1",
		Sender:    domain.Sender{Channel: "slack", UserID: "U1"},
		History: []domain.ConversationTurn{
			{Question: "primeira pergunta", Answer: "primeira resposta", Timestamp: time.Now()},
		},
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}
	embedding := []float32{0.5}
	answer := "segunda resposta"

	cache.On("Get", ctx, mock.AnythingOfType("string")).Return("", false)
	cache.On("Set", ctx, mock.AnythingOfType("string"), answer).Return()
	sessions.On("Get", ctx, "slack:U1").Return(existing, nil)
	sessions.On("Save", ctx, mock.AnythingOfType("domain.Session")).Return(nil)
	embedder.On("Embed", ctx, mock.AnythingOfType("string")).Return(embedding, nil)
	store.On("Search", ctx, embedding, 4, (map[string]string)(nil)).Return([]domain.RetrievedChunk{}, nil)
	llm.On("Generate", ctx, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "primeira pergunta") && strings.Contains(prompt, "primeira resposta")
	})).Return(answer, nil)

	uc := usecase.NewRAGQueryUseCase(sessions, embedder, store, llm, cache, log)
	result, err := uc.Execute(ctx, domain.Message{Text: "segunda pergunta", Sender: domain.Sender{Channel: "slack", UserID: "U1"}}, "slack:U1")

	assert.NoError(t, err)
	assert.Equal(t, answer, result.Answer)
	llm.AssertExpectations(t)
}
