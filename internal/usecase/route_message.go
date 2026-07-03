package usecase

import (
	"context"
	"fmt"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	"github.com/danubiobwm/chatbot-rag-go/pkg/logger"
)

// RouteMessageUseCase recebe uma mensagem já normalizada (independente do
// canal de origem) e decide o que fazer com ela. Hoje só existe o fluxo
// de pergunta -> RAG, mas é aqui que entrariam comandos como "resetar
// conversa" sem precisar tocar nos adapters de canal (Open/Closed).
type RouteMessageUseCase struct {
	ragQuery *RAGQueryUseCase
	channels map[string]domain.ChannelAdapter
	log      logger.Logger
}

func NewRouteMessageUseCase(ragQuery *RAGQueryUseCase, adapters []domain.ChannelAdapter, log logger.Logger) *RouteMessageUseCase {
	byName := make(map[string]domain.ChannelAdapter, len(adapters))
	for _, a := range adapters {
		byName[a.ChannelName()] = a
	}
	return &RouteMessageUseCase{ragQuery: ragQuery, channels: byName, log: log}
}

func (uc *RouteMessageUseCase) Execute(ctx context.Context, msg domain.Message) error {
	adapter, ok := uc.channels[msg.Sender.Channel]
	if !ok {
		return fmt.Errorf("no adapter registered for channel %q", msg.Sender.Channel)
	}

	sessionID := fmt.Sprintf("%s:%s", msg.Sender.Channel, msg.Sender.UserID)

	result, err := uc.ragQuery.Execute(ctx, msg, sessionID)
	if err != nil {
		uc.log.Error("rag query failed", "sessionID", sessionID, "error", err)
		return adapter.Send(ctx, msg.Sender, "Desculpe, tive um problema para responder agora. Tente novamente em instantes.")
	}

	return adapter.Send(ctx, msg.Sender, result.Answer)
}
