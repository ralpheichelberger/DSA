package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramNotifier_NotConfiguredWhenMissingFields(t *testing.T) {
	assert.False(t, NewTelegramNotifier("token", "").Configured())
	assert.False(t, NewTelegramNotifier("", "chat").Configured())
	assert.True(t, NewTelegramNotifier("token", "chat").Configured())
}

func TestTelegramNotifier_SendsCorrectMessage(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewTelegramNotifierWithBaseURL("token123", "chat123", srv.URL)
	err := n.Send(context.Background(), newTestNotification(SeverityCritical))
	require.NoError(t, err)
	assert.Equal(t, "chat123", got["chat_id"])
	text, _ := got["text"].(string)
	assert.Contains(t, text, "🔴")
	assert.Equal(t, "Markdown", got["parse_mode"])
}

func TestTelegramNotifier_IncludesContextInMessage(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewTelegramNotifierWithBaseURL("token123", "chat123", srv.URL)
	note := newTestNotification(SeverityWarning)
	note.Context = map[string]string{"credits": "15", "refill_at": "2026-06-05"}
	err := n.Send(context.Background(), note)
	require.NoError(t, err)
	text, _ := got["text"].(string)
	assert.Contains(t, text, "credits: 15")
}
