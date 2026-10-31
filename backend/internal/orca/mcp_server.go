package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpScopeRecords   = "records:read"
	mcpScopeReports   = "reports:read"
	mcpScopeSimulate  = "billing:simulate"
	mcpScopeProposals = "actions:propose"
)

var mcpScopes = []string{mcpScopeRecords, mcpScopeReports, mcpScopeSimulate, mcpScopeProposals}

type mcpGateway struct {
	api       *API
	clientsMu sync.Mutex
	clients   map[string]mcpClient
	flowMu    sync.Mutex
	pending   map[string]mcpPendingAuthorization
	codes     map[string]mcpAuthorizationCode
	protocol  http.Handler
}

type mcpClient struct {
	ID           string   `json:"client_id"`
	Name         string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type mcpPendingAuthorization struct {
	ClientID      string
	ClientName    string
	RedirectURI   string
	CodeChallenge string
	State         string
	Scopes        []string
	Resource      string
	Actor         Actor
	Nonce         string
	ExpiresAt     time.Time
}

type mcpAuthorizationCode struct {
	ClientID      string
	RedirectURI   string
	CodeChallenge string
	State         string
	Scopes        []string
	Resource      string
	Actor         Actor
	ExpiresAt     time.Time
}

type mcpActionReviewInput struct {
	Action         string `json:"action" jsonschema:"supported review action: confirm_invoice, cancel_invoice, send_invoice, record_payment, or close_subscription"`
	InvoiceID      string `json:"invoiceId,omitempty" jsonschema:"invoice ID for invoice actions"`
	SubscriptionID string `json:"subscriptionId,omitempty" jsonschema:"subscription ID for close_subscription"`
	Amount         string `json:"amount,omitempty" jsonschema:"decimal string amount for record_payment"`
	Method         string `json:"method,omitempty" jsonschema:"payment method for record_payment"`
	Date           string `json:"date,omitempty" jsonschema:"payment date for record_payment"`
	Note           string `json:"note,omitempty" jsonschema:"payment note for record_payment"`
	OperationKey   string `json:"operationKey,omitempty" jsonschema:"stable idempotency key for record_payment"`
}

func newMCPGateway(api *API) *mcpGateway {
	g := &mcpGateway{
		api:     api,
		clients: map[string]mcpClient{},
		pending: map[string]mcpPendingAuthorization{},
		codes:   map[string]mcpAuthorizationCode{},
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "orca", Version: "1.0.0"}, nil)
	g.addTools(server)
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		JSONResponse:                 true,
		Stateless:                    true,
		MaxRequestBodyBytes:          1 << 20,
		PropagateRequestCancellation: true,
	})
	g.protocol = g.protectProtocol(streamable)
	return g
}

// RegisterMCPRoutes installs the OAuth discovery, consent, token, revocation,
// and Streamable HTTP endpoints. Call it once while constructing the API router.
func (a *API) RegisterMCPRoutes(r chi.Router) {
	g := newMCPGateway(a)
	r.Get("/.well-known/oauth-protected-resource/mcp", g.protectedResourceMetadata)
	r.Get("/.well-known/oauth-authorization-server", g.authorizationServerMetadata)
	r.Post("/mcp/register", g.registerClient)
	r.Get("/mcp/oauth/authorize", a.protected(false, g.authorizeGet))
	r.Post("/mcp/oauth/authorize", a.protected(false, g.authorizePost))
	r.Post("/mcp/oauth/token", g.token)
	r.Post("/mcp/oauth/revoke", g.revokeToken)
	r.Handle("/mcp", g.protocol)
	r.Get("/api/v1/mcp/grants", a.protected(false, g.listGrants))
	r.Delete("/api/v1/mcp/grants/{id}", a.protected(true, g.revokeGrantForUser))
}

