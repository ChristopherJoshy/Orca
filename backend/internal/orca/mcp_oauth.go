package orca

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	mcpAuthorizationCodeTTL = 3 * time.Minute
	mcpAccessTokenTTL       = time.Hour
)

type mcpRegistrationRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ApplicationType         string   `json:"application_type"`
}

func (g *mcpGateway) registerClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !g.requireConfiguredPublicURL(w, r) {
		return
	}
	if r.URL.RawQuery != "" || !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_client_metadata"})
		return
	}
	if !g.api.rateLimit(r, "mcp_register") {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limit_exceeded"})
		return
	}
	var req mcpRegistrationRequest
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_client_metadata"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_client_metadata"})
		return
	}
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > 10 || !validMCPClientMetadata(req) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_client_metadata"})
		return
	}
	for _, uri := range req.RedirectURIs {
		if !validMCPRedirectURI(uri) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_redirect_uri"})
			return
		}
	}
	clientID, err := randomToken(24)
	if err != nil {
		http.Error(w, "unable to register client", http.StatusInternalServerError)
		return
	}
	client := mcpClient{ID: "orca_" + clientID, Name: strings.TrimSpace(req.ClientName), RedirectURIs: slices.Clone(req.RedirectURIs)}
	if client.Name == "" {
		client.Name = "MCP client"
	}
	if len(client.Name) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_client_metadata"})
		return
	}
	g.clientsMu.Lock()
	if len(g.clients) >= 10000 {
		g.clientsMu.Unlock()
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "registration_capacity_reached"})
		return
	}
	g.clients[client.ID] = client
	g.clientsMu.Unlock()

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  client.ID,
		"client_name":                client.Name,
		"redirect_uris":              client.RedirectURIs,
		"grant_types":                []string{"authorization_code"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
}

func validMCPClientMetadata(req mcpRegistrationRequest) bool {
	if req.TokenEndpointAuthMethod != "" && req.TokenEndpointAuthMethod != "none" {
		return false
	}
	if req.ApplicationType != "" && req.ApplicationType != "web" && req.ApplicationType != "native" {
		return false
	}
	for _, grant := range req.GrantTypes {
		if grant != "authorization_code" {
			return false
		}
	}
	for _, response := range req.ResponseTypes {
		if response != "code" {
			return false
		}
	}
	return true
}

func validMCPRedirectURI(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	return isLoopbackURL(u)
}

