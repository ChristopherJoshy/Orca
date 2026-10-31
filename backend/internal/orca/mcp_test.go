package orca

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPAuthorizationPKCEAndStreamableTools(t *testing.T) {
	_, store, handler, baseURL, sessionToken := newMCPTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	clientID, redirectURI := registerMCPTestClient(t, handler, baseURL)
	verifier := strings.Repeat("v", 43)
	challenge := mcpTestChallenge(verifier)
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {"state-from-client"},
		"resource":              {baseURL + "/mcp"},
		"scope":                 {mcpScopeRecords + " " + mcpScopeProposals},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	invalid := query.Clone()
	invalid.Set("redirect_uri", "https://attacker.example/steal")
	if got := requestMCPConsent(t, handler, baseURL, sessionToken, invalid); got != http.StatusBadRequest {
		t.Fatalf("unregistered redirect status = %d, want 400", got)
	}
	invalid = query.Clone()
	invalid.Del("code_challenge_method")
	if got := requestMCPConsent(t, handler, baseURL, sessionToken, invalid); got != http.StatusBadRequest {
		t.Fatalf("missing PKCE status = %d, want 400", got)
	}

	requestURL := baseURL + "/mcp/oauth/authorize?" + query.Encode()
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	getReq.AddCookie(&http.Cookie{Name: "orca_session", Value: sessionToken})
	page := serveMCPTestRequest(handler, getReq)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Connect Test Client to ORCA") || !strings.Contains(page.Body.String(), "Redirect hostname: <strong>client.example</strong>") {
		t.Fatalf("consent page response %d does not show client and redirect host: %s", page.Code, page.Body.String())
	}
	requestID := testConsentField(t, page.Body.String(), "request_id")
	nonce := testConsentField(t, page.Body.String(), "nonce")
	consentForm := url.Values{"request_id": {requestID}, "nonce": {nonce}, "decision": {"approve"}}
	postReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mcp/oauth/authorize", strings.NewReader(consentForm.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("Origin", baseURL)
	postReq.AddCookie(&http.Cookie{Name: "orca_session", Value: sessionToken})
	postResp := serveMCPTestRequest(handler, postReq)
	if postResp.Code != http.StatusFound {
		t.Fatalf("consent decision status = %d, want redirect", postResp.Code)
	}
	callback, err := url.Parse(postResp.Header().Get("Location"))
	if err != nil || callback.Query().Get("state") != "state-from-client" || callback.Query().Get("iss") != baseURL {
		t.Fatalf("callback has incorrect state or issuer: %q (%v)", postResp.Header().Get("Location"), err)
	}
	code := callback.Query().Get("code")
	if code == "" {
		t.Fatalf("authorization response has no code: %s", callback)
	}

	token := exchangeMCPTestCode(t, ctx, handler, baseURL, clientID, redirectURI, code, verifier)
	var grant OAuthGrant
	if err := store.Read(ctx, func(st *State) error {
		for _, v := range st.OAuthGrants {
			if v.TokenHash == tokenDigest(token) {
				grant = v
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if grant.ID == "" || grant.BusinessID != "biz-mcp-a" || grant.UserID != "user-mcp-a" || !slicesEqual(grant.Scopes, []string{mcpScopeRecords, mcpScopeProposals}) {
		t.Fatalf("issued grant was not persisted with expected binding and scopes: %#v", grant)
	}
	if !grant.ExpiresAt.After(time.Now()) || grant.TokenHash == token {
		t.Fatal("grant expiry or hashed-token storage is incorrect")
	}

	bearerClient := &http.Client{Transport: mcpBearerTransport{handler: handler, token: token}, Timeout: 5 * time.Second}
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "orca-test-client", Version: "1.0"}, nil)
	session, err := mcpClient.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: baseURL + "/mcp", HTTPClient: bearerClient, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	toolList, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(toolList.Tools) != 5 {
		t.Fatalf("MCP server advertises %d tools, want 5", len(toolList.Tools))
	}
	readResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "search_records", Arguments: map[string]any{"recordType": "customers", "query": "Mira", "limit": 10}})
	if err != nil || readResult.IsError {
		t.Fatalf("scoped search failed: result=%#v err=%v", readResult, err)
	}
	if !strings.Contains(fmt.Sprint(readResult.StructuredContent), "customer-mcp-a") {
		t.Fatalf("search did not return the caller's customer: %#v", readResult)
	}
	crossTenant, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "explain_invoice", Arguments: map[string]any{"invoiceId": "invoice-mcp-b"}})
	if err != nil || crossTenant == nil || !crossTenant.IsError {
		t.Fatalf("cross-business invoice should be hidden: result=%#v err=%v", crossTenant, err)
	}
	proposalResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "request_action_review", Arguments: map[string]any{"action": "confirm_invoice", "invoiceId": "invoice-mcp-a"}})
	if err != nil || proposalResult == nil || proposalResult.IsError {
		var detail string
		if proposalResult != nil && len(proposalResult.Content) > 0 {
			if content, ok := proposalResult.Content[0].(*mcp.TextContent); ok {
				detail = content.Text
			}
		}
		t.Fatalf("pending proposal call failed: result=%#v err=%v detail=%s", proposalResult, err, detail)
	}
	if err := store.Read(ctx, func(st *State) error {
		invoice := st.Invoices["invoice-mcp-a"]
		if invoice.Status != "Draft" {
			return fmt.Errorf("proposal mutated invoice to %s", invoice.Status)
		}
		found := false
		for _, approval := range st.Approvals {
			if approval.ResourceID == invoice.ID && approval.Status == "Pending" {
				found = true
			}
		}
		if !found {
			return errors.New("proposal did not create a pending approval")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	denied, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_report_summary", Arguments: map[string]any{}})
	if err == nil && (denied == nil || !denied.IsError) {
		t.Fatalf("report call without reports:read should be denied: result=%#v err=%v", denied, err)
	}

	// Client-side revocation is immediate; subsequent MCP calls fail at HTTP auth.
	revokeForm := url.Values{"token": {token}}
	revokeReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mcp/oauth/revoke", strings.NewReader(revokeForm.Encode()))
	revokeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	revokeResp := serveMCPTestRequest(handler, revokeReq)
	if revokeResp.Code != http.StatusOK {
		t.Fatalf("token revocation status = %d, want 200", revokeResp.Code)
	}
	_ = session.Close()
	revokedSession, err := mcp.NewClient(&mcp.Implementation{Name: "orca-revoked-test", Version: "1.0"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: baseURL + "/mcp", HTTPClient: bearerClient, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err == nil {
		_ = revokedSession.Close()
		t.Fatal("revoked bearer token was accepted by the MCP resource server")
	}
}

func TestMCPBearerRechecksExpiryMembershipAndTenantBinding(t *testing.T) {
	api, store, _, _, _ := newMCPTestServer(t)
	g := newMCPGateway(api)
	now := time.Now().UTC()
	grant := OAuthGrant{ID: "grant-mcp-expiry", BusinessID: "biz-mcp-a", UserID: "user-mcp-a", ClientID: "client", Scopes: []string{mcpScopeRecords}, TokenHash: tokenDigest("opaque-test-token"), ExpiresAt: now.Add(time.Minute)}
	if err := g.createGrant(context.Background(), grant, "https://client.example/callback"); err != nil {
		t.Fatal(err)
	}
	info, err := g.verifyBearerToken(context.Background(), "opaque-test-token", httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if err != nil || info.UserID != "user-mcp-a" || info.Extra["business_id"] != "biz-mcp-a" {
		t.Fatalf("valid grant did not resolve to bound actor: info=%#v err=%v", info, err)
	}
	if err := g.revokeGrant(context.Background(), grant.BusinessID, grant.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := g.verifyBearerToken(context.Background(), "opaque-test-token", nil); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("revoked bearer error = %v, want invalid token", err)
	}

	expired := OAuthGrant{ID: "grant-mcp-expired", BusinessID: "biz-mcp-a", UserID: "user-mcp-a", ClientID: "client", Scopes: []string{mcpScopeRecords}, TokenHash: tokenDigest("expired-token"), ExpiresAt: now.Add(-time.Second)}
	if err := g.createGrant(context.Background(), expired, "https://client.example/callback"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.verifyBearerToken(context.Background(), "expired-token", nil); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expired bearer error = %v, want invalid token", err)
	}

	if err := store.Update(context.Background(), func(st *State) error {
		m := st.Memberships[membershipKey("biz-mcp-a", "user-mcp-a")]
		revoked := time.Now().UTC()
		m.RevokedAt = &revoked
		st.Memberships[membershipKey(m.BusinessID, m.UserID)] = m
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	active := OAuthGrant{ID: "grant-mcp-membership", BusinessID: "biz-mcp-a", UserID: "user-mcp-a", ClientID: "client", Scopes: []string{mcpScopeRecords}, TokenHash: tokenDigest("membership-token"), ExpiresAt: now.Add(time.Hour)}
	// Restore membership to mint a grant, then revoke it and confirm existing grants are rejected.
	if err := store.Update(context.Background(), func(st *State) error {
		st.Memberships[membershipKey("biz-mcp-a", "user-mcp-a")] = Membership{BusinessID: "biz-mcp-a", UserID: "user-mcp-a", Role: "admin"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.createGrant(context.Background(), active, "https://client.example/callback"); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), func(st *State) error {
		m := st.Memberships[membershipKey("biz-mcp-a", "user-mcp-a")]
		revoked := time.Now().UTC()
		m.RevokedAt = &revoked
		st.Memberships[membershipKey(m.BusinessID, m.UserID)] = m
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.verifyBearerToken(context.Background(), "membership-token", nil); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("revoked membership token error = %v, want invalid token", err)
	}
}

func newMCPTestServer(t *testing.T) (*API, *Store, http.Handler, string, string) {
	t.Helper()
	baseURL := "http://localhost:8080"
	store := &Store{state: NewState()}
	now := time.Now().UTC()
	sessionToken := "mcp-test-session-secret"
	store.state.Businesses["biz-mcp-a"] = Business{ID: "biz-mcp-a", Name: "MCP Test Business", Currency: CurrencyINR, Timezone: "Asia/Kolkata"}
	store.state.Businesses["biz-mcp-b"] = Business{ID: "biz-mcp-b", Name: "Other MCP Business", Currency: CurrencyINR, Timezone: "UTC"}
	store.state.Users["user-mcp-a"] = User{ID: "user-mcp-a", Email: "mcp@example.test", Verified: true, CreatedAt: now}
	store.state.Memberships[membershipKey("biz-mcp-a", "user-mcp-a")] = Membership{BusinessID: "biz-mcp-a", UserID: "user-mcp-a", Role: "admin"}
	store.state.Sessions[tokenDigest(sessionToken)] = Session{UserID: "user-mcp-a", BusinessID: "biz-mcp-a", Role: "admin", ExpiresAt: now.Add(time.Hour)}
	store.state.Customers["customer-mcp-a"] = Customer{ID: "customer-mcp-a", BusinessID: "biz-mcp-a", Name: "Mira Test", Email: "mira@example.test"}
	store.state.Customers["customer-mcp-b"] = Customer{ID: "customer-mcp-b", BusinessID: "biz-mcp-b", Name: "Bina Test", Email: "bina@example.test"}
	store.state.Invoices["invoice-mcp-a"] = Invoice{ID: "invoice-mcp-a", BusinessID: "biz-mcp-a", CustomerID: "customer-mcp-a", Status: "Draft", Total: "10.00", Outstanding: "10.00", Version: 2}
	store.state.Invoices["invoice-mcp-b"] = Invoice{ID: "invoice-mcp-b", BusinessID: "biz-mcp-b", CustomerID: "customer-mcp-b", Status: "Confirmed", Total: "50.00", Outstanding: "50.00", Version: 7}
	api := &API{Store: store, Config: ServerConfig{Env: "test", CookieSecure: false, AllowedOrigin: baseURL, PublicURL: baseURL}, limits: map[string]rateWindow{}, agentCancels: map[string]context.CancelFunc{}}
	r := chi.NewRouter()
	r.Use(api.securityHeaders)
	api.RegisterMCPRoutes(r)
	return api, store, r, baseURL, sessionToken
}

func registerMCPTestClient(t *testing.T, handler http.Handler, baseURL string) (clientID, redirectURI string) {
	t.Helper()
	redirectURI = "https://client.example/callback"
	payload, _ := json.Marshal(map[string]any{"client_name": "Test Client", "redirect_uris": []string{redirectURI}, "grant_types": []string{"authorization_code"}, "response_types": []string{"code"}, "token_endpoint_auth_method": "none"})
	req := httptest.NewRequest(http.MethodPost, baseURL+"/mcp/register", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	response := serveMCPTestRequest(handler, req)
	var body struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusCreated || body.ClientID == "" {
		t.Fatalf("DCR status/body = %d/%#v", response.Code, body)
	}
	return body.ClientID, redirectURI
}

func requestMCPConsent(t *testing.T, handler http.Handler, baseURL, sessionToken string, query url.Values) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, baseURL+"/mcp/oauth/authorize?"+query.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: "orca_session", Value: sessionToken})
	return serveMCPTestRequest(handler, req).Code
}

func testConsentField(t *testing.T, html, field string) string {
	t.Helper()
	pattern := regexp.MustCompile(`name="` + regexp.QuoteMeta(field) + `" value="([^"]+)"`)
	matches := pattern.FindStringSubmatch(html)
	if len(matches) != 2 {
		t.Fatalf("consent form missing %s field", field)
	}
	return matches[1]
}

func exchangeMCPTestCode(t *testing.T, ctx context.Context, handler http.Handler, baseURL, clientID, redirectURI, code, verifier string) string {
	t.Helper()
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"code":          {code},
		"code_verifier": {verifier},
		"resource":      {baseURL + "/mcp"},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mcp/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := serveMCPTestRequest(handler, req)
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if resp.Code != http.StatusOK || body.AccessToken == "" {
		t.Fatalf("token exchange status/body = %d/%#v", resp.Code, body)
	}
	return body.AccessToken
}

func mcpTestChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

type mcpBearerTransport struct {
	handler http.Handler
	token   string
}

func (t mcpBearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := httptest.NewRequest(req.Method, req.URL.String(), req.Body)
	clone.Header = req.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+t.token)
	clone.RemoteAddr = "127.0.0.1:1234"
	recorder := serveMCPTestRequest(t.handler, clone)
	return recorder.Result(), nil
}

func serveMCPTestRequest(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	if req.RemoteAddr == "" {
		req.RemoteAddr = "127.0.0.1:1234"
	}
	handler.ServeHTTP(recorder, req)
	return recorder
}
