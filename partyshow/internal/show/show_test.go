package show

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validShow() Show {
	return Show{
		Title: "Weihnachtsshow",
		Rounds: []Round{{
			Title: "Runde 1",
			Questions: []Question{
				{Text: "Wann wurde die Firma gegründet?", Options: []string{"1999", "2005", "2012", "2020"}, Correct: 1},
			},
		}},
	}
}

func TestPlanMaxPlayers(t *testing.T) {
	tests := []struct {
		plan Plan
		want int
	}{
		{PlanFree, 15}, {PlanSmall, 50}, {PlanMedium, 200}, {PlanLarge, 500}, {Plan("unknown"), 15},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.plan.MaxPlayers(), string(tt.plan))
	}
}

func TestIntakeNormalizeAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      Intake
		wantErr string
	}{
		{"ok with facts", Intake{CompanyName: " Acme ", Facts: []string{" Kuchen ", ""}}, ""},
		{"ok with website", Intake{CompanyName: "Acme", WebsiteURL: "https://acme.example"}, ""},
		{"missing name", Intake{Facts: []string{"x"}}, "company name"},
		{"no content", Intake{CompanyName: "Acme", Facts: []string{"  "}}, "website or at least one fact"},
		{"long fact", Intake{CompanyName: "Acme", Facts: []string{strings.Repeat("a", MaxFactLen+1)}}, "fact is longer"},
		{"too many facts", Intake{CompanyName: "Acme", Facts: make31()}, "at most"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in
			in.Normalize()
			err := in.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func make31() []string {
	out := make([]string, MaxFacts+1)
	for i := range out {
		out[i] = "fact"
	}
	return out
}

func TestIntakeDefaults(t *testing.T) {
	in := Intake{CompanyName: "Acme", Language: "fr", Occasion: "nope", Facts: []string{"a", " ", "b"}}
	in.Normalize()
	assert.Equal(t, "de", in.Language)
	assert.Equal(t, "christmas", in.Occasion)
	assert.Equal(t, []string{"a", "b"}, in.Facts)
}

func TestShowValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Show)
		wantErr string
	}{
		{"valid", func(*Show) {}, ""},
		{"no title", func(s *Show) { s.Title = " " }, "title"},
		{"no rounds", func(s *Show) { s.Rounds = nil }, "no rounds"},
		{"empty round", func(s *Show) { s.Rounds[0].Questions = nil }, "no questions"},
		{"one option", func(s *Show) { s.Rounds[0].Questions[0].Options = []string{"a"} }, "2 to 4"},
		{"duplicate options", func(s *Show) { s.Rounds[0].Questions[0].Options = []string{"a", "A "} }, "same"},
		{"correct out of range", func(s *Show) { s.Rounds[0].Questions[0].Correct = 4 }, "out of range"},
		{"negative correct", func(s *Show) { s.Rounds[0].Questions[0].Correct = -1 }, "out of range"},
		{"empty option", func(s *Show) { s.Rounds[0].Questions[0].Options[2] = " " }, "empty"},
		{"too short time", func(s *Show) { s.Rounds[0].Questions[0].Seconds = 2 }, "answer time"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validShow()
			tt.mutate(&s)
			err := s.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, DefaultSeconds, s.Rounds[0].Questions[0].Seconds)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestIsUnsafe(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"Wer war auf der Weihnachtsfeier betrunken?", true},
		{"Wie alt ist unser Chef?", true},
		{"Who has the highest salary?", true},
		{"Who got fired last year?", true},
		{"Wer hat zugenommen und ist jetzt dick?", true},
		{"Walter bringt immer Kuchen mit", false},
		{"Dieter repariert die Kaffeemaschine", false},
		{"Whose father founded the company?", false},
		{"In welchem Jahr sind wir umgezogen?", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, IsUnsafe(tt.text), tt.text)
	}
}

func TestRemoveUnsafe(t *testing.T) {
	s := Show{Rounds: []Round{
		{Title: "A", Questions: []Question{
			{Text: "Wer trinkt am meisten Kaffee?", Options: []string{"Anna", "Ben"}},
			{Text: "Wer war betrunken?", Options: []string{"Anna", "Ben"}},
		}},
		{Title: "B", Questions: []Question{
			{Text: "Was ist dein Gehalt?", Options: []string{"1", "2"}},
		}},
	}}
	removed := s.RemoveUnsafe()
	assert.Equal(t, 2, removed)
	require.Len(t, s.Rounds, 1)
	assert.Len(t, s.Rounds[0].Questions, 1)
	assert.Equal(t, 1, s.QuestionCount())
}
