package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config centraliza tudo que vem do ambiente. Nenhum outro pacote lê
// os.Getenv diretamente — isso evita configuração espalhada pelo código
// e facilita testar com valores fixos.
type Config struct {
	Env            string
	AWSRegion      string
	AWSEndpointURL string // usado para apontar para o LocalStack em dev

	S3Bucket           string
	DynamoDocumentsTbl string
	DynamoChunksTbl    string
	DynamoSessionsTbl  string
	OpenSearchIndex    string
	OpenSearchURL      string

	BedrockLLMModelID string

	// Ollama — quando OllamaBaseURL estiver definido, Embedder e LLM usam Ollama em vez de Bedrock.
	OllamaBaseURL    string
	OllamaEmbedModel string
	OllamaLLMModel   string

	SlackBotToken      string
	WhatsAppToken      string
	WhatsAppPhoneID    string
	WhatsAppAPIBaseURL string

	HTTPPort string
}

// loadDotEnv reads .env from the working directory and sets any variable that
// is not already present in the environment. It silently skips missing files.
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
	_ = scanner.Err()
}

func Load() (*Config, error) {
	loadDotEnv()
	cfg := &Config{
		Env:                getEnv("ENV", "development"),
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
		AWSEndpointURL:     os.Getenv("AWS_ENDPOINT_URL"), // vazio = AWS real
		S3Bucket:           getEnv("S3_BUCKET", "chatbot-regulatorio-docs"),
		DynamoDocumentsTbl: getEnv("DYNAMO_DOCUMENTS_TABLE", "ChatbotDocuments"),
		DynamoChunksTbl:    getEnv("DYNAMO_CHUNKS_TABLE", "ChatbotChunks"),
		DynamoSessionsTbl:  getEnv("DYNAMO_SESSIONS_TABLE", "ChatbotSessions"),
		OpenSearchIndex:    getEnv("OPENSEARCH_INDEX", "chatbot-regulatorio"),
		OpenSearchURL:      getEnv("OPENSEARCH_URL", "http://localhost:9200"),
		BedrockLLMModelID:  getEnv("BEDROCK_LLM_MODEL_ID", "anthropic.claude-3-haiku-20240307-v1:0"),
		OllamaBaseURL:      os.Getenv("OLLAMA_BASE_URL"),
		OllamaEmbedModel:   getEnv("OLLAMA_EMBED_MODEL", "mxbai-embed-large"),
		OllamaLLMModel:     getEnv("OLLAMA_LLM_MODEL", "llama3.2"),
		SlackBotToken:      os.Getenv("SLACK_BOT_TOKEN"),
		WhatsAppToken:      os.Getenv("WHATSAPP_TOKEN"),
		WhatsAppPhoneID:    os.Getenv("WHATSAPP_PHONE_ID"),
		WhatsAppAPIBaseURL: getEnv("WHATSAPP_API_BASE_URL", "https://graph.facebook.com/v19.0"),
		HTTPPort:           getEnv("HTTP_PORT", "8080"),
	}
	return cfg, cfg.validate()
}

func (c *Config) validate() error {
	if c.AWSRegion == "" {
		return fmt.Errorf("AWS_REGION is required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
