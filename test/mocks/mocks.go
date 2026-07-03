package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
)

type EmbedderMock struct{ mock.Mock }

func (m *EmbedderMock) Embed(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	emb, _ := args.Get(0).([]float32)
	return emb, args.Error(1)
}

type VectorStoreMock struct{ mock.Mock }

func (m *VectorStoreMock) IndexChunks(ctx context.Context, chunks []domain.Chunk) error {
	return m.Called(ctx, chunks).Error(0)
}

func (m *VectorStoreMock) Search(ctx context.Context, emb []float32, topK int, filters map[string]string) ([]domain.RetrievedChunk, error) {
	args := m.Called(ctx, emb, topK, filters)
	res, _ := args.Get(0).([]domain.RetrievedChunk)
	return res, args.Error(1)
}

type LLMClientMock struct{ mock.Mock }

func (m *LLMClientMock) Generate(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

type SessionRepositoryMock struct{ mock.Mock }

func (m *SessionRepositoryMock) Get(ctx context.Context, sessionID string) (*domain.Session, error) {
	args := m.Called(ctx, sessionID)
	s, _ := args.Get(0).(*domain.Session)
	return s, args.Error(1)
}

func (m *SessionRepositoryMock) Save(ctx context.Context, session domain.Session) error {
	return m.Called(ctx, session).Error(0)
}

type ResponseCacheMock struct{ mock.Mock }

func (m *ResponseCacheMock) Get(ctx context.Context, key string) (string, bool) {
	args := m.Called(ctx, key)
	return args.String(0), args.Bool(1)
}

func (m *ResponseCacheMock) Set(ctx context.Context, key string, value string) {
	m.Called(ctx, key, value)
}

type DocumentRepositoryMock struct{ mock.Mock }

func (m *DocumentRepositoryMock) Save(ctx context.Context, doc domain.Document) error {
	return m.Called(ctx, doc).Error(0)
}

func (m *DocumentRepositoryMock) FindByHash(ctx context.Context, hash string) (*domain.Document, error) {
	args := m.Called(ctx, hash)
	d, _ := args.Get(0).(*domain.Document)
	return d, args.Error(1)
}

func (m *DocumentRepositoryMock) UpdateStatus(ctx context.Context, id string, status domain.DocumentStatus) error {
	return m.Called(ctx, id, status).Error(0)
}

type MessageQueueMock struct{ mock.Mock }

func (m *MessageQueueMock) Publish(ctx context.Context, queue string, payload []byte) error {
	return m.Called(ctx, queue, payload).Error(0)
}

type LoggerMock struct{ mock.Mock }

func (m *LoggerMock) Info(msg string, kv ...interface{})  { m.Called(msg, kv) }
func (m *LoggerMock) Error(msg string, kv ...interface{}) { m.Called(msg, kv) }
func (m *LoggerMock) Debug(msg string, kv ...interface{}) { m.Called(msg, kv) }

type ChannelAdapterMock struct{ mock.Mock }

func (m *ChannelAdapterMock) ChannelName() string {
	return m.Called().String(0)
}

func (m *ChannelAdapterMock) Send(ctx context.Context, sender domain.Sender, text string) error {
	return m.Called(ctx, sender, text).Error(0)
}

type RAGQueryerMock struct{ mock.Mock }

func (m *RAGQueryerMock) Execute(ctx context.Context, msg domain.Message, sessionID string) (domain.RAGAnswer, error) {
	args := m.Called(ctx, msg, sessionID)
	return args.Get(0).(domain.RAGAnswer), args.Error(1)
}
