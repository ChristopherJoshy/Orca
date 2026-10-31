package orca

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceAPIKeepsLocalDemoTenantsIsolated(t *testing.T) {
	store := &Store{state: NewState()}
	handler := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false, AllowedOrigin: "http://localhost:5173"})
	firstCookie, firstCSRF := signupForTest(t, handler, "one@example.test", "First Business")
	if items := listProductsForTest(t, handler, firstCookie); items == nil {
		t.Fatal("empty collection must be JSON [] rather than null")
	}
	createProductForTest(t, handler, firstCookie, firstCSRF, "First tenant product")
	secondCookie, secondCSRF := signupForTest(t, handler, "two@example.test", "Second Business")
	createProductForTest(t, handler, secondCookie, secondCSRF, "Second tenant product")
	firstItems := listProductsForTest(t, handler, firstCookie)
	secondItems := listProductsForTest(t, handler, secondCookie)
	if len(firstItems) != 1 || firstItems[0]["name"] != "First tenant product" {
		t.Fatalf("first tenant received unexpected records: %#v", firstItems)
	}
	if len(secondItems) != 1 || secondItems[0]["name"] != "Second tenant product" {
		t.Fatalf("second tenant received unexpected records: %#v", secondItems)
	}
}

func TestDiscountCreationPersistsValidityWindowAndRejectsReversedDates(t *testing.T) {
	store := &Store{state: NewState()}
	handler := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false, AllowedOrigin: "http://localhost:5173"})
	cookie, csrf := signupForTest(t, handler, "discount@example.test", "Discount Business")
	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	end := start.Add(24 * time.Hour)
	create := func(startsAt, endsAt time.Time) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]any{
			"name": "Launch offer", "kind": "percentage", "value": "10", "scope": "subscription",
			"minimumPurchase": "0.00", "minimumQuantity": 1, "maxUses": 25,
			"startsAt": startsAt.Format(time.RFC3339), "endsAt": endsAt.Format(time.RFC3339),
		})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts", bytes.NewReader(body))
		req.AddCookie(cookie)
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("X-CSRF-Token", csrf)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	created := create(start, end)
	if created.Code != http.StatusCreated {
		t.Fatalf("future discount returned %d: %s", created.Code, created.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["active"] != false {
		t.Fatalf("future discount should not be shown active: %#v", response)
	}
	if response["startsAt"] == nil || response["endsAt"] == nil {
		t.Fatalf("discount validity window was not returned: %#v", response)
	}
	invalid := create(end, start)
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("reversed discount window returned %d, want 422: %s", invalid.Code, invalid.Body.String())
	}
}

func signupForTest(t *testing.T, handler http.Handler, email, businessName string) (*http.Cookie, string) {
	t.Helper()
	body := map[string]string{"email": email, "password": "Password!2026", "businessName": businessName}
	encoded, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(encoded))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("signup returned %d: %s", rec.Code, rec.Body.String())
	}
	var value map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	csrf, ok := value["csrfToken"].(string)
	if !ok || csrf == "" {
		t.Fatalf("signup response has no CSRF token: %s", rec.Body.String())
	}
	cookie := rec.Result().Cookies()[0]
	if cookie.Name != "orca_session" || cookie.Value == "" {
		t.Fatalf("signup response has no session cookie: %#v", cookie)
	}
	return cookie, csrf
}
func createProductForTest(t *testing.T, handler http.Handler, cookie *http.Cookie, csrf, name string) {
	t.Helper()
	body := map[string]any{"name": name, "type": "Software", "salesPrice": "12.34", "costPrice": "2.00", "taxRate": "18", "variants": []any{}}
	encoded, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(encoded))
	req.AddCookie(cookie)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("X-CSRF-Token", csrf)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create product returned %d: %s", rec.Code, rec.Body.String())
	}
}
func listProductsForTest(t *testing.T, handler http.Handler, cookie *http.Cookie) []map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("list products returned %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Items
}

