package orca

import (
	"testing"
	"time"
)

func TestFixedDiscountLargestRemainderDoesNotOverAllocate(t *testing.T) {
	lines := make([]CalculationInputLine, 4)
	for i := range lines {
		lines[i] = CalculationInputLine{Product: Product{ID: "product", Version: 2}, Quantity: 1, UnitPrice: "0.01", TaxRate: "0", Description: "one paise", PlanID: "plan", PlanVersion: 4}
	}
	discount := &Discount{ID: "discount", Version: 3, Kind: "fixed", Value: "0.02", Scope: "subscription", MinimumPurchase: "0.00"}
	start := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	snapshot, hash, err := Calculate(lines, discount, "business", "subscription", start, start.AddDate(0, 1, 0), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := snapshot.DiscountTotal, "0.02"; got != want {
		t.Fatalf("discount total = %s, want %s", got, want)
	}
	allocatedPaise := 0
	for _, line := range snapshot.Lines {
		if line.DiscountAmount == "0.01" {
			allocatedPaise++
		}
	}
	if allocatedPaise != 2 {
		t.Fatalf("line discount allocation = %d paise, want 2", allocatedPaise)
	}
	for _, line := range snapshot.Lines {
		if line.DiscountAmount != "0.00" && line.DiscountAmount != "0.01" {
			t.Fatalf("invalid line discount %s", line.DiscountAmount)
		}
	}
	if snapshot.DiscountID != discount.ID || snapshot.DiscountVersion != discount.Version {
		t.Fatalf("discount source missing from snapshot: %+v", snapshot)
	}
	if snapshot.PlanID != "plan" || snapshot.PlanVersion != 4 {
		t.Fatalf("plan source missing from snapshot: %+v", snapshot)
	}
	if len(hash) != 64 {
		t.Fatalf("calculation hash length = %d", len(hash))
	}
}

func TestTaxRoundsHalfUpToPaise(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snapshot, _, err := Calculate([]CalculationInputLine{{Product: Product{ID: "p", Version: 1}, Quantity: 1, UnitPrice: "0.03", TaxRate: "18", Description: "rounding"}}, nil, "b", "s", start, start.AddDate(0, 1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := snapshot.TaxTotal, "0.01"; got != want {
		t.Fatalf("tax = %s, want %s", got, want)
	}
	if got, want := snapshot.Total, "0.04"; got != want {
		t.Fatalf("total = %s, want %s", got, want)
	}
}
