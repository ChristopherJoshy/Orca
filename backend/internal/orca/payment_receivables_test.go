package orca

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPartialPaymentKeepsReceivableOpenAndReportsOnlyPostedInvoices(t *testing.T) {
	store := &Store{state: NewState()}
	handler := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false, AllowedOrigin: "http://localhost:5173"})
	cookie, csrf := signupForTest(t, handler, "finance@example.test", "Finance Business")
	now := time.Now().UTC()
	store.state.Invoices["overdue"] = Invoice{ID: "overdue", BusinessID: store.state.Sessions[tokenDigest(cookie.Value)].BusinessID, Number: "INV-OVERDUE", Status: "Confirmed", Currency: CurrencyINR, Total: "100.00", Outstanding: "100.00", DueAt: now.Add(-24 * time.Hour), Version: 1, CreatedAt: now.Add(-48 * time.Hour)}
	store.state.Invoices["current"] = Invoice{ID: "current", BusinessID: store.state.Invoices["overdue"].BusinessID, Number: "INV-CURRENT", Status: "Confirmed", Currency: CurrencyINR, Total: "50.00", Outstanding: "50.00", DueAt: now.Add(24 * time.Hour), Version: 1, CreatedAt: now}
	store.state.Invoices["draft"] = Invoice{ID: "draft", BusinessID: store.state.Invoices["overdue"].BusinessID, Number: "INV-DRAFT", Status: "Draft", Currency: CurrencyINR, Total: "75.00", Outstanding: "75.00", DueAt: now.Add(24 * time.Hour), Version: 1, CreatedAt: now}
	store.state.Invoices["cancelled"] = Invoice{ID: "cancelled", BusinessID: store.state.Invoices["overdue"].BusinessID, Number: "INV-CANCELLED", Status: "Cancelled", Currency: CurrencyINR, Total: "25.00", Outstanding: "25.00", DueAt: now.Add(-24 * time.Hour), Version: 1, CreatedAt: now}

	postPayment := func(amount, key string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]string{"amount": amount, "method": "Bank transfer", "date": now.Format("2006-01-02"), "operationKey": key})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices/overdue/payments", bytes.NewReader(body))
		req.AddCookie(cookie)
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("X-CSRF-Token", csrf)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	readSummary := func() map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("report returned %d: %s", rec.Code, rec.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	assertSummary := func(summary map[string]any, receivable, invoiced string, overdue int) {
		t.Helper()
		if summary["receivables"] != receivable || summary["invoiced"] != invoiced || int(summary["overdueInvoices"].(float64)) != overdue {
			t.Fatalf("unexpected finance summary: %#v", summary)
		}
	}
	assertSummary(readSummary(), "150.00", "150.00", 1)
	if rec := postPayment("30.00", "partial-one"); rec.Code != http.StatusCreated {
		t.Fatalf("partial payment returned %d: %s", rec.Code, rec.Body.String())
	}
	if invoice := store.state.Invoices["overdue"]; invoice.Status != "Confirmed" || invoice.Outstanding != "70.00" {
		t.Fatalf("partial payment changed invoice lifecycle or balance incorrectly: %+v", invoice)
	}
	assertSummary(readSummary(), "120.00", "150.00", 1)
	if rec := postPayment("70.00", "partial-two"); rec.Code != http.StatusCreated {
		t.Fatalf("final allocation returned %d: %s", rec.Code, rec.Body.String())
	}
	if invoice := store.state.Invoices["overdue"]; invoice.Status != "Paid" || invoice.Outstanding != "0.00" {
		t.Fatalf("final allocation did not settle invoice: %+v", invoice)
	}
	assertSummary(readSummary(), "50.00", "150.00", 0)
}
