// Package generator turns an intake (company website text + insider facts) into a show.
package generator

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/ralpheichelberger/partyshow/internal/show"
	"github.com/ralpheichelberger/partyshow/internal/webtext"
)

// MinQuestions is the smallest show worth playing.
const MinQuestions = 5

// Generator creates a draft show. The result still needs Finalize.
type Generator interface {
	Generate(ctx context.Context, in show.Intake, site webtext.Page) (*show.Show, error)
}

// ErrTooFewQuestions means generation left too few usable questions.
var ErrTooFewQuestions = errors.New("too few usable questions were generated")

// Finalize cleans a generated draft: it drops broken and (in safe mode) unsafe questions,
// shuffles answer options so the correct answer isn't always in the same spot,
// and validates the result.
func Finalize(s *show.Show, in show.Intake, rng *rand.Rand) error {
	if strings.TrimSpace(s.Title) == "" {
		s.Title = defaultTitle(in)
	}
	s.Language = in.Language
	rounds := s.Rounds[:0]
	for _, r := range s.Rounds {
		r.Title = strings.TrimSpace(r.Title)
		if r.Title == "" {
			r.Title = fmt.Sprintf("Runde %d", len(rounds)+1)
		}
		kept := r.Questions[:0]
		for _, q := range r.Questions {
			one := show.Show{Title: "x", Rounds: []show.Round{{Title: "x", Questions: []show.Question{q}}}}
			if one.Validate() != nil {
				continue
			}
			q = one.Rounds[0].Questions[0]
			shuffle(&q, rng)
			kept = append(kept, q)
			if len(kept) == show.MaxQuestions {
				break
			}
		}
		r.Questions = kept
		if len(r.Questions) > 0 {
			rounds = append(rounds, r)
		}
		if len(rounds) == show.MaxRounds {
			break
		}
	}
	s.Rounds = rounds
	if in.SafeMode {
		s.RemoveUnsafe()
	}
	if s.QuestionCount() < MinQuestions {
		return ErrTooFewQuestions
	}
	return s.Validate()
}

func shuffle(q *show.Question, rng *rand.Rand) {
	if rng == nil {
		return
	}
	correct := q.Options[q.Correct]
	rng.Shuffle(len(q.Options), func(i, j int) { q.Options[i], q.Options[j] = q.Options[j], q.Options[i] })
	for i, o := range q.Options {
		if o == correct {
			q.Correct = i
			return
		}
	}
}

func defaultTitle(in show.Intake) string {
	if in.Language == "en" {
		return in.CompanyName + " – The Big Party Show"
	}
	return in.CompanyName + " – Die große Party-Show"
}
