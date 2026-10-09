package generator

import (
	"context"

	"github.com/ralpheichelberger/partyshow/internal/show"
	"github.com/ralpheichelberger/partyshow/internal/webtext"
)

// Stub builds a show without any AI: true-or-false questions from the facts plus
// fixed general-knowledge rounds. It's used for local development, demos and as
// a fallback when no API key is configured.
type Stub struct{}

// Generate never fails for a valid intake.
func (Stub) Generate(_ context.Context, in show.Intake, site webtext.Page) (*show.Show, error) {
	t := texts[in.Language]
	s := &show.Show{
		Title:   defaultTitle(in),
		Welcome: t.welcome(in.CompanyName),
		Outro:   t.outro,
	}
	if len(in.Facts) > 0 {
		r := show.Round{Title: t.insiderTitle, Intro: t.insiderIntro}
		for _, f := range in.Facts {
			r.Questions = append(r.Questions, show.Question{
				Text:     t.truePrefix + f,
				Options:  []string{t.yes, t.no},
				Correct:  0,
				HostLine: t.trueHost,
			})
			if len(r.Questions) == 8 {
				break
			}
		}
		s.Rounds = append(s.Rounds, r)
	}
	s.Rounds = append(s.Rounds, t.occasion, t.estimate)
	return s, nil
}

type stubTexts struct {
	welcome      func(company string) string
	outro        string
	insiderTitle string
	insiderIntro string
	truePrefix   string
	yes, no      string
	trueHost     string
	occasion     show.Round
	estimate     show.Round
}

var texts = map[string]stubTexts{
	"de": {
		welcome: func(c string) string {
			return "Herzlich willkommen zur großen Show von " + c + "! Handys raus, QR-Code scannen – wer am schnellsten richtig antwortet, holt die meisten Punkte."
		},
		outro:        "Das war's! Applaus für unsere Gewinner – und für euch alle.",
		insiderTitle: "Insider-Runde",
		insiderIntro: "Jetzt wird's persönlich: Stimmt das oder nicht?",
		truePrefix:   "Stimmt das? ",
		yes:          "Stimmt",
		no:           "Stimmt nicht",
		trueHost:     "Natürlich stimmt das – wer hier arbeitet, weiß das!",
		occasion: show.Round{Title: "Weihnachtsrunde", Intro: "Und jetzt: alles rund um Weihnachten.", Questions: []show.Question{
			{Text: "Wie viele Rentiere ziehen den Schlitten des Weihnachtsmanns – ohne Rudolph?", Options: []string{"6", "8", "10", "12"}, Correct: 1, HostLine: "Acht! Rudolph kam erst später dazu."},
			{Text: "Aus welchem Land stammt das Lied „Stille Nacht“?", Options: []string{"Deutschland", "Österreich", "Schweiz", "England"}, Correct: 1, HostLine: "Österreich – 1818 in Oberndorf bei Salzburg."},
			{Text: "An welchem Tag ist Nikolaus?", Options: []string{"1. Dezember", "6. Dezember", "13. Dezember", "24. Dezember"}, Correct: 1, HostLine: "Der 6. – hoffentlich waren eure Stiefel geputzt."},
			{Text: "Wie heißt der berühmte Weihnachtsmarkt in Nürnberg?", Options: []string{"Christkindlesmarkt", "Winterzauber", "Adventsdorf", "Lichtermarkt"}, Correct: 0, HostLine: "Der Christkindlesmarkt – mit Lebkuchen und Glühwein."},
		}},
		estimate: show.Round{Title: "Schätzrunde", Intro: "Schätzen ist erlaubt – raten auch.", Questions: []show.Question{
			{Text: "Wie weit ist der Mond ungefähr von der Erde entfernt?", Options: []string{"38.000 km", "384.000 km", "1,5 Mio. km", "3,8 Mio. km"}, Correct: 1, HostLine: "Rund 384.000 Kilometer – ein weiter Weg für den Schlitten."},
			{Text: "Wie viele Knochen hat ein erwachsener Mensch?", Options: []string{"106", "206", "306", "406"}, Correct: 1, HostLine: "206 – und nach dem Teamevent spürt man jeden einzelnen."},
			{Text: "Wie hoch ist der Mount Everest ungefähr?", Options: []string{"6.800 m", "7.900 m", "8.849 m", "9.500 m"}, Correct: 2, HostLine: "8.849 Meter – höher als jeder Stapel Tickets."},
			{Text: "Wie viele Länder gehören zur EU?", Options: []string{"25", "27", "28", "30"}, Correct: 1, HostLine: "27 – seit dem Brexit."},
		}},
	},
	"en": {
		welcome: func(c string) string {
			return "Welcome to the big " + c + " show! Phones out, scan the QR code – the fastest right answers win the most points."
		},
		outro:        "That's it! A big round of applause for our winners – and for all of you.",
		insiderTitle: "Insider round",
		insiderIntro: "Now it gets personal: true or false?",
		truePrefix:   "True or false? ",
		yes:          "True",
		no:           "False",
		trueHost:     "Of course it's true – everyone here knows that!",
		occasion: show.Round{Title: "Christmas round", Intro: "And now: all things Christmas.", Questions: []show.Question{
			{Text: "How many reindeer pull Santa's sleigh – not counting Rudolph?", Options: []string{"6", "8", "10", "12"}, Correct: 1, HostLine: "Eight! Rudolph joined later."},
			{Text: "Which country does the carol “Silent Night” come from?", Options: []string{"Germany", "Austria", "Switzerland", "England"}, Correct: 1, HostLine: "Austria – first sung in 1818 near Salzburg."},
			{Text: "On which date is St. Nicholas Day?", Options: []string{"December 1", "December 6", "December 13", "December 24"}, Correct: 1, HostLine: "December 6 – hope your boots were clean."},
			{Text: "Which country sends London a Christmas tree for Trafalgar Square every year?", Options: []string{"Norway", "Sweden", "Germany", "Canada"}, Correct: 0, HostLine: "Norway – a thank-you since 1947."},
		}},
		estimate: show.Round{Title: "Estimation round", Intro: "Estimating is allowed – guessing too.", Questions: []show.Question{
			{Text: "Roughly how far is the Moon from Earth?", Options: []string{"38,000 km", "384,000 km", "1.5 million km", "3.8 million km"}, Correct: 1, HostLine: "About 384,000 kilometres – a long sleigh ride."},
			{Text: "How many bones does an adult human have?", Options: []string{"106", "206", "306", "406"}, Correct: 1, HostLine: "206 – and after the team event you feel every one."},
			{Text: "Roughly how high is Mount Everest?", Options: []string{"6,800 m", "7,900 m", "8,849 m", "9,500 m"}, Correct: 2, HostLine: "8,849 metres – taller than any ticket backlog."},
			{Text: "How many countries are in the EU?", Options: []string{"25", "27", "28", "30"}, Correct: 1, HostLine: "27 – since Brexit."},
		}},
	},
}
