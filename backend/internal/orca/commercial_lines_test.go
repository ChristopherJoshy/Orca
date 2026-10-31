package orca

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSubscriptionLinesUseServerPlanAndVariantPrices(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, Name: "Business", SalesPrice: "250.00", MinimumQuantity: 2, TaxRate: "5", Version: 1}
	api.Store.state.Products["product-a"] = Product{ID: "product-a", BusinessID: admin.BusinessID, Name: "Subscription Management", SalesPrice: "1000.00", TaxRate: "18", Variants: []Variant{{Name: "Odoo", Value: "Odoo", Adjustment: "560.00"}}}
	start := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	request := httptest.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(`{"customerId":"customer-a","planId":"plan-a","startDate":"`+start+`","paymentTermsDays":30,"lines":[{"productId":"product-a","name":"Fake product","variant":"Odoo","quantity":3,"unitPrice":"1.00","taxRate":"0"},{"name":"Fake plan","quantity":2,"unitPrice":"1.00","taxRate":"0"}]}`))
	response := httptest.NewRecorder()
	api.createSubscription(response, request, Authenticated{Actor: admin})
	if response.Code != 201 {
		t.Fatalf("create subscription status %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Amount string             `json:"amount"`
		Lines  []SubscriptionLine `json:"lines"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Amount != "5180.00" || len(result.Lines) != 2 {
		t.Fatalf("server-derived amount/lines = %#v, want 5180.00 across two lines", result)
	}
	if got := result.Lines[0]; got.Name != "Business" || got.UnitPrice != "250.00" || got.TaxRate != "5.00" || got.Quantity != 2 {
		t.Fatalf("plan line did not use catalog terms: %+v", got)
	}
	if got := result.Lines[1]; got.Name != "Subscription Management" || got.UnitPrice != "1560.00" || got.TaxRate != "18.00" || got.Variant != "Odoo" {
		t.Fatalf("variant line did not use server catalog terms: %+v", got)
	}

	badVariant := httptest.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(`{"customerId":"customer-a","planId":"plan-a","startDate":"`+start+`","lines":[{"productId":"product-a","variant":"Unknown","quantity":1}]}`))
	badResponse := httptest.NewRecorder()
	api.createSubscription(badResponse, badVariant, Authenticated{Actor: admin})
	if badResponse.Code != 422 || len(api.Store.state.Subscriptions) != 1 {
		t.Fatalf("unknown variant was accepted or persisted: status=%d subscriptions=%d body=%s", badResponse.Code, len(api.Store.state.Subscriptions), badResponse.Body.String())
	}
}

func TestDirectQuotationLinesUseServerVariantPricing(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, Name: "Monthly", SalesPrice: "100.00", MinimumQuantity: 1, TaxRate: "0"}
	api.Store.state.Products["product-a"] = Product{ID: "product-a", BusinessID: admin.BusinessID, Name: "Odoo Add-on", SalesPrice: "300.00", TaxRate: "18", Variants: []Variant{{Name: "Odoo", Value: "Odoo", Adjustment: "560.00"}}}
	request := httptest.NewRequest("POST", "/api/v1/quotations", strings.NewReader(`{"customerId":"customer-a","planId":"plan-a","title":"Variant offer","validDays":14,"quantity":1,"lines":[{"name":"Plan","quantity":1,"unitPrice":"1.00"},{"productId":"product-a","variant":"Odoo","name":"Spoofed","quantity":1,"unitPrice":"1.00","taxRate":"0"}]}`))
	response := httptest.NewRecorder()
	api.createQuotation(response, request, Authenticated{Actor: admin})
	if response.Code != 201 {
		t.Fatalf("create quotation status %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Amount string             `json:"amount"`
		Lines  []SubscriptionLine `json:"lines"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Amount != "1114.80" || len(result.Lines) != 2 || result.Lines[1].UnitPrice != "860.00" || result.Lines[1].Name != "Odoo Add-on" {
		t.Fatalf("quotation did not preserve server-priced variant: %#v", result)
	}
}
