package orca

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

const (
	stripeTestSecret = "whsec_test_checkout"
	stripeTestURL    = "https://checkout.stripe.com/c/pay/cs_test_orca"
)

type stripeTestFixture struct {
	store      *Store
	api        *API
	cookie     *http.Cookie
	csrf       string
	invoiceID  string
	userID     string
	businessID string
	customerID string
}

func newStripeTestFixture(t *testing.T) stripeTestFixture {
	t.Helper()
	store := &Store{state: NewState()}
	now := time.Now().UTC()
	const userID, businessID, customerID, invoiceID = "user-customer", "business-checkout", "customer-linked", "invoice-checkout"
	store.state.Users[userID] = User{ID: userID, Email: "customer@example.test", EmailNormalized: "customer@example.test", Verified: true, CreatedAt: now}
	store.state.Businesses[businessID] = Business{ID: businessID, Name: "Checkout business", Currency: CurrencyINR, Timezone: "Asia/Kolkata"}
	store.state.Memberships[membershipKey(businessID, userID)] = Membership{BusinessID: businessID, UserID: userID, Role: "customer", CustomerID: customerID}
	store.state.Customers[customerID] = Customer{ID: customerID, BusinessID: businessID, UserID: userID, Name: "Asha Customer", Email: "customer@example.test", CreatedAt: now}
	store.state.Invoices[invoiceID] = Invoice{ID: invoiceID, BusinessID: businessID, CustomerID: customerID, Number: "INV-0001", Status: "Confirmed", Currency: CurrencyINR, Total: "100.01", Outstanding: "80.01", DueAt: now.Add(14 * 24 * time.Hour), Version: 1, CreatedAt: now}
	store.state.Payments["prior-payment"] = Payment{ID: "prior-payment", BusinessID: businessID, CustomerID: customerID, InvoiceID: invoiceID, Method: "Manual", Amount: "20.00", Provider: "manual", OperationKey: "prior-payment", ReceivedAt: now, CreatedAt: now}
	store.state.Allocations["prior-payment:"+invoiceID] = PaymentAllocation{BusinessID: businessID, PaymentID: "prior-payment", InvoiceID: invoiceID, Amount: "20.00", CreatedAt: now}

	sessionToken := "stripe-checkout-session-token"
	csrfToken := "stripe-checkout-csrf-token"
	store.state.Sessions[tokenDigest(sessionToken)] = Session{UserID: userID, BusinessID: businessID, Role: "customer", CustomerID: customerID, CSRFToken: tokenDigest(csrfToken), ExpiresAt: now.Add(time.Hour)}
	api := newAPI(store, ServerConfig{Env: "test", CookieSecure: false, AllowedOrigin: "http://localhost:5173", PublicURL: "http://localhost:5173", StripeAPIKey: "sk_test_unit", StripeWebhookSecret: stripeTestSecret})
	return stripeTestFixture{store: store, api: api, cookie: &http.Cookie{Name: "orca_session", Value: sessionToken}, csrf: csrfToken, invoiceID: invoiceID, userID: userID, businessID: businessID, customerID: customerID}
}

