// cmd/main.go

package main

import (
	"database/sql"
	"html/template"
	"io"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/invoicing-microservice/pkg/invoice"
	"github.com/jung-kurt/gofpdf"
	_ "github.com/lib/pq" // Import the PostgreSQL driver
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/generate-invoice", generateInvoiceHandler).Methods("POST")
	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("templates/index.html")
	t.Execute(w, nil)
}

func generateInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the form data to get the user's input
	r.ParseMultipartForm(5 * 1024 * 1024) // 5MB file size limit for logo
	invoiceNumber := r.Form.Get("invoice_number")
	purchaseOrder := r.Form.Get("purchase_order")
	companyName := r.Form.Get("company_name")
	invoiceDate := r.Form.Get("invoice_date")
	dueDate := r.Form.Get("due_date")
	billTo := r.Form.Get("bill_to")
	currency := r.Form.Get("currency")
	notes := r.Form.Get("notes")
	bankAccount := r.Form.Get("bank_account")
	subTotal := r.Form.Get("sub_total")
	taxPercentage := r.Form.Get("tax_percentage")
	discountAmount := r.Form.Get("discount_amount")
	shippingFee := r.Form.Get("shipping_fee")
	total := r.Form.Get("total")

	// Continue to parse other form fields as per the requirements

	// Connect to the PostgreSQL database
	db, err := sql.Open("postgres", "postgres://invoiceuser:invoicepass@localhost/invoicedb?sslmode=disable")
	if err != nil {
		http.Error(w, "Error connecting to the database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Insert the invoice data into the database
	_, err = db.Exec("INSERT INTO invoices (invoice_number, purchase_order, company_name) VALUES ($1, $2, $3)",
		invoiceNumber, purchaseOrder, companyName)
	if err != nil {
		http.Error(w, "Error inserting data into the database", http.StatusInternalServerError)
		return
	}

	// Create an Invoice instance
	inv := invoice.Invoice{
		InvoiceNumber:  invoiceNumber,
		PurchaseOrder:  purchaseOrder,
		CompanyName:    companyName,
		InvoiceDate:    invoiceDate,
		DueDate:        dueDate,
		BillTo:         billTo,
		Currency:       currency,
		Notes:          notes,
		BankAccount:    bankAccount,
		SubTotal:       subTotal,
		TaxPercentage:  taxPercentage,
		DiscountAmount: discountAmount,
		ShippingFee:    shippingFee,
		Total:          total,
		LogoPath:       logoPath,
	}

	// Handle the logo upload (if provided) and save the logo path in the database
	logoPath := ""
	file, _, err := r.FormFile("logo")
	if err == nil {
		defer file.Close()
		// Save the logo to a temporary file or a cloud storage service
		//use s3 bucket

		// For this example, we'll save it to a "logo.png" file in the current directory
		outFile, err := os.Create("logo.png")
		if err != nil {
			http.Error(w, "Error saving the logo", http.StatusInternalServerError)
			return
		}
		defer outFile.Close()
		io.Copy(outFile, file)

		logoPath = "logo.png"
		inv.LogoPath = logoPath
	}

	// Generate the PDF invoice
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Invoice Number: "+inv.InvoiceNumber)
	pdf.Cell(40, 10, "Purchase Order: "+inv.PurchaseOrder)
	pdf.Cell(40, 10, "Company Name: "+inv.CompanyName)
	pdf.Cell(40, 10, "Invoice Date: "+inv.InvoiceDate)
	pdf.Cell(40, 10, "Due Date: "+inv.DueDate)
	pdf.Cell(40, 10, "Bill To: "+inv.BillTo)
	pdf.Cell(40, 10, "Currency: "+inv.Currency)
	pdf.Cell(40, 10, "Notes: "+inv.Notes)
	pdf.Cell(40, 10, "Bank Account: "+inv.BankAccount)
	pdf.Cell(40, 10, "Sub Total: "+inv.SubTotal)
	pdf.Cell(40, 10, "Tax Percentage: "+inv.TaxPercentage)
	pdf.Cell(40, 10, "Discount Amount: "+inv.DiscountAmount)
	pdf.Cell(40, 10, "Shipping Fee: "+inv.ShippingFee)
	pdf.Cell(40, 10, "Total: "+inv.Total)
	pdf.Cell(40, 10, "Logo Path: "+inv.LogoPath)

	// Continue to add other invoice details to the PDF as per the requirements

	// Save the PDF to a file or send it as a response for download
	pdf.OutputFileAndClose("invoice.pdf")

	// Respond with the invoice.html template, passing the Invoice instance
	t, _ := template.ParseFiles("templates/invoice.html")
	t.Execute(w, inv)
}
