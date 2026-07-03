package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	"github.com/danubiobwm/chatbot-rag-go/pkg/logger"
)

const sessionTTL = 30 * time.Minute

// RAGQueryUseCase é o núcleo conversacional: recebe uma pergunta, busca
// contexto relevante, monta o prompt e gera a resposta — com cache e
// gestão de sessão pelo caminho.
type RAGQueryUseCase struct {
	sessions domain.SessionRepository
	embedder domain.Embedder
	store    domain.VectorStore
	llm      domain.LLMClient
	cache    domain.ResponseCache
	log      logger.Logger
	topK     int
}

func NewRAGQueryUseCase(
	sessions domain.SessionRepository,
	embedder domain.Embedder,
	store domain.VectorStore,
	llm domain.LLMClient,
	cache domain.ResponseCache,
	log logger.Logger,
) *RAGQueryUseCase {
	return &RAGQueryUseCase{
		sessions: sessions, embedder: embedder, store: store,
		llm: llm, cache: cache, log: log, topK: 4,
	}
}

func (uc *RAGQueryUseCase) Execute(ctx context.Context, msg domain.Message, sessionID string) (domain.RAGAnswer, error) {
	cacheKey := cacheKeyFor(msg.Text)
	if uc.cache != nil {
		if cached, ok := uc.cache.Get(ctx, cacheKey); ok {
			uc.log.Info("cache hit", "question", msg.Text)
			return domain.RAGAnswer{Answer: cached, FromCache: true}, nil
		}
	}

	session, err := uc.loadOrCreateSession(ctx, sessionID, msg.Sender)
	if err != nil {
		return domain.RAGAnswer{}, fmt.Errorf("loading session: %w", err)
	}

	queryEmbedding, err := uc.embedder.Embed(ctx, msg.Text)
	if err != nil {
		return domain.RAGAnswer{}, fmt.Errorf("embedding query: %w", err)
	}

	retrieved, err := uc.store.Search(ctx, queryEmbedding, uc.topK, nil)
	if err != nil {
		return domain.RAGAnswer{}, fmt.Errorf("searching vector store: %w", err)
	}

	prompt := buildPrompt(session, msg.Text, retrieved)

	answer, err := uc.llm.Generate(ctx, prompt)
	if err != nil {
		return domain.RAGAnswer{}, fmt.Errorf("generating answer: %w", err)
	}

	session.History = append(session.History, domain.ConversationTurn{
		Question: msg.Text, Answer: answer, Timestamp: time.Now(),
	})
	session.ExpiresAt = time.Now().Add(sessionTTL)
	if err := uc.sessions.Save(ctx, *session); err != nil {
		uc.log.Error("failed to persist session", "sessionID", session.SessionID, "error", err)
	}

	if uc.cache != nil {
		uc.cache.Set(ctx, cacheKey, answer)
	}

	return domain.RAGAnswer{Answer: answer, SourceChunks: retrieved}, nil
}

func (uc *RAGQueryUseCase) loadOrCreateSession(ctx context.Context, sessionID string, sender domain.Sender) (*domain.Session, error) {
	session, err := uc.sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || time.Now().After(session.ExpiresAt) {
		return &domain.Session{
			SessionID: sessionID,
			Sender:    sender,
			History:   []domain.ConversationTurn{},
			ExpiresAt: time.Now().Add(sessionTTL),
		}, nil
	}
	return session, nil
}

func buildPrompt(session *domain.Session, question string, retrieved []domain.RetrievedChunk) string {
	var b strings.Builder
	b.WriteString("Responda apenas com base no contexto abaixo. Se não souber, diga que não encontrou a informação.\n\n")
	b.WriteString("Contexto:\n")
	for _, r := range retrieved {
		b.WriteString("- ")
		b.WriteString(r.Chunk.Content)
		b.WriteString("\n")
	}
	if len(session.History) > 0 {
		b.WriteString("\nHistórico recente:\n")
		start := 0
		if len(session.History) > 3 {
			start = len(session.History) - 3
		}
		for _, turn := range session.History[start:] {
			b.WriteString(fmt.Sprintf("P: %s\nR: %s\n", turn.Question, turn.Answer))
		}
	}
	b.WriteString("\nPergunta: ")
	b.WriteString(question)
	return b.String()
}

func cacheKeyFor(question string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(question))))
	return hex.EncodeToString(h[:])
}