func TestStripeCheckoutSessionAndWebhookPostBalancedPaymentExactlyOnce(t *testing.T) {
	fixture := newStripeTestFixture(t)
	calls := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer sk_test_unit" {
			t.Errorf("Stripe request method or auth was unexpected: %s %q", r.Method, r.Header.Get("Authorization"))
		}
		if r.Header.Get("Idempotency-Key") == "" {
			t.Error("Stripe request omitted the persisted idempotency key")
		}
		body, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("parse Stripe form: %v", err)
		}
		if got := form.Get("line_items[0][price_data][unit_amount]"); got != "8001" {
			t.Errorf("Stripe unit amount = %q, want server-derived 8001 paise", got)
		}
		if got := form.Get("line_items[0][price_data][currency]"); got != "inr" {
			t.Errorf("Stripe currency = %q, want inr", got)
		}
		if form.Get("metadata[orca_checkout_id]") == "" || form.Get("client_reference_id") == "" {
			t.Error("Stripe request did not carry the local checkout reference")
		}
		return stripeHTTPResponse(r, http.StatusOK, "{\"id\":\"cs_test_orca\",\"url\":\""+stripeTestURL+"\",\"expires_at\":1900000000}"), nil
	})
	fixture.api.stripe = stripeCheckoutSessionCreatorHTTP{apiKey: "sk_test_unit", endpoint: stripeCheckoutSessionsEndpoint, client: &http.Client{Transport: transport}}

	response := postPortalCheckout(t, fixture)
	if response["sessionId"] != "cs_test_orca" || response["url"] != stripeTestURL || response["amount"] != "80.01" || response["currency"] != "INR" {
		t.Fatalf("Checkout response is incomplete or incorrect: %#v", response)
	}
	if calls != 1 {
		t.Fatalf("provider calls = %d, want 1", calls)
	}
	var checkout StripeCheckout
	for _, value := range fixture.store.state.StripeCheckouts {
		checkout = value
	}
	if checkout.ID == "" || checkout.Status != "Open" || checkout.Amount != "80.01" {
		t.Fatalf("persisted checkout attempt is incomplete: %#v", checkout)
	}

	event := stripeSessionWebhook("evt_checkout_paid", "checkout.session.completed", map[string]any{
		"id": checkout.ProviderSessionID, "client_reference_id": checkout.ID,
		"metadata":     map[string]string{"orca_checkout_id": checkout.ID, "orca_invoice_id": fixture.invoiceID, "orca_customer_id": fixture.customerID, "orca_business_id": fixture.businessID},
		"amount_total": 8001, "currency": "inr", "payment_status": "paid",
	})
	if got := sendStripeWebhook(t, fixture.api.mux, event); got.Code != http.StatusOK {
		t.Fatalf("success webhook returned %d: %s", got.Code, got.Body.String())
	}
	assertStripePaidState(t, fixture, checkout.ID)

	if got := sendStripeWebhook(t, fixture.api.mux, event); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "\"duplicate\":true") {
		t.Fatalf("replayed success webhook was not acknowledged as duplicate: %d %s", got.Code, got.Body.String())
	}
	asyncSuccess := stripeSessionWebhook("evt_checkout_async_paid", "checkout.session.async_payment_succeeded", map[string]any{
		"id": checkout.ProviderSessionID, "client_reference_id": checkout.ID,
		"metadata": map[string]string{"orca_checkout_id": checkout.ID}, "amount_total": 8001, "currency": "inr", "payment_status": "paid",
	})
	if got := sendStripeWebhook(t, fixture.api.mux, asyncSuccess); got.Code != http.StatusOK {
		t.Fatalf("second success event returned %d: %s", got.Code, got.Body.String())
	}
	expired := stripeSessionWebhook("evt_checkout_expired_late", "checkout.session.expired", map[string]any{
		"id": checkout.ProviderSessionID, "client_reference_id": checkout.ID, "metadata": map[string]string{"orca_checkout_id": checkout.ID},
	})
	if got := sendStripeWebhook(t, fixture.api.mux, expired); got.Code != http.StatusOK {
		t.Fatalf("late expired event returned %d: %s", got.Code, got.Body.String())
	}
	assertStripePaidState(t, fixture, checkout.ID)
}

