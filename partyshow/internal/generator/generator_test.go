package generator

import (
	"context"
	"errors"
	"math/rand/v2"
	"testing"

	openai "github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ralpheichelberger/partyshow/internal/show"
	"github.com/ralpheichelberger/partyshow/internal/webtext"
)

type fakeChat struct {
	content string
	err     error
	got     openai.ChatCompletionRequest
}

func (f *fakeChat) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	f.got = req
	if f.err != nil {
		return openai.ChatCompletionResponse{}, f.err
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: f.content}}}}, nil
}

const modelJSON = "```json\n" + `{"title":"Acme Show","welcome":"Hallo","outro":"Tschüss","rounds":[
 {"title":"Firma","questions":[
   {"text":"Wann gegründet?","options":["1999","2005","2012","2020"],"correct":0},
   {"text":"Wo ist der Sitz?","options":["Wien","Graz","Linz","Salzburg"],"correct":0},
   {"text":"Kaputt","options":["nur eine"],"correct":0},
   {"text":"Wer war betrunken?","options":["Anna","Ben"],"correct":0}]},
 {"title":"Mix","questions":[
   {"text":"Frage 3","options":["a","b","c"],"correct":2},
   {"text":"Frage 4","options":["a","b"],"correct":1},
   {"text":"Frage 5","options":["x","y","z","w"],"correct":3}]}]}` + "\n```"

func TestOpenAIGenerateAndFinalize(t *testing.T) {
	chat := &fakeChat{content: modelJSON}
	g := &OpenAI{Client: chat, Model: "test-model"}
	in := show.Intake{CompanyName: "Acme", Facts: []string{"Anna backt Kuchen"}, Language: "de", Occasion: "christmas", SafeMode: true}

	s, err := g.Generate(context.Background(), in, webtext.Page{Title: "Acme", Text: "Gegründet 1999 in Wien"})
	require.NoError(t, err)
	assert.Equal(t, "test-model", chat.got.Model)
	assert.Contains(t, chat.got.Messages[1].Content, "Anna backt Kuchen")
	assert.Contains(t, chat.got.Messages[1].Content, "Gegründet 1999 in Wien")
	assert.Contains(t, chat.got.Messages[0].Content, "SAFE MODE")

	rng := rand.New(rand.NewPCG(1, 2))
	require.NoError(t, Finalize(s, in, rng))
	// The broken question and the unsafe one are gone.
	assert.Equal(t, 5, s.QuestionCount())
	// Shuffling keeps the right answer.
	q := s.Rounds[0].Questions[0]
	assert.Equal(t, "1999", q.Options[q.Correct])
	q = s.Rounds[1].Questions[2]
	assert.Equal(t, "w", q.Options[q.Correct])
	assert.Equal(t, show.DefaultSeconds, q.Seconds)
}

func TestOpenAIErrors(t *testing.T) {
	g := &OpenAI{Client: &fakeChat{err: errors.New("boom")}}
	_, err := g.Generate(context.Background(), show.Intake{}, webtext.Page{})
	require.Error(t, err)

	g = &OpenAI{Client: &fakeChat{content: "not json"}}
	_, err = g.Generate(context.Background(), show.Intake{}, webtext.Page{})
	require.Error(t, err)
}

func TestFinalizeTooFew(t *testing.T) {
	s := &show.Show{Title: "x", Rounds: []show.Round{{Title: "r", Questions: []show.Question{
		{Text: "q", Options: []string{"a", "b"}, Correct: 0},
	}}}}
	err := Finalize(s, show.Intake{CompanyName: "Acme"}, nil)
	assert.ErrorIs(t, err, ErrTooFewQuestions)
}

func TestStub(t *testing.T) {
	for _, lang := range []string{"de", "en"} {
		in := show.Intake{CompanyName: "Acme", Facts: []string{"Wir haben einen Bürohund"}, Language: lang, Occasion: "christmas"}
		s, err := Stub{}.Generate(context.Background(), in, webtext.Page{})
		require.NoError(t, err)
		require.NoError(t, Finalize(s, in, rand.New(rand.NewPCG(3, 4))))
		assert.Equal(t, 9, s.QuestionCount(), lang)
		assert.Contains(t, s.Title, "Acme")
	}
}

func TestPromptsLanguage(t *testing.T) {
	en := show.Intake{CompanyName: "Acme", Language: "en", Occasion: "wedding", Teams: []string{"Sales"}}
	assert.Contains(t, SystemPrompt(en), "English")
	assert.NotContains(t, SystemPrompt(en), "SAFE MODE")
	up := UserPrompt(en, webtext.Page{})
	assert.Contains(t, up, "wedding")
	assert.Contains(t, up, "Sales")
	assert.NotContains(t, up, "website")
}
