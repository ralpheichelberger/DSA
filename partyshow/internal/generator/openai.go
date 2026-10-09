package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/ralpheichelberger/partyshow/internal/show"
	"github.com/ralpheichelberger/partyshow/internal/webtext"
)

// ChatClient is the part of the OpenAI client the generator uses, so tests can fake it.
type ChatClient interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// OpenAI generates shows with a chat model.
type OpenAI struct {
	Client ChatClient
	Model  string
}

// NewOpenAI returns a generator for the given API key and model.
func NewOpenAI(apiKey, model string) *OpenAI {
	if model == "" {
		model = openai.GPT4oMini
	}
	return &OpenAI{Client: openai.NewClient(apiKey), Model: model}
}

// Generate asks the model for a show as JSON.
func (g *OpenAI) Generate(ctx context.Context, in show.Intake, site webtext.Page) (*show.Show, error) {
	resp, err := g.Client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:          g.Model,
		Temperature:    0.8,
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: SystemPrompt(in)},
			{Role: openai.ChatMessageRoleUser, Content: UserPrompt(in, site)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("openai: empty response")
	}
	return ParseShow(resp.Choices[0].Message.Content)
}

// ParseShow reads the model's JSON answer. It tolerates a ```json fence around it.
func ParseShow(content string) (*show.Show, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	var s show.Show
	if err := json.Unmarshal([]byte(content), &s); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w", err)
	}
	return &s, nil
}

var occasionNames = map[string][2]string{
	"christmas": {"Weihnachtsfeier", "Christmas party"},
	"farewell":  {"Abschiedsfeier", "farewell party"},
	"birthday":  {"Geburtstagsfeier", "birthday party"},
	"wedding":   {"Hochzeit", "wedding"},
	"teamevent": {"Teamevent", "team event"},
}

// SystemPrompt explains the format and the rules to the model.
func SystemPrompt(in show.Intake) string {
	lang := "German (informal 'ihr' for the audience)"
	if in.Language == "en" {
		lang = "English"
	}
	safe := ""
	if in.SafeMode {
		safe = `
- SAFE MODE: never ask about or joke about weight, looks, age, health, alcohol, sex, relationships,
  religion, politics, origin, salary or job loss. Humor must be kind; nobody may be embarrassed.`
	}
	return fmt.Sprintf(`You write live quiz game shows for parties. Players answer on their phones; a host voice reads everything aloud.
Write everything in %s.

Return ONLY a JSON object with this shape:
{"title": string, "welcome": string, "outro": string,
 "rounds": [{"title": string, "intro": string,
   "questions": [{"text": string, "options": [string, string, string, string], "correct": number,
                  "host_line": string, "explanation": string}]}]}

Rules:
- 4 or 5 rounds, 4 or 5 questions each (18–22 questions total).
- "correct" is the 0-based index of the right option. Exactly one option is right.
- Questions about the company may ONLY use facts from the website text or the insider facts given.
  Never invent company facts. If there is little material, use fewer company questions and more
  questions about the occasion.
- Wrong options must be plausible and similar in length to the right one. No "all of the above".
- Questions: max 140 characters. Options: max 60 characters.
- "welcome": 2–3 sentences the host says at the start. "outro": 1–2 sentences at the end.
- "intro": one short host sentence before each round.
- "host_line": one short, witty sentence the host says after revealing the answer.
- "explanation": optional half sentence with the background, or "".
- Suggested rounds: the company (from the website), insider round (from the facts),
  estimation round (numbers as answer ranges), a round about the occasion, and a mixed final round.%s`, lang, safe)
}

// UserPrompt passes the customer's material to the model.
func UserPrompt(in show.Intake, site webtext.Page) string {
	occ := occasionNames[in.Occasion]
	name := occ[0]
	if in.Language == "en" {
		name = occ[1]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Company: %s\nOccasion: %s\n", in.CompanyName, name)
	if len(in.Teams) > 0 {
		fmt.Fprintf(&b, "Teams/departments: %s\n", strings.Join(in.Teams, ", "))
	}
	if len(in.Facts) > 0 {
		b.WriteString("\nInsider facts from the organizer:\n")
		for _, f := range in.Facts {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	if site.Text != "" || site.Title != "" {
		b.WriteString("\nCompany website (extract):\n")
		if site.Title != "" {
			fmt.Fprintf(&b, "Title: %s\n", site.Title)
		}
		if site.Description != "" {
			fmt.Fprintf(&b, "Description: %s\n", site.Description)
		}
		b.WriteString(site.Text)
		b.WriteString("\n")
	}
	return b.String()
}
