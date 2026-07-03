// Package domain contém as entidades centrais do negócio.
// Regra de ouro do Clean Architecture: este pacote NUNCA importa
// nada de internal/infrastructure. Ele não sabe o que é S3, SQS,
// Bedrock ou OpenSearch. Isso é o que permite trocar AWS por
// LocalStack, ou Bedrock por OpenAI, sem tocar em uma linha aqui.
package domain

import "time"

// DocumentStatus representa o estágio do documento no pipeline.
type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "PENDING"
	DocumentStatusExtracting DocumentStatus = "EXTRACTING"
	DocumentStatusSegmenting DocumentStatus = "SEGMENTING"
	DocumentStatusEmbedding  DocumentStatus = "EMBEDDING"
	DocumentStatusDone       DocumentStatus = "DONE"
	DocumentStatusFailed     DocumentStatus = "FAILED"
)

// Document representa um arquivo de origem (PDF de RH, jurídico etc).
type Document struct {
	ID          string
	SourceKey   string // chave no bucket de origem
	ContentHash string // hash do conteúdo, usado para deduplicação
	Category    string // ex: "rh", "juridico"
	Status      DocumentStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Chunk é um pedaço de texto segmentado de um documento, pronto para embedding.
type Chunk struct {
	ID         string
	DocumentID string
	Content    string
	Order      int
	Embedding  []float32
	Metadata   map[string]string
}

// Sender identifica de onde veio uma mensagem de chat.
type Sender struct {
	Channel string // "whatsapp", "slack", "teams"
	UserID  string
}

// Message é uma mensagem recebida de um usuário em qualquer canal.
type Message struct {
	ID        string
	Sender    Sender
	Text      string
	Timestamp time.Time
}

// ConversationTurn é um par pergunta/resposta guardado na sessão.
type ConversationTurn struct {
	Question  string
	Answer    string
	Timestamp time.Time
}

// Session guarda o contexto curto de uma conversa, com expiração (TTL).
type Session struct {
	SessionID string
	Sender    Sender
	History   []ConversationTurn
	ExpiresAt time.Time
}

// RetrievedChunk é um chunk retornado pela busca vetorial, com score de relevância.
type RetrievedChunk struct {
	Chunk Chunk
	Score float64
}

// RAGAnswer é o resultado final entregue ao usuário.
type RAGAnswer struct {
	Answer       string
	SourceChunks []RetrievedChunk
	FromCache    bool
}
