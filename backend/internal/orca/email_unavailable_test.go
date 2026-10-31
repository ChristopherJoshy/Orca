package orca

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProductionSignupAndPasswordResetDoNotCreateDeadEndRecordsWithoutEmail(t *testing.T) {
	store := &Store{state: NewState()}
	api := NewAPI(store, ServerConfig{Env: "production", CookieSecure: true, AllowedOrigin: "https://orca.example.test"})
	signupBody, _ := json.Marshal(map[string]string{"email": "new@example.test", "password": "Password!2026", "businessName": "New Business"})
	signup := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader(string(signupBody)))
	signup.RemoteAddr = "127.0.0.1:1234"
	signupRec := httptest.NewRecorder()
	api.ServeHTTP(signupRec, signup)
	if signupRec.Code != http.StatusServiceUnavailable || !strings.Contains(signupRec.Body.String(), "email delivery is not configured") {
		t.Fatalf("production signup returned %d: %s", signupRec.Code, signupRec.Body.String())
	}
	reset := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset", strings.NewReader(`{"email":"new@example.test"}`))
	resetRec := httptest.NewRecorder()
	api.ServeHTTP(resetRec, reset)
	if resetRec.Code != http.StatusServiceUnavailable || !strings.Contains(resetRec.Body.String(), "email delivery is not configured") {
		t.Fatalf("password reset returned %d: %s", resetRec.Code, resetRec.Body.String())
	}
	if len(store.state.Users) != 0 || len(store.state.Businesses) != 0 || len(store.state.EmailVerifications) != 0 || len(store.state.ResetTokens) != 0 {
		t.Fatalf("unavailable email flows left unusable records: users=%#v businesses=%#v verification=%#v reset=%#v", store.state.Users, store.state.Businesses, store.state.EmailVerifications, store.state.ResetTokens)
	}
}

func TestEmailActionsDoNotChangeRecordsWithoutProvider(t *testing.T) {
	store := &Store{state: NewState()}
	api := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false, AllowedOrigin: "http://localhost:5173"})
	cookie, csrf := signupForTest(t, api, "email@example.test", "Email Business")
	session := store.state.Sessions[tokenDigest(cookie.Value)]
	now := time.Now().UTC()
	store.state.Invoices["invoice-a"] = Invoice{ID: "invoice-a", BusinessID: session.BusinessID, CustomerID: "customer-a", Number: "INV-1042", Status: "Confirmed", Currency: CurrencyINR, Total: "118.00", Outstanding: "118.00", DueAt: now, Version: 2}
	store.state.Quotations["quote-a"] = Quotation{ID: "quote-a", BusinessID: session.BusinessID, CustomerID: "customer-a", Number: "QUO-1042", Status: "Draft", Amount: "118.00", CreatedAt: now}

	post := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.AddCookie(cookie)
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("X-CSRF-Token", csrf)
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		return rec
	}
	for _, path := range []string{"/api/v1/invoices/invoice-a/send", "/api/v1/quotations/quote-a/send"} {
		if rec := post(path); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "email delivery is not configured") {
			t.Fatalf("email action %s returned %d: %s", path, rec.Code, rec.Body.String())
		}
	}
	if store.state.Invoices["invoice-a"].Status != "Confirmed" || store.state.Quotations["quote-a"].Status != "Draft" || len(store.state.Outbox) != 0 {
		t.Fatalf("unconfigured email action mutated a record: invoice=%+v quote=%+v outbox=%#v", store.state.Invoices["invoice-a"], store.state.Quotations["quote-a"], store.state.Outbox)
	}
}

func TestEmailApprovalCannotQueueAfterProviderConfigurationIsRemoved(t *testing.T) {
	store := &Store{state: NewState()}
	api := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false, AllowedOrigin: "http://localhost:5173", ResendAPIKey: "re_test_key", EmailFrom: "ORCA <billing@example.test>"})
	cookie, csrf := signupForTest(t, api, "approval-email@example.test", "Approval Email Business")
	session := store.state.Sessions[tokenDigest(cookie.Value)]
	now := time.Now().UTC()
	store.state.Invoices["invoice-a"] = Invoice{ID: "invoice-a", BusinessID: session.BusinessID, CustomerID: "customer-a", Number: "INV-1043", Status: "Confirmed", Currency: CurrencyINR, Total: "118.00", Outstanding: "118.00", DueAt: now, Version: 1}
	actor := Actor{UserID: session.UserID, BusinessID: session.BusinessID, Role: session.Role}
	approvalID, _, err := api.createAgentApproval(context.Background(), actor, actionReviewArguments{Action: "send_invoice", InvoiceID: "invoice-a"})
	if err != nil {
		t.Fatal(err)
	}
	api.Config.ResendAPIKey = ""
	body := strings.NewReader(`{"decision":"approve"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/approvals/"+approvalID+"/decision", body)
	req.AddCookie(cookie)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("X-CSRF-Token", csrf)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "email delivery is not configured") {
		t.Fatalf("unconfigured email approval returned %d: %s", rec.Code, rec.Body.String())
	}
	if store.state.Approvals[approvalID].Status != "Pending" || len(store.state.Outbox) != 0 {
		t.Fatalf("unavailable email approval was recorded as complete: approval=%+v outbox=%#v", store.state.Approvals[approvalID], store.state.Outbox)
	}
}

func TestCopilotCannotProposeEmailWithoutProvider(t *testing.T) {
	api, actor := newAgentToolTestAPI()
	api.Store.state.Invoices["invoice-a"] = Invoice{ID: "invoice-a", BusinessID: actor.BusinessID, CustomerID: "customer-a", Number: "INV-1044", Status: "Confirmed", Currency: CurrencyINR, Total: "118.00", Outstanding: "118.00", Version: 1}
	if _, _, err := api.createAgentApproval(context.Background(), actor, actionReviewArguments{Action: "send_invoice", InvoiceID: "invoice-a"}); !errors.Is(err, ErrEmailDeliveryUnavailable) {
		t.Fatalf("send proposal error = %v, want email delivery unavailable", err)
	}
	if len(api.Store.state.Approvals) != 0 || len(api.Store.state.Outbox) != 0 {
		t.Fatalf("unavailable email proposal wrote state: approvals=%#v outbox=%#v", api.Store.state.Approvals, api.Store.state.Outbox)
	}
}
