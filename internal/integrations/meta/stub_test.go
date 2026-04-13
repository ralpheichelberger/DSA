package meta

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ AdPlatform = (*Stub)(nil)

func TestStub_CreateAndGetCampaign(t *testing.T) {
	s := NewStub()
	id, err := s.CreateCampaign(context.Background(), "Smart Bottle", 30, []AdCreative{
		{Type: "video", Headline: "Hydrate smarter", Body: "Track intake automatically", CTA: "Buy now", MediaURL: "https://example.com/video.mp4"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, id)

	m, err := s.GetMetrics(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, id, m.CampaignID)
	assert.Greater(t, m.ROAS, 0.0)
}
