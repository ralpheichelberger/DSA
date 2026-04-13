package tiktok

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ AdPlatform = (*Stub)(nil)

func TestStub_CreateAndGetCampaign(t *testing.T) {
	s := NewStub()
	id, err := s.CreateCampaign(context.Background(), "Pet Feeder", 25, []AdCreative{
		{Type: "video", Headline: "Feed from phone", Body: "Control meals remotely", CTA: "Try now", MediaURL: "https://example.com/tiktok.mp4"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, id)

	m, err := s.GetMetrics(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, id, m.CampaignID)
	assert.Greater(t, m.ROAS, 0.0)
	assert.Greater(t, m.CTRPct, 0.0)
}
