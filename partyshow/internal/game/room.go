// Package game runs a live show: players join with their phones, answer
// questions against a timer and collect points; the host screen drives the flow.
package game

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ralpheichelberger/partyshow/internal/show"
)

// Phase is the step the show is in.
type Phase string

const (
	PhaseLobby    Phase = "lobby"    // QR code, players join
	PhaseIntro    Phase = "intro"    // round title and intro line
	PhaseQuestion Phase = "question" // players answer, timer runs
	PhaseReveal   Phase = "reveal"   // correct answer and answer distribution
	PhaseScores   Phase = "scores"   // leaderboard after a round
	PhaseFinal    Phase = "final"    // winners
)

// Points for a correct answer: Base plus up to Speed for answering fast.
const (
	BasePoints   = 500
	SpeedPoints  = 500
	MaxNameRunes = 20
)

var (
	ErrFull        = errors.New("the show is full")
	ErrBadName     = errors.New("please enter a name")
	ErrNotAnswer   = errors.New("not accepting answers right now")
	ErrAnswered    = errors.New("already answered")
	ErrBadOption   = errors.New("invalid option")
	ErrNoPlayer    = errors.New("unknown player")
	ErrShowEnded   = errors.New("the show has ended")
	ErrBadRejoin   = errors.New("cannot rejoin")
	ErrNoQuestions = errors.New("show has no questions")
)

// Clock abstracts time for tests.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is a stoppable timer.
type Timer interface{ Stop() bool }

type realClock struct{}

