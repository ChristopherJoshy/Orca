package orca

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteExportZipAppliesFiltersAndKeepsRelatedTenantRecords(t *testing.T) {
	t.Setenv("ORCA_EXPORT_DIR", t.TempDir())
	state := exportFixture()
	filters := map[string]any{"kind": "filtered", "resource": "invoices", "status": "paid"}
	path, _, err := writeExportZip(state, ExportJob{ID: "export-filtered", BusinessID: "biz-a", Filters: filters})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()

	entries := map[string][]byte{}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		contents, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		entries[file.Name] = contents
	}

	var invoices []Invoice
	if err := json.Unmarshal(entries["invoices.json"], &invoices); err != nil {
		t.Fatal(err)
	}
	if len(invoices) != 1 || invoices[0].ID != "inv-paid" {
		t.Fatalf("filtered invoices = %#v, want only inv-paid", invoices)
	}
	var customers []Customer
	if err := json.Unmarshal(entries["customers.json"], &customers); err != nil {
		t.Fatal(err)
	}
	if len(customers) != 1 || customers[0].ID != "cust-a" {
		t.Fatalf("related customers = %#v, want only cust-a", customers)
	}
	var taxes []TaxRule
	if err := json.Unmarshal(entries["tax_rules.json"], &taxes); err != nil {
		t.Fatal(err)
	}
	if len(taxes) != 1 || taxes[0].ID != "tax-a" {
		t.Fatalf("related tax rules = %#v, want tax-a", taxes)
	}
	if _, ok := entries["payments.json"]; !ok {
		t.Fatal("payments.json missing")
	}
	var payments []Payment
	if err := json.Unmarshal(entries["payments.json"], &payments); err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 || payments[0].ID != "payment-a" {
		t.Fatalf("related payments = %#v, want payment-a", payments)
	}
	if _, ok := entries["invoices/print/"+hex.EncodeToString(sha256Sum("inv-paid"))+".html"]; !ok {
		t.Fatal("print-ready HTML for selected invoice missing")
	}
	if strings.Contains(string(entries["customers.json"]), "other tenant") || strings.Contains(string(entries["invoices.json"]), "inv-other") {
		t.Fatal("export contains data from another business")
	}

	var manifest struct {
		Counts    map[string]int    `json:"counts"`
		Checksums map[string]string `json:"checksums"`
	}
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	printFile := "invoices/print/" + hex.EncodeToString(sha256Sum("inv-paid")) + ".html"
	if manifest.Counts["invoices"] != 1 || manifest.Counts["tax_rules"] != 1 || manifest.Counts["print_invoice_files"] != 1 {
		t.Fatalf("manifest counts = %#v", manifest.Counts)
	}
	sum := sha256.Sum256(entries[printFile])
	if manifest.Checksums[printFile] != hex.EncodeToString(sum[:]) {
		t.Fatalf("print invoice checksum = %q, want %x", manifest.Checksums[printFile], sum)
	}
}

func TestFilteredExportWithEmptyIDSetIsEmpty(t *testing.T) {
	state := exportFixture()
	filtered, err := filterExportState(state, "biz-a", map[string]any{"kind": "filtered", "resource": "invoices", "ids": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Invoices) != 0 || len(filtered.Customers) != 0 || len(filtered.Payments) != 0 {
		t.Fatalf("empty ID filter selected related records: invoices=%d customers=%d payments=%d", len(filtered.Invoices), len(filtered.Customers), len(filtered.Payments))
	}
}

func TestCSVSafeNeutralizesFormulaAndControlPrefixes(t *testing.T) {
	cases := []string{
		"=1+1", "+SUM(A1:A2)", "-2+3", "@SUM(A1)",
		"\t=1+1", "\r=1+1", "\tplain text", "\rplain text", "  =1+1", "\n@SUM(A1)",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			if got, want := csvSafe(input), "'"+input; got != want {
				t.Fatalf("csvSafe(%q) = %q, want %q", input, got, want)
			}
		})
	}
	for _, input := range []string{"plain text", "  ordinary", "123.45"} {
		if got := csvSafe(input); got != input {
			t.Fatalf("csvSafe(%q) changed benign value to %q", input, got)
		}
	}
}

