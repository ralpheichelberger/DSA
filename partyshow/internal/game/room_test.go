package game

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ralpheichelberger/partyshow/internal/show"
)

type fakeTimer struct {
	c       *fakeClock
	at      time.Time
	f       func()
	stopped bool
}

func (t *fakeTimer) Stop() bool { t.stopped = true; return true }

type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{c: c, at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves time forward and fires due timers.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*fakeTimer
	for _, t := range c.timers {
		if !t.stopped && !t.at.After(c.now) {
			t.stopped = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	for _, t := range due {
		t.f()
	}
}

func testShow() *show.Show {
	return &show.Show{ID: "s1", Title: "Show", Language: "de", Plan: show.PlanFree, Rounds: []show.Round{
		{Title: "R1", Questions: []show.Question{
			{Text: "Q1", Options: []string{"a", "b", "c", "d"}, Correct: 2, Seconds: 20},
			{Text: "Q2", Options: []string{"x", "y"}, Correct: 0, Seconds: 10},
		}},
		{Title: "R2", Questions: []show.Question{
			{Text: "Q3", Options: []string{"1", "2", "3"}, Correct: 1, Seconds: 10},
		}},
	}}
}

func newTestRoom(t *testing.T, max int) (*Room, *fakeClock, *int) {
	clock := &fakeClock{now: time.Date(2026, 12, 12, 19, 0, 0, 0, time.UTC)}
	changes := 0
	r, err := NewRoom("ABCDE", testShow(), max, clock, func() { changes++ })
	require.NoError(t, err)
	return r, clock, &changes
}

func TestJoinNamesAndLimits(t *testing.T) {
	r, _, _ := newTestRoom(t, 3)
	a, err := r.Join("  Anna  ")
	require.NoError(t, err)
	assert.Equal(t, "Anna", a.Name)
	b, err := r.Join("anna")
	require.NoError(t, err)
	assert.Equal(t, "anna 2", b.Name)
	_, err = r.Join("   ")
	assert.ErrorIs(t, err, ErrBadName)
	long, err := r.Join(strings.Repeat("x", 50) + "\x07")
	require.NoError(t, err)
	assert.Len(t, []rune(long.Name), MaxNameRunes)
	_, err = r.Join("Dora")
	assert.ErrorIs(t, err, ErrFull)

	again, err := r.Rejoin(a.ID, a.Token)
	require.NoError(t, err)
	assert.Equal(t, "Anna", again.Name)
	_, err = r.Rejoin(a.ID, "bad")
	assert.ErrorIs(t, err, ErrBadRejoin)
}

func TestFullFlowWithScoring(t *testing.T) {
	r, clock, changes := newTestRoom(t, 15)
	anna, _ := r.Join("Anna")
	ben, _ := r.Join("Ben")

	assert.Equal(t, PhaseLobby, r.HostView().Phase)
	assert.ErrorIs(t, r.Answer(anna.ID, 0), ErrNotAnswer)

	r.Next() // intro R1
	hv := r.HostView()
	assert.Equal(t, PhaseIntro, hv.Phase)
	assert.Equal(t, "R1", hv.RoundTitle)

	r.Next() // Q1
	hv = r.HostView()
	require.NotNil(t, hv.Question)
	assert.Equal(t, -1, hv.Question.Correct, "correct answer must stay hidden")
	pv, _ := r.PlayerView(anna.ID)
	assert.Equal(t, -1, pv.Correct)
	assert.Equal(t, int64(20000), pv.MsLeft)

	// Anna answers right immediately: full points. Ben answers right after 10 of 20 s.
	require.NoError(t, r.Answer(anna.ID, 2))
	assert.ErrorIs(t, r.Answer(anna.ID, 1), ErrAnswered)
	assert.ErrorIs(t, r.Answer(ben.ID, 9), ErrBadOption)
	clock.Advance(10 * time.Second)
	require.NoError(t, r.Answer(ben.ID, 2))

	// Everyone answered: reveal without waiting for the timer.
	hv = r.HostView()
	assert.Equal(t, PhaseReveal, hv.Phase)
	assert.Equal(t, 2, hv.Question.Correct)
	assert.Equal(t, []int{0, 0, 2, 0}, hv.Question.Counts)
	assert.Equal(t, "Anna", hv.Players[0].Name)
	assert.Equal(t, 1000, hv.Players[0].Score)
	assert.Equal(t, 750, hv.Players[1].Score)

	// The stale timer of Q1 must not affect anything later.
	before := *changes
	clock.Advance(20 * time.Second)
	assert.Equal(t, before, *changes)

	r.Next()                                 // Q2
	require.NoError(t, r.Answer(anna.ID, 1)) // wrong
	clock.Advance(10 * time.Second)          // Ben never answers: timeout reveals
	assert.Equal(t, PhaseReveal, r.HostView().Phase)
	assert.Equal(t, before+1, *changes)
	pv, _ = r.PlayerView(anna.ID)
	assert.False(t, pv.LastRight)
	assert.Equal(t, 0, pv.LastPoints)
	assert.Equal(t, 1, pv.Rank)

	r.Next() // scores after R1
	assert.Equal(t, PhaseScores, r.HostView().Phase)
	r.Next() // intro R2
	assert.Equal(t, "R2", r.HostView().RoundTitle)
	r.Next() // Q3
	r.Next() // host skips the timer
	assert.Equal(t, PhaseReveal, r.HostView().Phase)
	r.Next() // scores
	r.Next() // final
	hv = r.HostView()
	assert.Equal(t, PhaseFinal, hv.Phase)
	_, err := r.Join("Late")
	assert.ErrorIs(t, err, ErrShowEnded)
	r.Next() // stays final
	assert.Equal(t, PhaseFinal, r.HostView().Phase)
}

func TestDisconnectedPlayersDontBlockReveal(t *testing.T) {
	r, _, _ := newTestRoom(t, 15)
	a, _ := r.Join("Anna")
	b, _ := r.Join("Ben")
	r.SetConnected(b.ID, false)
	r.Next()
	r.Next()
	require.NoError(t, r.Answer(a.ID, 0))
	assert.Equal(t, PhaseReveal, r.HostView().Phase)
}

func TestKickAndTies(t *testing.T) {
	r, _, _ := newTestRoom(t, 15)
	a, _ := r.Join("Anna")
	b, _ := r.Join("Ben")
	c, _ := r.Join("Rude")
	r.Kick(c.ID)
	hv := r.HostView()
	assert.Equal(t, 2, hv.PlayerCount)
	assert.Equal(t, 1, hv.Players[0].Rank)
	assert.Equal(t, 1, hv.Players[1].Rank, "equal scores share the rank")
	_, err := r.PlayerView(c.ID)
	assert.ErrorIs(t, err, ErrNoPlayer)
	_ = a
	_ = b
}

func TestRooms(t *testing.T) {
	clock := &fakeClock{now: time.Now()}
	rs := NewRooms(clock)
	s := testShow()
	r1, err := rs.Open(s, func(string) {})
	require.NoError(t, err)
	assert.Len(t, r1.Code, CodeLength)
	r2, _ := rs.Open(s, func(string) {})
	assert.Same(t, r1, r2, "reopening the host page reuses the room")

	got, err := rs.Get(r1.Code)
	require.NoError(t, err)
	assert.Same(t, r1, got)
	_, err = rs.Get("ZZZZZ")
	assert.ErrorIs(t, err, ErrNoRoom)

	rs.Restart(s.ID)
	_, err = rs.Get(r1.Code)
	assert.ErrorIs(t, err, ErrNoRoom)

	r3, _ := rs.Open(s, func(string) {})
	clock.Advance(13 * time.Hour)
	assert.Equal(t, 1, rs.Sweep(12*time.Hour))
	_, err = rs.Get(r3.Code)
	assert.ErrorIs(t, err, ErrNoRoom)

	_, err = rs.Open(&show.Show{ID: "empty"}, func(string) {})
	assert.ErrorIs(t, err, ErrNoQuestions)
}
