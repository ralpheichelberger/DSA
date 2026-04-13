package sup

import (
	"context"
	"hash/fnv"
)

type Stub struct{}

func NewStub() *Stub {
	return &Stub{}
}

func (s *Stub) GetProductCost(ctx context.Context, productID string) (*SupplierData, error) {
	_ = ctx

	h := fnv.New32a()
	_, _ = h.Write([]byte(productID))
	n := h.Sum32()

	cogs := 8.0 + float64(n%11)          // 8-18
	shipping := 3.0 + float64((n/7)%4)   // 3-6
	shippingDays := 5 + int((n/13)%10)   // 5-14
	regions := []string{"CN", "EU", "US"}
	region := regions[int(n)%len(regions)]

	return &SupplierData{
		ProductID:       productID,
		COGSEur:         cogs,
		ShippingCostEur: shipping,
		ShippingDays:    shippingDays,
		WarehouseRegion: region,
		StockAvailable:  true,
	}, nil
}

func (s *Stub) ImportProduct(ctx context.Context, productID string, shopifyDomain string) error {
	_ = ctx
	_ = productID
	_ = shopifyDomain
	return nil
}
