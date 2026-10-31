package orca

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func writeExportZip(state *State, job ExportJob) (string, exportManifest, error) {
	generated := time.Now().UTC()
	exportState, err := filterExportState(state, job.BusinessID, job.Filters)
	if err != nil {
		return "", exportManifest{}, err
	}
	datasets := map[string]any{
		"business":  sortedValues(exportState.Businesses),
		"customers": sortedValues(exportState.Customers), "products": sortedValues(exportState.Products), "plans": sortedValues(exportState.Plans), "tax_rules": sortedValues(exportState.Taxes), "discounts": sortedValues(exportState.Discounts),
		"quotation_templates": sortedValues(exportState.QuoteTemplates), "quotations": sortedValues(exportState.Quotations), "subscriptions": sortedValues(exportState.Subscriptions),
		"invoices": sortedValues(exportState.Invoices), "payments": sortedValues(exportState.Payments), "payment_allocations": sortedValues(exportState.Allocations),
		"journal": exportState.Journal, "audit": exportState.Audit, "approvals": sortedValues(exportState.Approvals),
	}
	counts := make(map[string]int, len(datasets))
	files := make(map[string][]byte, len(datasets)+3)
	checksums := map[string]string{}
	for name, value := range datasets {
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return "", exportManifest{}, err
		}
		filename := name + ".json"
		files[filename] = encoded
		counts[name] = count(value)
		sum := sha256.Sum256(encoded)
		checksums[filename] = hex.EncodeToString(sum[:])
	}
	for name, rows := range map[string][][]string{"customers.csv": customerCSV(exportState), "invoices.csv": invoiceCSV(exportState), "payments.csv": paymentCSV(exportState)} {
		var b bytes.Buffer
		writer := csv.NewWriter(&b)
		for _, row := range rows {
			safe := make([]string, len(row))
			for i, value := range row {
				safe[i] = csvSafe(value)
			}
			if err := writer.Write(safe); err != nil {
				return "", exportManifest{}, err
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return "", exportManifest{}, err
		}
		files[name] = b.Bytes()
		sum := sha256.Sum256(b.Bytes())
		checksums[name] = hex.EncodeToString(sum[:])
	}
	printCount, err := addInvoicePrintFiles(files, checksums, exportState)
	if err != nil {
		return "", exportManifest{}, err
	}
	counts["print_invoice_files"] = printCount
	manifestData := map[string]any{"version": "1.0.0", "businessId": job.BusinessID, "generatedAt": generated, "filters": job.Filters, "counts": counts, "checksums": checksums}
	manifestBytes, err := json.MarshalIndent(manifestData, "", "  ")
	if err != nil {
		return "", exportManifest{}, err
	}
	files["manifest.json"] = manifestBytes
	dir := os.Getenv("ORCA_EXPORT_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "orca-private-exports")
	}
	businessDir := filepath.Join(dir, job.BusinessID)
	if err := os.MkdirAll(businessDir, 0700); err != nil {
		return "", exportManifest{}, err
	}
	path := filepath.Join(businessDir, job.ID+".zip")
	temp := path + ".tmp"
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", exportManifest{}, err
	}
	zipWriter := zip.NewWriter(file)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, e := zipWriter.Create(name)
		if e != nil {
			_ = zipWriter.Close()
			_ = file.Close()
			_ = os.Remove(temp)
			return "", exportManifest{}, e
		}
		if _, e = entry.Write(files[name]); e != nil {
			_ = zipWriter.Close()
			_ = file.Close()
			_ = os.Remove(temp)
			return "", exportManifest{}, e
		}
	}
	if err := zipWriter.Close(); err != nil {
		_ = file.Close()
		_ = os.Remove(temp)
		return "", exportManifest{}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temp)
		return "", exportManifest{}, err
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return "", exportManifest{}, err
	}
	return path, exportManifest{GeneratedAt: generated, Data: manifestData}, nil
}

