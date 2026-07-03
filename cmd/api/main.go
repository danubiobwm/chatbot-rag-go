// cmd/api é o "composition root": o único lugar do sistema onde
// implementações concretas (AWS, Slack, WhatsApp) são instanciadas e
// amarradas às interfaces que os usecases esperam. Se um dia trocarmos
// Bedrock por OpenAI, é AQUI e só aqui que a linha muda.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/opensearch-project/opensearch-go/v2"

	"github.com/danubiobwm/chatbot-rag-go/internal/config"
	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
	awsinfra "github.com/danubiobwm/chatbot-rag-go/internal/infrastructure/aws"
	"github.com/danubiobwm/chatbot-rag-go/internal/infrastructure/cache"
	"github.com/danubiobwm/chatbot-rag-go/internal/infrastructure/channels"
	"github.com/danubiobwm/chatbot-rag-go/internal/handler"
	"github.com/danubiobwm/chatbot-rag-go/internal/usecase"
	"github.com/danubiobwm/chatbot-rag-go/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	logg, err := logger.New(cfg.Env)
	if err != nil {
		log.Fatalf("creating logger: %v", err)
	}

	ctx := context.Background()
	awsCfg, err := loadAWSConfig(ctx, cfg)
	if err != nil {
		logg.Error("loading aws config", "error", err)
		log.Fatal(err)
	}

	// --- clientes de infraestrutura ---
	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	sqsClient := sqs.NewFromConfig(awsCfg)
	bedrockClient := bedrockruntime.NewFromConfig(awsCfg)
	osClient, err := opensearch.NewClient(opensearch.Config{Addresses: []string{cfg.OpenSearchURL}})
	if err != nil {
		log.Fatalf("creating opensearch client: %v", err)
	}

	// --- implementações concretas das interfaces de domain ---
	docsRepo := awsinfra.NewDynamoDocumentRepository(dynamoClient, cfg.DynamoDocumentsTbl)
	sessionsRepo := awsinfra.NewDynamoSessionRepository(dynamoClient, cfg.DynamoSessionsTbl)
	embedder := awsinfra.NewBedrockEmbedder(bedrockClient)
	llm := awsinfra.NewBedrockLLM(bedrockClient, cfg.BedrockLLMModelID)
	vectorStore := awsinfra.NewOpenSearchVectorStore(osClient, cfg.OpenSearchIndex)
	responseCache := cache.NewInMemoryResponseCache(10 * time.Minute)
	queue := awsinfra.NewSQSMessageQueue(sqsClient, map[string]string{
		"extraction-queue": "http://localhost:4566/000000000000/extraction-queue", // LocalStack em dev
	})

	httpClient := &http.Client{Timeout: 10 * time.Second}
	adapters := []domain.ChannelAdapter{
		channels.NewWhatsAppAdapter(httpClient, cfg.WhatsAppAPIBaseURL, cfg.WhatsAppToken, cfg.WhatsAppPhoneID),
		channels.NewSlackAdapter(httpClient, cfg.SlackBotToken),
	}

	// --- usecases recebem só interfaces (injeção de dependência manual) ---
	ragQuery := usecase.NewRAGQueryUseCase(sessionsRepo, embedder, vectorStore, llm, responseCache, logg)
	router := usecase.NewRouteMessageUseCase(ragQuery, adapters, logg)
	_ = usecase.NewIngestDocumentUseCase(docsRepo, nil, queue, logg) // exposto via outro endpoint/worker, se necessário

	// --- HTTP ---
	mux := http.NewServeMux()
	handler.NewChatHandler(router, logg).RegisterRoutes(mux)

	addr := ":" + cfg.HTTPPort
	logg.Info("starting http server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("http server failed: %v", err)
	}
}

func loadAWSConfig(ctx context.Context, cfg *config.Config) (aws.Config, error) {
	opts := []func(*awscfg.LoadOptions) error{awscfg.WithRegion(cfg.AWSRegion)}
	if cfg.AWSEndpointURL != "" {
		opts = append(opts, awscfg.WithBaseEndpoint(cfg.AWSEndpointURL))
	}
	return awscfg.LoadDefaultConfig(ctx, opts...)
}
