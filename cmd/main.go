package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gorilla/mux"
	"github.com/invoicing-microservice/pkg/invoice"
	"github.com/jung-kurt/gofpdf"
	_ "github.com/lib/pq"
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
	r.ParseMultipartForm(5 * 1024 * 1024) // 5MB file size limit for logo
	// Create a slice to hold line items
	var items []invoice.Item
	// Parse the form data to get the user's input
	invoiceNumber := r.Form.Get("invoice_number")
	purchaseOrder := r.Form.Get("purchase_order")
	companyName := r.Form.Get("company_name")
	invoiceDate := r.Form.Get("invoice_date")
	dueDate := r.Form.Get("due_date")
	billTo := r.Form.Get("bill_to")
	currency := r.Form.Get("currency")
	notes := r.Form.Get("notes")
	bankAccount := r.Form.Get("bank_account")

	// Collect user input arrays for line items
	itemDescriptions := r.Form["item_description[]"]
	unitCosts, _ := parseNumericArray(r.Form["unit_cost[]"])
	quantities, _ := parseIntegerArray(r.Form["quantity[]"])
	amounts, _ := parseNumericArray(r.Form["amount[]"])

	var logoPath string

	for i := 0; i < len(itemDescriptions); i++ {
		item := invoice.Item{
			Description: itemDescriptions[i],
			UnitCost:    unitCosts[i],
			Quantity:    quantities[i],
			Amount:      amounts[i],
		}
		items = append(items, item)
	}
	// Calculate sub total based on line item amounts
	var subTotalValue float64
	for _, item := range items {
		subTotalValue += item.Amount

	}

	// Parse user-provided inputs
	subTotal, err := strconv.ParseFloat(r.Form.Get("sub_total"), 64)
	if err != nil {
		http.Error(w, "Invalid sub total value", http.StatusBadRequest)
		return
	}

	taxPercentage, err := strconv.ParseFloat(r.Form.Get("tax_percentage"), 64)
	if err != nil {
		http.Error(w, "Invalid tax percentage value", http.StatusBadRequest)
		return
	}

	discountAmount, err := strconv.ParseFloat(r.Form.Get("discount_amount"), 64)
	if err != nil {
		http.Error(w, "Invalid discount amount value", http.StatusBadRequest)
		return
	}

	shippingFee, err := strconv.ParseFloat(r.Form.Get("shipping_fee"), 64)
	if err != nil {
		http.Error(w, "Invalid shipping fee value", http.StatusBadRequest)
		return
	}

	// Calculate total based on user input
	total := (subTotal+shippingFee)*(1+taxPercentage/100) - discountAmount

	// Connect to the PostgreSQL database
	db, err := sql.Open("postgres", "postgres://invoiceuser:invoicepass@localhost/invoicedb?sslmode=disable")
	if err != nil {
		http.Error(w, "Error connecting to the database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Insert the invoice data into the database
	// Insert the invoice data into the database
	var insertedID int // Variable to hold the returned id value

	row := db.QueryRow(
		"INSERT INTO invoices (invoice_number, purchase_order, company_name, invoice_date, due_date, bill_to, currency, notes, bank_account, sub_total, tax_percentage, discount_amount, shipping_fee, total, logo_path) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id",
		invoiceNumber, purchaseOrder, companyName, invoiceDate, dueDate, billTo, currency, notes, bankAccount, subTotal, taxPercentage, discountAmount, shippingFee, total, logoPath,
	)
	err = row.Scan(&insertedID) // Scan the returned id value

	if err != nil {
		http.Error(w, "Error inserting data into the database", http.StatusInternalServerError)
		return
	}

	// Get the inserted invoice ID
	var invoiceID int
	err = row.Scan(&invoiceID)
	if err != nil {
		http.Error(w, "Error retrieving inserted invoice ID", http.StatusInternalServerError)
		return
	}

	// Insert line items into the database
	for _, item := range items {
		_, err = db.Exec(
			"INSERT INTO line_items (invoice_id, description, unit_cost, quantity, amount) VALUES ($1, $2, $3, $4, $5)",
			invoiceID, item.Description, item.UnitCost, item.Quantity, item.Amount,
		)
		if err != nil {
			http.Error(w, "Error inserting line item into the database", http.StatusInternalServerError)
			return
		}
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
		Items:          []invoice.Item{},
	}

	// Handle the logo upload (

	// Handle the logo upload (if provided) and save the logo path in the database

	//using Amazon S3 involves additional setup and considerations
	// configuring bucket permissions,
	//  handling error cases, and
	//  managing access to S3 resources.

	file, _, err := r.FormFile("logo")
	if err == nil {
		defer file.Close()
		// Upload the logo to an S3 bucket
		sess, _ := session.NewSession(&aws.Config{
			Region: aws.String("us-east-1"), // Replace with your AWS region
		})

		// Specify the S3 bucket and key (path within the bucket) for the logo
		bucket := "logo-file-upload"
		key := "logo.png"

		// Upload the logo file to the S3 bucket
		uploader := s3manager.NewUploader(sess)
		_, err := uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   file,
		})
		if err != nil {
			http.Error(w, "Error uploading the logo to S3", http.StatusInternalServerError)
			return
		}

		// Construct the S3 URL for the logo
		logoPath = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, key)
	} else {
		// Handle the case where no logo was provided
		logoPath = ""
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
	pdf.Cell(40, 10, "Sub Total: "+strconv.FormatFloat(inv.SubTotal, 'f', 2, 64))             // Convert float to string with 2 decimal places
	pdf.Cell(40, 10, "Tax Percentage: "+strconv.FormatFloat(inv.TaxPercentage, 'f', 2, 64))   // Convert float to string with 2 decimal places
	pdf.Cell(40, 10, "Discount Amount: "+strconv.FormatFloat(inv.DiscountAmount, 'f', 2, 64)) // Convert float to string with 2 decimal places
	pdf.Cell(40, 10, "Shipping Fee: "+strconv.FormatFloat(inv.ShippingFee, 'f', 2, 64))       // Convert float to string with 2 decimal places
	pdf.Cell(40, 10, "Total: "+strconv.FormatFloat(inv.Total, 'f', 2, 64))                    // Convert float to string with 2 decimal places
	pdf.Cell(40, 10, "Logo Path: "+inv.LogoPath)

	// Save the PDF to a file or send it as a response for download
	// pdf.OutputFileAndClose("invoice.pdf")

	// Save the PDF to a buffer
	var pdfBuffer bytes.Buffer
	pdf.Output(&pdfBuffer)

	// If you want to save the PDF to a file
	pdfFilePath := "invoice.pdf"
	pdfFile, err := os.Create(pdfFilePath)
	if err != nil {
		http.Error(w, "Error creating PDF file", http.StatusInternalServerError)
		return
	}
	defer pdfFile.Close()
	pdfFile.Write(pdfBuffer.Bytes())

	// If you want to send the PDF as a response for download
	w.Header().Set("Content-Disposition", "attachment; filename=invoice.pdf")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBuffer.Bytes())))
	w.Write(pdfBuffer.Bytes())

	// Respond with the invoice.html template, passing the Invoice instance
	t, err := template.ParseFiles("templates/invoice.html")
	if err != nil {
		http.Error(w, "Error parsing invoice template", http.StatusInternalServerError)
		return
	}
	t.Execute(w, inv)
}

func parseNumericArray(input []string) ([]float64, error) {
	result := make([]float64, len(input))
	for i, val := range input {
		num, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, err // Return an error if parsing fails
		}
		result[i] = num
	}
	return result, nil
}

func parseIntegerArray(input []string) ([]int, error) {
	result := make([]int, len(input))
	for i, val := range input {
		num, err := strconv.Atoi(val)
		if err != nil {
			return nil, err // Return an error if parsing fails
		}
		result[i] = num
	}
	return result, nil
}
