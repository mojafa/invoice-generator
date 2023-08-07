// pkg/invoice/invoice.go

package invoice

// Invoice represents the invoice data model.
type Invoice struct {
	InvoiceNumber  string
	PurchaseOrder  string
	CompanyName    string
	BillTo         string
	Currency       string
	InvoiceDate    string
	DueDate        string
	Items          []Item
	Notes          string
	BankAccount    string
	SubTotal       float64
	TaxPercentage  float64
	DiscountAmount float64
	ShippingFee    float64
	Total          float64
	LogoPath       string
}

// Item represents an item in the invoice.
type Item struct {
	Description string
	UnitCost    float64
	Quantity    int
	Amount      float64
}
