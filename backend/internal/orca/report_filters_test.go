package orca

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReportSummaryDateFiltersUseBusinessLocalInvoiceAndPaymentDates(t *testing.T) {
	store := &Store{state: NewState()}
	handler := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false, AllowedOrigin: "http://localhost:5173"})
	cookie, _ := signupForTest(t, handler, "reports@example.test", "Reports Business")
	businessID := store.state.Sessions[tokenDigest(cookie.Value)].BusinessID
	store.state.Businesses[businessID] = Business{ID: businessID, Name: "Reports Business", Currency: CurrencyINR, Timezone: "Asia/Kolkata"}

	// The two included invoices fall exactly on the local-date boundaries. The
	// surrounding UTC timestamps prove that filtering uses the business timezone.
	store.state.Invoices["inv-start"] = reportTestInvoice(businessID, "inv-start", "Confirmed", "100.00", "60.00", localTestInstant(2026, time.January, 2, 0, 0), time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC))
	store.state.Invoices["inv-end"] = reportTestInvoice(businessID, "inv-end", "Paid", "40.00", "0.00", localTestInstant(2026, time.January, 3, 23, 59), time.Date(2026, time.January, 4, 0, 0, 0, 0, time.UTC))
	store.state.Invoices["inv-before"] = reportTestInvoice(businessID, "inv-before", "Confirmed", "70.00", "70.00", localTestInstant(2026, time.January, 1, 23, 59), time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC))
	store.state.Invoices["inv-after"] = reportTestInvoice(businessID, "inv-after", "Confirmed", "90.00", "90.00", localTestInstant(2026, time.January, 4, 0, 0), time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC))
	store.state.Invoices["inv-draft"] = reportTestInvoice(businessID, "inv-draft", "Draft", "500.00", "500.00", localTestInstant(2026, time.January, 2, 12, 0), time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC))
	store.state.Invoices["inv-cancelled"] = reportTestInvoice(businessID, "inv-cancelled", "Cancelled", "600.00", "0.00", localTestInstant(2026, time.January, 2, 12, 0), time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC))

	// One payment inside the date window belongs to an invoice issued before it;
	// an in-window invoice also has a payment received outside the window.
	store.state.Payments["payment-inside"] = Payment{ID: "payment-inside", BusinessID: businessID, InvoiceID: "inv-before", Amount: "25.00", ReceivedAt: localTestInstant(2026, time.January, 2, 0, 0)}
	store.state.Payments["payment-end"] = Payment{ID: "payment-end", BusinessID: businessID, InvoiceID: "inv-end", Amount: "7.00", ReceivedAt: localTestInstant(2026, time.January, 3, 23, 59)}
	store.state.Payments["payment-outside"] = Payment{ID: "payment-outside", BusinessID: businessID, InvoiceID: "inv-start", Amount: "13.00", ReceivedAt: localTestInstant(2026, time.January, 4, 0, 0)}
	store.state.Plans["active-plan"] = Plan{ID: "active-plan", BusinessID: businessID, SalesPrice: "100.00"}
	store.state.Subscriptions["active-sub"] = Subscription{ID: "active-sub", BusinessID: businessID, PlanID: "active-plan", Status: "Active", Lines: []SubscriptionLine{{Quantity: 3}}}

	got := readReportSummary(t, handler, cookie, "from=2026-01-02&to=2026-01-03")
	if got.Invoiced != "140.00" || got.Receivables != "60.00" || got.OverdueInvoices != 1 {
		t.Fatalf("issue-date metrics do not match selected local dates: %+v", got)
	}
	if got.Collected != "32.00" {
		t.Fatalf("collection total must use received dates: got %s, want 32.00", got.Collected)
	}
	if got.ActiveSubscriptions != 1 || got.RecurringRevenue != "300.00" {
		t.Fatalf("current subscription metrics should remain current under historical filters: %+v", got)
	}
	if got.Filters.From != "2026-01-02" || got.Filters.To != "2026-01-03" {
		t.Fatalf("applied date filters were not returned: %+v", got.Filters)
	}
}

func TestReportSummaryRejectsInvalidAndReversedDates(t *testing.T) {
	store := &Store{state: NewState()}
	handler := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false, AllowedOrigin: "http://localhost:5173"})
	cookie, _ := signupForTest(t, handler, "reports-validation@example.test", "Reports Business")
	for _, query := range []string{"from=2026-02-30", "from=2026-02-03&to=2026-02-02"} {
		t.Run(query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary?"+query, nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("GET summary with %q returned %d, want 422: %s", query, rec.Code, rec.Body.String())
			}
		})
	}
}

type reportSummaryTestResponse struct {
	ActiveSubscriptions int    `json:"activeSubscriptions"`
	RecurringRevenue    string `json:"recurringRevenue"`
	Invoiced            string `json:"invoiced"`
	Receivables         string `json:"receivables"`
	OverdueInvoices     int    `json:"overdueInvoices"`
	Collected           string `json:"collected"`
	Filters             struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"filters"`
}

func readReportSummary(t *testing.T, handler http.Handler, cookie *http.Cookie, query string) reportSummaryTestResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary?"+query, nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET summary returned %d: %s", rec.Code, rec.Body.String())
	}
	var got reportSummaryTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func reportTestInvoice(businessID, id, status, total, outstanding string, issueDate, dueAt time.Time) Invoice {
	return Invoice{ID: id, BusinessID: businessID, Status: status, Currency: CurrencyINR, Total: total, Outstanding: outstanding, CreatedAt: issueDate, DueAt: dueAt}
}

func localTestInstant(year int, month time.Month, day, hour, minute int) time.Time {
	location, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		panic(err)
	}
	return time.Date(year, month, day, hour, minute, 0, 0, location).UTC()
}