func TestStripeWebhookAmountMismatchGoesToReconciliationWithoutAllocating(t *testing.T) {
	fixture := newStripeTestFixture(t)
	stripeProviderForTest(fixture.api, http.StatusOK, stripeTestURL)
	response := postPortalCheckout(t, fixture)
	checkoutID := ""
	for id := range fixture.store.state.StripeCheckouts {
		checkoutID = id
	}
	event := stripeSessionWebhook("evt_checkout_mismatch", "checkout.session.completed", map[string]any{
		"id": response["sessionId"], "client_reference_id": checkoutID, "metadata": map[string]string{"orca_checkout_id": checkoutID},
		"amount_total": 8000, "currency": "inr", "payment_status": "paid",
	})
	if got := sendStripeWebhook(t, fixture.api.mux, event); got.Code != http.StatusOK {
		t.Fatalf("mismatched success webhook returned %d: %s", got.Code, got.Body.String())
	}
	invoice := fixture.store.state.Invoices[fixture.invoiceID]
	if invoice.Status != "Confirmed" || invoice.Outstanding != "80.01" {
		t.Fatalf("mismatched session mutated the receivable: %#v", invoice)
	}
	if len(fixture.store.state.Payments) != 1 || len(fixture.store.state.Allocations) != 1 || len(fixture.store.state.Journal) != 0 {
		t.Fatalf("mismatched session created financial entries: payments=%d allocations=%d journal=%d", len(fixture.store.state.Payments), len(fixture.store.state.Allocations), len(fixture.store.state.Journal))
	}
	if got := fixture.store.state.StripeCheckouts[checkoutID].Status; got != "ReconciliationRequired" {
		t.Fatalf("checkout status = %q, want ReconciliationRequired", got)
	}
	if len(fixture.store.state.StripeReconciliations) != 1 {
		t.Fatalf("reconciliation outbox size = %d, want 1", len(fixture.store.state.StripeReconciliations))
	}
	if !hasOutbox(fixture.store.state.Outbox, "stripe.reconciliation_required") {
		t.Fatal("matched mismatch did not add a business reconciliation outbox event")
	}
}

func TestStripeWebhookUnmatchedSuccessUsesGlobalReconciliationOutbox(t *testing.T) {
	fixture := newStripeTestFixture(t)
	event := stripeSessionWebhook("evt_unmatched_success", "checkout.session.async_payment_succeeded", map[string]any{
		"id": "cs_test_unmatched", "client_reference_id": "missing-local-checkout", "metadata": map[string]string{"orca_checkout_id": "missing-local-checkout"},
		"amount_total": 8001, "currency": "inr", "payment_status": "paid",
	})
	if got := sendStripeWebhook(t, fixture.api.mux, event); got.Code != http.StatusOK {
		t.Fatalf("unmatched success webhook returned %d: %s", got.Code, got.Body.String())
	}
	if len(fixture.store.state.StripeReconciliations) != 1 {
		t.Fatalf("global reconciliation outbox size = %d, want 1", len(fixture.store.state.StripeReconciliations))
	}
	var reconciliation StripeReconciliation
	for _, value := range fixture.store.state.StripeReconciliations {
		reconciliation = value
	}
	if reconciliation.BusinessID != "" || reconciliation.SessionID != "cs_test_unmatched" || reconciliation.Reason == "" {
		t.Fatalf("unmatched provider payment was not retained for global reconciliation: %#v", reconciliation)
	}
	if invoice := fixture.store.state.Invoices[fixture.invoiceID]; invoice.Status != "Confirmed" || invoice.Outstanding != "80.01" {
		t.Fatalf("unmatched session mutated an invoice: %#v", invoice)
	}
}

