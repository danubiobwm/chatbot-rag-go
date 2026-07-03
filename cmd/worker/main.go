// cmd/worker é o segundo composition root: monta as dependências do
// pipeline de processamento (extração -> segmentação -> embedding ->
// indexação) e fica em loop consumindo a fila SQS.
package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/textract"
	"github.com/opensearch-project/opensearch-go/v2"

	"github.com/danubiobwm/chatbot-rag-go/internal/config"
	awsinfra "github.com/danubiobwm/chatbot-rag-go/internal/infrastructure/aws"
	"github.com/danubiobwm/chatbot-rag-go/internal/usecase"
	"github.com/danubiobwm/chatbot-rag-go/pkg/logger"
)

const extractionQueueURL = "http://localhost:4566/000000000000/extraction-queue" // LocalStack em dev

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
	opts := []func(*awscfg.LoadOptions) error{awscfg.WithRegion(cfg.AWSRegion)}
	if cfg.AWSEndpointURL != "" {
		opts = append(opts, awscfg.WithBaseEndpoint(cfg.AWSEndpointURL))
	}
	awsCfg, err := awscfg.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		log.Fatalf("loading aws config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	sqsClient := sqs.NewFromConfig(awsCfg)
	textractClient := textract.NewFromConfig(awsCfg)
	bedrockClient := bedrockruntime.NewFromConfig(awsCfg)
	osClient, err := opensearch.NewClient(opensearch.Config{Addresses: []string{cfg.OpenSearchURL}})
	if err != nil {
		log.Fatalf("creating opensearch client: %v", err)
	}

	docsRepo := awsinfra.NewDynamoDocumentRepository(dynamoClient, cfg.DynamoDocumentsTbl)
	chunksRepo := awsinfra.NewDynamoChunkRepository(dynamoClient, cfg.DynamoChunksTbl)
	extractor := awsinfra.NewTextractExtractor(textractClient, cfg.S3Bucket)
	embedder := awsinfra.NewBedrockEmbedder(bedrockClient)
	llm := awsinfra.NewBedrockLLM(bedrockClient, cfg.BedrockLLMModelID)
	vectorStore := awsinfra.NewOpenSearchVectorStore(osClient, cfg.OpenSearchIndex)

	// Decorator: segmentação por LLM com fallback determinístico embutido.
	fallback := awsinfra.NewFixedSizeSegmenter(300, 50)
	segmenter := awsinfra.NewLLMSegmenter(llm, fallback, logg)

	processDoc := usecase.NewProcessDocumentUseCase(
		docsRepo, chunksRepo, extractor, segmenter, embedder, vectorStore, nil, logg,
	)

	logg.Info("worker started, polling extraction queue")
	for {
		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(extractionQueueURL),
			MaxNumberOfMessages: 5,
			WaitTimeSeconds:     10, // long polling
		})
		if err != nil {
			logg.Error("receive message failed", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, m := range out.Messages {
			documentID := aws.ToString(m.Body)
			doc, err := docsRepo.FindByHash(ctx, documentID) // placeholder simplificado de lookup
			if err != nil || doc == nil {
				logg.Error("document not found for processing", "documentID", documentID)
				continue
			}

			if err := processDoc.Execute(ctx, *doc); err != nil {
				logg.Error("processing failed, message stays in queue for redrive/DLQ", "documentID", documentID, "error", err)
				continue // não deleta — deixa o redrive policy da fila decidir (retry ou DLQ)
			}

			_, _ = sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(extractionQueueURL),
				ReceiptHandle: m.ReceiptHandle,
			})
		}
	}
}
