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
