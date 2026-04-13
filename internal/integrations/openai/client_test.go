package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockOpenAIServer(t *testing.T, statusCode int, responseBody string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(responseBody))
	}))
}

func TestReason_Success(t *testing.T) {
	server := mockOpenAIServer(t, http.StatusOK, `{"choices":[{"message":{"content":"test response","role":"assistant"}}]}`)
	defer server.Close()

	client := NewWithBaseURL("test-key", server.URL)
	got, err := client.Reason(context.Background(), "system prompt", "user prompt")
	require.NoError(t, err)
	assert.Equal(t, "test response", got)
}

func TestReason_APIError(t *testing.T) {
	server := mockOpenAIServer(t, http.StatusTooManyRequests, `{"error":{"message":"rate limit","type":"rate_limit_exceeded"}}`)
	defer server.Close()

	client := NewWithBaseURL("test-key", server.URL)
	got, err := client.Reason(context.Background(), "system prompt", "user prompt")
	assert.Empty(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openai reason")
}

func TestGenerateCreativeBriefs_ParsesJSON(t *testing.T) {
	assistantJSON := `[{"angle":"problem_solution","headline":"Pet hair solved fast","body":"Remove loose hair in minutes with less stress for your pet.","cta":"Shop now","hook_script":"Shedding everywhere? Watch this in 3 seconds.","platform":"tiktok"},{"angle":"social_proof","headline":"5,000 owners switched","body":"Real pet owners report cleaner homes after one week.","cta":"See results","hook_script":"From fur-covered sofa to clean fabric instantly.","platform":"meta"}]`
	responseBody := fmt.Sprintf(`{"choices":[{"message":{"content":%q,"role":"assistant"}}]}`, assistantJSON)

	server := mockOpenAIServer(t, http.StatusOK, responseBody)
	defer server.Close()

	client := NewWithBaseURL("test-key", server.URL)
	briefs, err := client.GenerateCreativeBriefs(context.Background(), "Pet Brush", "pets", []string{"problem_solution", "social_proof"})
	require.NoError(t, err)
	require.Len(t, briefs, 2)
	assert.Equal(t, "problem_solution", briefs[0].Angle)
	assert.Equal(t, "Pet hair solved fast", briefs[0].Headline)
	assert.Equal(t, "meta", briefs[1].Platform)
}

func TestGenerateCreativeBriefs_StripsFences(t *testing.T) {
	assistantContent := "```json\n[{\"angle\":\"transformation\",\"headline\":\"Desk setup upgrade\",\"body\":\"Before/after posture improvement in one week.\",\"cta\":\"Try it\",\"hook_script\":\"Bad posture? Watch this quick fix.\",\"platform\":\"both\"}]\n```"
	responseBody := fmt.Sprintf(`{"choices":[{"message":{"content":%q,"role":"assistant"}}]}`, assistantContent)

	server := mockOpenAIServer(t, http.StatusOK, responseBody)
	defer server.Close()

	client := NewWithBaseURL("test-key", server.URL)
	briefs, err := client.GenerateCreativeBriefs(context.Background(), "Laptop Stand", "tech", []string{"transformation"})
	require.NoError(t, err)
	require.Len(t, briefs, 1)
	assert.Equal(t, "transformation", briefs[0].Angle)
	assert.Equal(t, "both", briefs[0].Platform)
}

func TestExtractLessons_ParsesJSON(t *testing.T) {
	assistantJSON := `[{"category":"creative","lesson":"UGC hooks improved CTR.","confidence":0.82},{"category":"platform","lesson":"Meta retargeting closed better.","confidence":0.74}]`
	responseBody := fmt.Sprintf(`{"choices":[{"message":{"content":%q,"role":"assistant"}}]}`, assistantJSON)

	server := mockOpenAIServer(t, http.StatusOK, responseBody)
	defer server.Close()

	client := NewWithBaseURL("test-key", server.URL)
	lessons, err := client.ExtractLessons(context.Background(), "Product did well", "Meta outperformed TikTok")
	require.NoError(t, err)
	require.Len(t, lessons, 2)
	for _, lesson := range lessons {
		assert.GreaterOrEqual(t, lesson.Confidence, 0.0)
		assert.LessOrEqual(t, lesson.Confidence, 1.0)
	}
}

func TestExtractLessons_InvalidJSON(t *testing.T) {
	responseBody := `{"choices":[{"message":{"content":"not json at all","role":"assistant"}}]}`
	server := mockOpenAIServer(t, http.StatusOK, responseBody)
	defer server.Close()

	client := NewWithBaseURL("test-key", server.URL)
	lessons, err := client.ExtractLessons(context.Background(), "summary", "campaign")
	assert.Nil(t, lessons)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "parse lessons") || strings.Contains(err.Error(), "invalid"))
}
