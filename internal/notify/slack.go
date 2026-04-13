package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SlackNotifier struct{ webhookURL string }

func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{webhookURL: webhookURL}
}
func (s *SlackNotifier) Name() string     { return "slack" }
func (s *SlackNotifier) Configured() bool { return s.webhookURL != "" }

func (s *SlackNotifier) Send(ctx context.Context, n Notification) error {
	fields := make([]map[string]any, 0, len(n.Context))
	for k, v := range n.Context {
		fields = append(fields, map[string]any{"title": k, "value": v, "short": true})
	}
	payload := map[string]any{
		"text": fmt.Sprintf("*[%s] %s*\n%s", stringsUpper(n.Severity), n.Subject, n.Body),
		"attachments": []map[string]any{
			{
				"color":  slackColor(n.Severity),
				"fields": fields,
				"footer": "Dropship Agent",
				"ts":     n.Timestamp.Unix(),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack notifier status: %d", resp.StatusCode)
	}
	return nil
}

func slackColor(severity string) string {
	switch SeverityLevel(severity) {
	case 2:
		return "#FF0000"
	case 1:
		return "#FFA500"
	default:
		return "#36A64F"
	}
}