func (g *mcpGateway) addTools(server *mcp.Server) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPointer(false)}
	proposal := &mcp.ToolAnnotations{ReadOnlyHint: false, IdempotentHint: true, DestructiveHint: boolPointer(false), OpenWorldHint: boolPointer(false)}
	mcp.AddTool(server, &mcp.Tool{Name: "search_records", Description: "Search ORCA customers, products, plans, quotations, subscriptions, invoices, or payments within the connected user's business and record permissions.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		RecordType string `json:"recordType" jsonschema:"record collection to search: customers, products, plans, quotations, subscriptions, invoices, or payments"`
		Query      string `json:"query,omitempty" jsonschema:"case-insensitive text filter, up to 120 characters"`
		Limit      int    `json:"limit,omitempty" jsonschema:"maximum results from 1 to 20; defaults to 8"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := g.runTool(ctx, mcpScopeRecords, "search_records", in)
		return nil, out, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "explain_invoice", Description: "Read the stored invoice calculation snapshot and hash evidence. This does not recalculate or change the invoice.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		InvoiceID string `json:"invoiceId" jsonschema:"ORCA invoice ID"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := g.runTool(ctx, mcpScopeRecords, "explain_invoice", in)
		return nil, out, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_report_summary", Description: "Read the business billing summary and measured integrity indicators.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		out, err := g.runTool(ctx, mcpScopeReports, "get_report_summary", struct{}{})
		return nil, out, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "preview_next_invoice", Description: "Simulate the next invoice for a subscription without writing financial records.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		SubscriptionID string `json:"subscriptionId" jsonschema:"ORCA subscription ID"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := g.runTool(ctx, mcpScopeSimulate, "preview_next_invoice", in)
		return nil, out, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "request_action_review", Description: "Create a pending ORCA approval proposal for a supported billing action. This never performs the business action; a staff member must review it in ORCA.", Annotations: proposal}, func(ctx context.Context, _ *mcp.CallToolRequest, in mcpActionReviewInput) (*mcp.CallToolResult, any, error) {
		out, err := g.runTool(ctx, mcpScopeProposals, "request_action_review", in)
		return nil, out, err
	})
}

func boolPointer(value bool) *bool { return &value }

func (g *mcpGateway) protectProtocol(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scopes, err := mcpRequiredScopes(r, w)
		if err != nil {
			return
		}
		protected := auth.RequireBearerToken(g.verifyBearerToken, &auth.RequireBearerTokenOptions{
			ResourceMetadataURL: g.publicBase() + "/.well-known/oauth-protected-resource/mcp",
			Scopes:              scopes,
		})
		protected(next).ServeHTTP(w, r)
	})
}

