package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresHTTPBillingFlow(t *testing.T) {
	dsn := os.Getenv("ORCA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ORCA_TEST_DATABASE_URL to run PostgreSQL HTTP billing integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schemaID, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "orca_http_test_" + strings.ReplaceAll(schemaID, "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := &Store{pool: pool}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	handler := NewAPI(store, ServerConfig{Env: "development", CookieSecure: false, AllowedOrigin: "http://localhost:5173"})
	counter := 0
	call := func(method, path string, body any, cookie *http.Cookie, csrf string, want int) map[string]any {
		t.Helper()
		var reader *bytes.Reader
		if body == nil {
			reader = bytes.NewReader(nil)
		} else {
			data, e := json.Marshal(body)
			if e != nil {
				t.Fatal(e)
			}
			reader = bytes.NewReader(data)
		}
		req := httptest.NewRequest(method, path, reader)
		req.RemoteAddr = fmt.Sprintf("127.0.0.1:%d", 9000+counter)
		counter++
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
			req.Header.Set("Origin", "http://localhost:5173")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s returned %d, want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		var response map[string]any
		if rec.Body.Len() > 0 {
			if e := json.Unmarshal(rec.Body.Bytes(), &response); e != nil {
				t.Fatalf("decode response: %v (%s)", e, rec.Body.String())
			}
		}
		return response
	}
	encoded, _ := json.Marshal(map[string]string{"email": "api@example.test", "password": "Password!2026", "businessName": "API Flow"})
	signupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(encoded))
	signupReq.RemoteAddr = "127.0.0.1:9100"
	signupRec := httptest.NewRecorder()
	handler.ServeHTTP(signupRec, signupReq)
	if signupRec.Code != 201 {
		t.Fatalf("signup failed: %d %s", signupRec.Code, signupRec.Body.String())
	}
	var signup map[string]any
	if err := json.Unmarshal(signupRec.Body.Bytes(), &signup); err != nil {
		t.Fatal(err)
	}
	cookies := signupRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("signup did not set a session cookie")
	}
	cookie := cookies[0]
	csrf := signup["csrfToken"].(string)
	product := call(http.MethodPost, "/api/v1/products", map[string]any{"name": "Service", "type": "Software", "salesPrice": "100.00", "costPrice": "10.00", "currency": "INR", "taxRate": "18", "variants": []any{}}, cookie, csrf, 201)
	customer := call(http.MethodPost, "/api/v1/customers", map[string]any{"name": "Test Customer", "email": "customer@example.test", "company": "Example", "phone": ""}, cookie, csrf, 201)
	plan := call(http.MethodPost, "/api/v1/plans", map[string]any{"name": "Monthly", "interval": "monthly", "salesPrice": "100.00", "minimumQuantity": 1, "taxRate": "18", "allowPause": true, "allowEarlyClose": true, "allowRenew": true}, cookie, csrf, 201)
	productID := product["id"].(string)
	customerID := customer["id"].(string)
	planID := plan["id"].(string)
	date := time.Now().In(time.FixedZone("IST", 19800)).Format("2006-01-02")
	sub := call(http.MethodPost, "/api/v1/subscriptions", map[string]any{"customerId": customerID, "planId": planID, "startDate": date, "paymentTermsDays": 14, "lines": []any{map[string]any{"productId": productID, "name": "Service", "quantity": 1, "unitPrice": "100.00", "taxRate": "18"}}}, cookie, csrf, 201)
	planOnly := call(http.MethodPost, "/api/v1/subscriptions", map[string]any{"customerId": customerID, "planId": planID, "startDate": date, "paymentTermsDays": 14}, cookie, csrf, 201)
	if planOnly["status"] != "Draft" || len(planOnly["lines"].([]any)) != 1 {
		t.Fatalf("plan-only subscription did not receive a valid default line: %#v", planOnly)
	}
	subID := sub["id"].(string)
	call(http.MethodPost, "/api/v1/subscriptions/"+subID+"/transition", map[string]any{"action": "request_quote"}, cookie, csrf, 200)
	call(http.MethodPost, "/api/v1/subscriptions/"+subID+"/transition", map[string]any{"action": "confirm"}, cookie, csrf, 200)
	call(http.MethodPost, "/api/v1/subscriptions/"+subID+"/transition", map[string]any{"action": "activate"}, cookie, csrf, 200)
	run := call(http.MethodPost, "/api/v1/billing/run", map[string]any{}, cookie, csrf, 200)
	if run["created"].(float64) < 1 {
		t.Fatalf("billing run did not create an invoice: %#v", run)
	}
	list := call(http.MethodGet, "/api/v1/invoices", nil, cookie, "", 200)
	items := list["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("invoice count=%d, want 1", len(items))
	}
	invoice := items[0].(map[string]any)
	invoiceID := invoice["id"].(string)
	total := invoice["total"].(string)
	call(http.MethodPost, "/api/v1/invoices/"+invoiceID+"/confirm", map[string]any{}, cookie, csrf, 200)
	paymentBody := map[string]any{"amount": total, "method": "UPI", "date": date, "operationKey": "http-smoke-payment-1"}
	payment := call(http.MethodPost, "/api/v1/invoices/"+invoiceID+"/payments", paymentBody, cookie, csrf, 201)
	replayed := call(http.MethodPost, "/api/v1/invoices/"+invoiceID+"/payments", paymentBody, cookie, csrf, 201)
	if replayed["id"] != payment["id"] {
		t.Fatalf("idempotent payment retry returned a different record: first=%#v replay=%#v", payment, replayed)
	}
	conflictingReplay := map[string]any{"amount": "1.00", "method": "UPI", "date": date, "operationKey": "http-smoke-payment-1"}
	call(http.MethodPost, "/api/v1/invoices/"+invoiceID+"/payments", conflictingReplay, cookie, csrf, 409)
	final := call(http.MethodGet, "/api/v1/invoices/"+invoiceID, nil, cookie, "", 200)
	if final["status"] != "Paid" || final["outstanding"] != "0.00" {
		t.Fatalf("invoice after full payment = %#v", final)
	}
	report := call(http.MethodGet, "/api/v1/reports/summary", nil, cookie, "", 200)
	if report["activeSubscriptions"].(float64) != 1 {
		t.Fatalf("summary did not include active subscription: %#v", report)
	}
	authn, err := store.ResolveSession(ctx, cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	proposalAPI := &API{Store: store}
	approvalID, _, err := proposalAPI.createAgentApproval(ctx, authn.Actor, actionReviewArguments{Action: "close_subscription", SubscriptionID: subID})
	if err != nil {
		t.Fatal(err)
	}
	approved := call(http.MethodPost, "/api/v1/agent/approvals/"+approvalID+"/decision", map[string]any{"decision": "approve"}, cookie, csrf, 200)
	execution, ok := approved["executionResult"].(map[string]any)
	if !ok || execution["executed"] != true || execution["status"] != "Closed" {
		t.Fatalf("approved action execution result did not round-trip through PostgreSQL: %#v", approved)
	}
	var storedApproval Approval
	if err := store.ReadFor(ctx, authn.Actor.BusinessID, func(state *State) error {
		storedApproval = state.Approvals[approvalID]
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if storedApproval.Status != "Approved" || storedApproval.ExecutionResult["status"] != "Closed" {
		t.Fatalf("approval execution result was not persisted: %#v", storedApproval)
	}

	gateway := newMCPGateway(&API{Store: store})
	grantToken := "postgres-mcp-test-token"
	grantID, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	grant := OAuthGrant{ID: grantID, BusinessID: authn.Actor.BusinessID, UserID: authn.Actor.UserID, ClientID: "test-client", Scopes: []string{mcpScopeRecords}, TokenHash: tokenDigest(grantToken), ExpiresAt: time.Now().UTC().Add(time.Minute)}
	if err := gateway.createGrant(ctx, grant, "http://127.0.0.1:34567/callback"); err != nil {
		t.Fatal(err)
	}
	loadedGrant, loadedActor, err := gateway.lookupGrantByToken(ctx, grantToken)
	if err != nil || loadedGrant.ID != grant.ID || loadedActor.BusinessID != authn.Actor.BusinessID {
		t.Fatalf("MCP grant did not load for its authorized tenant: grant=%#v actor=%#v err=%v", loadedGrant, loadedActor, err)
	}
	if err := gateway.revokeGrant(ctx, authn.Actor.BusinessID, grant.ID, &authn.Actor); err != nil {
		t.Fatal(err)
	}
	if _, _, err := gateway.lookupGrantByToken(ctx, grantToken); err == nil {
		t.Fatal("revoked MCP grant still authorized its bearer token")
	}
}
