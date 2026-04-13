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

func TestDiscordNotifier_NotConfiguredWhenEmpty(t *testing.T) {
	assert.False(t, NewDiscordNotifier("").Configured())
}

func TestDiscordNotifier_SendsEmbed(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	n := NewDiscordNotifier(srv.URL)
	err := n.Send(context.Background(), newTestNotification(SeverityCritical))
	require.NoError(t, err)

	embeds, ok := got["embeds"].([]any)
	require.True(t, ok)
	first, ok := embeds[0].(map[string]any)
	require.True(t, ok)
	title, _ := first["title"].(string)
	assert.Contains(t, title, "Minea credits low")
	assert.Equal(t, float64(16711680), first["color"])
}

func TestDiscordNotifier_ReturnsErrorOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	n := NewDiscordNotifier(srv.URL)
	err := n.Send(context.Background(), newTestNotification(SeverityWarning))
	assert.Error(t, err)
}