func TestStripeAsyncCheckoutEventsTrackProcessingFailureAndExpiry(t *testing.T) {
	fixture := newStripeTestFixture(t)
	stripeProviderForTest(fixture.api, http.StatusOK, stripeTestURL)
	created := postPortalCheckout(t, fixture)
	checkoutID := ""
	for id := range fixture.store.state.StripeCheckouts {
		checkoutID = id
	}
	completedUnpaid := stripeSessionWebhook("evt_checkout_processing", "checkout.session.completed", map[string]any{
		"id": created["sessionId"], "client_reference_id": checkoutID, "metadata": map[string]string{"orca_checkout_id": checkoutID},
		"amount_total": 8001, "currency": "inr", "payment_status": "unpaid",
	})
	if got := sendStripeWebhook(t, fixture.api.mux, completedUnpaid); got.Code != http.StatusOK {
		t.Fatalf("unpaid completed webhook returned %d: %s", got.Code, got.Body.String())
	}
	if got := fixture.store.state.StripeCheckouts[checkoutID].Status; got != "Processing" {
		t.Fatalf("unpaid completed status = %q, want Processing", got)
	}
	if got := fixture.store.state.Invoices[fixture.invoiceID].Outstanding; got != "80.01" {
		t.Fatalf("unpaid completion changed invoice outstanding to %s", got)
	}
	failed := stripeSessionWebhook("evt_checkout_failed", "checkout.session.async_payment_failed", map[string]any{
		"id": created["sessionId"], "client_reference_id": checkoutID, "metadata": map[string]string{"orca_checkout_id": checkoutID},
	})
	if got := sendStripeWebhook(t, fixture.api.mux, failed); got.Code != http.StatusOK {
		t.Fatalf("async failure webhook returned %d: %s", got.Code, got.Body.String())
	}
	if got := fixture.store.state.StripeCheckouts[checkoutID].Status; got != "Failed" {
		t.Fatalf("async failure status = %q, want Failed", got)
	}

	second := postPortalCheckout(t, fixture)
	secondCheckoutID := ""
	for id, checkout := range fixture.store.state.StripeCheckouts {
		if checkout.ProviderSessionID == second["sessionId"] {
			secondCheckoutID = id
		}
	}
	expired := stripeSessionWebhook("evt_checkout_expired", "checkout.session.expired", map[string]any{
		"id": second["sessionId"], "client_reference_id": secondCheckoutID, "metadata": map[string]string{"orca_checkout_id": secondCheckoutID},
	})
	if got := sendStripeWebhook(t, fixture.api.mux, expired); got.Code != http.StatusOK {
		t.Fatalf("expired webhook returned %d: %s", got.Code, got.Body.String())
	}
	if got := fixture.store.state.StripeCheckouts[secondCheckoutID].Status; got != "Expired" {
		t.Fatalf("expired status = %q, want Expired", got)
	}
	if !hasOutbox(fixture.store.state.Outbox, "stripe.checkout_async_payment_failed") || !hasOutbox(fixture.store.state.Outbox, "stripe.checkout_expired") {
		t.Fatal("failure and expiry events were not persisted to the outbox")
	}
	if got := fixture.store.state.Invoices[fixture.invoiceID].Outstanding; got != "80.01" {
		t.Fatalf("failed/expired sessions changed invoice outstanding to %s", got)
	}
}

func TestStripeCheckoutRejectsAnotherCustomerInvoice(t *testing.T) {
	fixture := newStripeTestFixture(t)
	fixture.store.state.Customers["other-customer"] = Customer{ID: "other-customer", BusinessID: fixture.businessID, UserID: "other-user", Name: "Other", Email: "other@example.test"}
	fixture.store.state.Invoices["other-invoice"] = Invoice{ID: "other-invoice", BusinessID: fixture.businessID, CustomerID: "other-customer", Number: "INV-OTHER", Status: "Confirmed", Currency: CurrencyINR, Total: "80.01", Outstanding: "80.01", CreatedAt: time.Now().UTC()}
	stripeProviderForTest(fixture.api, http.StatusOK, stripeTestURL)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/invoices/other-invoice/checkout", nil)
	req.AddCookie(fixture.cookie)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("X-CSRF-Token", fixture.csrf)
	rec := httptest.NewRecorder()
	fixture.api.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-customer checkout status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
	if len(fixture.store.state.StripeCheckouts) != 0 {
		t.Fatalf("unauthorized checkout created local attempts: %d", len(fixture.store.state.StripeCheckouts))
	}
}

