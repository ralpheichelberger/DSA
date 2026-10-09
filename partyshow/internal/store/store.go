// Package store keeps shows as JSON files, one per show.
package store

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/ralpheichelberger/partyshow/internal/show"
)

// ErrNotFound means no show has that ID.
var ErrNotFound = errors.New("show not found")

// ErrForbidden means the edit token doesn't match.
var ErrForbidden = errors.New("wrong edit token")

var validID = regexp.MustCompile(`^[a-f0-9]{16}$`)

// Store saves shows under Dir.
type Store struct {
	Dir string
	mu  sync.Mutex
	now func() time.Time
}

// New creates the directory if needed.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Store{Dir: dir, now: time.Now}, nil
}

// Create assigns an ID and edit token and saves the show.
func (s *Store) Create(sh *show.Show) error {
	sh.ID = randomHex(8)
	sh.EditToken = randomHex(16)
	sh.CreatedAt = s.now().UTC()
	sh.UpdatedAt = sh.CreatedAt
	if sh.Plan == "" {
		sh.Plan = show.PlanFree
	}
	return s.write(sh)
}

// Get loads a show by ID.
func (s *Store) Get(id string) (*show.Show, error) {
	if !validID.MatchString(id) {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var sh show.Show
	if err := json.Unmarshal(data, &sh); err != nil {
		return nil, fmt.Errorf("corrupt show %s: %w", id, err)
	}
	return &sh, nil
}

// Authorize loads a show and checks its edit token.
func (s *Store) Authorize(id, token string) (*show.Show, error) {
	sh, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if token == "" || subtle.ConstantTimeCompare([]byte(sh.EditToken), []byte(token)) != 1 {
		return nil, ErrForbidden
	}
	return sh, nil
}

// UpdateContent replaces the editable parts of a show after validating them.
func (s *Store) UpdateContent(id, token string, edited show.Show) (*show.Show, error) {
	sh, err := s.Authorize(id, token)
	if err != nil {
		return nil, err
	}
	sh.Title = edited.Title
	sh.Welcome = edited.Welcome
	sh.Outro = edited.Outro
	sh.Rounds = edited.Rounds
	if err := sh.Validate(); err != nil {
		return nil, err
	}
	sh.UpdatedAt = s.now().UTC()
	return sh, s.write(sh)
}

// SetPlan changes the plan, e.g. after a payment.
func (s *Store) SetPlan(id string, plan show.Plan) error {
	sh, err := s.Get(id)
	if err != nil {
		return err
	}
	sh.Plan = plan
	sh.UpdatedAt = s.now().UTC()
	return s.write(sh)
}

// DeleteOlderThan removes shows last updated before the cutoff (data minimization).
func (s *Store) DeleteOlderThan(cutoff time.Time) (int, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		id := trimExt(e.Name())
		sh, err := s.Get(id)
		if err != nil {
			continue
		}
		if sh.UpdatedAt.Before(cutoff) {
			s.mu.Lock()
			err = os.Remove(s.path(id))
			s.mu.Unlock()
			if err == nil {
				n++
			}
		}
	}
	return n, nil
}

func (s *Store) write(sh *show.Show) error {
	data, err := json.MarshalIndent(sh, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp := s.path(sh.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(sh.ID))
}

func (s *Store) path(id string) string { return filepath.Join(s.Dir, id+".json") }

func trimExt(name string) string { return name[:len(name)-len(filepath.Ext(name))] }

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
