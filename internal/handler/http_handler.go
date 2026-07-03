package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	"github.com/danubiobwm/chatbot-rag-go/internal/usecase"
	"github.com/danubiobwm/chatbot-rag-go/pkg/logger"
)

// inboundMessage é o formato normalizado que todo webhook de canal deve
// converter para, antes de chegar aqui. O handler não conhece o formato
// nativo do WhatsApp ou do Slack — isso fica nos adapters de entrada.
type inboundMessage struct {
	Channel string `json:"channel"`
	UserID  string `json:"user_id"`
	Text    string `json:"text"`
}

type ChatHandler struct {
	router *usecase.RouteMessageUseCase
	log    logger.Logger
}

func NewChatHandler(router *usecase.RouteMessageUseCase, log logger.Logger) *ChatHandler {
	return &ChatHandler{router: router, log: log}
}

func (h *ChatHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/webhook/message", h.handleInboundMessage)
}

func (h *ChatHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *ChatHandler) handleInboundMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var in inboundMessage
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if in.Channel == "" || in.UserID == "" || in.Text == "" {
		http.Error(w, "channel, user_id and text are required", http.StatusBadRequest)
		return
	}

	msg := domain.Message{
		ID:        uuid.NewString(),
		Sender:    domain.Sender{Channel: in.Channel, UserID: in.UserID},
		Text:      in.Text,
		Timestamp: time.Now(),
	}

	if err := h.router.Execute(r.Context(), msg); err != nil {
		h.log.Error("failed to route message", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