var invoicePrintTemplate = template.Must(template.New("invoice-print").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Invoice {{.Invoice.Number}}</title>
  <style>
    @page { size: A4; margin: 18mm; }
    body { color: #1b1a21; font: 14px/1.5 Arial, sans-serif; margin: 0 auto; max-width: 190mm; }
    header, .totals { border-bottom: 1px solid #dedce5; padding: 0 0 16px; }
    header { display: flex; justify-content: space-between; gap: 24px; }
    h1 { font-size: 26px; margin: 0 0 4px; }
    h2 { font-size: 13px; margin: 0 0 8px; text-transform: uppercase; letter-spacing: .06em; }
    p { margin: 2px 0; }
    table { border-collapse: collapse; margin: 26px 0 18px; width: 100%; }
    th, td { border-bottom: 1px solid #dedce5; padding: 9px 6px; text-align: left; vertical-align: top; }
    th:last-child, td.amount { text-align: right; white-space: nowrap; }
    .totals { border-bottom: 0; margin-left: auto; max-width: 260px; }
    .total-row { display: flex; justify-content: space-between; padding: 4px 0; }
    .grand-total { border-top: 1px solid #8b8796; font-size: 17px; font-weight: 700; margin-top: 6px; padding-top: 9px; }
    .muted { color: #666270; }
    @media print { body { max-width: none; } }
  </style>
</head>
<body>
  <header>
    <div><p class="muted">{{.Business.Name}}</p><h1>Invoice {{.Invoice.Number}}</h1><p class="muted">{{.Invoice.Status}} · {{.Invoice.Currency}}</p></div>
    <div><h2>Bill to</h2><p>{{.Customer.Name}}</p><p>{{.Customer.Company}}</p><p>{{.Customer.Email}}</p><p>{{.CustomerAddress}}</p></div>
  </header>
  <p class="muted">Billing period: {{.PeriodStart}} to {{.PeriodEnd}} · Due: {{.DueAt}}</p>
  <table>
    <thead><tr><th>Description</th><th>Qty</th><th>Unit price</th><th>Subtotal</th><th>Tax</th><th>Total</th></tr></thead>
    <tbody>{{range .Lines}}<tr><td>{{.Description}}{{if .Variant}}<br><span class="muted">{{.Variant}}</span>{{end}}</td><td>{{.Quantity}}</td><td class="amount">{{.UnitPrice}}</td><td class="amount">{{.RawSubtotal}}</td><td class="amount">{{.TaxAmount}}</td><td class="amount">{{.LineTotal}}</td></tr>{{end}}</tbody>
  </table>
  <section class="totals">
    <div class="total-row"><span>Subtotal</span><strong>{{.Invoice.Subtotal}}</strong></div>
    <div class="total-row"><span>Discount</span><strong>{{.Invoice.DiscountTotal}}</strong></div>
    <div class="total-row"><span>Tax</span><strong>{{.Invoice.TaxTotal}}</strong></div>
    <div class="total-row grand-total"><span>Total</span><strong>{{.Invoice.Currency}} {{.Invoice.Total}}</strong></div>
    <div class="total-row"><span>Outstanding</span><strong>{{.Invoice.Currency}} {{.Invoice.Outstanding}}</strong></div>
  </section>
</body>
</html>`))

type invoicePrintData struct {
	Business        Business
	Customer        Customer
	CustomerAddress string
	Invoice         Invoice
	Lines           []CalculationLine
	PeriodStart     string
	PeriodEnd       string
	DueAt           string
}

func addInvoicePrintFiles(files map[string][]byte, checksums map[string]string, state *State) (int, error) {
	for _, invoice := range sortedValues(state.Invoices) {
		customer := state.Customers[invoice.CustomerID]
		data := invoicePrintData{
			Business: state.Businesses[invoice.BusinessID], Customer: customer, Invoice: invoice,
			CustomerAddress: addressText(customer.Address),
			Lines:           invoice.Snapshot.Lines, PeriodStart: invoice.PeriodStart.UTC().Format("2006-01-02"),
			PeriodEnd: invoice.PeriodEnd.UTC().Format("2006-01-02"), DueAt: invoice.DueAt.UTC().Format("2006-01-02"),
		}
		var body bytes.Buffer
		if err := invoicePrintTemplate.Execute(&body, data); err != nil {
			return 0, err
		}
		idHash := sha256.Sum256([]byte(invoice.ID))
		name := "invoices/print/" + hex.EncodeToString(idHash[:]) + ".html"
		files[name] = body.Bytes()
		fileHash := sha256.Sum256(body.Bytes())
		checksums[name] = hex.EncodeToString(fileHash[:])
	}
	return len(state.Invoices), nil
}

func addressText(address map[string]string) string {
	keys := make([]string, 0, len(address))
	for key := range address {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := strings.TrimSpace(address[key]); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, ", ")
}
func sortedValues[T any](values map[string]T) []T {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]T, 0, len(keys))
	for _, key := range keys {
		out = append(out, values[key])
	}
	return out
}
func count(value any) int {
	switch v := value.(type) {
	case []Business:
		return len(v)
	case []Customer:
		return len(v)
	case []Product:
		return len(v)
	case []Plan:
		return len(v)
	case []TaxRule:
		return len(v)
	case []Discount:
		return len(v)
	case []QuoteTemplate:
		return len(v)
	case []Quotation:
		return len(v)
	case []Subscription:
		return len(v)
	case []Invoice:
		return len(v)
	case []Payment:
		return len(v)
	case []PaymentAllocation:
		return len(v)
	case []Approval:
		return len(v)
	case []AuditEvent:
		return len(v)
	case []JournalEntry:
		return len(v)
	}
	return 0
}
func csvSafe(s string) string {
	if strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	trim := strings.TrimLeft(s, " \t\r\n")
	if trim != "" && strings.ContainsRune("=+-@", rune(trim[0])) {
		return "'" + s
	}
	return s
}
func customerCSV(s *State) [][]string {
	rows := [][]string{{"id", "name", "email", "company", "phone"}}
	for _, v := range sortedValues(s.Customers) {
		rows = append(rows, []string{v.ID, v.Name, v.Email, v.Company, v.Phone})
	}
	return rows
}
func invoiceCSV(s *State) [][]string {
	rows := [][]string{{"id", "number", "customer_id", "status", "currency", "subtotal", "discount", "tax", "total", "outstanding", "due_at", "calculation_hash"}}
	for _, v := range sortedValues(s.Invoices) {
		rows = append(rows, []string{v.ID, v.Number, v.CustomerID, v.Status, v.Currency, money2(v.Subtotal), money2(v.DiscountTotal), money2(v.TaxTotal), money2(v.Total), money2(v.Outstanding), v.DueAt.UTC().Format(time.RFC3339), v.CalculationHash})
	}
	return rows
}
func paymentCSV(s *State) [][]string {
	rows := [][]string{{"id", "invoice_id", "customer_id", "amount", "method", "received_at", "provider", "operation_key"}}
	for _, v := range sortedValues(s.Payments) {
		rows = append(rows, []string{v.ID, v.InvoiceID, v.CustomerID, money2(v.Amount), v.Method, v.ReceivedAt.UTC().Format(time.RFC3339), v.Provider, v.OperationKey})
	}
	return rows
}
