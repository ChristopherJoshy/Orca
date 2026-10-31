package orca

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateProductUsesVersionAndAuditsCatalogTerms(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Products["product-a"] = Product{ID: "product-a", BusinessID: admin.BusinessID, Name: "Old", Type: "Software", SalesPrice: "100.00", CostPrice: "20.00", TaxRate: "18.00", Version: 4}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/products/product-a", strings.NewReader(`{"expectedVersion":4,"name":"Updated","type":"Add-on","salesPrice":"250.00","costPrice":"70.00","taxRate":"5.00","notes":"support","variants":[{"name":"Brand","value":"Odoo","adjustment":"560.00"}]}`))
	request = request.WithContext(quotationRequestContext(request, "product-a"))
	response := httptest.NewRecorder()
	api.updateProduct(response, request, Authenticated{Actor: admin})
	if response.Code != http.StatusOK {
		t.Fatalf("update product returned %d: %s", response.Code, response.Body.String())
	}
	updated := api.Store.state.Products["product-a"]
	if updated.Name != "Updated" || updated.SalesPrice != "250.00" || updated.Variants[0].Adjustment != "560.00" || updated.Version != 5 {
		t.Fatalf("updated product terms/version = %+v", updated)
	}
	if len(api.Store.state.Audit) != 1 || api.Store.state.Audit[0].Action != "product.updated" || api.Store.state.Audit[0].ResourceID != "product-a" {
		t.Fatalf("product update was not audited: %+v", api.Store.state.Audit)
	}

	stale := httptest.NewRequest(http.MethodPut, "/api/v1/products/product-a", strings.NewReader(`{"expectedVersion":4,"name":"Stale","type":"Software","salesPrice":"1.00","costPrice":"0.00","taxRate":"0"}`))
	stale = stale.WithContext(quotationRequestContext(stale, "product-a"))
	staleResponse := httptest.NewRecorder()
	api.updateProduct(staleResponse, stale, Authenticated{Actor: admin})
	if staleResponse.Code != http.StatusConflict || api.Store.state.Products["product-a"].Name != "Updated" {
		t.Fatalf("stale update changed catalog: status=%d product=%+v", staleResponse.Code, api.Store.state.Products["product-a"])
	}
}

func TestUpdatePlanValidatesAvailabilityAndPersistsPolicy(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, Name: "Monthly", Interval: "monthly", SalesPrice: "100.00", MinimumQuantity: 1, TaxRate: "18.00", Version: 2}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/plans/plan-a", strings.NewReader(`{"expectedVersion":2,"name":"Annual","interval":"yearly","salesPrice":"900.00","minimumQuantity":3,"availableFrom":"2026-11-01","availableUntil":"2027-11-01","taxRate":"5.50","autoClose":true,"allowEarlyClose":false,"allowPause":true,"allowRenew":true}`))
	request = request.WithContext(quotationRequestContext(request, "plan-a"))
	response := httptest.NewRecorder()
	api.updatePlan(response, request, Authenticated{Actor: admin})
	if response.Code != http.StatusOK {
		t.Fatalf("update plan returned %d: %s", response.Code, response.Body.String())
	}
	updated := api.Store.state.Plans["plan-a"]
	if updated.Interval != "yearly" || updated.SalesPrice != "900.00" || updated.MinimumQuantity != 3 || updated.AllowEarlyClose || !updated.AutoClose || updated.Version != 3 {
		t.Fatalf("updated plan terms/version = %+v", updated)
	}

	invalid := httptest.NewRequest(http.MethodPut, "/api/v1/plans/plan-a", strings.NewReader(`{"expectedVersion":3,"name":"Bad dates","interval":"monthly","salesPrice":"10.00","minimumQuantity":1,"availableFrom":"2027-12-01","availableUntil":"2027-11-30","taxRate":"0"}`))
	invalid = invalid.WithContext(quotationRequestContext(invalid, "plan-a"))
	invalidResponse := httptest.NewRecorder()
	api.updatePlan(invalidResponse, invalid, Authenticated{Actor: admin})
	if invalidResponse.Code != http.StatusUnprocessableEntity || api.Store.state.Plans["plan-a"].Name != "Annual" {
		t.Fatalf("invalid date range was persisted: status=%d plan=%+v", invalidResponse.Code, api.Store.state.Plans["plan-a"])
	}
}

