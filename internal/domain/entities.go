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
	ID          string         `dynamodbav:"id"`
	SourceKey   string         `dynamodbav:"source_key"`
	ContentHash string         `dynamodbav:"content_hash"`
	Category    string         `dynamodbav:"category"`
	Status      DocumentStatus `dynamodbav:"status"`
	CreatedAt   time.Time      `dynamodbav:"created_at"`
	UpdatedAt   time.Time      `dynamodbav:"updated_at"`
}

// Chunk é um pedaço de texto segmentado de um documento, pronto para embedding.
type Chunk struct {
	ID         string            `dynamodbav:"id"`
	DocumentID string            `dynamodbav:"document_id"`
	Content    string            `dynamodbav:"content"`
	Order      int               `dynamodbav:"order"`
	Embedding  []float32         `dynamodbav:"embedding"`
	Metadata   map[string]string `dynamodbav:"metadata"`
}

// Sender identifica de onde veio uma mensagem de chat.
type Sender struct {
	Channel string `dynamodbav:"channel"` // "whatsapp", "slack", "teams"
	UserID  string `dynamodbav:"user_id"`
}

// Message é uma mensagem recebida de um usuário em qualquer canal.
type Message struct {
	ID        string    `dynamodbav:"id"`
	Sender    Sender    `dynamodbav:"sender"`
	Text      string    `dynamodbav:"text"`
	Timestamp time.Time `dynamodbav:"timestamp"`
}

// ConversationTurn é um par pergunta/resposta guardado na sessão.
type ConversationTurn struct {
	Question  string    `dynamodbav:"question"`
	Answer    string    `dynamodbav:"answer"`
	Timestamp time.Time `dynamodbav:"timestamp"`
}

// Session guarda o contexto curto de uma conversa, com expiração (TTL).
type Session struct {
	SessionID string             `dynamodbav:"session_id"`
	Sender    Sender             `dynamodbav:"sender"`
	History   []ConversationTurn `dynamodbav:"history"`
	ExpiresAt time.Time          `dynamodbav:"expires_at"`
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
