package orca

import (
	"context"
	"testing"
	"time"
)

func discountCalculationLine(productID, price string) CalculationInputLine {
	return CalculationInputLine{
		Product: Product{ID: productID, Version: 1}, Quantity: 1,
		UnitPrice: price, TaxRate: "0", Description: productID,
	}
}

func calculateDiscountAt(t *testing.T, discount *Discount, at time.Time, lines ...CalculationInputLine) CalculationSnapshot {
	t.Helper()
	snapshot, _, err := Calculate(lines, discount, "business", "subscription", at, at.AddDate(0, 1, 0), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestDiscountDateWindowIncludesBoundaries(t *testing.T) {
	starts := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	ends := starts.Add(24 * time.Hour)
	discount := &Discount{
		ID: "discount-window", Version: 1, Kind: "percentage", Value: "10.00",
		Scope: "subscription", MinimumPurchase: "0.00", MinimumQuantity: 1,
		StartsAt: &starts, EndsAt: &ends,
	}
	line := discountCalculationLine("product-a", "100.00")

	tests := []struct {
		name       string
		at         time.Time
		wantAmount string
		wantID     string
	}{
		{name: "before start", at: starts.Add(-time.Nanosecond), wantAmount: "0.00"},
		{name: "at start", at: starts, wantAmount: "10.00", wantID: discount.ID},
		{name: "at end", at: ends, wantAmount: "10.00", wantID: discount.ID},
		{name: "after end", at: ends.Add(time.Nanosecond), wantAmount: "0.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := calculateDiscountAt(t, discount, tt.at, line)
			if got := snapshot.Lines[0].DiscountAmount; got != tt.wantAmount {
				t.Fatalf("discount amount = %s, want %s", got, tt.wantAmount)
			}
			if got := snapshot.DiscountID; got != tt.wantID {
				t.Fatalf("snapshot discount ID = %q, want %q", got, tt.wantID)
			}
		})
	}
}

func TestDiscountProductAndSubscriptionScopes(t *testing.T) {
	lines := []CalculationInputLine{
		discountCalculationLine("product-a", "100.00"),
		discountCalculationLine("product-b", "50.00"),
	}
	productDiscount := &Discount{
		ID: "product-discount", Kind: "percentage", Value: "10.00", Scope: "product",
		ProductIDs: []string{"product-a"}, MinimumPurchase: "0.00", MinimumQuantity: 1,
	}
	productSnapshot := calculateDiscountAt(t, productDiscount, time.Now().UTC(), lines...)
	if got := productSnapshot.Lines[0].DiscountAmount; got != "10.00" {
		t.Fatalf("eligible product discount = %s, want 10.00", got)
	}
	if got := productSnapshot.Lines[1].DiscountAmount; got != "0.00" {
		t.Fatalf("non-target product discount = %s, want 0.00", got)
	}
	if got := productSnapshot.DiscountID; got != productDiscount.ID {
		t.Fatalf("product-scope snapshot discount ID = %q", got)
	}

	subscriptionDiscount := *productDiscount
	subscriptionDiscount.ID = "subscription-discount"
	subscriptionDiscount.Scope = "subscription"
	subscriptionSnapshot := calculateDiscountAt(t, &subscriptionDiscount, time.Now().UTC(), lines...)
	if got := subscriptionSnapshot.Lines[0].DiscountAmount; got != "10.00" {
		t.Fatalf("first subscription line discount = %s, want 10.00", got)
	}
	if got := subscriptionSnapshot.Lines[1].DiscountAmount; got != "5.00" {
		t.Fatalf("second subscription line discount = %s, want 5.00", got)
	}
}

func TestDiscountUsageCapPreventsFurtherApplication(t *testing.T) {
	discount := &Discount{
		ID: "discount-cap", Kind: "fixed", Value: "10.00", Scope: "subscription",
		MinimumPurchase: "0.00", MinimumQuantity: 1, MaxUses: 2, UseCount: 2,
	}
	at := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	snapshot := calculateDiscountAt(t, discount, at, discountCalculationLine("product-a", "100.00"))
	if got := snapshot.Lines[0].DiscountAmount; got != "0.00" {
		t.Fatalf("capped discount amount = %s, want 0.00", got)
	}
	if snapshot.DiscountID != "" {
		t.Fatalf("exhausted discount was recorded in snapshot: %q", snapshot.DiscountID)
	}
}

func TestBillingConsumesDiscountUsageOnlyUpToCap(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Businesses[admin.BusinessID] = Business{
		ID: admin.BusinessID, Name: "A", Currency: CurrencyINR, Timezone: "UTC", NextInvoiceNumber: 1,
	}
	api.Store.state.Plans["plan-monthly"] = Plan{
		ID: "plan-monthly", BusinessID: admin.BusinessID, Name: "Monthly", Interval: "monthly",
		SalesPrice: "100.00", MinimumQuantity: 1, TaxRate: "0", Version: 1,
	}
	api.Store.state.Discounts["discount-a"] = Discount{
		ID: "discount-a", BusinessID: admin.BusinessID, Kind: "fixed", Value: "10.00",
		Scope: "subscription", MinimumPurchase: "0.00", MinimumQuantity: 1, MaxUses: 1,
	}
	for _, id := range []string{"subscription-a", "subscription-b"} {
		api.Store.state.Subscriptions[id] = Subscription{
			ID: id, BusinessID: admin.BusinessID, CustomerID: "customer-a", PlanID: "plan-monthly",
			DiscountID: "discount-a", Status: "Active", StartDate: "2026-10-01", Version: 1,
			Lines: []SubscriptionLine{{Name: "Monthly", Quantity: 1, UnitPrice: "100.00", TaxRate: "0"}},
		}
	}

	created, err := runBillingForBusiness(
		context.Background(), api.Store, admin.BusinessID,
		Actor{BusinessID: admin.BusinessID, Role: "system"}, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if created != 2 || len(api.Store.state.Invoices) != 2 {
		t.Fatalf("billing created=%d invoices=%d, want two", created, len(api.Store.state.Invoices))
	}
	if got := api.Store.state.Discounts["discount-a"].UseCount; got != 1 {
		t.Fatalf("discount useCount=%d, want cap of 1", got)
	}
	discounted, undiscounted := 0, 0
	for _, invoice := range api.Store.state.Invoices {
		if invoice.Snapshot.DiscountID == "discount-a" && invoice.DiscountTotal == "10.00" {
			discounted++
		} else if invoice.Snapshot.DiscountID == "" && invoice.DiscountTotal == "0.00" {
			undiscounted++
		} else {
			t.Fatalf("unexpected invoice discount result: id=%q total=%s", invoice.Snapshot.DiscountID, invoice.DiscountTotal)
		}
	}
	if discounted != 1 || undiscounted != 1 {
		t.Fatalf("discounted=%d undiscounted=%d, want one of each", discounted, undiscounted)
	}
}

func TestBillingFailureDoesNotConsumeDiscountUse(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Businesses[admin.BusinessID] = Business{
		ID: admin.BusinessID, Name: "A", Currency: CurrencyINR, Timezone: "UTC", NextInvoiceNumber: 7,
	}
	api.Store.state.Plans["plan-monthly"] = Plan{
		ID: "plan-monthly", BusinessID: admin.BusinessID, Name: "Monthly", Interval: "monthly",
		SalesPrice: "100.00", MinimumQuantity: 1, TaxRate: "0", Version: 1,
	}
	api.Store.state.Discounts["discount-a"] = Discount{
		ID: "discount-a", BusinessID: admin.BusinessID, Kind: "fixed", Value: "10.00",
		Scope: "subscription", MinimumPurchase: "0.00", MinimumQuantity: 1, MaxUses: 5, UseCount: 2,
	}
	api.Store.state.Subscriptions["subscription-a"] = Subscription{
		ID: "subscription-a", BusinessID: admin.BusinessID, CustomerID: "customer-a", PlanID: "plan-monthly",
		DiscountID: "discount-a", Number: "SUB-1", Status: "Active", StartDate: "2026-10-01", Version: 1,
		Lines: []SubscriptionLine{{Name: "Broken", Quantity: 1, UnitPrice: "not-money", TaxRate: "0"}},
	}

	created, err := runBillingForBusiness(
		context.Background(), api.Store, admin.BusinessID,
		Actor{BusinessID: admin.BusinessID, Role: "system"}, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
	)
	if err == nil {
		t.Fatal("billing with an invalid line price should fail")
	}
	if created != 0 || len(api.Store.state.Invoices) != 0 {
		t.Fatalf("failed billing created=%d invoices=%d, want 0 and 0", created, len(api.Store.state.Invoices))
	}
	if got := api.Store.state.Discounts["discount-a"].UseCount; got != 2 {
		t.Fatalf("failed billing consumed discount use: useCount=%d, want 2", got)
	}
	if got := api.Store.state.Businesses[admin.BusinessID].NextInvoiceNumber; got != 7 {
		t.Fatalf("failed billing advanced invoice sequence: got %d, want 7", got)
	}
}
