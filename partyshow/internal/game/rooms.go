package game

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/ralpheichelberger/partyshow/internal/show"
)

// Code alphabet without look-alikes (0/O, 1/I/L).
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// CodeLength is the length of a join code.
const CodeLength = 5

// ErrNoRoom means no running show has that code.
var ErrNoRoom = errors.New("no show with this code")

// Rooms holds the running shows.
type Rooms struct {
	mu     sync.Mutex
	rooms  map[string]*Room
	byShow map[string]string // show ID -> code, so a reload of the host page reuses the room
	clock  Clock
}

// NewRooms returns an empty registry.
func NewRooms(clock Clock) *Rooms {
	if clock == nil {
		clock = RealClock
	}
	return &Rooms{rooms: map[string]*Room{}, byShow: map[string]string{}, clock: clock}
}

// Open returns the running room for a show, or starts a new one.
// onChange is used only for a new room.
func (rs *Rooms) Open(s *show.Show, onChange func(code string)) (*Room, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if code, ok := rs.byShow[s.ID]; ok {
		if r, ok := rs.rooms[code]; ok {
			return r, nil
		}
	}
	code := rs.newCodeLocked()
	r, err := NewRoom(code, s, s.Plan.MaxPlayers(), rs.clock, func() { onChange(code) })
	if err != nil {
		return nil, err
	}
	rs.rooms[code] = r
	rs.byShow[s.ID] = code
	return r, nil
}

// Restart throws away a show's running room so the host can start over.
func (rs *Rooms) Restart(showID string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if code, ok := rs.byShow[showID]; ok {
		if r, ok := rs.rooms[code]; ok {
			r.Close()
		}
		delete(rs.rooms, code)
		delete(rs.byShow, showID)
	}
}

// Running returns the show's room, if one is open.
func (rs *Rooms) Running(showID string) (*Room, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	code, ok := rs.byShow[showID]
	if !ok {
		return nil, false
	}
	r, ok := rs.rooms[code]
	return r, ok
}

// Get finds a room by its join code.
func (rs *Rooms) Get(code string) (*Room, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	r, ok := rs.rooms[code]
	if !ok {
		return nil, ErrNoRoom
	}
	return r, nil
}

// Sweep removes rooms unused for longer than maxIdle.
func (rs *Rooms) Sweep(maxIdle time.Duration) int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	cutoff := rs.clock.Now().Add(-maxIdle)
	n := 0
	for code, r := range rs.rooms {
		if r.LastTouch().Before(cutoff) {
			r.Close()
			delete(rs.rooms, code)
			delete(rs.byShow, r.Show.ID)
			n++
		}
	}
	return n
}

func (rs *Rooms) newCodeLocked() string {
	for {
		b := make([]byte, CodeLength)
		for i := range b {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
			if err != nil {
				panic(err)
			}
			b[i] = codeAlphabet[n.Int64()]
		}
		if _, taken := rs.rooms[string(b)]; !taken {
			return string(b)
		}
	}
}
