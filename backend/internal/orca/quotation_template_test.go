package orca

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateQuotationTemplateAndUseItWithoutCrossTenantLeak(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, Name: "Monthly", Interval: "monthly", SalesPrice: "125.50", TaxRate: "18", MinimumQuantity: 2}
	api.Store.state.Products["product-a"] = Product{ID: "product-a", BusinessID: admin.BusinessID, Name: "Priority support", SalesPrice: "40.25", TaxRate: "5"}
	api.Store.state.Products["product-b"] = Product{ID: "product-b", BusinessID: "biz-b", Name: "Other business product", SalesPrice: "900.00", TaxRate: "18"}

	request := httptest.NewRequest("POST", "/api/v1/quotation-templates", strings.NewReader(`{"name":"Monthly with support","planId":"plan-a","productId":"product-a","quantity":2,"productQuantity":1,"validDays":21}`))
	response := httptest.NewRecorder()
	api.createQuotationTemplate(response, request, Authenticated{Actor: admin})
	if response.Code != 201 {
		t.Fatalf("create template status = %d, body = %s", response.Code, response.Body.String())
	}
	template := api.Store.state.QuoteTemplates[func() string {
		for id := range api.Store.state.QuoteTemplates {
			return id
		}
		return ""
	}()]
	if template.PlanID != "plan-a" || template.ValidDays != 21 || len(template.Lines) != 2 {
		t.Fatalf("stored template = %#v", template)
	}
	if template.Lines[0].UnitPrice != "125.50" || template.Lines[0].Quantity != 2 || template.Lines[1].ProductID != "product-a" || template.Lines[1].UnitPrice != "40.25" {
		t.Fatalf("template lines did not preserve exact prices and references: %#v", template.Lines)
	}

	adminState, err := stateForBusiness(api.Store.state, admin.BusinessID)
	if err != nil {
		t.Fatal(err)
	}
	actorItems := collectionItems(adminState, "quotation-templates", admin)
	if len(actorItems) != 1 {
		t.Fatalf("business template list length = %d, want 1", len(actorItems))
	}
	other := Actor{UserID: "other-admin", BusinessID: "biz-b", Role: "admin"}
	otherState, err := stateForBusiness(api.Store.state, other.BusinessID)
	if err != nil {
		t.Fatal(err)
	}
	if items := collectionItems(otherState, "quotation-templates", other); len(items) != 0 {
		t.Fatalf("other business received quotation templates: %#v", items)
	}
	customer := Actor{UserID: "customer-user", BusinessID: admin.BusinessID, CustomerID: "customer-a", Role: "customer"}
	if items := collectionItems(adminState, "quotation-templates", customer); len(items) != 0 {
		t.Fatalf("customer received business quotation templates: %#v", items)
	}

	createQuoteRequest := httptest.NewRequest("POST", "/api/v1/quotations", strings.NewReader(`{"customerId":"customer-a","templateId":"`+template.ID+`"}`))
	createQuoteResponse := httptest.NewRecorder()
	api.createQuotation(createQuoteResponse, createQuoteRequest, Authenticated{Actor: admin})
	if createQuoteResponse.Code != 201 {
		t.Fatalf("create quotation from template status = %d, body = %s", createQuoteResponse.Code, createQuoteResponse.Body.String())
	}
	quote := api.Store.state.Quotations[func() string {
		for id := range api.Store.state.Quotations {
			return id
		}
		return ""
	}()]
	if quote.PlanID != template.PlanID || quote.Title != template.Name || len(quote.Lines) != len(template.Lines) || quote.Amount != "338.44" {
		t.Fatalf("quotation from template = %#v", quote)
	}
	wantValidUntil := quote.CreatedAt.AddDate(0, 0, template.ValidDays).Format("2006-01-02")
	if quote.ValidUntil != wantValidUntil {
		t.Fatalf("quotation validity = %s, want %s", quote.ValidUntil, wantValidUntil)
	}

	foreignTemplate := QuoteTemplate{ID: "template-b", BusinessID: "biz-b", PlanID: "plan-b", Name: "Foreign", ValidDays: 7, Lines: []SubscriptionLine{{Name: "Foreign plan", Quantity: 1, UnitPrice: "1.00", TaxRate: "0"}}, CreatedAt: time.Now().UTC()}
	api.Store.state.QuoteTemplates[foreignTemplate.ID] = foreignTemplate
	foreignRequest := httptest.NewRequest("POST", "/api/v1/quotations", strings.NewReader(`{"customerId":"customer-a","templateId":"template-b"}`))
	foreignResponse := httptest.NewRecorder()
	api.createQuotation(foreignResponse, foreignRequest, Authenticated{Actor: admin})
	if foreignResponse.Code != 404 {
		t.Fatalf("cross-business template status = %d, want 404", foreignResponse.Code)
	}
}

func TestCreateQuotationTemplateRejectsCrossBusinessProductsAndCustomerActor(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, Name: "Monthly", SalesPrice: "100.00", MinimumQuantity: 1}
	request := httptest.NewRequest("POST", "/api/v1/quotation-templates", strings.NewReader(`{"name":"Foreign item","planId":"plan-a","productId":"product-b","validDays":30}`))
	response := httptest.NewRecorder()
	api.createQuotationTemplate(response, request, Authenticated{Actor: admin})
	if response.Code != 404 {
		t.Fatalf("cross-business product status = %d, want 404", response.Code)
	}
	if len(api.Store.state.QuoteTemplates) != 0 {
		t.Fatal("cross-business product was stored in a quotation template")
	}

	customer := Actor{UserID: "customer-user", BusinessID: admin.BusinessID, CustomerID: "customer-a", Role: "customer"}
	customerRequest := httptest.NewRequest("POST", "/api/v1/quotation-templates", strings.NewReader(`{"name":"Customer template","planId":"plan-a","validDays":30}`))
	customerResponse := httptest.NewRecorder()
	api.createQuotationTemplate(customerResponse, customerRequest, Authenticated{Actor: customer})
	if customerResponse.Code != 403 {
		t.Fatalf("customer create status = %d, want 403", customerResponse.Code)
	}
}
