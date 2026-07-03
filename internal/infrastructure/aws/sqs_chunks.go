package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
)

// SQSMessageQueue implementa domain.MessageQueue. A política de
// redrive (quantas tentativas até cair na DLQ) é configurada na
// infraestrutura (Terraform/CDK), não no código — o Go só publica.
type SQSMessageQueue struct {
	client    *sqs.Client
	queueURLs map[string]string
}

func NewSQSMessageQueue(client *sqs.Client, queueURLs map[string]string) *SQSMessageQueue {
	return &SQSMessageQueue{client: client, queueURLs: queueURLs}
}

func (q *SQSMessageQueue) Publish(ctx context.Context, queue string, payload []byte) error {
	url, ok := q.queueURLs[queue]
	if !ok {
		return fmt.Errorf("unknown queue %q", queue)
	}
	_, err := q.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(url),
		MessageBody: aws.String(string(payload)),
	})
	if err != nil {
		return fmt.Errorf("sqs SendMessage: %w", err)
	}
	return nil
}

// DynamoChunkRepository implementa domain.ChunkRepository.
type DynamoChunkRepository struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoChunkRepository(client *dynamodb.Client, table string) *DynamoChunkRepository {
	return &DynamoChunkRepository{client: client, table: table}
}

func (r *DynamoChunkRepository) SaveBatch(ctx context.Context, chunks []domain.Chunk) error {
	// DynamoDB BatchWriteItem aceita no máximo 25 itens por chamada.
	const batchLimit = 25
	for start := 0; start < len(chunks); start += batchLimit {
		end := start + batchLimit
		if end > len(chunks) {
			end = len(chunks)
		}
		writeRequests := make([]types.WriteRequest, 0, end-start)
		for _, c := range chunks[start:end] {
			item, err := attributevalue.MarshalMap(c)
			if err != nil {
				return fmt.Errorf("marshal chunk %s: %w", c.ID, err)
			}
			writeRequests = append(writeRequests, types.WriteRequest{
				PutRequest: &types.PutRequest{Item: item},
			})
		}
		_, err := r.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{r.table: writeRequests},
		})
		if err != nil {
			return fmt.Errorf("batch write chunks: %w", err)
		}
	}
	return nil
}