func (realClock) Now() time.Time                            { return time.Now() }
func (realClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// RealClock is the wall clock.
var RealClock Clock = realClock{}

// Player is one phone.
type Player struct {
	ID         string
	Token      string
	Name       string
	Score      int
	LastPoints int
	LastRight  bool
	Connected  bool
	joinedAt   time.Time
}

type answer struct {
	option int
	points int
}

// Room is one running show.
type Room struct {
	Code       string
	Show       *show.Show
	MaxPlayers int

	mu        sync.Mutex
	clock     Clock
	onChange  func()
	phase     Phase
	round     int
	question  int
	seq       int // increments per question, guards stale timers
	deadline  time.Time
	started   time.Time
	timer     Timer
	players   map[string]*Player
	order     []string // join order
	answers   map[string]answer
	lastTouch time.Time
}

// NewRoom prepares a room in the lobby. onChange is called (outside the lock)
// after a timer moves the show forward, so the server can broadcast.
func NewRoom(code string, s *show.Show, maxPlayers int, clock Clock, onChange func()) (*Room, error) {
	if s.QuestionCount() == 0 {
		return nil, ErrNoQuestions
	}
	if clock == nil {
		clock = RealClock
	}
	if onChange == nil {
		onChange = func() {}
	}
	return &Room{
		Code: code, Show: s, MaxPlayers: maxPlayers, clock: clock, onChange: onChange,
		phase: PhaseLobby, players: map[string]*Player{}, answers: map[string]answer{},
		lastTouch: clock.Now(),
	}, nil
}

// Join adds a player. Duplicate names get a number appended.
func (r *Room) Join(name string) (*Player, error) {
	name = cleanName(name)
	if name == "" {
		return nil, ErrBadName
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastTouch = r.clock.Now()
	if r.phase == PhaseFinal {
		return nil, ErrShowEnded
	}
	if len(r.players) >= r.MaxPlayers {
		return nil, ErrFull
	}
	p := &Player{ID: randomHex(8), Token: randomHex(16), Name: r.uniqueName(name), Connected: true, joinedAt: r.clock.Now()}
	r.players[p.ID] = p
	r.order = append(r.order, p.ID)
	cp := *p
	return &cp, nil
}

// Rejoin reconnects a player after a dropped connection or page reload.
func (r *Room) Rejoin(id, token string) (*Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[id]
	if !ok || subtle.ConstantTimeCompare([]byte(p.Token), []byte(token)) != 1 {
		return nil, ErrBadRejoin
	}
	p.Connected = true
	r.lastTouch = r.clock.Now()
	cp := *p
	return &cp, nil
}

// SetConnected marks a player's phone as online or offline.
func (r *Room) SetConnected(id string, connected bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.players[id]; ok {
		p.Connected = connected
	}
}

// Kick removes a player (host action, e.g. for a rude name).
func (r *Room) Kick(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.players[id]; !ok {
		return
	}
	delete(r.players, id)
	delete(r.answers, id)
	for i, pid := range r.order {
		if pid == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

// Next moves the show one step forward (host button).
func (r *Room) Next() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastTouch = r.clock.Now()
	switch r.phase {
	case PhaseLobby:
		r.phase = PhaseIntro
	case PhaseIntro:
		r.startQuestionLocked()
	case PhaseQuestion:
		r.revealLocked()
	case PhaseReveal:
		if r.question+1 < len(r.Show.Rounds[r.round].Questions) {
			r.question++
			r.startQuestionLocked()
		} else {
			r.phase = PhaseScores
		}
	case PhaseScores:
		if r.round+1 < len(r.Show.Rounds) {
			r.round++
			r.question = 0
			r.phase = PhaseIntro
		} else {
			r.phase = PhaseFinal
		}
	}
}

// Answer records a player's choice and scores it.
func (r *Room) Answer(playerID string, option int) error {
	r.mu.Lock()
	if r.phase != PhaseQuestion {
		r.mu.Unlock()
		return ErrNotAnswer
	}
	if _, ok := r.players[playerID]; !ok {
		r.mu.Unlock()
		return ErrNoPlayer
	}
	if _, done := r.answers[playerID]; done {
		r.mu.Unlock()
		return ErrAnswered
	}
	q := r.currentQuestionLocked()
	if option < 0 || option >= len(q.Options) {
		r.mu.Unlock()
		return ErrBadOption
	}
	now := r.clock.Now()
	pts := 0
	if option == q.Correct {
		total := r.deadline.Sub(r.started)
		left := r.deadline.Sub(now)
		if left < 0 {
			left = 0
		}
		pts = BasePoints
		if total > 0 {
			pts += int(float64(SpeedPoints) * float64(left) / float64(total))
		}
	}
	r.answers[playerID] = answer{option: option, points: pts}
	if r.allAnsweredLocked() {
		r.revealLocked()
	}
	r.mu.Unlock()
	return nil
}

func (r *Room) allAnsweredLocked() bool {
	online := 0
	for id, p := range r.players {
		if !p.Connected {
			continue
		}
		online++
		if _, ok := r.answers[id]; !ok {
			return false
		}
	}
	return online > 0
}

func (r *Room) startQuestionLocked() {
	q := r.currentQuestionLocked()
	r.phase = PhaseQuestion
	r.answers = map[string]answer{}
	r.seq++
	seq := r.seq
	r.started = r.clock.Now()
	secs := q.Seconds
	if secs == 0 {
		secs = show.DefaultSeconds
	}
	d := time.Duration(secs) * time.Second
	r.deadline = r.started.Add(d)
	if r.timer != nil {
		r.timer.Stop()
	}
	r.timer = r.clock.AfterFunc(d, func() { r.timeout(seq) })
}

func (r *Room) timeout(seq int) {
	r.mu.Lock()
	if r.phase != PhaseQuestion || r.seq != seq {
		r.mu.Unlock()
		return
	}
	r.revealLocked()
	r.mu.Unlock()
	r.onChange()
}

func (r *Room) revealLocked() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	for id, p := range r.players {
		a, ok := r.answers[id]
		p.LastPoints = a.points
		p.LastRight = ok && a.points > 0
		p.Score += a.points
	}
	r.phase = PhaseReveal
}

func (r *Room) currentQuestionLocked() show.Question {
	return r.Show.Rounds[r.round].Questions[r.question]
}

func (r *Room) uniqueName(name string) string {
	taken := map[string]bool{}
	for _, p := range r.players {
		taken[strings.ToLower(p.Name)] = true
	}
	if !taken[strings.ToLower(name)] {
		return name
	}
	for i := 2; ; i++ {
		n := fmt.Sprintf("%s %d", name, i)
		if !taken[strings.ToLower(n)] {
			return n
		}
	}
}

// Phase returns the current step.
func (r *Room) Phase() Phase {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.phase
}

// LastTouch is when the room was last used.
func (r *Room) LastTouch() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastTouch
}

// Close stops the timer.
func (r *Room) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

func cleanName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	var b strings.Builder
	for _, c := range name {
		if c < 32 || c == 0x7f {
			continue
		}
		b.WriteRune(c)
	}
	name = b.String()
	if utf8.RuneCountInString(name) > MaxNameRunes {
		name = string([]rune(name)[:MaxNameRunes])
	}
	return strings.TrimSpace(name)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// ---- views sent to screens ----

// Ranked is a leaderboard entry.
type Ranked struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Score      int    `json:"score"`
	LastPoints int    `json:"last_points"`
	Rank       int    `json:"rank"`
}

// QuestionView is a question as screens see it. Correct is -1 until the reveal.
type QuestionView struct {
	Text        string   `json:"text"`
	Options     []string `json:"options"`
	Correct     int      `json:"correct"`
	HostLine    string   `json:"host_line,omitempty"`
	Explanation string   `json:"explanation,omitempty"`
	Seconds     int      `json:"seconds"`
	MsLeft      int64    `json:"ms_left"`
	Counts      []int    `json:"counts,omitempty"`
}

