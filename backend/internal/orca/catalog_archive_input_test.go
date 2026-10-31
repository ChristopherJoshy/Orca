package orca

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogCreationCannotSetArchivedState(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	productRequest := httptest.NewRequest("POST", "/api/v1/products", strings.NewReader(`{"name":"Support","type":"Add-on","salesPrice":"20.00","costPrice":"1.00","taxRate":"18","archived":true}`))
	productResponse := httptest.NewRecorder()
	api.createProduct(productResponse, productRequest, Authenticated{Actor: admin})
	if productResponse.Code != 201 {
		t.Fatalf("create product returned %d: %s", productResponse.Code, productResponse.Body.String())
	}
	for _, product := range api.Store.state.Products {
		if product.Archived {
			t.Fatal("client create payload set a product as archived")
		}
	}

	planRequest := httptest.NewRequest("POST", "/api/v1/plans", strings.NewReader(`{"name":"Monthly","interval":"monthly","salesPrice":"100.00","minimumQuantity":1,"taxRate":"18","archived":true}`))
	planResponse := httptest.NewRecorder()
	api.createPlan(planResponse, planRequest, Authenticated{Actor: admin})
	if planResponse.Code != 201 {
		t.Fatalf("create plan returned %d: %s", planResponse.Code, planResponse.Body.String())
	}
	for _, plan := range api.Store.state.Plans {
		if plan.Archived {
			t.Fatal("client create payload set a plan as archived")
		}
	}
}

func TestOnlyAdminsCanCreateCatalogItems(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	staff := Authenticated{Actor: Actor{UserID: "staff-user", BusinessID: admin.BusinessID, Role: "staff"}}
	productRequest := httptest.NewRequest("POST", "/api/v1/products", strings.NewReader(`{"name":"Support","type":"Add-on","salesPrice":"20.00","costPrice":"1.00","taxRate":"18"}`))
	productResponse := httptest.NewRecorder()
	api.createProduct(productResponse, productRequest, staff)
	if productResponse.Code != 403 {
		t.Fatalf("staff product creation status = %d, want 403", productResponse.Code)
	}
	planRequest := httptest.NewRequest("POST", "/api/v1/plans", strings.NewReader(`{"name":"Monthly","interval":"monthly","salesPrice":"100.00","minimumQuantity":1,"taxRate":"18"}`))
	planResponse := httptest.NewRecorder()
	api.createPlan(planResponse, planRequest, staff)
	if planResponse.Code != 403 {
		t.Fatalf("staff plan creation status = %d, want 403", planResponse.Code)
	}
}
