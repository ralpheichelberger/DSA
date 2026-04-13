package sup

import "context"

type SupplierData struct {
	ProductID       string
	COGSEur         float64
	ShippingCostEur float64
	ShippingDays    int
	WarehouseRegion string
	StockAvailable  bool
}

type Supplier interface {
	GetProductCost(ctx context.Context, productID string) (*SupplierData, error)
	ImportProduct(ctx context.Context, productID string, shopifyDomain string) error
}
