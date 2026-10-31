package orca

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type emailRoundTripFunc func(*http.Request) (*http.Response, error)

func (f emailRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func emailResponse(request *http.Request, status int) *http.Response {
	return &http.Response{
		StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"resend-email-id"}`)), Request: request,
	}
}

func emailOutboxFixture() (*API, *Store) {
	now := time.Now().UTC()
	store := &Store{state: NewState()}
	store.state.Businesses["business-a"] = Business{ID: "business-a", Name: `<Brightloop & Co>`, Currency: CurrencyINR, Timezone: "UTC"}
	store.state.Customers["customer-a"] = Customer{ID: "customer-a", BusinessID: "business-a", Name: `<script>alert(1)</script>`, Email: "billing@example.test"}
	store.state.Invoices["invoice-a"] = Invoice{
		ID: "invoice-a", BusinessID: "business-a", CustomerID: "customer-a", Number: "INV-1042", Status: "Confirmed", Currency: CurrencyINR,
		Total: "118.00", Outstanding: "118.00", DueAt: now.Add(24 * time.Hour), PeriodStart: now, PeriodEnd: now.AddDate(0, 1, 0),
		Snapshot: CalculationSnapshot{Lines: []CalculationLine{{Description: `<img src=x onerror=alert(1)>`, Quantity: 1, UnitPrice: "100.00", LineTotal: "118.00"}}},
	}
	store.state.Quotations["quotation-a"] = Quotation{
		ID: "quotation-a", BusinessID: "business-a", CustomerID: "customer-a", Number: "QUO-1042", Title: `<svg onload=alert(1)>`, Status: "Sent", ValidUntil: now.AddDate(0, 0, 14).Format("2006-01-02"), Amount: "118.00",
		Lines: []SubscriptionLine{{Name: `<b onclick=alert(1)>Service</b>`, Quantity: 1, UnitPrice: "118.00"}},
	}
	store.state.Outbox = []OutboxEvent{
		{ID: "event-invoice", BusinessID: "business-a", Type: "invoice.email", IdempotencyKey: "invoice.email:invoice-a", Payload: map[string]any{"invoiceId": "invoice-a"}, CreatedAt: now},
		{ID: "event-quote", BusinessID: "business-a", Type: "quotation.email", IdempotencyKey: "quotation.email:quotation-a", Payload: map[string]any{"quotationId": "quotation-a"}, CreatedAt: now.Add(time.Second)},
	}
	api := newAPI(store, ServerConfig{ResendAPIKey: "re_test_unit", EmailFrom: "ORCA <billing@example.test>", PublicURL: "https://orca.example.test"})
	return api, store
}

func TestEmailOutboxDeliversInvoiceAndQuoteOnceWithEscapedHTML(t *testing.T) {
	api, store := emailOutboxFixture()
	requests := 0
	keys := map[string]bool{}
	bodies := make([]string, 0, 2)
	api.resendClient = &http.Client{Transport: emailRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Method != http.MethodPost || request.URL.String() != "https://api.resend.com/emails" {
			t.Errorf("unexpected Resend request: %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Authorization") != "Bearer re_test_unit" {
			t.Errorf("unexpected auth header")
		}
		key := request.Header.Get("Idempotency-Key")
		if key == "" || len(key) > 256 {
			t.Errorf("invalid idempotency key %q", key)
		}
		if keys[key] {
			t.Errorf("different outbox events reused key %q", key)
		}
		keys[key] = true
		body, _ := io.ReadAll(request.Body)
		bodies = append(bodies, string(body))
		return emailResponse(request, http.StatusOK), nil
	})}

	delivered, err := api.ProcessEmailOutbox(context.Background())
	if err != nil || delivered != 2 {
		t.Fatalf("first outbox pass delivered=%d err=%v, want 2 deliveries", delivered, err)
	}
	delivered, err = api.ProcessEmailOutbox(context.Background())
	if err != nil || delivered != 0 || requests != 2 {
		t.Fatalf("second outbox pass delivered=%d requests=%d err=%v; delivered messages must not be sent again", delivered, requests, err)
	}
	joinedBodies := strings.Join(bodies, "\n")
	var joinedHTML strings.Builder
	for _, body := range bodies {
		var payload struct {
			HTML string `json:"html"`
		}
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatalf("decode fake Resend payload: %v", err)
		}
		joinedHTML.WriteString(payload.HTML)
	}
	joinedBodies = joinedHTML.String()
	for _, unsafe := range []string{"<script>alert(1)</script>", "<img src=x onerror=alert(1)>", "<svg onload=alert(1)>", "<b onclick=alert(1)>"} {
		if strings.Contains(joinedBodies, unsafe) {
			t.Fatalf("email body contains unescaped HTML %q", unsafe)
		}
	}
	for _, escaped := range []string{"&lt;script&gt;", "&lt;img", "&lt;svg", "&lt;b"} {
		if !strings.Contains(joinedBodies, escaped) {
			t.Fatalf("email body did not escape %q", escaped)
		}
	}
	for _, event := range store.state.Outbox {
		if event.DeliveredAt == nil || event.Attempts != 1 || event.LastError != "" {
			t.Fatalf("outbox event state is wrong after successful delivery: %+v", event)
		}
	}
}

func TestEmailOutboxRetriesAmbiguousProviderFailureWithStableKey(t *testing.T) {
	api, store := emailOutboxFixture()
	store.state.Outbox = store.state.Outbox[:1]
	requests := 0
	var firstKey, retryKey string
	api.resendClient = &http.Client{Transport: emailRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			firstKey = request.Header.Get("Idempotency-Key")
			return nil, errors.New("connection reset after provider accepted request")
		}
		retryKey = request.Header.Get("Idempotency-Key")
		return emailResponse(request, http.StatusOK), nil
	})}
	if delivered, err := api.ProcessEmailOutbox(context.Background()); err == nil || delivered != 0 {
		t.Fatalf("ambiguous first attempt delivered=%d err=%v, want retryable error", delivered, err)
	}
	event := store.state.Outbox[0]
	if event.Attempts != 1 || event.DeliveredAt != nil || event.LastError == "" {
		t.Fatalf("failed delivery state was not retained: %+v", event)
	}
	if delivered, err := api.ProcessEmailOutbox(context.Background()); err != nil || delivered != 1 {
		t.Fatalf("retry delivered=%d err=%v, want one successful delivery", delivered, err)
	}
	if requests != 2 || firstKey == "" || firstKey != retryKey {
		t.Fatalf("provider retry did not reuse stable idempotency key: requests=%d first=%q retry=%q", requests, firstKey, retryKey)
	}
	event = store.state.Outbox[0]
	if event.Attempts != 2 || event.DeliveredAt == nil || event.LastError != "" {
		t.Fatalf("successful retry state is wrong: %+v", event)
	}
}

func TestEmailOutboxProviderFailureRemainsRetryable(t *testing.T) {
	api, store := emailOutboxFixture()
	store.state.Outbox = store.state.Outbox[:1]
	api.resendClient = &http.Client{Transport: emailRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return emailResponse(request, http.StatusServiceUnavailable), nil
	})}
	if delivered, err := api.ProcessEmailOutbox(context.Background()); err == nil || delivered != 0 {
		t.Fatalf("provider error delivered=%d err=%v, want error", delivered, err)
	}
	event := store.state.Outbox[0]
	if event.Attempts != 1 || event.DeliveredAt != nil || !strings.Contains(event.LastError, "status 503") {
		t.Fatalf("provider error state is not retryable/diagnosable: %+v", event)
	}
}

func TestResendOutboxIdempotencyKeyIsStableAndTenantBound(t *testing.T) {
	event := OutboxEvent{BusinessID: "business-a", IdempotencyKey: "invoice.email:invoice-a"}
	key := resendOutboxIdempotencyKey(event)
	if key == "" || len(key) > 256 || key != resendOutboxIdempotencyKey(event) {
		t.Fatalf("unstable or invalid key %q", key)
	}
	event.BusinessID = "business-b"
	if key == resendOutboxIdempotencyKey(event) {
		t.Fatal("different tenant event identities must not share a provider key")
	}
}

func TestEmailOutboxWorkerStaysDisabledWithoutProviderConfiguration(t *testing.T) {
	api := newAPI(&Store{state: NewState()}, ServerConfig{})
	done := StartEmailOutboxWorker(context.Background(), api, time.Second, nil)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("unconfigured email worker should return a completed channel")
	}
}
