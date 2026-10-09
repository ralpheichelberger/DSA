// Package show defines a generated party game show and validates it.
package show

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Plan limits how many players can join a show.
type Plan string

const (
	PlanFree   Plan = "free"   // up to 15 players
	PlanSmall  Plan = "small"  // up to 50 players
	PlanMedium Plan = "medium" // up to 200 players
	PlanLarge  Plan = "large"  // up to 500 players
)

// MaxPlayers returns the player cap for a plan.
func (p Plan) MaxPlayers() int {
	switch p {
	case PlanSmall:
		return 50
	case PlanMedium:
		return 200
	case PlanLarge:
		return 500
	default:
		return 15
	}
}

// Intake is what the organizer enters on the create form.
type Intake struct {
	CompanyName string   `json:"company_name"`
	WebsiteURL  string   `json:"website_url"`
	Facts       []string `json:"facts"`
	Teams       []string `json:"teams"`
	Occasion    string   `json:"occasion"` // "christmas", "farewell", "birthday", "wedding", "teamevent"
	Language    string   `json:"language"` // "de" or "en"
	SafeMode    bool     `json:"safe_mode"`
	Email       string   `json:"email"`
}

// Question is one multiple-choice question (2–4 options).
type Question struct {
	Text        string   `json:"text"`
	Options     []string `json:"options"`
	Correct     int      `json:"correct"`
	HostLine    string   `json:"host_line,omitempty"`   // what the host says after the reveal
	Explanation string   `json:"explanation,omitempty"` // shown on the reveal screen
	Seconds     int      `json:"seconds,omitempty"`     // answer time, default 20
}

// Round groups questions under a title.
type Round struct {
	Title     string     `json:"title"`
	Intro     string     `json:"intro,omitempty"` // host line before the round
	Questions []Question `json:"questions"`
}

// Show is a complete, editable game show.
type Show struct {
	ID        string    `json:"id"`
	EditToken string    `json:"edit_token"`
	Title     string    `json:"title"`
	Welcome   string    `json:"welcome"`
	Outro     string    `json:"outro"`
	Language  string    `json:"language"`
	Rounds    []Round   `json:"rounds"`
	Plan      Plan      `json:"plan"`
	Intake    Intake    `json:"intake"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Limits on sizes, so a show stays playable and cheap to store.
const (
	MaxFacts         = 30
	MaxFactLen       = 300
	MaxTeams         = 20
	MaxRounds        = 10
	MaxQuestions     = 15
	MaxTextLen       = 300
	MaxOptionLen     = 120
	DefaultSeconds   = 20
	MinSeconds       = 5
	MaxSeconds       = 90
	MaxCompanyLength = 100
)

var occasions = map[string]bool{"christmas": true, "farewell": true, "birthday": true, "wedding": true, "teamevent": true}

// Normalize trims the intake, drops empty entries and applies defaults.
func (in *Intake) Normalize() {
	in.CompanyName = strings.TrimSpace(in.CompanyName)
	in.WebsiteURL = strings.TrimSpace(in.WebsiteURL)
	in.Email = strings.TrimSpace(in.Email)
	in.Facts = cleanList(in.Facts)
	in.Teams = cleanList(in.Teams)
	if in.Language != "en" {
		in.Language = "de"
	}
	if !occasions[in.Occasion] {
		in.Occasion = "christmas"
	}
}

// Validate checks the intake after Normalize.
func (in Intake) Validate() error {
	if in.CompanyName == "" {
		return errors.New("company name is required")
	}
	if len(in.CompanyName) > MaxCompanyLength {
		return fmt.Errorf("company name is longer than %d characters", MaxCompanyLength)
	}
	if len(in.Facts) > MaxFacts {
		return fmt.Errorf("at most %d facts", MaxFacts)
	}
	for _, f := range in.Facts {
		if len(f) > MaxFactLen {
			return fmt.Errorf("a fact is longer than %d characters", MaxFactLen)
		}
	}
	if len(in.Teams) > MaxTeams {
		return fmt.Errorf("at most %d teams", MaxTeams)
	}
	if in.WebsiteURL == "" && len(in.Facts) == 0 {
		return errors.New("enter a website or at least one fact")
	}
	return nil
}

// Validate checks that the show can be played. It also fills in default answer times.
func (s *Show) Validate() error {
	if strings.TrimSpace(s.Title) == "" {
		return errors.New("show title is required")
	}
	if len(s.Rounds) == 0 {
		return errors.New("show has no rounds")
	}
	if len(s.Rounds) > MaxRounds {
		return fmt.Errorf("at most %d rounds", MaxRounds)
	}
	for ri := range s.Rounds {
		r := &s.Rounds[ri]
		if strings.TrimSpace(r.Title) == "" {
			return fmt.Errorf("round %d has no title", ri+1)
		}
		if len(r.Questions) == 0 {
			return fmt.Errorf("round %d has no questions", ri+1)
		}
		if len(r.Questions) > MaxQuestions {
			return fmt.Errorf("round %d has more than %d questions", ri+1, MaxQuestions)
		}
		for qi := range r.Questions {
			if err := r.Questions[qi].validate(); err != nil {
				return fmt.Errorf("round %d, question %d: %w", ri+1, qi+1, err)
			}
		}
	}
	return nil
}

func (q *Question) validate() error {
	q.Text = strings.TrimSpace(q.Text)
	if q.Text == "" {
		return errors.New("question text is empty")
	}
	if len(q.Text) > MaxTextLen {
		return fmt.Errorf("question is longer than %d characters", MaxTextLen)
	}
	if len(q.Options) < 2 || len(q.Options) > 4 {
		return errors.New("needs 2 to 4 answer options")
	}
	seen := map[string]bool{}
	for i, o := range q.Options {
		o = strings.TrimSpace(o)
		q.Options[i] = o
		if o == "" {
			return errors.New("an answer option is empty")
		}
		if len(o) > MaxOptionLen {
			return fmt.Errorf("an answer option is longer than %d characters", MaxOptionLen)
		}
		key := strings.ToLower(o)
		if seen[key] {
			return errors.New("two answer options are the same")
		}
		seen[key] = true
	}
	if q.Correct < 0 || q.Correct >= len(q.Options) {
		return errors.New("correct answer is out of range")
	}
	if q.Seconds == 0 {
		q.Seconds = DefaultSeconds
	}
	if q.Seconds < MinSeconds || q.Seconds > MaxSeconds {
		return fmt.Errorf("answer time must be %d–%d seconds", MinSeconds, MaxSeconds)
	}
	return nil
}

// QuestionCount returns the total number of questions.
func (s *Show) QuestionCount() int {
	n := 0
	for _, r := range s.Rounds {
		n += len(r.Questions)
	}
	return n
}

func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