func TestStripeProviderRetryReusesPersistedIdempotencyKey(t *testing.T) {
	fixture := newStripeTestFixture(t)
	var keys []string
	calls := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if calls == 1 {
			return stripeHTTPResponse(r, http.StatusBadGateway, "temporary provider error"), nil
		}
		return stripeHTTPResponse(r, http.StatusOK, "{\"id\":\"cs_test_retry\",\"url\":\""+stripeTestURL+"\",\"expires_at\":1900000000}"), nil
	})
	fixture.api.stripe = stripeCheckoutSessionCreatorHTTP{apiKey: "sk_test_unit", endpoint: stripeCheckoutSessionsEndpoint, client: &http.Client{Transport: transport}}
	if rec := callPortalCheckout(t, fixture); rec.Code != http.StatusBadGateway {
		t.Fatalf("first provider attempt status = %d, want 502: %s", rec.Code, rec.Body.String())
	}
	if len(fixture.store.state.StripeCheckouts) != 1 {
		t.Fatalf("failed provider attempt was not persisted: %d", len(fixture.store.state.StripeCheckouts))
	}
	var persisted StripeCheckout
	for _, value := range fixture.store.state.StripeCheckouts {
		persisted = value
	}
	if persisted.Status != "Creating" || persisted.IdempotencyKey == "" {
		t.Fatalf("provider retry record is not reusable: %#v", persisted)
	}
	response := portalCheckoutResponse(t, callPortalCheckout(t, fixture), http.StatusOK)
	if response["sessionId"] != "cs_test_retry" || calls != 2 {
		t.Fatalf("retry response/calls unexpected: response=%#v calls=%d", response, calls)
	}
	if len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] || keys[0] != persisted.IdempotencyKey {
		t.Fatalf("provider retry keys did not remain stable: %#v, stored %q", keys, persisted.IdempotencyKey)
	}
	if len(fixture.store.state.StripeCheckouts) != 1 || fixture.store.state.StripeCheckouts[persisted.ID].Status != "Open" {
		t.Fatalf("provider retry created a second attempt or failed to open the saved one: %#v", fixture.store.state.StripeCheckouts)
	}
}

func TestStripeCheckoutRejectsProviderURLOutsideStripeCheckout(t *testing.T) {
	fixture := newStripeTestFixture(t)
	stripeProviderForTest(fixture.api, http.StatusOK, "https://checkout.stripe.com.evil.example/c/pay/cs_test_fake")
	if rec := callPortalCheckout(t, fixture); rec.Code != http.StatusBadGateway {
		t.Fatalf("unsafe provider URL status = %d, want 502: %s", rec.Code, rec.Body.String())
	}
	if len(fixture.store.state.StripeCheckouts) != 1 {
		t.Fatalf("rejected provider URL lost its idempotent attempt: %d", len(fixture.store.state.StripeCheckouts))
	}
	for _, checkout := range fixture.store.state.StripeCheckouts {
		if checkout.URL != "" || checkout.Status != "Creating" {
			t.Fatalf("unsafe URL was stored or attempt became unusable: %#v", checkout)
		}
	}
}

func TestStripeCheckoutRequiresTestKeyAndCustomerSession(t *testing.T) {
	fixture := newStripeTestFixture(t)
	fixture.api.Config.StripeAPIKey = "sk_live_not_allowed"
	if rec := callPortalCheckout(t, fixture); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("live Stripe key checkout status = %d, want 503", rec.Code)
	}
	if len(fixture.store.state.StripeCheckouts) != 0 {
		t.Fatal("checkout attempt was persisted while provider was not in test mode")
	}
	unauthenticated := httptest.NewRequest(http.MethodPost, "/api/v1/portal/invoices/"+fixture.invoiceID+"/checkout", nil)
	unauthenticated.Header.Set("Origin", "http://localhost:5173")
	unauthenticated.Header.Set("X-CSRF-Token", fixture.csrf)
	rec := httptest.NewRecorder()
	fixture.api.mux.ServeHTTP(rec, unauthenticated)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing customer session status = %d, want 401", rec.Code)
	}
}

