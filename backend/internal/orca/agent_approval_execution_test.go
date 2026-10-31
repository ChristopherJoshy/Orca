package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestApprovedCopilotPaymentExecutesOnceAndPersistsResult(t *testing.T) {
	api, actor := newAgentToolTestAPI()
	api.Store.state.Invoices["invoice-a"] = Invoice{ID: "invoice-a", BusinessID: actor.BusinessID, CustomerID: "customer-a", Number: "INV-1", Status: "Confirmed", Currency: CurrencyINR, Total: "100.00", Outstanding: "100.00", Version: 7}
	approvalID, _, err := api.createAgentApproval(context.Background(), actor, actionReviewArguments{
		Action: "record_payment", InvoiceID: "invoice-a", Amount: "100", Method: " UPI ", Note: "Received", OperationKey: "approved-payment-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	csrf, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	sessionToken, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	api.Store.state.Sessions[tokenDigest(sessionToken)] = Session{UserID: actor.UserID, BusinessID: actor.BusinessID, Role: actor.Role, CSRFToken: tokenDigest(csrf), ExpiresAt: now.Add(time.Hour)}
	handler := NewAPI(api.Store, ServerConfig{Env: "development", CookieSecure: false, AllowedOrigin: "http://localhost:5173"})
	approve := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"decision": "approve"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/approvals/"+approvalID+"/decision", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "orca_session", Value: sessionToken})
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("X-CSRF-Token", csrf)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	first := approve()
	if first.Code != http.StatusOK {
		t.Fatalf("approval returned %d: %s", first.Code, first.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	result, ok := response["executionResult"].(map[string]any)
	if !ok || result["executed"] != true || result["status"] != "Paid" || result["outstanding"] != "0.00" {
		t.Fatalf("approval response did not report the executed payment: %#v", response)
	}
	if len(api.Store.state.Payments) != 1 || len(api.Store.state.Allocations) != 1 || len(api.Store.state.Journal) != 2 || !journalBalanced(api.Store.state.Journal) {
		t.Fatalf("payment side effects were missing or unbalanced: payments=%d allocations=%d journal=%#v", len(api.Store.state.Payments), len(api.Store.state.Allocations), api.Store.state.Journal)
	}
	if invoice := api.Store.state.Invoices["invoice-a"]; invoice.Status != "Paid" || invoice.Outstanding != "0.00" || invoice.Version != 8 {
		t.Fatalf("invoice was not settled exactly once: %#v", invoice)
	}
	if _, ok := api.Store.state.Approvals[approvalID].ExecutionResult["paymentId"]; !ok {
		t.Fatal("approved payment result has no payment id")
	}
	if second := approve(); second.Code != http.StatusConflict {
		t.Fatalf("second approval attempt returned %d, want 409: %s", second.Code, second.Body.String())
	}
	if len(api.Store.state.Payments) != 1 {
		t.Fatal("retry executed a second payment")
	}
}

func TestStaleApprovalCannotExecuteAndRemainsPending(t *testing.T) {
	api, actor := newAgentToolTestAPI()
	api.Store.state.Invoices["invoice-a"] = Invoice{ID: "invoice-a", BusinessID: actor.BusinessID, CustomerID: "customer-a", Status: "Confirmed", Currency: CurrencyINR, Total: "100.00", Outstanding: "100.00", Version: 7}
	approvalID, _, err := api.createAgentApproval(context.Background(), actor, actionReviewArguments{Action: "record_payment", InvoiceID: "invoice-a", Amount: "25.00", Method: "Cash", OperationKey: "stale-payment-1"})
	if err != nil {
		t.Fatal(err)
	}
	api.Store.state.Invoices["invoice-a"] = func(v Invoice) Invoice { v.Version++; return v }(api.Store.state.Invoices["invoice-a"])

	approval := api.Store.state.Approvals[approvalID]
	result, err := executeApprovedAction(api.Store.state, actor, approval, time.Now().UTC())
	if err != ErrConflict || result != nil {
		t.Fatalf("stale approval execution result=%#v err=%v, want conflict", result, err)
	}
	if len(api.Store.state.Payments) != 0 || api.Store.state.Approvals[approvalID].Status != "Pending" {
		t.Fatal("stale approval changed invoice payment state or approval status")
	}
}

func TestApprovedInvoiceConfirmationPostsBalancedJournal(t *testing.T) {
	api, actor := newAgentToolTestAPI()
	snapshot := CalculationSnapshot{EngineVersion: EngineVersion, Subtotal: "100.00", DiscountTotal: "0.00", TaxTotal: "18.00", Total: "118.00"}
	hash, _, err := canonicalHash(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	api.Store.state.Invoices["invoice-a"] = Invoice{ID: "invoice-a", BusinessID: actor.BusinessID, CustomerID: "customer-a", Number: "INV-2", Status: "Draft", Currency: CurrencyINR, Subtotal: "100.00", DiscountTotal: "0.00", TaxTotal: "18.00", Total: "118.00", Outstanding: "118.00", Snapshot: snapshot, CalculationHash: hash, Version: 2}
	approvalID, _, err := api.createAgentApproval(context.Background(), actor, actionReviewArguments{Action: "confirm_invoice", InvoiceID: "invoice-a"})
	if err != nil {
		t.Fatal(err)
	}
	approval := api.Store.state.Approvals[approvalID]
	result, err := executeApprovedAction(api.Store.state, actor, approval, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "Confirmed" || !journalBalanced(api.Store.state.Journal) || len(api.Store.state.Outbox) != 1 {
		t.Fatalf("confirmation result/journal/outbox incorrect: result=%#v journal=%#v outbox=%#v", result, api.Store.state.Journal, api.Store.state.Outbox)
	}
}

func TestApprovedCopilotSubscriptionLifecycleTransitions(t *testing.T) {
	for _, test := range []struct {
		action string
		audit  string
		status string
		sub    Subscription
		plan   Plan
	}{
		{action: "pause_subscription", audit: "pause", status: "Paused", sub: Subscription{Status: "Active"}, plan: Plan{AllowPause: true}},
		{action: "resume_subscription", audit: "resume", status: "Active", sub: Subscription{Status: "Paused", PauseIntervals: []PauseInterval{{Start: time.Now().Add(-time.Hour)}}}, plan: Plan{AllowPause: true}},
		{action: "renew_subscription", audit: "renew", status: "Draft", sub: Subscription{Status: "Closed", StartDate: "2025-01-01", ExpiryDate: "2025-12-31"}, plan: Plan{AllowRenew: true}},
	} {
		t.Run(test.action, func(t *testing.T) {
			api, actor := newAgentToolTestAPI()
			test.plan.ID, test.plan.BusinessID = "plan-a", actor.BusinessID
			api.Store.state.Plans[test.plan.ID] = test.plan
			test.sub.ID, test.sub.BusinessID, test.sub.PlanID, test.sub.Version = "sub-a", actor.BusinessID, test.plan.ID, 4
			api.Store.state.Subscriptions[test.sub.ID] = test.sub
			approvalID, _, err := api.createAgentApproval(context.Background(), actor, actionReviewArguments{Action: test.action, SubscriptionID: test.sub.ID})
			if err != nil {
				t.Fatal(err)
			}
			approvedAt := time.Now().UTC()
			result, err := executeApprovedAction(api.Store.state, actor, api.Store.state.Approvals[approvalID], approvedAt)
			if err != nil {
				t.Fatal(err)
			}
			got := api.Store.state.Subscriptions[test.sub.ID]
			if result["executed"] != true || result["status"] != test.status || got.Status != test.status || got.Version != 5 {
				t.Fatalf("approved transition result=%#v subscription=%+v", result, got)
			}
			if test.action == "pause_subscription" && (len(got.PauseIntervals) != 1 || got.PauseIntervals[0].End != nil) {
				t.Fatalf("pause did not open an interval: %+v", got.PauseIntervals)
			}
			if test.action == "resume_subscription" && (len(got.PauseIntervals) != 1 || got.PauseIntervals[0].End == nil) {
				t.Fatalf("resume did not close the open interval: %+v", got.PauseIntervals)
			}
			if test.action == "renew_subscription" {
				business := api.Store.state.Businesses[actor.BusinessID]
				location, _ := time.LoadLocation(business.Timezone)
				if got.StartDate != approvedAt.In(location).Format("2006-01-02") || got.ExpiryDate != "" || got.BillingCursor != nil {
					t.Fatalf("renewal retained stale term/cursor or wrong start date: %+v", got)
				}
			}
			if len(api.Store.state.Audit) == 0 || api.Store.state.Audit[len(api.Store.state.Audit)-1].Action != "subscription."+test.audit {
				t.Fatalf("approved transition was not audited: %+v", api.Store.state.Audit)
			}
		})
	}
}

func TestCopilotLifecycleApprovalRevalidatesPermissionAtExecution(t *testing.T) {
	api, actor := newAgentToolTestAPI()
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: actor.BusinessID, AllowPause: true}
	api.Store.state.Subscriptions["sub-a"] = Subscription{ID: "sub-a", BusinessID: actor.BusinessID, PlanID: "plan-a", Status: "Active", Version: 2}
	approvalID, _, err := api.createAgentApproval(context.Background(), actor, actionReviewArguments{Action: "pause_subscription", SubscriptionID: "sub-a"})
	if err != nil {
		t.Fatal(err)
	}
	plan := api.Store.state.Plans["plan-a"]
	plan.AllowPause = false
	api.Store.state.Plans["plan-a"] = plan
	result, err := executeApprovedAction(api.Store.state, actor, api.Store.state.Approvals[approvalID], time.Now().UTC())
	if err != ErrConflict || result != nil {
		t.Fatalf("approval after plan permission change result=%#v err=%v, want conflict", result, err)
	}
	if api.Store.state.Subscriptions["sub-a"].Status != "Active" || api.Store.state.Subscriptions["sub-a"].Version != 2 {
		t.Fatal("stale lifecycle approval changed subscription state")
	}
}
