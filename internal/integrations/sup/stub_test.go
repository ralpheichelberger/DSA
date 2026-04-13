package sup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ Supplier = (*Stub)(nil)

func TestStub_GetProductCost(t *testing.T) {
	s := NewStub()
	data, err := s.GetProductCost(context.Background(), "prod-123")
	require.NoError(t, err)
	require.NotNil(t, data)

	assert.NotEmpty(t, data.ProductID)
	assert.Greater(t, data.COGSEur, 0.0)
	assert.Greater(t, data.ShippingCostEur, 0.0)
	assert.Greater(t, data.ShippingDays, 0)
	assert.True(t, data.StockAvailable)
}
