package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	"github.com/danubiobwm/chatbot-rag-go/internal/usecase"
	"github.com/danubiobwm/chatbot-rag-go/test/mocks"
)

func TestRoute_HappyPath(t *testing.T) {
	ctx := context.Background()
	ragMock := new(mocks.RAGQueryerMock)
	adapter := new(mocks.ChannelAdapterMock)
	log := new(mocks.LoggerMock)

	msg := domain.Message{Text: "qual a política de férias?", Sender: domain.Sender{Channel: "whatsapp", UserID: "55999"}}
	ragResult := domain.RAGAnswer{Answer: "São 30 dias corridos."}

	adapter.On("ChannelName").Return("whatsapp")
	ragMock.On("Execute", ctx, msg, "whatsapp:55999").Return(ragResult, nil)
	adapter.On("Send", ctx, msg.Sender, "São 30 dias corridos.").Return(nil)

	uc := usecase.NewRouteMessageUseCase(ragMock, []domain.ChannelAdapter{adapter}, log)
	err := uc.Execute(ctx, msg)

	assert.NoError(t, err)
	adapter.AssertExpectations(t)
	ragMock.AssertExpectations(t)
}

func TestRoute_UnknownChannel(t *testing.T) {
	ctx := context.Background()
	ragMock := new(mocks.RAGQueryerMock)
	log := new(mocks.LoggerMock)

	msg := domain.Message{Text: "oi", Sender: domain.Sender{Channel: "telegram", UserID: "123"}}

	uc := usecase.NewRouteMessageUseCase(ragMock, []domain.ChannelAdapter{}, log)
	err := uc.Execute(ctx, msg)

	assert.ErrorContains(t, err, "telegram")
	ragMock.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoute_RAGFails_SendsFallback(t *testing.T) {
	ctx := context.Background()
	ragMock := new(mocks.RAGQueryerMock)
	adapter := new(mocks.ChannelAdapterMock)
	log := new(mocks.LoggerMock)

	msg := domain.Message{Text: "pergunta", Sender: domain.Sender{Channel: "slack", UserID: "U2"}}

	adapter.On("ChannelName").Return("slack")
	ragMock.On("Execute", ctx, msg, "slack:U2").Return(domain.RAGAnswer{}, errors.New("llm failed"))
	log.On("Error", mock.Anything, mock.Anything).Return()
	adapter.On("Send", ctx, msg.Sender, "Desculpe, tive um problema para responder agora. Tente novamente em instantes.").Return(nil)

	uc := usecase.NewRouteMessageUseCase(ragMock, []domain.ChannelAdapter{adapter}, log)
	err := uc.Execute(ctx, msg)

	assert.NoError(t, err)
	adapter.AssertExpectations(t)
	log.AssertExpectations(t)
}

func TestRoute_AdapterSendFails(t *testing.T) {
	ctx := context.Background()
	ragMock := new(mocks.RAGQueryerMock)
	adapter := new(mocks.ChannelAdapterMock)
	log := new(mocks.LoggerMock)

	msg := domain.Message{Text: "oi", Sender: domain.Sender{Channel: "whatsapp", UserID: "55999"}}

	adapter.On("ChannelName").Return("whatsapp")
	ragMock.On("Execute", ctx, msg, "whatsapp:55999").Return(domain.RAGAnswer{Answer: "tudo bem"}, nil)
	adapter.On("Send", ctx, msg.Sender, "tudo bem").Return(errors.New("network error"))

	uc := usecase.NewRouteMessageUseCase(ragMock, []domain.ChannelAdapter{adapter}, log)
	err := uc.Execute(ctx, msg)

	assert.ErrorContains(t, err, "network error")
}

func TestRoute_MultipleAdapters_RoutesToCorrectOne(t *testing.T) {
	ctx := context.Background()
	ragMock := new(mocks.RAGQueryerMock)
	whatsapp := new(mocks.ChannelAdapterMock)
	slack := new(mocks.ChannelAdapterMock)
	log := new(mocks.LoggerMock)

	msg := domain.Message{Text: "oi", Sender: domain.Sender{Channel: "slack", UserID: "U9"}}

	whatsapp.On("ChannelName").Return("whatsapp")
	slack.On("ChannelName").Return("slack")
	ragMock.On("Execute", ctx, msg, "slack:U9").Return(domain.RAGAnswer{Answer: "olá"}, nil)
	slack.On("Send", ctx, msg.Sender, "olá").Return(nil)

	uc := usecase.NewRouteMessageUseCase(ragMock, []domain.ChannelAdapter{whatsapp, slack}, log)
	err := uc.Execute(ctx, msg)

	assert.NoError(t, err)
	whatsapp.AssertNotCalled(t, "Send", mock.Anything, mock.Anything, mock.Anything)
	slack.AssertExpectations(t)
}
