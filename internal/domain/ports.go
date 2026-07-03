package domain

import "context"

// ---------------------------------------------------------------------------
// Cada interface aqui é pequena e focada em UMA responsabilidade (ISP).
// Os usecases dependem só dessas abstrações, nunca de implementações
// concretas (DIP). Quem implementa fica em internal/infrastructure/*.
// ---------------------------------------------------------------------------

// TextExtractor extrai texto bruto de um documento (ex: via Textract).
type TextExtractor interface {
	Extract(ctx context.Context, doc Document) (rawText string, err error)
}

// Segmenter divide texto bruto em chunks coerentes.
// Implementações podem usar LLM com fallback determinístico — essa decisão
// é escondida atrás da interface; o usecase não sabe nem precisa saber.
type Segmenter interface {
	Segment(ctx context.Context, documentID string, rawText string) ([]Chunk, error)
}

// Embedder gera vetores de embedding para um texto.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// VectorStore é a abstração da busca semântica (ex: OpenSearch).
type VectorStore interface {
	IndexChunks(ctx context.Context, chunks []Chunk) error
	Search(ctx context.Context, queryEmbedding []float32, topK int, filters map[string]string) ([]RetrievedChunk, error)
}

// LLMClient é a abstração de um modelo de linguagem generativo (ex: Bedrock).
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// DocumentRepository persiste metadados de documentos e permite checar duplicidade.
type DocumentRepository interface {
	Save(ctx context.Context, doc Document) error
	FindByHash(ctx context.Context, hash string) (*Document, error)
	UpdateStatus(ctx context.Context, id string, status DocumentStatus) error
}

// ChunkRepository persiste chunks segmentados (antes/depois do embedding).
type ChunkRepository interface {
	SaveBatch(ctx context.Context, chunks []Chunk) error
}

// SessionRepository guarda sessões de conversa com expiração.
type SessionRepository interface {
	Get(ctx context.Context, sessionID string) (*Session, error)
	Save(ctx context.Context, session Session) error
}

// ResponseCache evita chamar o LLM de novo para perguntas repetidas.
type ResponseCache interface {
	Get(ctx context.Context, key string) (string, bool)
	Set(ctx context.Context, key string, value string)
}

// ChannelAdapter normaliza canais de entrada/saída (WhatsApp, Slack, Teams)
// para um formato único. Adicionar um canal novo = implementar essa
// interface, sem tocar no resto do sistema (Open/Closed Principle).
type ChannelAdapter interface {
	ChannelName() string
	Send(ctx context.Context, sender Sender, text string) error
}

// MessageQueue abstrai o sistema de filas (SQS), incluindo DLQ.
type MessageQueue interface {
	Publish(ctx context.Context, queue string, payload []byte) error
}

// Notifier abstrai alertas de observabilidade (ex: alarme quando algo cai na DLQ).
type Notifier interface {
	Notify(ctx context.Context, subject string, message string) error
}
