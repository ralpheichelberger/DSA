package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ralpheichelberger/partyshow/internal/show"
)

func sample() *show.Show {
	return &show.Show{Title: "T", Rounds: []show.Round{{Title: "R", Questions: []show.Question{
		{Text: "Q", Options: []string{"a", "b"}, Correct: 1},
	}}}}
}

func TestCreateGetAuthorize(t *testing.T) {
	st, err := New(t.TempDir())
	require.NoError(t, err)
	sh := sample()
	require.NoError(t, st.Create(sh))
	assert.Len(t, sh.ID, 16)
	assert.Len(t, sh.EditToken, 32)
	assert.Equal(t, show.PlanFree, sh.Plan)

	got, err := st.Get(sh.ID)
	require.NoError(t, err)
	assert.Equal(t, "T", got.Title)

	_, err = st.Authorize(sh.ID, "wrong")
	assert.ErrorIs(t, err, ErrForbidden)
	_, err = st.Authorize(sh.ID, "")
	assert.ErrorIs(t, err, ErrForbidden)
	_, err = st.Authorize(sh.ID, sh.EditToken)
	assert.NoError(t, err)

	_, err = st.Get("../../etc/passwd")
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = st.Get("0123456789abcdef")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUpdateContent(t *testing.T) {
	st, _ := New(t.TempDir())
	sh := sample()
	require.NoError(t, st.Create(sh))

	edited := *sample()
	edited.Title = "Neu"
	edited.Plan = show.PlanLarge // must be ignored
	got, err := st.UpdateContent(sh.ID, sh.EditToken, edited)
	require.NoError(t, err)
	assert.Equal(t, "Neu", got.Title)
	assert.Equal(t, show.PlanFree, got.Plan)

	bad := *sample()
	bad.Rounds = nil
	_, err = st.UpdateContent(sh.ID, sh.EditToken, bad)
	assert.Error(t, err)
	reloaded, _ := st.Get(sh.ID)
	assert.Equal(t, "Neu", reloaded.Title)
}

func TestSetPlanAndCleanup(t *testing.T) {
	st, _ := New(t.TempDir())
	now := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return now }
	old := sample()
	require.NoError(t, st.Create(old))
	require.NoError(t, st.SetPlan(old.ID, show.PlanSmall))
	got, _ := st.Get(old.ID)
	assert.Equal(t, show.PlanSmall, got.Plan)

	now = now.Add(40 * 24 * time.Hour)
	fresh := sample()
	require.NoError(t, st.Create(fresh))

	n, err := st.DeleteOlderThan(now.Add(-30 * 24 * time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	_, err = st.Get(old.ID)
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = st.Get(fresh.ID)
	assert.NoError(t, err)
}
