package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func newAgentToolTestAPI() (*API, Actor) {
	store := &Store{state: NewState()}
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	store.state.Businesses["biz-a"] = Business{ID: "biz-a", Name: "A", Currency: CurrencyINR, Timezone: "Asia/Kolkata"}
	store.state.Businesses["biz-b"] = Business{ID: "biz-b", Name: "B", Currency: CurrencyINR, Timezone: "UTC"}
	store.state.Users["staff-a"] = User{ID: "staff-a", Email: "staff@example.test", Verified: true, CreatedAt: now}
	store.state.Memberships[membershipKey("biz-a", "staff-a")] = Membership{BusinessID: "biz-a", UserID: "staff-a", Role: "admin"}
	store.state.Customers["customer-a"] = Customer{ID: "customer-a", BusinessID: "biz-a", UserID: "customer-user", Name: "Asha", Email: "asha@example.test"}
	store.state.Customers["customer-other"] = Customer{ID: "customer-other", BusinessID: "biz-a", Name: "Other", Email: "other@example.test"}
	store.state.Customers["customer-b"] = Customer{ID: "customer-b", BusinessID: "biz-b", Name: "Business B", Email: "b@example.test"}
	return &API{Store: store}, Actor{UserID: "staff-a", BusinessID: "biz-a", Role: "admin"}
}

func TestAgentSearchAndInvoiceReadsRespectActorScope(t *testing.T) {
	api, _ := newAgentToolTestAPI()
	customer := Actor{UserID: "customer-user", BusinessID: "biz-a", CustomerID: "customer-a", Role: "customer"}
	result, _, err := api.searchAgentRecords(context.Background(), customer, "customers", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["id"] != "customer-a" {
		t.Fatalf("customer search leaked records outside ownership: %#v", items)
	}
	if _, _, err := api.executeAgentTool(context.Background(), customer, "get_report_summary", []byte(`{}`)); err == nil {
		t.Fatal("customer should not receive a business wide financial report")
	}

	api.Store.state.Invoices["invoice-b"] = Invoice{ID: "invoice-b", BusinessID: "biz-b", CustomerID: "customer-b", Total: "20.00", Outstanding: "20.00"}
	if _, _, err := api.explainAgentInvoice(context.Background(), customer, "invoice-b"); err != ErrNotFound {
		t.Fatalf("cross-business invoice read error = %v, want not found", err)
	}
}

func TestAgentPreviewUsesCalculationEngineWithoutWrites(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	start := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, Name: "Monthly", Interval: "monthly", SalesPrice: "100.00", TaxRate: "18", Version: 3}
	api.Store.state.Subscriptions["sub-a"] = Subscription{ID: "sub-a", BusinessID: admin.BusinessID, CustomerID: "customer-a", PlanID: "plan-a", Status: "Active", StartDate: "2026-09-30", BillingCursor: &start, Lines: []SubscriptionLine{{Name: "Seats", ProductID: "product-a", Quantity: 2, UnitPrice: "50.00", TaxRate: "18"}}, Version: 4}
	api.Store.state.Discounts["discount-a"] = Discount{ID: "discount-a", BusinessID: admin.BusinessID, Kind: "fixed", Value: "10.00", Scope: "subscription", UseCount: 2}
	api.Store.state.Subscriptions["sub-a"] = func() Subscription {
		s := api.Store.state.Subscriptions["sub-a"]
		s.DiscountID = "discount-a"
		return s
	}()
	beforeUseCount := api.Store.state.Discounts["discount-a"].UseCount
	result, _, err := api.previewAgentInvoice(context.Background(), Actor{UserID: "customer-user", BusinessID: admin.BusinessID, CustomerID: "customer-a", Role: "customer"}, "sub-a")
	if err != nil {
		t.Fatal(err)
	}
	if result["writesPerformed"] != false {
		t.Fatalf("preview reported write activity: %#v", result)
	}
	if result["total"] != "106.20" {
		t.Fatalf("preview total = %#v, want 106.20 after discount and tax", result["total"])
	}
	if len(api.Store.state.Invoices) != 0 || api.Store.state.Discounts["discount-a"].UseCount != beforeUseCount {
		t.Fatal("preview created an invoice or consumed discount usage")
	}
}