// HostView is the full state for the projector screen.
type HostView struct {
	Code        string        `json:"code"`
	Phase       Phase         `json:"phase"`
	Title       string        `json:"title"`
	Welcome     string        `json:"welcome,omitempty"`
	Outro       string        `json:"outro,omitempty"`
	Language    string        `json:"language"`
	RoundNo     int           `json:"round_no"`
	RoundCount  int           `json:"round_count"`
	RoundTitle  string        `json:"round_title,omitempty"`
	RoundIntro  string        `json:"round_intro,omitempty"`
	QuestionNo  int           `json:"question_no"`
	InRound     int           `json:"questions_in_round"`
	Question    *QuestionView `json:"question,omitempty"`
	Answered    int           `json:"answered"`
	Players     []Ranked      `json:"players"`
	PlayerCount int           `json:"player_count"`
	MaxPlayers  int           `json:"max_players"`
}

// PlayerView is what one phone sees.
type PlayerView struct {
	Phase      Phase    `json:"phase"`
	Title      string   `json:"title"`
	Language   string   `json:"language"`
	Name       string   `json:"name"`
	Score      int      `json:"score"`
	Rank       int      `json:"rank"`
	Players    int      `json:"players"`
	RoundTitle string   `json:"round_title,omitempty"`
	Question   string   `json:"question,omitempty"`
	Options    []string `json:"options,omitempty"`
	MsLeft     int64    `json:"ms_left,omitempty"`
	Answered   bool     `json:"answered"`
	Choice     int      `json:"choice"`
	Correct    int      `json:"correct"`
	LastPoints int      `json:"last_points"`
	LastRight  bool     `json:"last_right"`
}

func (r *Room) rankingLocked() []Ranked {
	out := make([]Ranked, 0, len(r.players))
	for _, id := range r.order {
		p := r.players[id]
		out = append(out, Ranked{ID: p.ID, Name: p.Name, Score: p.Score, LastPoints: p.LastPoints})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	for i := range out {
		out[i].Rank = i + 1
		if i > 0 && out[i].Score == out[i-1].Score {
			out[i].Rank = out[i-1].Rank
		}
	}
	return out
}

// HostView returns the projector state.
func (r *Room) HostView() HostView {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.Show
	v := HostView{
		Code: r.Code, Phase: r.phase, Title: s.Title, Language: s.Language,
		RoundNo: r.round + 1, RoundCount: len(s.Rounds),
		PlayerCount: len(r.players), MaxPlayers: r.MaxPlayers,
		Players: r.rankingLocked(), Answered: len(r.answers),
	}
	switch r.phase {
	case PhaseLobby:
		v.Welcome = s.Welcome
	case PhaseFinal:
		v.Outro = s.Outro
	}
	if r.phase != PhaseLobby && r.phase != PhaseFinal {
		rd := s.Rounds[r.round]
		v.RoundTitle, v.RoundIntro = rd.Title, rd.Intro
		v.QuestionNo, v.InRound = r.question+1, len(rd.Questions)
	}
	if r.phase == PhaseQuestion || r.phase == PhaseReveal {
		q := r.currentQuestionLocked()
		qv := &QuestionView{Text: q.Text, Options: q.Options, Correct: -1, Seconds: q.Seconds}
		if r.phase == PhaseQuestion {
			qv.MsLeft = msLeft(r.deadline, r.clock.Now())
		} else {
			qv.Correct, qv.HostLine, qv.Explanation = q.Correct, q.HostLine, q.Explanation
			qv.Counts = make([]int, len(q.Options))
			for _, a := range r.answers {
				qv.Counts[a.option]++
			}
		}
		v.Question = qv
	}
	return v
}

// PlayerView returns one phone's state.
func (r *Room) PlayerView(id string) (PlayerView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[id]
	if !ok {
		return PlayerView{}, ErrNoPlayer
	}
	v := PlayerView{
		Phase: r.phase, Title: r.Show.Title, Language: r.Show.Language, Name: p.Name,
		Score: p.Score, Players: len(r.players), Choice: -1, Correct: -1,
		LastPoints: p.LastPoints, LastRight: p.LastRight,
	}
	for _, rk := range r.rankingLocked() {
		if rk.ID == id {
			v.Rank = rk.Rank
		}
	}
	if r.phase != PhaseLobby && r.phase != PhaseFinal {
		v.RoundTitle = r.Show.Rounds[r.round].Title
	}
	if r.phase == PhaseQuestion || r.phase == PhaseReveal {
		q := r.currentQuestionLocked()
		v.Question, v.Options = q.Text, q.Options
		if a, ok := r.answers[id]; ok {
			v.Answered, v.Choice = true, a.option
		}
		if r.phase == PhaseQuestion {
			v.MsLeft = msLeft(r.deadline, r.clock.Now())
		} else {
			v.Correct = q.Correct
		}
	}
	return v, nil
}

func msLeft(deadline, now time.Time) int64 {
	if d := deadline.Sub(now); d > 0 {
		return d.Milliseconds()
	}
	return 0
}
