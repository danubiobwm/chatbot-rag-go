package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/danubiobwm/chatbot-rag-go/internal/domain"
)

// ---------------------------------------------------------------------------
// Cada canal novo = um arquivo novo implementando domain.ChannelAdapter.
// Nada na camada de usecase muda quando adicionamos Teams, Telegram etc.
// Isso é o Open/Closed Principle na prática.
// ---------------------------------------------------------------------------

// WhatsAppAdapter envia mensagens via WhatsApp Business API (Cloud API).
type WhatsAppAdapter struct {
	httpClient  *http.Client
	apiBaseURL  string
	accessToken string
	phoneID     string
}

func NewWhatsAppAdapter(httpClient *http.Client, apiBaseURL, accessToken, phoneID string) *WhatsAppAdapter {
	return &WhatsAppAdapter{httpClient: httpClient, apiBaseURL: apiBaseURL, accessToken: accessToken, phoneID: phoneID}
}

func (a *WhatsAppAdapter) ChannelName() string { return "whatsapp" }

func (a *WhatsAppAdapter) Send(ctx context.Context, sender domain.Sender, text string) error {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                 sender.UserID,
		"type":               "text",
		"text":               map[string]string{"body": text},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal whatsapp payload: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", a.apiBaseURL, a.phoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build whatsapp request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send whatsapp message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("whatsapp api returned status %d", resp.StatusCode)
	}
	return nil
}

// SlackAdapter envia mensagens via Slack Web API (chat.postMessage).
type SlackAdapter struct {
	httpClient *http.Client
	botToken   string
}

func NewSlackAdapter(httpClient *http.Client, botToken string) *SlackAdapter {
	return &SlackAdapter{httpClient: httpClient, botToken: botToken}
}

func (a *SlackAdapter) ChannelName() string { return "slack" }

func (a *SlackAdapter) Send(ctx context.Context, sender domain.Sender, text string) error {
	payload := map[string]string{"channel": sender.UserID, "text": text}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build slack request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.botToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack api returned status %d", resp.StatusCode)
	}
	return nil
}
