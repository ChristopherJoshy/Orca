package orca

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestBusinessIDsReturnsSortedTenantIdentifiers(t *testing.T) {
	store := &Store{state: NewState()}
	store.state.Businesses["business-b"] = Business{ID: "business-b", Name: "Private B"}
	store.state.Businesses["business-a"] = Business{ID: "business-a", Name: "Private A"}
	ids, err := store.BusinessIDs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "business-a" || ids[1] != "business-b" {
		t.Fatalf("BusinessIDs() = %#v", ids)
	}
}

func TestBillingWorkerRunsCatchupAndStopsOnCancellation(t *testing.T) {
	store := &Store{state: NewState()}
	store.state.Businesses["business-a"] = Business{ID: "business-a", Name: "A", Currency: CurrencyINR, Timezone: "Asia/Kolkata", NextInvoiceNumber: 1}
	store.state.Businesses["business-b"] = Business{ID: "business-b", Name: "B", Currency: CurrencyINR, Timezone: "UTC", NextInvoiceNumber: 1}
	store.state.Customers["customer-a"] = Customer{ID: "customer-a", BusinessID: "business-a", Name: "A", Email: "a@example.test"}
	store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: "business-a", Name: "Monthly", Interval: "monthly", SalesPrice: "1250.00", MinimumQuantity: 1, TaxRate: "0", Version: 1}
	store.state.Subscriptions["subscription-a"] = Subscription{ID: "subscription-a", BusinessID: "business-a", CustomerID: "customer-a", PlanID: "plan-a", Number: "SUB-0001", Status: "Active", StartDate: "2026-08-01", Lines: []SubscriptionLine{{Name: "Monthly", Quantity: 1, UnitPrice: "1250.00", TaxRate: "0"}}, Version: 1}

	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	done := StartBillingWorker(ctx, store, time.Hour, logger)
	deadline := time.Now().Add(2 * time.Second)
	var businessAInvoices, businessBInvoices int
	for time.Now().Before(deadline) {
		_ = store.Read(context.Background(), func(st *State) error {
			businessAInvoices, businessBInvoices = 0, 0
			for _, invoice := range st.Invoices {
				if invoice.BusinessID == "business-a" {
					businessAInvoices++
				}
				if invoice.BusinessID == "business-b" {
					businessBInvoices++
				}
			}
			return nil
		})
		if businessAInvoices == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if businessAInvoices != 3 || businessBInvoices != 0 {
		cancel()
		<-done
		t.Fatalf("immediate catch-up invoices = (business A: %d, business B: %d), want (3, 0)", businessAInvoices, businessBInvoices)
	}

	// Rewind the cursor to model a restart/retry. Existing period identities
	// must be reused while the cursor catches up again.
	if err := store.UpdateFor(context.Background(), "business-a", func(st *State) error {
		v := st.Subscriptions["subscription-a"]
		v.BillingCursor = nil
		st.Subscriptions[v.ID] = v
		return nil
	}); err != nil {
		cancel()
		<-done
		t.Fatal(err)
	}
	if _, err := runBillingForBusiness(context.Background(), store, "business-a", Actor{BusinessID: "business-a", Role: "system"}, time.Now().UTC()); err != nil {
		cancel()
		<-done
		t.Fatal(err)
	}
	_ = store.Read(context.Background(), func(st *State) error {
		businessAInvoices, businessBInvoices = 0, 0
		for _, invoice := range st.Invoices {
			if invoice.BusinessID == "business-a" {
				businessAInvoices++
			}
			if invoice.BusinessID == "business-b" {
				businessBInvoices++
			}
		}
		return nil
	})
	if businessAInvoices != 3 || businessBInvoices != 0 {
		cancel()
		<-done
		t.Fatalf("repeat catch-up invoices = (business A: %d, business B: %d), want (3, 0)", businessAInvoices, businessBInvoices)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("billing worker did not stop after cancellation")
	}
}

func TestBillingWorkerContinuesAfterBusinessFailure(t *testing.T) {
	store := &Store{state: NewState()}
	store.state.Businesses["business-a"] = Business{ID: "business-a", Currency: CurrencyINR, Timezone: "UTC", NextInvoiceNumber: 1}
	store.state.Businesses["business-b"] = Business{ID: "business-b", Currency: CurrencyINR, Timezone: "UTC", NextInvoiceNumber: 1}
	store.state.Customers["customer-a"] = Customer{ID: "customer-a", BusinessID: "business-a", Name: "A", Email: "a@example.test"}
	store.state.Customers["customer-b"] = Customer{ID: "customer-b", BusinessID: "business-b", Name: "B", Email: "b@example.test"}
	store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: "business-a", Name: "Broken", Interval: "invalid", SalesPrice: "100.00", MinimumQuantity: 1, Version: 1}
	store.state.Plans["plan-b"] = Plan{ID: "plan-b", BusinessID: "business-b", Name: "Monthly", Interval: "monthly", SalesPrice: "200.00", MinimumQuantity: 1, Version: 1}
	store.state.Subscriptions["subscription-a"] = Subscription{ID: "subscription-a", BusinessID: "business-a", CustomerID: "customer-a", PlanID: "plan-a", Status: "Active", StartDate: "2026-10-01", Lines: []SubscriptionLine{{Name: "Broken", Quantity: 1, UnitPrice: "100.00", TaxRate: "0"}}, Version: 1}
	store.state.Subscriptions["subscription-b"] = Subscription{ID: "subscription-b", BusinessID: "business-b", CustomerID: "customer-b", PlanID: "plan-b", Status: "Active", StartDate: "2026-10-01", Lines: []SubscriptionLine{{Name: "Monthly", Quantity: 1, UnitPrice: "200.00", TaxRate: "0"}}, Version: 1}

	ctx, cancel := context.WithCancel(context.Background())
	done := StartBillingWorker(ctx, store, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	deadline := time.Now().Add(2 * time.Second)
	var businessBInvoices int
	for time.Now().Before(deadline) {
		_ = store.Read(context.Background(), func(st *State) error {
			businessBInvoices = 0
			for _, invoice := range st.Invoices {
				if invoice.BusinessID == "business-b" {
					businessBInvoices++
				}
			}
			return nil
		})
		if businessBInvoices == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("billing worker did not stop after cancellation")
	}
	if businessBInvoices != 1 {
		t.Fatalf("invoices for business B = %d, want 1 after business A failure", businessBInvoices)
	}
}