func TestAgentActionReviewIsVersionBoundIdempotentAndDoesNotExecute(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Invoices["invoice-a"] = Invoice{ID: "invoice-a", BusinessID: admin.BusinessID, CustomerID: "customer-a", Status: "Confirmed", Currency: CurrencyINR, Total: "100.00", Outstanding: "100.00", Version: 7}
	args := actionReviewArguments{Action: "record_payment", InvoiceID: "invoice-a", Amount: "25.50", Method: " Cash ", Note: "received at counter", OperationKey: "agent-payment-001"}
	id, proposal, err := api.createAgentApproval(context.Background(), admin, args)
	if err != nil {
		t.Fatal(err)
	}
	if proposal["executed"] != false {
		t.Fatalf("proposal claims execution: %#v", proposal)
	}
	approval := api.Store.state.Approvals[id]
	inputs := approval.Inputs.(map[string]any)
	wantHash, _, err := canonicalHash(inputs)
	if err != nil || approval.InputsHash != wantHash {
		t.Fatalf("proposal hash does not match exact stored inputs: hash=%s want=%s err=%v", approval.InputsHash, wantHash, err)
	}
	if approval.ResourceVersion != 7 || fmt.Sprint(inputs["resourceVersion"]) != "7" || inputs["operationKey"] != "agent-payment-001" || inputs["method"] != "Cash" {
		t.Fatalf("proposal did not preserve normalized exact inputs and version: %#v", approval)
	}
	if len(api.Store.state.Payments) != 0 || len(api.Store.state.Journal) != 0 || len(api.Store.state.Outbox) != 0 {
		t.Fatal("proposal executed payment side effects before ORCA approval")
	}
	if current := api.Store.state.Invoices["invoice-a"]; current.Version != 7 || current.Outstanding != "100.00" {
		t.Fatalf("invoice changed while proposing payment: %#v", current)
	}
	secondID, _, err := api.createAgentApproval(context.Background(), admin, args)
	if err != nil {
		t.Fatal(err)
	}
	if secondID != id {
		t.Fatalf("identical pending proposal was not idempotent: first=%s second=%s", id, secondID)
	}
	if len(api.Store.state.Approvals) != 1 {
		t.Fatalf("duplicate proposal created extra approval: %d", len(api.Store.state.Approvals))
	}
}

func TestAgentCustomerCannotPrepareActionReview(t *testing.T) {
	api, _ := newAgentToolTestAPI()
	api.Store.state.Invoices["invoice-a"] = Invoice{ID: "invoice-a", BusinessID: "biz-a", CustomerID: "customer-a", Status: "Draft", Total: "10.00", Outstanding: "10.00"}
	customer := Actor{UserID: "customer-user", BusinessID: "biz-a", CustomerID: "customer-a", Role: "customer"}
	if _, _, err := api.createAgentApproval(context.Background(), customer, actionReviewArguments{Action: "confirm_invoice", InvoiceID: "invoice-a"}); err != ErrForbidden {
		t.Fatalf("customer action proposal error = %v, want forbidden", err)
	}
	if len(api.Store.state.Approvals) != 0 {
		t.Fatal("customer created an approval proposal")
	}
}