// mcpRequiredScopes peeks at tool names only. It leaves argument parsing and
// validation to the SDK, but lets OAuth middleware return a 403 scope challenge
// before any tool handler executes.
func mcpRequiredScopes(r *http.Request, w http.ResponseWriter) ([]string, error) {
	if r.Method != http.MethodPost || r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "MCP request body exceeds 1 MiB", http.StatusRequestEntityTooLarge)
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var messages []json.RawMessage
	if len(body) > 0 && body[0] == '[' {
		if err := json.Unmarshal(body, &messages); err != nil {
			return nil, nil
		}
	} else if len(body) > 0 {
		messages = []json.RawMessage{body}
	}
	var scopes []string
	for _, message := range messages {
		var call struct {
			Method string `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if json.Unmarshal(message, &call) != nil || call.Method != "tools/call" {
			continue
		}
		var scope string
		switch call.Params.Name {
		case "search_records", "explain_invoice":
			scope = mcpScopeRecords
		case "get_report_summary":
			scope = mcpScopeReports
		case "preview_next_invoice":
			scope = mcpScopeSimulate
		case "request_action_review":
			scope = mcpScopeProposals
		}
		if scope != "" && !slices.Contains(scopes, scope) {
			scopes = append(scopes, scope)
		}
	}
	return scopes, nil
}

func (g *mcpGateway) runTool(ctx context.Context, requiredScope, name string, input any) (any, error) {
	info := auth.TokenInfoFromContext(ctx)
	if info == nil || !slices.Contains(info.Scopes, requiredScope) {
		return nil, errors.New("authorization scope is required")
	}
	actor, ok := actorFromMCPToken(info)
	if !ok {
		return nil, errors.New("authorization required")
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > 8192 {
		return nil, errors.New("tool input is invalid or too large")
	}
	result, approvalID, err := g.api.executeAgentTool(ctx, actor, name, encoded)
	if err != nil {
		return nil, mcpToolError(err)
	}
	if approvalID != "" {
		return map[string]any{"approvalId": approvalID, "status": "Pending", "executed": false, "result": result}, nil
	}
	return result, nil
}

func actorFromMCPToken(info *auth.TokenInfo) (Actor, bool) {
	if info == nil || info.Extra == nil {
		return Actor{}, false
	}
	str := func(key string) string {
		v, _ := info.Extra[key].(string)
		return v
	}
	actor := Actor{UserID: info.UserID, BusinessID: str("business_id"), CustomerID: str("customer_id"), Role: str("role")}
	return actor, actor.UserID != "" && actor.BusinessID != "" && actor.Role != ""
}

func (g *mcpGateway) verifyBearerToken(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	if token == "" || len(token) > 256 {
		return nil, auth.ErrInvalidToken
	}
	grant, actor, err := g.lookupGrantByToken(ctx, token)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFound) {
			return nil, auth.ErrInvalidToken
		}
		return nil, err
	}
	return &auth.TokenInfo{
		Scopes: grant.Scopes, Expiration: grant.ExpiresAt, UserID: actor.UserID,
		Extra: map[string]any{"business_id": actor.BusinessID, "customer_id": actor.CustomerID, "role": actor.Role, "grant_id": grant.ID},
	}, nil
}

func (g *mcpGateway) publicBase() string {
	return strings.TrimRight(strings.TrimSpace(g.api.Config.PublicURL), "/")
}

func (g *mcpGateway) resourceURL() string { return g.publicBase() + "/mcp" }

func (g *mcpGateway) requireConfiguredPublicURL(w http.ResponseWriter, r *http.Request) bool {
	base := g.publicBase()
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(g.api.Config.Env != "production" && isLoopbackURL(u))) {
		http.Error(w, "MCP public HTTPS URL is not configured", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (g *mcpGateway) protectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !g.requireConfiguredPublicURL(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"resource": g.resourceURL(), "authorization_servers": []string{g.publicBase()},
		"scopes_supported": mcpScopes, "bearer_methods_supported": []string{"header"}, "resource_name": "ORCA MCP",
	})
}

func (g *mcpGateway) authorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !g.requireConfiguredPublicURL(w, r) {
		return
	}
	base := g.publicBase()
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         base,
		"authorization_endpoint":                         base + "/mcp/oauth/authorize",
		"token_endpoint":                                 base + "/mcp/oauth/token",
		"revocation_endpoint":                            base + "/mcp/oauth/revoke",
		"registration_endpoint":                          base + "/mcp/register",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"scopes_supported":                               mcpScopes,
		"code_challenge_methods_supported":               []string{"S256"},
		"authorization_response_iss_parameter_supported": true,
	})
}

func isLoopbackURL(u *url.URL) bool {
	if u == nil || u.Scheme != "http" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (g *mcpGateway) listGrants(w http.ResponseWriter, r *http.Request, authn Authenticated) {
	if err := requireAdmin(authn); err != nil {
		writeErr(w, err)
		return
	}
	items := []OAuthGrant{}
	err := g.api.Store.ReadFor(r.Context(), authn.Actor.BusinessID, func(st *State) error {
		for _, v := range st.OAuthGrants {
			items = append(items, v)
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (g *mcpGateway) revokeGrantForUser(w http.ResponseWriter, r *http.Request, authn Authenticated) {
	if err := requireAdmin(authn); err != nil {
		writeErr(w, err)
		return
	}
	err := g.revokeGrant(r.Context(), authn.Actor.BusinessID, chi.URLParam(r, "id"), &authn.Actor)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isKnownMCPScope(scope string) bool { return slices.Contains(mcpScopes, scope) }

func mcpToolError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return errors.New("record_not_found_in_actor_scope")
	}
	if errors.Is(err, ErrForbidden) {
		return errors.New("actor_not_authorized_for_tool")
	}
	return err
}