func postPortalCheckout(t *testing.T, fixture stripeTestFixture) map[string]any {
	t.Helper()
	return portalCheckoutResponse(t, callPortalCheckout(t, fixture), http.StatusOK)
}

func callPortalCheckout(t *testing.T, fixture stripeTestFixture) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/invoices/"+fixture.invoiceID+"/checkout", nil)
	req.AddCookie(fixture.cookie)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("X-CSRF-Token", fixture.csrf)
	rec := httptest.NewRecorder()
	fixture.api.mux.ServeHTTP(rec, req)
	return rec
}

func portalCheckoutResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) map[string]any {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("checkout request returned %d, want %d: %s", rec.Code, wantStatus, rec.Body.String())
	}
	var value map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode checkout response: %v", err)
	}
	return value
}

func stripeProviderForTest(api *API, status int, checkoutURL string) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if status != http.StatusOK {
			return stripeHTTPResponse(r, status, ""), nil
		}
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		checkoutID := form.Get("metadata[orca_checkout_id]")
		response := fmt.Sprintf("{\"id\":\"cs_test_for_%s\",\"url\":%q,\"expires_at\":1900000000}", checkoutID, checkoutURL)
		return stripeHTTPResponse(r, http.StatusOK, response), nil
	})
	api.stripe = stripeCheckoutSessionCreatorHTTP{apiKey: api.Config.StripeAPIKey, endpoint: stripeCheckoutSessionsEndpoint, client: &http.Client{Transport: transport}}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func stripeHTTPResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: request}
}

func stripeSessionWebhook(eventID, eventType string, session map[string]any) []byte {
	data, _ := json.Marshal(map[string]any{"id": eventID, "type": eventType, "data": map[string]any{"object": session}})
	return data
}

func sendStripeWebhook(t *testing.T, handler http.Handler, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(payload))
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(stripeTestSecret))
	_, _ = fmt.Fprintf(mac, "%s.", timestamp)
	_, _ = mac.Write(payload)
	request.Header.Set("Stripe-Signature", "t="+timestamp+",v1="+hex.EncodeToString(mac.Sum(nil)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, request)
	return rec
}

func assertStripePaidState(t *testing.T, fixture stripeTestFixture, checkoutID string) {
	t.Helper()
	invoice := fixture.store.state.Invoices[fixture.invoiceID]
	if invoice.Status != "Paid" || invoice.Outstanding != "0.00" {
		t.Fatalf("Stripe reconciliation did not close the receivable: %#v", invoice)
	}
	if len(fixture.store.state.Payments) != 2 || len(fixture.store.state.Allocations) != 2 {
		t.Fatalf("Stripe payment was duplicated or missing: payments=%d allocations=%d", len(fixture.store.state.Payments), len(fixture.store.state.Allocations))
	}
	if got := fixture.store.state.StripeCheckouts[checkoutID].Status; got != "Completed" {
		t.Fatalf("checkout status = %q, want Completed", got)
	}
	debits, credits := decimal.Zero, decimal.Zero
	for _, entry := range fixture.store.state.Journal {
		debit, err := ParseDecimal(entry.Debit)
		if err != nil {
			t.Fatal(err)
		}
		credit, err := ParseDecimal(entry.Credit)
		if err != nil {
			t.Fatal(err)
		}
		debits = debits.Add(debit)
		credits = credits.Add(credit)
	}
	if !debits.Equal(decimal.RequireFromString("80.01")) || !credits.Equal(debits) {
		t.Fatalf("Stripe journal is not balanced for the exact received amount: debit=%s credit=%s", debits, credits)
	}
	if !hasAudit(fixture.store.state.Audit, "payment.allocated") || !hasOutbox(fixture.store.state.Outbox, "payment.recorded") {
		t.Fatal("Stripe payment omitted its audit or transactional outbox event")
	}
}

func hasAudit(events []AuditEvent, action string) bool {
	for _, event := range events {
		if event.Action == action {
			return true
		}
	}
	return false
}

func hasOutbox(events []OutboxEvent, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}
