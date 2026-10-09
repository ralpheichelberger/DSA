package server

import (
	"sync"
	"time"
)

// limiter caps AI generations per client IP per hour and in total per day,
// so a bot can't burn through the OpenAI budget.
type limiter struct {
	mu       sync.Mutex
	now      func() time.Time
	perIP    int
	perDay   int
	ipHits   map[string][]time.Time
	day      string
	dayCount int
}

func newLimiter(perIPPerHour, perDay int, now func() time.Time) *limiter {
	return &limiter{now: now, perIP: perIPPerHour, perDay: perDay, ipHits: map[string][]time.Time{}}
}

// allow records a hit and reports whether it is within both limits.
func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if d := now.UTC().Format("2006-01-02"); d != l.day {
		l.day, l.dayCount = d, 0
		l.ipHits = map[string][]time.Time{}
	}
	if l.perDay > 0 && l.dayCount >= l.perDay {
		return false
	}
	cutoff := now.Add(-time.Hour)
	hits := l.ipHits[ip][:0]
	for _, t := range l.ipHits[ip] {
		if t.After(cutoff) {
			hits = append(hits, t)
		}
	}
	if l.perIP > 0 && len(hits) >= l.perIP {
		l.ipHits[ip] = hits
		return false
	}
	l.ipHits[ip] = append(hits, now)
	l.dayCount++
	return true
}
