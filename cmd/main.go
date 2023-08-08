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

// @title Invoice Generator API
// @version 1.0
// @description This is an API for generating and managing invoices.
// @host localhost:8080
// @BasePath /
func main() {
	r := mux.NewRouter()
	// Serve the Swagger UI at /swagger/index.html
	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/generate-invoice", generateInvoiceHandler).Methods("POST")
	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("templates/index.html")
	t.Execute(w, nil)
}

// @Summary Generate Invoice
// @Description Generate an invoice based on the provided data
// @Tags Invoice
// @Accept json
// @Produce json
// @Param invoice_number formData string true "Invoice Number"
// @Param purchase_order formData string false "Purchase Order"
// @Param company_name formData string true "Company Name"
// @Param logo formData file false "Company Logo (JPG, JPEG, PNG, max 5MB)"
// @Param bill_to formData string true "Bill To"
// @Param currency formData string true "Currency"
// @Param invoice_date formData string true "Invoice Date"
// @Param due_date formData string true "Due Date"
// @Param notes formData string false "Notes / Payment Terms"
// @Param bank_account formData string false "Bank Account Details"
// @Param sub_total formData number true "Sub Total"
// @Param tax_percentage formData number false "Tax Percentage"
// @Param discount_amount formData number false "Discount Amount"
// @Param shipping_fee formData number false "Shipping Fee"
// @Param total formData number true "Total"
// @Param item_description formData array true "Item Description"
// @Param unit_cost formData array true "Unit Cost"
// @Param quantity formData array true "Quantity"
// @Param amount formData array true "Amount"
// @Success 200 {string} string "Invoice generated."
// @Failure 400 {string} string "Bad Request"
// @Router /generate-invoice [post]
func generateInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the form data to get the user's input
	r.ParseMultipartForm(5 * 1024 * 1024) // 5MB file size limit for logo
	invoiceNumber := r.Form.Get("invoice_number")
	purchaseOrder := r.Form.Get("purchase_order")
	companyName := r.Form.Get("company_name")
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
		InvoiceNumber: invoiceNumber,
		PurchaseOrder: purchaseOrder,
		CompanyName:   companyName,
		// Continue to set other fields as per the requirements
	}

	// Handle the logo upload (if provided) and save the logo path in the database
	logoPath := ""
	file, _, err := r.FormFile("logo")
	if err == nil {
		defer file.Close()
		// Save the logo to a temporary file or a cloud storage service
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
	// Continue to add other invoice details to the PDF as per the requirements

	// Save the PDF to a file or send it as a response for download
	pdf.OutputFileAndClose("invoice.pdf")

	// Respond with the invoice.html template, passing the Invoice instance
	t, _ := template.ParseFiles("templates/invoice.html")
	t.Execute(w, inv)
}