func (g *mcpGateway) authorizeGet(w http.ResponseWriter, r *http.Request, authn Authenticated) {
	if !g.api.rateLimit(r, "mcp_authorize") {
		http.Error(w, "authorization request limit reached", http.StatusTooManyRequests)
		return
	}
	if !g.requireConfiguredPublicURL(w, r) {
		return
	}
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	if r.URL.Query().Get("response_type") != "code" || state == "" || len(state) > 512 {
		http.Error(w, "invalid authorization request", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("resource") != g.resourceURL() {
		http.Error(w, "invalid resource parameter", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("code_challenge_method") != "S256" || !validPKCEChallenge(r.URL.Query().Get("code_challenge")) {
		http.Error(w, "S256 PKCE is required", http.StatusBadRequest)
		return
	}
	g.clientsMu.Lock()
	client, ok := g.clients[clientID]
	g.clientsMu.Unlock()
	if !ok || !slices.Contains(client.RedirectURIs, redirectURI) {
		http.Error(w, "registered client and redirect URI are required", http.StatusBadRequest)
		return
	}
	scopes, err := parseMCPScopeRequest(r.URL.Query().Get("scope"))
	if err != nil {
		http.Error(w, "unsupported scope requested", http.StatusBadRequest)
		return
	}
	requestID, err := randomToken(24)
	if err != nil {
		http.Error(w, "unable to start authorization", http.StatusInternalServerError)
		return
	}
	nonce, err := randomToken(24)
	if err != nil {
		http.Error(w, "unable to start authorization", http.StatusInternalServerError)
		return
	}
	pending := mcpPendingAuthorization{
		ClientID: client.ID, ClientName: client.Name, RedirectURI: redirectURI,
		CodeChallenge: r.URL.Query().Get("code_challenge"), State: state,
		Scopes: scopes, Resource: g.resourceURL(), Actor: authn.Actor,
		Nonce: nonce, ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}
	g.flowMu.Lock()
	g.pruneAuthorizationStateLocked(time.Now())
	if len(g.pending) >= 10000 {
		g.flowMu.Unlock()
		http.Error(w, "authorization service is busy", http.StatusServiceUnavailable)
		return
	}
	g.pending[requestID] = pending
	g.flowMu.Unlock()

	redirect, _ := url.Parse(redirectURI)
	data := struct {
		RequestID string
		Nonce     string
		Client    string
		Scopes    []string
		Redirect  string
		Resource  string
		Business  string
	}{requestID, nonce, client.Name, scopes, redirect.Host, g.resourceURL(), authn.Actor.BusinessID}
	if redirect.Host == "" {
		data.Redirect = redirectURI
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_ = mcpConsentTemplate.Execute(w, data)
}

var mcpConsentTemplate = template.Must(template.New("mcp-consent").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Connect MCP client · ORCA</title></head>
<body><main><h1>Connect {{.Client}} to ORCA</h1>
<p>This client is requesting access to business {{.Business}}.</p>
<p>Redirect hostname: <strong>{{.Redirect}}</strong></p>
<p>ORCA server: {{.Resource}}</p>
<h2>Requested access</h2><ul>{{range .Scopes}}<li>{{.}}</li>{{end}}</ul>
<form method="post" action="/mcp/oauth/authorize">
<input type="hidden" name="request_id" value="{{.RequestID}}"><input type="hidden" name="nonce" value="{{.Nonce}}">
<button type="submit" name="decision" value="approve">Allow access</button>
<button type="submit" name="decision" value="deny">Cancel</button>
</form></main></body></html>`))

func (g *mcpGateway) authorizePost(w http.ResponseWriter, r *http.Request, authn Authenticated) {
	if !g.api.rateLimit(r, "mcp_consent") {
		http.Error(w, "authorization request limit reached", http.StatusTooManyRequests)
		return
	}
	if r.Header.Get("Origin") == "" || g.api.checkOrigin(r) != nil {
		http.Error(w, "same-origin consent is required", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid consent request", http.StatusBadRequest)
		return
	}
	requestID, nonce := r.Form.Get("request_id"), r.Form.Get("nonce")
	g.flowMu.Lock()
	pending, ok := g.pending[requestID]
	if ok && pending.Nonce == nonce && pending.ExpiresAt.After(time.Now()) && pending.Actor.UserID == authn.Actor.UserID && pending.Actor.BusinessID == authn.Actor.BusinessID {
		delete(g.pending, requestID)
	} else {
		ok = false
	}
	g.flowMu.Unlock()
	if !ok {
		http.Error(w, "authorization request expired or invalid", http.StatusBadRequest)
		return
	}
	if r.Form.Get("decision") != "approve" {
		g.redirectAuthorizationResult(w, r, pending, "access_denied", "")
		return
	}
	if err := g.ensureActiveMembership(r.Context(), pending.Actor); err != nil {
		http.Error(w, "active ORCA membership is required", http.StatusForbidden)
		return
	}
	code, err := randomToken(32)
	if err != nil {
		http.Error(w, "unable to issue authorization code", http.StatusInternalServerError)
		return
	}
	issued := mcpAuthorizationCode{
		ClientID: pending.ClientID, RedirectURI: pending.RedirectURI,
		CodeChallenge: pending.CodeChallenge, State: pending.State,
		Scopes: slices.Clone(pending.Scopes), Resource: pending.Resource,
		Actor: pending.Actor, ExpiresAt: time.Now().UTC().Add(mcpAuthorizationCodeTTL),
	}
	g.flowMu.Lock()
	g.pruneAuthorizationStateLocked(time.Now())
	g.codes[tokenDigest(code)] = issued
	g.flowMu.Unlock()
	g.redirectAuthorizationResult(w, r, pending, "", code)
}

func (g *mcpGateway) redirectAuthorizationResult(w http.ResponseWriter, r *http.Request, pending mcpPendingAuthorization, oauthError, code string) {
	target, err := url.Parse(pending.RedirectURI)
	if err != nil {
		http.Error(w, "registered redirect URI is invalid", http.StatusBadRequest)
		return
	}
	query := target.Query()
	query.Set("state", pending.State)
	query.Set("iss", g.publicBase())
	if oauthError != "" {
		query.Set("error", oauthError)
	} else {
		query.Set("code", code)
	}
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (g *mcpGateway) token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !g.requireConfiguredPublicURL(w, r) {
		return
	}
	if r.URL.RawQuery != "" || !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
		oauthTokenError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		oauthTokenError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("resource") != g.resourceURL() {
		oauthTokenError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	clientID := r.Form.Get("client_id")
	g.clientsMu.Lock()
	client, clientOK := g.clients[clientID]
	g.clientsMu.Unlock()
	if !clientOK {
		oauthTokenError(w, http.StatusUnauthorized, "invalid_client")
		return
	}
	code := r.Form.Get("code")
	if code == "" || len(code) > 128 {
		oauthTokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	codeKey := tokenDigest(code)
	g.flowMu.Lock()
	issued, ok := g.codes[codeKey]
	g.flowMu.Unlock()
	if !ok || !issued.ExpiresAt.After(time.Now()) || issued.ClientID != clientID || issued.RedirectURI != r.Form.Get("redirect_uri") || !slices.Contains(client.RedirectURIs, issued.RedirectURI) || issued.Resource != r.Form.Get("resource") || !verifyPKCE(issued.CodeChallenge, r.Form.Get("code_verifier")) {
		oauthTokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	g.flowMu.Lock()
	current, stillAvailable := g.codes[codeKey]
	if stillAvailable && current.ExpiresAt.After(time.Now()) && current.ClientID == issued.ClientID && current.RedirectURI == issued.RedirectURI {
		delete(g.codes, codeKey)
	} else {
		stillAvailable = false
	}
	g.flowMu.Unlock()
	if !stillAvailable {
		oauthTokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	if err := g.ensureActiveMembership(r.Context(), issued.Actor); err != nil {
		oauthTokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	plainToken, err := randomToken(32)
	if err != nil {
		http.Error(w, "unable to issue access token", http.StatusInternalServerError)
		return
	}
	plainToken = "orca_mcp_" + plainToken
	grantID, err := newUUID()
	if err != nil {
		http.Error(w, "unable to issue access token", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().UTC().Add(mcpAccessTokenTTL)
	grant := OAuthGrant{
		ID: grantID, BusinessID: issued.Actor.BusinessID, UserID: issued.Actor.UserID,
		ClientID: client.ID, Scopes: slices.Clone(issued.Scopes), TokenHash: tokenDigest(plainToken), ExpiresAt: expiresAt,
	}
	if err := g.createGrant(r.Context(), grant, issued.RedirectURI); err != nil {
		http.Error(w, "unable to persist access grant", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": plainToken, "token_type": "Bearer", "expires_in": int(mcpAccessTokenTTL.Seconds()),
		"scope": strings.Join(grant.Scopes, " "),
	})
}

func (g *mcpGateway) revokeToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.URL.RawQuery != "" || !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
		oauthTokenError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		oauthTokenError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	token := r.Form.Get("token")
	if token != "" && len(token) <= 256 {
		_ = g.revokeTokenByValue(r.Context(), token)
	}
	w.WriteHeader(http.StatusOK)
}

func oauthTokenError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, status, map[string]string{"error": code})
}

func parseMCPScopeRequest(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{mcpScopeRecords}, nil
	}
	parts := strings.Fields(raw)
	seen := map[string]bool{}
	for _, scope := range parts {
		if !isKnownMCPScope(scope) || seen[scope] {
			return nil, ErrValidation
		}
		seen[scope] = true
	}
	return parts, nil
}

func validPKCEChallenge(challenge string) bool {
	if len(challenge) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(challenge)
	return err == nil && len(decoded) == sha256.Size
}

func verifyPKCE(challenge, verifier string) bool {
	if !validPKCEChallenge(challenge) || len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	for _, char := range verifier {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-._~", char)) {
			return false
		}
	}
	sum := sha256.Sum256([]byte(verifier))
	actual := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(challenge)) == 1
}

func (g *mcpGateway) pruneAuthorizationStateLocked(now time.Time) {
	for id, pending := range g.pending {
		if !pending.ExpiresAt.After(now) {
			delete(g.pending, id)
		}
	}
	for id, code := range g.codes {
		if !code.ExpiresAt.After(now) {
			delete(g.codes, id)
		}
	}
}

func (g *mcpGateway) ensureActiveMembership(ctx context.Context, actor Actor) error {
	return g.api.Store.ReadFor(ctx, actor.BusinessID, func(st *State) error {
		membership, ok := st.Memberships[membershipKey(actor.BusinessID, actor.UserID)]
		if !ok || membership.RevokedAt != nil || membership.Role != actor.Role || membership.CustomerID != actor.CustomerID {
			return ErrUnauthorized
		}
		return nil
	})
}

func (g *mcpGateway) lookupGrantByToken(ctx context.Context, token string) (OAuthGrant, Actor, error) {
	hash := tokenDigest(token)
	if g.api.Store.pool != nil {
		hashBytes, err := hex.DecodeString(hash)
		if err != nil {
			return OAuthGrant{}, Actor{}, ErrUnauthorized
		}
		var grant OAuthGrant
		var actor Actor
		err = g.api.Store.pool.QueryRow(ctx, `SELECT g.id::text,g.business_id::text,g.user_id::text,g.client_id,g.scopes,encode(g.token_hash,'hex'),g.expires_at,m.role,COALESCE(m.customer_id::text,'')
			FROM oauth_grants g JOIN memberships m ON m.business_id=g.business_id AND m.user_id=g.user_id
			WHERE g.token_hash=$1 AND g.revoked_at IS NULL AND g.expires_at>now() AND m.revoked_at IS NULL`, hashBytes).Scan(&grant.ID, &grant.BusinessID, &grant.UserID, &grant.ClientID, &grant.Scopes, &grant.TokenHash, &grant.ExpiresAt, &actor.Role, &actor.CustomerID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return OAuthGrant{}, Actor{}, ErrUnauthorized
			}
			return OAuthGrant{}, Actor{}, err
		}
		actor.UserID, actor.BusinessID = grant.UserID, grant.BusinessID
		return grant, actor, nil
	}
	var grant OAuthGrant
	var actor Actor
	err := g.api.Store.Read(ctx, func(st *State) error {
		for _, candidate := range st.OAuthGrants {
			if candidate.TokenHash == hash && candidate.RevokedAt == nil && candidate.ExpiresAt.After(time.Now()) {
				grant = candidate
				break
			}
		}
		if grant.ID == "" {
			return ErrUnauthorized
		}
		membership, ok := st.Memberships[membershipKey(grant.BusinessID, grant.UserID)]
		if !ok || membership.RevokedAt != nil {
			return ErrUnauthorized
		}
		actor = Actor{UserID: grant.UserID, BusinessID: grant.BusinessID, CustomerID: membership.CustomerID, Role: membership.Role}
		return nil
	})
	return grant, actor, err
}

func (g *mcpGateway) createGrant(ctx context.Context, grant OAuthGrant, redirectURI string) error {
	actor := Actor{UserID: grant.UserID, BusinessID: grant.BusinessID}
	event := auditEvent(actor, "mcp.grant_created", "mcp_grant", grant.ID, map[string]any{"clientId": grant.ClientID, "redirectHost": redirectHost(redirectURI), "scopes": grant.Scopes, "expiresAt": grant.ExpiresAt})
	if g.api.Store.pool == nil {
		return g.api.Store.UpdateFor(ctx, grant.BusinessID, func(st *State) error {
			st.OAuthGrants[grant.ID] = grant
			st.Audit = append(st.Audit, event)
			return nil
		})
	}
	hashBytes, err := hex.DecodeString(grant.TokenHash)
	if err != nil {
		return err
	}
	tx, err := g.api.Store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO oauth_grants(id,business_id,user_id,client_id,redirect_uri,scopes,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, grant.ID, grant.BusinessID, grant.UserID, grant.ClientID, redirectURI, grant.Scopes, hashBytes, grant.ExpiresAt); err != nil {
		return err
	}
	if err := insertMCPAudit(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func redirectHost(redirectURI string) string {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return ""
	}
	return u.Host
}

func (g *mcpGateway) revokeGrant(ctx context.Context, businessID, id string, actor *Actor) error {
	now := time.Now().UTC()
	if g.api.Store.pool == nil {
		return g.api.Store.UpdateFor(ctx, businessID, func(st *State) error {
			grant, ok := st.OAuthGrants[id]
			if !ok {
				return ErrNotFound
			}
			if grant.RevokedAt != nil {
				return nil
			}
			grant.RevokedAt = &now
			st.OAuthGrants[id] = grant
			who := Actor{UserID: grant.UserID, BusinessID: businessID}
			if actor != nil {
				who = *actor
			}
			st.Audit = append(st.Audit, auditEvent(who, "mcp.grant_revoked", "mcp_grant", grant.ID, map[string]any{"clientId": grant.ClientID}))
			return nil
		})
	}
	tx, err := g.api.Store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID, clientID string
	err = tx.QueryRow(ctx, `UPDATE oauth_grants SET revoked_at=COALESCE(revoked_at,$1) WHERE id=$2 AND business_id=$3 RETURNING user_id::text,client_id`, now, id, businessID).Scan(&userID, &clientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	who := Actor{UserID: userID, BusinessID: businessID}
	if actor != nil {
		who = *actor
	}
	event := auditEvent(who, "mcp.grant_revoked", "mcp_grant", id, map[string]any{"clientId": clientID})
	if err := insertMCPAudit(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (g *mcpGateway) revokeTokenByValue(ctx context.Context, token string) error {
	hash := tokenDigest(token)
	if g.api.Store.pool == nil {
		return g.api.Store.Update(ctx, func(st *State) error {
			for id, grant := range st.OAuthGrants {
				if grant.TokenHash == hash && grant.RevokedAt == nil {
					now := time.Now().UTC()
					grant.RevokedAt = &now
					st.OAuthGrants[id] = grant
					actor := Actor{UserID: grant.UserID, BusinessID: grant.BusinessID}
					st.Audit = append(st.Audit, auditEvent(actor, "mcp.grant_revoked", "mcp_grant", id, map[string]any{"clientId": grant.ClientID, "revokedByClient": true}))
					break
				}
			}
			return nil
		})
	}
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return err
	}
	tx, err := g.api.Store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var grantID, businessID, userID, clientID string
	err = tx.QueryRow(ctx, `UPDATE oauth_grants SET revoked_at=COALESCE(revoked_at,now()) WHERE token_hash=$1 RETURNING id::text,business_id::text,user_id::text,client_id`, hashBytes).Scan(&grantID, &businessID, &userID, &clientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	event := auditEvent(Actor{UserID: userID, BusinessID: businessID}, "mcp.grant_revoked", "mcp_grant", grantID, map[string]any{"clientId": clientID, "revokedByClient": true})
	if err := insertMCPAudit(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertMCPAudit(ctx context.Context, tx pgx.Tx, event AuditEvent) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_events(id,business_id,actor_user_id,action,resource_type,resource_id,details,created_at) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5,NULLIF($6,'')::uuid,$7::jsonb,$8) ON CONFLICT(id) DO NOTHING`, event.ID, event.BusinessID, event.ActorID, event.Action, event.Resource, event.ResourceID, jsonBytes(event.Details), event.CreatedAt)
	return err
}