func TestCustomerRoleCannotReadBusinessWideReports(t *testing.T) {
	store := &Store{state: NewState()}
	now := time.Now().UTC()
	user := User{ID: "customer-user", Email: "customer@example.test", EmailNormalized: "customer@example.test", Verified: true, CreatedAt: now}
	business := Business{ID: "business", Name: "Example", Currency: CurrencyINR, Timezone: "UTC"}
	customer := Customer{ID: "customer", BusinessID: business.ID, UserID: user.ID, Name: "Customer", Email: user.Email, CreatedAt: now}
	store.state.Users[user.ID] = user
	store.state.Businesses[business.ID] = business
	store.state.Memberships[membershipKey(business.ID, user.ID)] = Membership{BusinessID: business.ID, UserID: user.ID, CustomerID: customer.ID, Role: "customer"}
	store.state.Customers[customer.ID] = customer
	raw, _ := randomToken(32)
	csrf, _ := randomToken(32)
	store.state.Sessions[tokenDigest(raw)] = Session{UserID: user.ID, BusinessID: business.ID, Role: "customer", CustomerID: customer.ID, CSRFToken: tokenDigest(csrf), ExpiresAt: now.Add(time.Hour)}
	handler := NewAPI(store, ServerConfig{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary", nil)
	req.AddCookie(&http.Cookie{Name: "orca_session", Value: raw})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("customer reports status = %d, want 403", rec.Code)
	}
}

func TestCustomerCanPauseAndResumeOwnedSubscriptionWhenPlanAllows(t *testing.T) {
	store := &Store{state: NewState()}
	now := time.Now().UTC()
	const businessID, userID, customerID, planID, subscriptionID = "business", "customer-user", "customer", "plan", "subscription"
	store.state.Users[userID] = User{ID: userID, Email: "customer@example.test", EmailNormalized: "customer@example.test", Verified: true, CreatedAt: now}
	store.state.Businesses[businessID] = Business{ID: businessID, Name: "Example", Currency: CurrencyINR, Timezone: "UTC"}
	store.state.Memberships[membershipKey(businessID, userID)] = Membership{BusinessID: businessID, UserID: userID, CustomerID: customerID, Role: "customer"}
	store.state.Customers[customerID] = Customer{ID: customerID, BusinessID: businessID, UserID: userID, Email: "customer@example.test", CreatedAt: now}
	store.state.Plans[planID] = Plan{ID: planID, BusinessID: businessID, AllowPause: true}
	store.state.Subscriptions[subscriptionID] = Subscription{ID: subscriptionID, BusinessID: businessID, CustomerID: customerID, PlanID: planID, Status: "Active", Version: 1}
	const sessionToken, csrfToken = "customer-self-service-session", "customer-self-service-csrf"
	store.state.Sessions[tokenDigest(sessionToken)] = Session{UserID: userID, BusinessID: businessID, Role: "customer", CustomerID: customerID, CSRFToken: tokenDigest(csrfToken), ExpiresAt: now.Add(time.Hour)}
	handler := NewAPI(store, ServerConfig{Env: "test", CookieSecure: false, AllowedOrigin: "http://localhost:5173"})
	transition := func(action string) *httptest.ResponseRecorder {
		t.Helper()
		version := store.state.Subscriptions[subscriptionID].Version
		body, err := json.Marshal(map[string]any{"action": action, "expectedVersion": version})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/"+subscriptionID+"/transition", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "orca_session", Value: sessionToken})
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("X-CSRF-Token", csrfToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	if paused := transition("pause"); paused.Code != http.StatusOK {
		t.Fatalf("customer pause returned %d: %s", paused.Code, paused.Body.String())
	}
	if got := store.state.Subscriptions[subscriptionID]; got.Status != "Paused" || len(got.PauseIntervals) != 1 || got.PauseIntervals[0].End != nil {
		t.Fatalf("pause interval was not opened: %+v", got)
	}
	if resumed := transition("resume"); resumed.Code != http.StatusOK {
		t.Fatalf("customer resume returned %d: %s", resumed.Code, resumed.Body.String())
	}
	if got := store.state.Subscriptions[subscriptionID]; got.Status != "Active" || got.PauseIntervals[0].End == nil {
		t.Fatalf("pause interval was not closed on resume: %+v", got)
	}
}

func TestCustomerSignupCreatesBusinessScopedPortalAccess(t *testing.T) {
	store := &Store{state: NewState()}
	now := time.Now().UTC()
	store.state.Businesses["business-a"] = Business{ID: "business-a", Name: "Example", Currency: CurrencyINR, Timezone: "Asia/Kolkata"}
	store.state.Customers["customer-a"] = Customer{ID: "customer-a", BusinessID: "business-a", Name: "Asha Customer", Email: "asha@example.test", CreatedAt: now}
	store.state.Customers["customer-b"] = Customer{ID: "customer-b", BusinessID: "business-a", Name: "Other Customer", Email: "other@example.test", CreatedAt: now}
	handler := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false})
	body, _ := json.Marshal(map[string]string{"email": "asha@example.test", "password": "Password!2026", "businessId": "business-a"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("customer signup returned %d: %s", rec.Code, rec.Body.String())
	}
	cookie := rec.Result().Cookies()[0]
	portalReq := httptest.NewRequest(http.MethodGet, "/api/v1/portal/me", nil)
	portalReq.AddCookie(cookie)
	portalRec := httptest.NewRecorder()
	handler.ServeHTTP(portalRec, portalReq)
	if portalRec.Code != http.StatusOK || !strings.Contains(portalRec.Body.String(), `"customer-a"`) {
		t.Fatalf("portal context did not return the linked customer: %d %s", portalRec.Code, portalRec.Body.String())
	}
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil)
	listReq.AddCookie(cookie)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	var response struct {
		Items []Customer `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if listRec.Code != http.StatusOK || len(response.Items) != 1 || response.Items[0].ID != "customer-a" {
		t.Fatalf("customer received records outside their ownership: %d %#v", listRec.Code, response.Items)
	}
	reportReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary", nil)
	reportReq.AddCookie(cookie)
	reportRec := httptest.NewRecorder()
	handler.ServeHTTP(reportRec, reportReq)
	if reportRec.Code != http.StatusForbidden {
		t.Fatalf("customer report status = %d, want 403", reportRec.Code)
	}
}

func TestPlanAvailabilityBoundsSubscriptionStartDate(t *testing.T) {
	store := &Store{state: NewState()}
	handler := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false})
	cookie, csrf := signupForTest(t, handler, "plan-admin@example.test", "Plan Business")
	post := func(path string, body map[string]any, want int) map[string]any {
		t.Helper()
		encoded, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
		req.AddCookie(cookie)
		req.Header.Set("X-CSRF-Token", csrf)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("POST %s returned %d, want %d: %s", path, rec.Code, want, rec.Body.String())
		}
		var value map[string]any
		if rec.Body.Len() > 0 {
			_ = json.Unmarshal(rec.Body.Bytes(), &value)
		}
		return value
	}
	createdCustomer := post("/api/v1/customers", map[string]any{"name": "Test Customer", "email": "customer-plan@example.test", "company": "Example"}, http.StatusCreated)
	customerID := createdCustomer["id"].(string)
	invalidPlan := map[string]any{"name": "Invalid dates", "interval": "monthly", "salesPrice": "10.00", "minimumQuantity": 1, "taxRate": "0", "availableFrom": "2027-12-01", "availableUntil": "2027-11-30"}
	post("/api/v1/plans", invalidPlan, http.StatusUnprocessableEntity)
	validPlan := map[string]any{"name": "Available plan", "interval": "monthly", "salesPrice": "10.00", "minimumQuantity": 1, "taxRate": "0", "availableFrom": "2026-12-01", "availableUntil": "2027-12-01"}
	plan := post("/api/v1/plans", validPlan, http.StatusCreated)
	post("/api/v1/subscriptions", map[string]any{"customerId": customerID, "planId": plan["id"], "startDate": "2026-11-30", "paymentTermsDays": 0}, http.StatusUnprocessableEntity)
	accepted := post("/api/v1/subscriptions", map[string]any{"customerId": customerID, "planId": plan["id"], "startDate": "2026-12-01", "paymentTermsDays": 0}, http.StatusCreated)
	if accepted["status"] != "Draft" {
		t.Fatalf("subscription inside the plan availability range has unexpected status: %#v", accepted)
	}
}
