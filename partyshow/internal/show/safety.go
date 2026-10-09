package show

import (
	"strings"
	"unicode"
)

// Word stems that point to topics which embarrass colleagues at a company party.
// A stem matches any word that starts with it ("betrunken" matches "betrunkene").
var unsafeStems = []string{
	// body, age, health
	"gewicht", "übergewicht", "fett", "diät", "weight", "krank", "schwanger", "pregnan",
	// alcohol, sex, relationships
	"betrunken", "besoffen", "drunk", "hangover", "sexu", "affäre", "affair", "flirt", "dating",
	// religion, politics, origin
	"religion", "religiö", "kirche", "church", "muslim", "jüd", "jewish",
	"politik", "politisch", "politic", "partei", "herkunft", "ausländer", "foreigner", "rasse", "racis",
	// money and job security
	"gehalt", "salary", "kündigung", "gekündigt", "layoff", "entlassung", "entlassen",
}

// Short words matched only as whole words, because as stems they would hit
// harmless words ("fat" in "father", "diet" in "Dieter").
var unsafeWords = map[string]bool{"dick": true, "fat": true, "diet": true, "sick": true, "sex": true, "fired": true, "jude": true}

// Multi-word phrases matched anywhere in the lower-cased text.
var unsafePhrases = []string{"wie alt", "how old", "ex-freund", "ex-boyfriend", "ex-girlfriend"}

// IsUnsafe reports whether text touches a topic safe mode filters out.
func IsUnsafe(text string) bool {
	t := strings.ToLower(text)
	for _, p := range unsafePhrases {
		if strings.Contains(t, p) {
			return true
		}
	}
	words := strings.FieldsFunc(t, func(r rune) bool { return !unicode.IsLetter(r) })
	for _, w := range words {
		if unsafeWords[w] {
			return true
		}
		for _, stem := range unsafeStems {
			if strings.HasPrefix(w, stem) {
				return true
			}
		}
	}
	return false
}

// RemoveUnsafe drops questions whose text, options or host line touch unsafe topics,
// and drops rounds left empty. It returns how many questions were removed.
func (s *Show) RemoveUnsafe() int {
	removed := 0
	rounds := s.Rounds[:0]
	for _, r := range s.Rounds {
		kept := r.Questions[:0]
		for _, q := range r.Questions {
			if questionUnsafe(q) {
				removed++
				continue
			}
			kept = append(kept, q)
		}
		r.Questions = kept
		if len(r.Questions) > 0 {
			rounds = append(rounds, r)
		}
	}
	s.Rounds = rounds
	return removed
}

func questionUnsafe(q Question) bool {
	if IsUnsafe(q.Text) || IsUnsafe(q.HostLine) || IsUnsafe(q.Explanation) {
		return true
	}
	for _, o := range q.Options {
		if IsUnsafe(o) {
			return true
		}
	}
	return false
}
