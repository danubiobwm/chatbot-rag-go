package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
)

// DynamoDocumentRepository implementa domain.DocumentRepository.
// A consulta por hash usa um GSI (content_hash-index) — é isso que
// torna a deduplicação O(1) em vez de varrer a tabela inteira.
type DynamoDocumentRepository struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoDocumentRepository(client *dynamodb.Client, table string) *DynamoDocumentRepository {
	return &DynamoDocumentRepository{client: client, table: table}
}

func (r *DynamoDocumentRepository) Save(ctx context.Context, doc domain.Document) error {
	item, err := attributevalue.MarshalMap(doc)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(r.table), Item: item})
	return err
}

func (r *DynamoDocumentRepository) FindByHash(ctx context.Context, hash string) (*domain.Document, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		IndexName:               aws.String("content_hash-index"),
		KeyConditionExpression: aws.String("content_hash = :h"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":h": &types.AttributeValueMemberS{Value: hash},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("query by hash: %w", err)
	}
	if len(out.Items) == 0 {
		return nil, nil
	}
	var doc domain.Document
	if err := attributevalue.UnmarshalMap(out.Items[0], &doc); err != nil {
		return nil, fmt.Errorf("unmarshal document: %w", err)
	}
	return &doc, nil
}

func (r *DynamoDocumentRepository) UpdateStatus(ctx context.Context, id string, status domain.DocumentStatus) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		UpdateExpression: aws.String("SET #s = :s, updated_at = :u"),
		ExpressionAttributeNames: map[string]string{"#s": "status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":s": &types.AttributeValueMemberS{Value: string(status)},
			":u": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})
	return err
}

// DynamoSessionRepository implementa domain.SessionRepository, com TTL
// nativo do DynamoDB (atributo expires_at) — o item se autodestrói,
// sem precisar de um job de limpeza.
type DynamoSessionRepository struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoSessionRepository(client *dynamodb.Client, table string) *DynamoSessionRepository {
	return &DynamoSessionRepository{client: client, table: table}
}

func (r *DynamoSessionRepository) Get(ctx context.Context, sessionID string) (*domain.Session, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"session_id": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}
	var session domain.Session
	if err := attributevalue.UnmarshalMap(out.Item, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

func (r *DynamoSessionRepository) Save(ctx context.Context, session domain.Session) error {
	item, err := attributevalue.MarshalMap(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	// expires_at em epoch seconds é o que o DynamoDB TTL espera.
	item["expires_at"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", session.ExpiresAt.Unix())}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(r.table), Item: item})
	return err
}
