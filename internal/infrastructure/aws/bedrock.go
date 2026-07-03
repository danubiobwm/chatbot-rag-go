package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// BedrockEmbedder implementa domain.Embedder usando o Titan Text Embeddings v2.
type BedrockEmbedder struct {
	client  *bedrockruntime.Client
	modelID string
}

func NewBedrockEmbedder(client *bedrockruntime.Client) *BedrockEmbedder {
	return &BedrockEmbedder{client: client, modelID: "amazon.titan-embed-text-v2:0"}
}

type titanEmbedRequest struct {
	InputText string `json:"inputText"`
}

type titanEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (e *BedrockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(titanEmbedRequest{InputText: text})
	if err != nil {
		return nil, fmt.Errorf("marshal titan request: %w", err)
	}

	out, err := e.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(e.modelID),
		ContentType: aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock InvokeModel (embedding): %w", err)
	}

	var resp titanEmbedResponse
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal titan response: %w", err)
	}
	return resp.Embedding, nil
}

// BedrockLLM implementa domain.LLMClient para geração de texto (ex: Claude/Llama via Bedrock).
type BedrockLLM struct {
	client  *bedrockruntime.Client
	modelID string
}

func NewBedrockLLM(client *bedrockruntime.Client, modelID string) *BedrockLLM {
	return &BedrockLLM{client: client, modelID: modelID}
}

type bedrockGenericRequest struct {
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

type bedrockGenericResponse struct {
	Completion string `json:"completion"`
}

func (l *BedrockLLM) Generate(ctx context.Context, prompt string) (string, error) {
	reqBody, err := json.Marshal(bedrockGenericRequest{
		Prompt:      prompt,
		MaxTokens:   1024,
		Temperature: 0.2,
	})
	if err != nil {
		return "", fmt.Errorf("marshal bedrock request: %w", err)
	}

	out, err := l.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(l.modelID),
		ContentType: aws.String("application/json"),
		Body:        reqBody,
	})
	if err != nil {
		return "", fmt.Errorf("bedrock InvokeModel (generate): %w", err)
	}

	var resp bedrockGenericResponse
	if err := json.NewDecoder(bytes.NewReader(out.Body)).Decode(&resp); err != nil {
		return "", fmt.Errorf("unmarshal bedrock response: %w", err)
	}
	return resp.Completion, nil
}
