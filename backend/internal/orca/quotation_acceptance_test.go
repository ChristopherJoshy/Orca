package orca

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestCustomerCanAcceptOwnValidQuotationAndStaffCanConvertIt(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, Name: "Monthly", Interval: "monthly", SalesPrice: "1250.00", MinimumQuantity: 1}
	api.Store.state.Products["support-a"] = Product{ID: "support-a", BusinessID: admin.BusinessID, Name: "Support", SalesPrice: "80.00"}
	quote := Quotation{ID: "quote-a", BusinessID: admin.BusinessID, CustomerID: "customer-a", PlanID: "plan-a", Number: "QUO-001", Title: "Monthly with support", Status: "Sent", ValidUntil: time.Now().AddDate(0, 0, 3).Format("2006-01-02"), Amount: "1330.00", CreatedAt: time.Now().UTC(), Lines: []SubscriptionLine{{Name: "Monthly", Quantity: 1, UnitPrice: "1250.00", TaxRate: "0"}, {ProductID: "support-a", Name: "Support", Quantity: 1, UnitPrice: "80.00", TaxRate: "0"}}}
	api.Store.state.Quotations[quote.ID] = quote
	customer := Actor{UserID: "customer-user", BusinessID: admin.BusinessID, CustomerID: quote.CustomerID, Role: "customer"}
	request := httptest.NewRequest("POST", "/api/v1/quotations/quote-a/accept", nil)
	request = request.WithContext(quotationRequestContext(request, quote.ID))
	response := httptest.NewRecorder()
	api.acceptQuotation(response, request, Authenticated{Actor: customer})
	if response.Code != 200 || api.Store.state.Quotations[quote.ID].Status != "Accepted" {
		t.Fatalf("customer accept status=%d body=%s stored=%#v", response.Code, response.Body.String(), api.Store.state.Quotations[quote.ID])
	}

	conversion := httptest.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(`{"quotationId":"quote-a"}`))
	converted := httptest.NewRecorder()
	api.createSubscription(converted, conversion, Authenticated{Actor: admin})
	if converted.Code != 201 {
		t.Fatalf("subscription conversion status=%d body=%s", converted.Code, converted.Body.String())
	}
	var responseBody struct {
		ID     string `json:"id"`
		PlanID string `json:"planId"`
		Amount string `json:"amount"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(converted.Body.Bytes(), &responseBody); err != nil {
		t.Fatal(err)
	}
	if responseBody.PlanID != quote.PlanID || responseBody.Amount != "1330.00" || responseBody.Status != "Draft" {
		t.Fatalf("converted subscription response = %#v", responseBody)
	}
	if api.Store.state.Quotations[quote.ID].Status != "Converted" {
		t.Fatalf("quotation status after conversion = %s, want Converted", api.Store.state.Quotations[quote.ID].Status)
	}
}

func TestCustomerCannotAcceptExpiredOrForeignQuotation(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Quotations["expired"] = Quotation{ID: "expired", BusinessID: admin.BusinessID, CustomerID: "customer-a", Status: "Sent", ValidUntil: time.Now().AddDate(0, 0, -1).Format("2006-01-02")}
	api.Store.state.Quotations["other-customer"] = Quotation{ID: "other-customer", BusinessID: admin.BusinessID, CustomerID: "customer-other", Status: "Sent", ValidUntil: time.Now().AddDate(0, 0, 2).Format("2006-01-02")}
	customer := Actor{UserID: "customer-user", BusinessID: admin.BusinessID, CustomerID: "customer-a", Role: "customer"}
	for _, tc := range []struct {
		id   string
		want int
	}{{"expired", 409}, {"other-customer", 404}} {
		request := httptest.NewRequest("POST", "/api/v1/quotations/"+tc.id+"/accept", nil)
		request = request.WithContext(quotationRequestContext(request, tc.id))
		response := httptest.NewRecorder()
		api.acceptQuotation(response, request, Authenticated{Actor: customer})
		if response.Code != tc.want {
			t.Fatalf("accept quotation %s status=%d want=%d body=%s", tc.id, response.Code, tc.want, response.Body.String())
		}
	}
	if api.Store.state.Quotations["expired"].Status != "Sent" || api.Store.state.Quotations["other-customer"].Status != "Sent" {
		t.Fatal("rejected quotation was modified")
	}
}

func quotationRequestContext(request *http.Request, id string) context.Context {
	route := chi.NewRouteContext()
	route.URLParams.Add("id", id)
	return context.WithValue(request.Context(), chi.RouteCtxKey, route)
}
