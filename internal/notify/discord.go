package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DiscordNotifier struct{ webhookURL string }

func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{webhookURL: webhookURL}
}
func (d *DiscordNotifier) Name() string     { return "discord" }
func (d *DiscordNotifier) Configured() bool { return d.webhookURL != "" }

func (d *DiscordNotifier) Send(ctx context.Context, n Notification) error {
	fields := make([]map[string]any, 0, len(n.Context))
	for k, v := range n.Context {
		fields = append(fields, map[string]any{"name": k, "value": v, "inline": true})
	}
	payload := map[string]any{
		"embeds": []map[string]any{
			{
				"title":       fmt.Sprintf("[%s] %s", stringsUpper(n.Severity), n.Subject),
				"description": n.Body,
				"color":       discordColor(n.Severity),
				"fields":      fields,
				"footer":      map[string]any{"text": fmt.Sprintf("Dropship Agent • %s", n.Timestamp.Format(time.RFC3339))},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, d.webhookURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord notifier status: %d", resp.StatusCode)
	}
	return nil
}

func discordColor(severity string) int {
	switch SeverityLevel(severity) {
	case 2:
		return 16711680
	case 1:
		return 16753920
	default:
		return 3394611
	}
}
