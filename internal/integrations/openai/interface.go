package openai

import "context"

type Reasoner interface {
	Reason(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
	GenerateCreativeBriefs(ctx context.Context, productName string, niche string, angles []string) ([]CreativeBrief, error)
	ExtractLessons(ctx context.Context, productSummary string, campaignSummary string) ([]LessonDraft, error)
}

type CreativeBrief struct {
	Angle      string `json:"angle"`
	Headline   string `json:"headline"`
	Body       string `json:"body"`
	CTA        string `json:"cta"`
	HookScript string `json:"hook_script"`
	Platform   string `json:"platform"`
}

type LessonDraft struct {
	Category   string  `json:"category"`
	Lesson     string  `json:"lesson"`
	Confidence float64 `json:"confidence"`
}
