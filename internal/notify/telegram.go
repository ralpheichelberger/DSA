package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type TelegramNotifier struct {
	botToken string
	chatID   string
	baseURL  string
}

func NewTelegramNotifier(botToken, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: botToken,
		chatID:   chatID,
		baseURL:  "https://api.telegram.org",
	}
}

func NewTelegramNotifierWithBaseURL(botToken, chatID, baseURL string) *TelegramNotifier {
	return &TelegramNotifier{botToken: botToken, chatID: chatID, baseURL: baseURL}
}

func (t *TelegramNotifier) Name() string { return "telegram" }
func (t *TelegramNotifier) Configured() bool {
	return t.botToken != "" && t.chatID != ""
}

func (t *TelegramNotifier) Send(ctx context.Context, n Notification) error {
	var icon string
	switch SeverityLevel(n.Severity) {
	case 2:
		icon = "🔴"
	case 1:
		icon = "🟡"
	default:
		icon = "🟢"
	}

	var contextLines []string
	for k, v := range n.Context {
		contextLines = append(contextLines, fmt.Sprintf("%s: %s", k, v))
	}
	text := fmt.Sprintf("%s *[%s]* %s\n\n%s", icon, stringsUpper(n.Severity), n.Subject, n.Body)
	if len(contextLines) > 0 {
		text += "\n\n" + strings.Join(contextLines, "\n")
	}

	payload := map[string]any{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(t.baseURL, "/"), t.botToken)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram notifier status: %d", resp.StatusCode)
	}
	return nil
}
