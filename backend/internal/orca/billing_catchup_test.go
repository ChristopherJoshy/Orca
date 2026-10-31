package orca

import (
	"context"
	"testing"
	"time"
)

func TestBillingCatchupCreatesEachDueMonthlyPeriodOnce(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Businesses[admin.BusinessID] = Business{ID: admin.BusinessID, Name: "A", Currency: CurrencyINR, Timezone: "Asia/Kolkata", NextInvoiceNumber: 1}
	api.Store.state.Plans["plan-monthly"] = Plan{ID: "plan-monthly", BusinessID: admin.BusinessID, Name: "Monthly", Interval: "monthly", SalesPrice: "1000.00", MinimumQuantity: 1, TaxRate: "0", Version: 1}
	api.Store.state.Subscriptions["sub-monthly"] = Subscription{
		ID: "sub-monthly", BusinessID: admin.BusinessID, CustomerID: "customer-a", PlanID: "plan-monthly",
		Number: "SUB-0001", Status: "Confirmed", StartDate: "2026-01-31", PaymentTermsDays: 30, Version: 1,
		Lines: []SubscriptionLine{{Name: "Monthly", Quantity: 1, UnitPrice: "1000.00", TaxRate: "0"}},
	}
	cutoff := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	actor := Actor{BusinessID: admin.BusinessID, Role: "system"}

	created, err := runBillingForBusiness(context.Background(), api.Store, admin.BusinessID, actor, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if created != 2 || len(api.Store.state.Invoices) != 2 {
		t.Fatalf("first catch-up created=%d invoices=%d, want two", created, len(api.Store.state.Invoices))
	}
	subscription := api.Store.state.Subscriptions["sub-monthly"]
	if subscription.Status != "Active" || subscription.BillingCursor == nil {
		t.Fatalf("confirmed subscription was not activated with a cursor: %#v", subscription)
	}
	if got := api.Store.state.Invoices; len(got) == 2 {
		periods := map[string]bool{}
		for _, invoice := range got {
			periods[invoice.PeriodStart.Format("2006-01-02")] = true
			if invoice.Total != "1000.00" || invoice.Status != "Draft" {
				t.Fatalf("unexpected generated invoice: %#v", invoice)
			}
		}
		if !periods["2026-01-30"] || !periods["2026-02-27"] {
			t.Fatalf("monthly anchor periods were not preserved: %#v", periods)
		}
	}

	created, err = runBillingForBusiness(context.Background(), api.Store, admin.BusinessID, actor, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 || len(api.Store.state.Invoices) != 2 {
		t.Fatalf("repeat catch-up created=%d invoices=%d, want zero and two", created, len(api.Store.state.Invoices))
	}
}

func TestBillingCatchupDoesNotActivateFutureSubscription(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Plans["plan-monthly"] = Plan{ID: "plan-monthly", BusinessID: admin.BusinessID, Name: "Monthly", Interval: "monthly", SalesPrice: "1000.00", MinimumQuantity: 1, TaxRate: "0", Version: 1}
	api.Store.state.Subscriptions["sub-future"] = Subscription{
		ID: "sub-future", BusinessID: admin.BusinessID, CustomerID: "customer-a", PlanID: "plan-monthly",
		Status: "Confirmed", StartDate: "2026-04-01", Version: 1,
		Lines: []SubscriptionLine{{Name: "Monthly", Quantity: 1, UnitPrice: "1000.00", TaxRate: "0"}},
	}
	created, err := runBillingForBusiness(context.Background(), api.Store, admin.BusinessID, Actor{BusinessID: admin.BusinessID, Role: "system"}, time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 || len(api.Store.state.Invoices) != 0 || api.Store.state.Subscriptions["sub-future"].Status != "Confirmed" {
		t.Fatal("future subscription was activated or billed before its start date")
	}
}

func TestBillingCatchupFinishesPeriodsThroughExpiryBeforeAutoClose(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Businesses[admin.BusinessID] = Business{ID: admin.BusinessID, Name: "A", Currency: CurrencyINR, Timezone: "UTC", NextInvoiceNumber: 1}
	api.Store.state.Plans["plan-auto-close"] = Plan{ID: "plan-auto-close", BusinessID: admin.BusinessID, Name: "Auto close", Interval: "monthly", SalesPrice: "100.00", MinimumQuantity: 1, TaxRate: "0", AutoClose: true, Version: 1}
	api.Store.state.Plans["plan-open"] = Plan{ID: "plan-open", BusinessID: admin.BusinessID, Name: "Manual close", Interval: "monthly", SalesPrice: "200.00", MinimumQuantity: 1, TaxRate: "0", Version: 1}
	for _, item := range []struct{ id, planID string }{{"sub-auto-close", "plan-auto-close"}, {"sub-manual-close", "plan-open"}} {
		api.Store.state.Subscriptions[item.id] = Subscription{
			ID: item.id, BusinessID: admin.BusinessID, CustomerID: "customer-a", PlanID: item.planID,
			Status: "Active", StartDate: "2026-10-01", ExpiryDate: "2026-10-15", Version: 1,
			Lines: []SubscriptionLine{{Name: item.planID, Quantity: 1, UnitPrice: "100.00", TaxRate: "0"}},
		}
	}

	cutoff := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	created, err := runBillingForBusiness(context.Background(), api.Store, admin.BusinessID, Actor{BusinessID: admin.BusinessID, Role: "system"}, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if created != 2 || len(api.Store.state.Invoices) != 2 {
		t.Fatalf("expired-term catch-up created=%d invoices=%d, want two eligible October periods", created, len(api.Store.state.Invoices))
	}
	if got := api.Store.state.Subscriptions["sub-auto-close"].Status; got != "Closed" {
		t.Fatalf("auto-close plan subscription status = %q, want Closed", got)
	}
	if got := api.Store.state.Subscriptions["sub-manual-close"].Status; got != "Active" {
		t.Fatalf("manual-close plan subscription status = %q, want Active", got)
	}
	for _, invoice := range api.Store.state.Invoices {
		if invoice.PeriodStart.Format("2006-01-02") != "2026-10-01" {
			t.Fatalf("catch-up generated a period beyond expiry: %s", invoice.PeriodStart.Format("2006-01-02"))
		}
	}
	autoCloseEvents := 0
	for _, event := range api.Store.state.Audit {
		if event.Action == "subscription.auto_closed" && event.ResourceID == "sub-auto-close" {
			autoCloseEvents++
		}
	}
	if autoCloseEvents != 1 {
		t.Fatalf("auto-close audit events = %d, want one", autoCloseEvents)
	}
}