func TestAgentLifecycleReviewProposalsCheckPlanPolicyWithoutWriting(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, AllowPause: true, AllowRenew: true}
	api.Store.state.Subscriptions["sub-pause"] = Subscription{ID: "sub-pause", BusinessID: admin.BusinessID, PlanID: "plan-a", Status: "Active", Version: 3}
	api.Store.state.Subscriptions["sub-resume"] = Subscription{ID: "sub-resume", BusinessID: admin.BusinessID, PlanID: "plan-a", Status: "Paused", Version: 5}
	api.Store.state.Subscriptions["sub-renew"] = Subscription{ID: "sub-renew", BusinessID: admin.BusinessID, PlanID: "plan-a", Status: "Closed", StartDate: "2025-01-01", ExpiryDate: "2025-12-31", Version: 8}

	for _, test := range []struct {
		action  string
		id      string
		status  string
		version int64
	}{
		{action: "pause_subscription", id: "sub-pause", status: "Active", version: 3},
		{action: "resume_subscription", id: "sub-resume", status: "Paused", version: 5},
		{action: "renew_subscription", id: "sub-renew", status: "Closed", version: 8},
	} {
		id, proposal, err := api.createAgentApproval(context.Background(), admin, actionReviewArguments{Action: test.action, SubscriptionID: test.id})
		if err != nil {
			t.Fatalf("%s proposal failed: %v", test.action, err)
		}
		approval := api.Store.state.Approvals[id]
		inputs := approval.Inputs.(map[string]any)
		wantHash, _, err := canonicalHash(inputs)
		if err != nil || approval.InputsHash != wantHash {
			t.Fatalf("%s proposal is not bound to its exact inputs: hash=%s want=%s err=%v", test.action, approval.InputsHash, wantHash, err)
		}
		if proposal["executed"] != false || approval.ResourceType != "subscription" || approval.ResourceID != test.id || approval.ResourceVersion != test.version {
			t.Fatalf("%s proposal has wrong target/version or claims execution: approval=%+v proposal=%#v", test.action, approval, proposal)
		}
		if approval.Status != "Pending" || api.Store.state.Subscriptions[test.id].Status != test.status || api.Store.state.Subscriptions[test.id].Version != test.version {
			t.Fatalf("%s proposal changed subscription state: approval=%+v subscription=%+v", test.action, approval, api.Store.state.Subscriptions[test.id])
		}
		if test.action == "renew_subscription" {
			if date, ok := inputs["startDate"].(string); !ok || date == "" {
				t.Fatalf("renew proposal did not bind its proposed start date: %#v", inputs)
			}
			wire := approvalWire(approval)
			if wire["title"] != "Renew subscription" || !strings.Contains(fmt.Sprint(wire["detail"]), fmt.Sprint(inputs["startDate"])) {
				t.Fatalf("review does not show the renewal and exact start date: %#v", wire)
			}
		}
	}

	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, AllowPause: false, AllowRenew: true}
	if _, _, err := api.createAgentApproval(context.Background(), admin, actionReviewArguments{Action: "pause_subscription", SubscriptionID: "sub-pause"}); err != ErrConflict {
		t.Fatalf("pause proposal with plan permission disabled returned %v, want conflict", err)
	}
}

func TestGroqToolCallingLoopPersistsProgressWithoutProviderKey(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.AgentRuns["run-a"] = AgentRun{ID: "run-a", BusinessID: admin.BusinessID, UserID: admin.UserID, Status: "Running", Transcript: []any{map[string]any{"role": "user", "content": "Summarize billing."}}}
	calls := 0
	transport := agentTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer offline-test-key" {
			t.Errorf("unexpected provider request: %s auth=%q", r.Method, r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode provider request: %v", err)
		}
		if body["model"] != "test-model" || len(body["tools"].([]any)) == 0 {
			t.Errorf("request omitted configured model or tools: %#v", body)
		}
		var payload string
		if calls == 1 {
			payload = `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-1","type":"function","function":{"name":"get_report_summary","arguments":"{}"}}]}}]}`
		} else {
			payload = `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"There are no active billing records yet."}}]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(payload)), Request: r}, nil
	})

	run := api.Store.state.AgentRuns["run-a"]
	answer, proposalID, transcript, err := api.runGroqConversation(context.Background(), &http.Client{Transport: transport}, "https://groq.test/chat/completions", "offline-test-key", "test-model", admin, run, []map[string]any{{"role": "system", "content": groqSystemPrompt}, {"role": "user", "content": "Summarize billing."}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || answer != "There are no active billing records yet." || proposalID != "" {
		t.Fatalf("unexpected tool loop result calls=%d answer=%q proposal=%q", calls, answer, proposalID)
	}
	if len(transcript) < 3 || !strings.Contains(fmt.Sprint(transcript[1]), "get_report_summary") {
		t.Fatalf("tool progress was not included in transcript: %#v", transcript)
	}
	persisted := api.Store.state.AgentRuns["run-a"]
	if len(persisted.Transcript) < 3 {
		t.Fatalf("tool progress was not persisted: %#v", persisted.Transcript)
	}
}

type agentTestRoundTripper func(*http.Request) (*http.Response, error)

func (f agentTestRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
