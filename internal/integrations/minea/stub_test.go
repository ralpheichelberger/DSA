package minea

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ Discoverer = (*Stub)(nil)

func TestStub_GetTrendingProducts(t *testing.T) {
	s := NewStub()
	products, err := s.GetTrendingProducts(context.Background(), "", "DE", 10)
	require.NoError(t, err)
	require.NotEmpty(t, products)

	for _, p := range products {
		assert.NotEmpty(t, p.ID)
		assert.NotEmpty(t, p.Name)
		assert.NotEmpty(t, p.Niche)
		assert.NotEmpty(t, p.ShopifyStore)
		assert.NotEmpty(t, p.Platforms)
		assert.Greater(t, p.ActiveAdCount, 0)
		assert.Greater(t, p.EstimatedSellEur, 0.0)
	}
}
