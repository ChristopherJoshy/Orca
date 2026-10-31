package orca

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQuotationTemplateSnapshotsSelectedVariantPrice(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, Name: "Monthly", SalesPrice: "100.00", MinimumQuantity: 1, TaxRate: "0"}
	api.Store.state.Products["product-a"] = Product{ID: "product-a", BusinessID: admin.BusinessID, Name: "Support", SalesPrice: "300.00", TaxRate: "18", Variants: []Variant{{Name: "Brand", Value: "Odoo", Adjustment: "560.00"}}}

	request := httptest.NewRequest("POST", "/api/v1/quotation-templates", strings.NewReader(`{"name":"Odoo support offer","planId":"plan-a","productId":"product-a","productVariant":"Odoo","quantity":1,"validDays":30}`))
	response := httptest.NewRecorder()
	api.createQuotationTemplate(response, request, Authenticated{Actor: admin})
	if response.Code != 201 {
		t.Fatalf("create quotation template returned %d: %s", response.Code, response.Body.String())
	}
	var template QuoteTemplate
	if err := json.Unmarshal(response.Body.Bytes(), &template); err != nil {
		t.Fatal(err)
	}
	if len(template.Lines) != 2 || template.Lines[1].UnitPrice != "860.00" || template.Lines[1].Variant != "Odoo" {
		t.Fatalf("template did not snapshot selected variant terms: %+v", template.Lines)
	}

	quoteRequest := httptest.NewRequest("POST", "/api/v1/quotations", strings.NewReader(`{"customerId":"customer-a","templateId":"`+template.ID+`","planId":"plan-a","lines":[{"productId":"product-a","name":"Spoofed","quantity":1,"unitPrice":"0.01","taxRate":"0","variant":"Unknown"}]}`))
	quoteResponse := httptest.NewRecorder()
	api.createQuotation(quoteResponse, quoteRequest, Authenticated{Actor: admin})
	if quoteResponse.Code != 201 {
		t.Fatalf("create quotation from template returned %d: %s", quoteResponse.Code, quoteResponse.Body.String())
	}
	var quote Quotation
	if err := json.Unmarshal(quoteResponse.Body.Bytes(), &quote); err != nil {
		t.Fatal(err)
	}
	if quote.Amount != "1114.80" || len(quote.Lines) != 2 || quote.Lines[1].UnitPrice != "860.00" || quote.Lines[1].Variant != "Odoo" {
		t.Fatalf("quotation did not retain server-priced variant: amount=%s lines=%+v", quote.Amount, quote.Lines)
	}
}