func TestArchiveHidesCatalogRecordsButPreservesReferencesAndAudit(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Products["product-a"] = Product{ID: "product-a", BusinessID: admin.BusinessID, Name: "Support", Type: "Add-on", SalesPrice: "10.00", CostPrice: "2.00", TaxRate: "18.00", Version: 1}
	api.Store.state.Plans["plan-a"] = Plan{ID: "plan-a", BusinessID: admin.BusinessID, Name: "Monthly", Interval: "monthly", SalesPrice: "100.00", MinimumQuantity: 1, TaxRate: "0.00", Version: 1}
	api.Store.state.Subscriptions["sub-a"] = Subscription{ID: "sub-a", BusinessID: admin.BusinessID, CustomerID: "customer-a", PlanID: "plan-a", Status: "Active", Version: 1}

	productRequest := httptest.NewRequest(http.MethodPost, "/api/v1/products/product-a/archive", strings.NewReader(`{"expectedVersion":1}`))
	productRequest = productRequest.WithContext(quotationRequestContext(productRequest, "product-a"))
	productResponse := httptest.NewRecorder()
	api.archiveProduct(productResponse, productRequest, Authenticated{Actor: admin})
	if productResponse.Code != http.StatusOK || !api.Store.state.Products["product-a"].Archived {
		t.Fatalf("product archive failed: status=%d body=%s", productResponse.Code, productResponse.Body.String())
	}

	planRequest := httptest.NewRequest(http.MethodPost, "/api/v1/plans/plan-a/archive", strings.NewReader(`{"expectedVersion":1}`))
	planRequest = planRequest.WithContext(quotationRequestContext(planRequest, "plan-a"))
	planResponse := httptest.NewRecorder()
	api.archivePlan(planResponse, planRequest, Authenticated{Actor: admin})
	if planResponse.Code != http.StatusOK || !api.Store.state.Plans["plan-a"].Archived {
		t.Fatalf("plan archive failed: status=%d body=%s", planResponse.Code, planResponse.Body.String())
	}
	if _, ok := api.Store.state.Subscriptions["sub-a"]; !ok {
		t.Fatal("archiving a plan deleted a customer's existing subscription")
	}

	products := collectionItems(api.Store.state, "products", admin)
	plans := collectionItems(api.Store.state, "plans", admin)
	if len(products) != 0 || len(plans) != 0 {
		t.Fatalf("archived resources remain selectable: products=%d plans=%d", len(products), len(plans))
	}
	if len(api.Store.state.Audit) != 2 || api.Store.state.Audit[0].Action != "product.archived" || api.Store.state.Audit[1].Action != "plan.archived" {
		t.Fatalf("archive actions were not audited: %+v", api.Store.state.Audit)
	}

	// A retry stays successful after the first request has incremented the row version.
	retry := httptest.NewRequest(http.MethodPost, "/api/v1/plans/plan-a/archive", strings.NewReader(`{"expectedVersion":1}`))
	retry = retry.WithContext(quotationRequestContext(retry, "plan-a"))
	retryResponse := httptest.NewRecorder()
	api.archivePlan(retryResponse, retry, Authenticated{Actor: admin})
	if retryResponse.Code != http.StatusOK || len(api.Store.state.Audit) != 2 {
		t.Fatalf("archive retry was not idempotent: status=%d audit=%d", retryResponse.Code, len(api.Store.state.Audit))
	}
}

func TestCatalogMutationsRequireAdminAndHideForeignIDs(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	api.Store.state.Products["product-b"] = Product{ID: "product-b", BusinessID: "biz-b", Name: "B product", Type: "Software", SalesPrice: "10.00", CostPrice: "1.00", TaxRate: "0.00", Version: 1}
	staff := admin
	staff.Role = "staff"
	request := httptest.NewRequest(http.MethodPost, "/api/v1/products/product-a/archive", strings.NewReader(`{"expectedVersion":1}`))
	request = request.WithContext(quotationRequestContext(request, "product-a"))
	response := httptest.NewRecorder()
	api.archiveProduct(response, request, Authenticated{Actor: staff})
	if response.Code != http.StatusForbidden {
		t.Fatalf("staff archive status=%d, want 403", response.Code)
	}

	foreign := httptest.NewRequest(http.MethodPut, "/api/v1/products/product-b", strings.NewReader(`{"expectedVersion":1,"name":"Changed","type":"Software","salesPrice":"11.00","costPrice":"1.00","taxRate":"0"}`))
	foreign = foreign.WithContext(quotationRequestContext(foreign, "product-b"))
	foreignResponse := httptest.NewRecorder()
	api.updateProduct(foreignResponse, foreign, Authenticated{Actor: admin})
	if foreignResponse.Code != http.StatusNotFound || api.Store.state.Products["product-b"].Name != "B product" {
		t.Fatalf("foreign product was visible or modified: status=%d product=%+v", foreignResponse.Code, api.Store.state.Products["product-b"])
	}
}