func TestExportFilterValidationRejectsUnknownAndUnsupportedSelectors(t *testing.T) {
	for name, filters := range map[string]map[string]any{
		"unknown key":        {"resource": "invoices", "status": "Paid", "oops": true},
		"unsupported filter": {"resource": "customers", "invoiceId": "inv-a"},
		"unknown resource":   {"resource": "invoicez"},
		"bad date":           {"resource": "invoices", "dateFrom": "10/31/2026"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := filterExportState(exportFixture(), "biz-a", filters); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func exportFixture() *State {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	state := NewState()
	state.Businesses["biz-a"] = Business{ID: "biz-a", Name: "Acme <Billing>", Currency: CurrencyINR}
	state.Businesses["biz-b"] = Business{ID: "biz-b", Name: "Other business", Currency: CurrencyINR}
	state.Customers["cust-a"] = Customer{ID: "cust-a", BusinessID: "biz-a", Name: "A & B", Email: "a@example.test", Company: "Acme", Address: map[string]string{"city": "Pune", "street": "Main St"}, CreatedAt: now}
	state.Customers["cust-b"] = Customer{ID: "cust-b", BusinessID: "biz-a", Name: "Unrelated", CreatedAt: now}
	state.Customers["cust-other"] = Customer{ID: "cust-other", BusinessID: "biz-b", Name: "other tenant customer", CreatedAt: now}
	state.Products["product-a"] = Product{ID: "product-a", BusinessID: "biz-a", Name: "Service", SalesPrice: "100.00", CreatedAt: now}
	state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: "biz-a", Name: "Monthly", SalesPrice: "100.00", CreatedAt: now}
	state.Taxes["tax-a"] = TaxRule{ID: "tax-a", BusinessID: "biz-a", Name: "GST", Rate: "18", CreatedAt: now}
	state.Discounts["discount-a"] = Discount{ID: "discount-a", BusinessID: "biz-a", Name: "Welcome", Kind: "fixed", Value: "5.00"}
	state.Subscriptions["sub-a"] = Subscription{ID: "sub-a", BusinessID: "biz-a", CustomerID: "cust-a", PlanID: "plan-a", Number: "SUB-1", Status: "Active", DiscountID: "discount-a", Lines: []SubscriptionLine{{ProductID: "product-a", Name: "Service", Quantity: 1, UnitPrice: "100.00"}}, CreatedAt: now}
	state.Invoices["inv-paid"] = Invoice{ID: "inv-paid", BusinessID: "biz-a", SubscriptionID: "sub-a", CustomerID: "cust-a", Number: "INV-1", Status: "Paid", Currency: CurrencyINR, Subtotal: "100.00", DiscountTotal: "5.00", TaxTotal: "17.10", Total: "112.10", Outstanding: "0.00", PeriodStart: now.AddDate(0, -1, 0), PeriodEnd: now, DueAt: now, CreatedAt: now, Snapshot: CalculationSnapshot{BusinessID: "biz-a", SubscriptionID: "sub-a", PlanID: "plan-a", DiscountID: "discount-a", Lines: []CalculationLine{{ProductID: "product-a", TaxID: "tax-a", Description: "Support <tier>", Quantity: 1, UnitPrice: "100.00", RawSubtotal: "100.00", TaxAmount: "17.10", LineTotal: "112.10"}}}}
	state.Invoices["inv-open"] = Invoice{ID: "inv-open", BusinessID: "biz-a", SubscriptionID: "sub-a", CustomerID: "cust-a", Number: "INV-2", Status: "Confirmed", Currency: CurrencyINR, CreatedAt: now}
	state.Invoices["inv-other"] = Invoice{ID: "inv-other", BusinessID: "biz-b", CustomerID: "cust-other", Number: "OTHER-1", Status: "Paid", Currency: CurrencyINR, CreatedAt: now}
	state.Payments["payment-a"] = Payment{ID: "payment-a", BusinessID: "biz-a", CustomerID: "cust-a", InvoiceID: "inv-paid", Method: "manual", Amount: "112.10", ReceivedAt: now, CreatedAt: now}
	state.Allocations["allocation-a"] = PaymentAllocation{BusinessID: "biz-a", PaymentID: "payment-a", InvoiceID: "inv-paid", Amount: "112.10", CreatedAt: now}
	state.Journal = []JournalEntry{{ID: "journal-a", BusinessID: "biz-a", SourceType: "invoice", SourceID: "inv-paid", Account: "receivable", Debit: "112.10", Currency: CurrencyINR, CreatedAt: now}}
	state.Audit = []AuditEvent{{ID: "audit-a", BusinessID: "biz-a", Action: "invoice.paid", Resource: "invoice", ResourceID: "inv-paid", CreatedAt: now}}
	return state
}

func sha256Sum(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func TestInvoicePrintHTMLIsEscaped(t *testing.T) {
	t.Setenv("ORCA_EXPORT_DIR", filepath.Join(t.TempDir(), "private"))
	state := exportFixture()
	path, _, err := writeExportZip(state, ExportJob{ID: "export-safe-html", BusinessID: "biz-a", Filters: map[string]any{"kind": "filtered", "resource": "invoices", "id": "inv-paid"}})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if !strings.HasSuffix(file.Name, ".html") {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		body := string(data)
		if strings.Contains(body, "<Billing>") || strings.Contains(body, "<tier>") || !strings.Contains(body, "&lt;Billing&gt;") || !strings.Contains(body, "&lt;tier&gt;") {
			t.Fatalf("print-ready HTML was not escaped: %s", body)
		}
		return
	}
	t.Fatal("no printable invoice was emitted")
}

func TestExportArchiveUsesPrivateFilePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ORCA_EXPORT_DIR", dir)
	path, _, err := writeExportZip(exportFixture(), ExportJob{ID: "export-permissions", BusinessID: "biz-a"})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("archive mode = %o, want 600", got)
	}
	info, err = os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got&0077 != 0 {
		t.Fatalf("business export directory mode = %o, expected no group/other permissions", got)
	}
}
