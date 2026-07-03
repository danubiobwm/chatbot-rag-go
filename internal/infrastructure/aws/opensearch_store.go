package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
)

// OpenSearchVectorStore implementa domain.VectorStore. Mantemos toda a
// query DSL do OpenSearch isolada aqui — o resto do sistema só conhece
// "Search(embedding, topK, filters) -> []RetrievedChunk".
type OpenSearchVectorStore struct {
	client *opensearch.Client
	index  string
}

func NewOpenSearchVectorStore(client *opensearch.Client, index string) *OpenSearchVectorStore {
	return &OpenSearchVectorStore{client: client, index: index}
}

type indexDoc struct {
	DocumentID string    `json:"document_id"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"embedding"`
}

func (s *OpenSearchVectorStore) IndexChunks(ctx context.Context, chunks []domain.Chunk) error {
	for _, c := range chunks {
		body, err := json.Marshal(indexDoc{
			DocumentID: c.DocumentID,
			Content:    c.Content,
			Embedding:  c.Embedding,
		})
		if err != nil {
			return fmt.Errorf("marshal chunk %s: %w", c.ID, err)
		}

		req := opensearchapi.IndexRequest{
			Index:      s.index,
			DocumentID: c.ID,
			Body:       bytes.NewReader(body),
		}
		res, err := req.Do(ctx, s.client)
		if err != nil {
			return fmt.Errorf("indexing chunk %s: %w", c.ID, err)
		}
		defer res.Body.Close()
		if res.IsError() {
			return fmt.Errorf("opensearch returned error indexing chunk %s: %s", c.ID, res.Status())
		}
	}
	return nil
}

func (s *OpenSearchVectorStore) Search(ctx context.Context, queryEmbedding []float32, topK int, filters map[string]string) ([]domain.RetrievedChunk, error) {
	query := map[string]interface{}{
		"size": topK,
		"query": map[string]interface{}{
			"knn": map[string]interface{}{
				"embedding": map[string]interface{}{
					"vector": queryEmbedding,
					"k":      topK,
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("encode search query: %w", err)
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(&buf),
	)
	if err != nil {
		return nil, fmt.Errorf("opensearch search: %w", err)
	}
	defer res.Body.Close()

	var parsed struct {
		Hits struct {
			Hits []struct {
				ID     string  `json:"_id"`
				Score  float64 `json:"_score"`
				Source indexDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	results := make([]domain.RetrievedChunk, 0, len(parsed.Hits.Hits))
	for _, h := range parsed.Hits.Hits {
		results = append(results, domain.RetrievedChunk{
			Chunk: domain.Chunk{
				ID:         h.ID,
				DocumentID: h.Source.DocumentID,
				Content:    h.Source.Content,
			},
			Score: h.Score,
		})
	}
	return results, nil
}
