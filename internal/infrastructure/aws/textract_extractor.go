package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/textract"
	"github.com/aws/aws-sdk-go-v2/service/textract/types"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
)

// TextractExtractor implementa domain.TextExtractor usando o Textract.
// É a única peça do sistema que sabe que "extração" significa "chamar
// a API da AWS" — o resto do código só conhece a interface.
type TextractExtractor struct {
	client *textract.Client
	bucket string
}

func NewTextractExtractor(client *textract.Client, bucket string) *TextractExtractor {
	return &TextractExtractor{client: client, bucket: bucket}
}

func (e *TextractExtractor) Extract(ctx context.Context, doc domain.Document) (string, error) {
	out, err := e.client.DetectDocumentText(ctx, &textract.DetectDocumentTextInput{
		Document: &types.Document{
			S3Object: &types.S3Object{
				Bucket: aws.String(e.bucket),
				Name:   aws.String(doc.SourceKey),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("textract DetectDocumentText: %w", err)
	}

	var b strings.Builder
	for _, block := range out.Blocks {
		if block.BlockType == types.BlockTypeLine && block.Text != nil {
			b.WriteString(*block.Text)
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

// S3DocumentStore é um wrapper fino sobre o S3, usado para baixar/subir
// artefatos intermediários do pipeline (ex: texto extraído).
type S3DocumentStore struct {
	client *s3.Client
	bucket string
}

func NewS3DocumentStore(client *s3.Client, bucket string) *S3DocumentStore {
	return &S3DocumentStore{client: client, bucket: bucket}
}

func (s *S3DocumentStore) PutObject(ctx context.Context, key string, body []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(string(body)),
	})
	return err
}
