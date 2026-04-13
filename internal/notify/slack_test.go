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

func TestSlackNotifier_NotConfiguredWhenEmpty(t *testing.T) {
	assert.False(t, NewSlackNotifier("").Configured())
}

func TestSlackNotifier_SendsCorrectPayload(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewSlackNotifier(srv.URL)
	err := n.Send(context.Background(), newTestNotification(SeverityCritical))
	require.NoError(t, err)

	text, _ := got["text"].(string)
	assert.Contains(t, text, "Minea credits low")

	attachments, ok := got["attachments"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, attachments)
	first, ok := attachments[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "#FF0000", first["color"])
}

func TestSlackNotifier_ReturnsErrorOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := NewSlackNotifier(srv.URL)
	err := n.Send(context.Background(), newTestNotification(SeverityCritical))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
