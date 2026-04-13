package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	goopenai "github.com/sashabaranov/go-openai"
)

type Client struct {
	client *goopenai.Client
	model  string
}

func New(apiKey string) *Client {
	return &Client{
		client: goopenai.NewClient(apiKey),
		model:  "gpt-4o",
	}
}

func NewWithBaseURL(apiKey string, baseURL string) *Client {
	cfg := goopenai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return &Client{
		client: goopenai.NewClientWithConfig(cfg),
		model:  "gpt-4o",
	}
}

func (c *Client) Reason(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	resp, err := c.client.CreateChatCompletion(ctx, goopenai.ChatCompletionRequest{
		Model: c.model,
		Messages: []goopenai.ChatCompletionMessage{
			{Role: goopenai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: goopenai.ChatMessageRoleUser, Content: userPrompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai reason: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai reason: no choices returned")
	}
	return resp.Choices[0].Message.Content, nil
}

func (c *Client) GenerateCreativeBriefs(ctx context.Context, productName string, niche string, angles []string) ([]CreativeBrief, error) {
	userPrompt := fmt.Sprintf(
		"Generate ad creative briefs for: %s (niche: %s).\nFor each of these angles: %s.\nReturn ONLY a valid JSON array, no markdown, no explanation.\nEach element: {angle, headline (max 40 chars), body (max 125 chars), cta (max 20 chars), hook_script (1-2 sentences, first 3 seconds), platform (meta|tiktok|both)}",
		productName,
		niche,
		strings.Join(angles, ", "),
	)

	content, err := c.Reason(ctx, BuildSystemPrompt("creative_generation", niche, ""), userPrompt)
	if err != nil {
		return nil, err
	}

	content = stripMarkdownFences(content)
	var briefs []CreativeBrief
	if err := json.Unmarshal([]byte(content), &briefs); err != nil {
		return nil, fmt.Errorf("parse creative briefs: %w", err)
	}
	return briefs, nil
}

func (c *Client) ExtractLessons(ctx context.Context, productSummary string, campaignSummary string) ([]LessonDraft, error) {
	userPrompt := fmt.Sprintf(
		"A dropshipping product test completed. Extract 2-4 concrete, actionable lessons.\nProduct summary: %s\nCampaign summary: %s\nReturn ONLY a valid JSON array, no markdown.\nEach element: {category, lesson, confidence (0.0-1.0)}",
		productSummary,
		campaignSummary,
	)

	content, err := c.Reason(ctx, BuildSystemPrompt("learning", "", ""), userPrompt)
	if err != nil {
		return nil, err
	}

	content = stripMarkdownFences(content)
	var lessons []LessonDraft
	if err := json.Unmarshal([]byte(content), &lessons); err != nil {
		return nil, fmt.Errorf("parse lessons: %w", err)
	}
	return lessons, nil
}

func stripMarkdownFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
		}
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return trimmed
}

func BuildSystemPrompt(callType string, niche string, memoryContext string) string {
	_ = niche
	base := "You are an expert autonomous dropshipping marketing agent operating in 2026."
	creative := "You are generating ad creative briefs for a dropshipping operation in 2026."

	var sb strings.Builder
	switch callType {
	case "creative_generation":
		sb.WriteString(base)
		sb.WriteString("\n\n")
		sb.WriteString(creative)
	default:
		sb.WriteString(base)
	}
	if memoryContext != "" {
		sb.WriteString("\n\n")
		sb.WriteString(memoryContext)
	}
	return sb.String()
}
