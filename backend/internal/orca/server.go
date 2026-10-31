package orca

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

type ServerConfig struct {
	CookieSecure        bool
	AllowedOrigin       string
	Env                 string
	ResendAPIKey        string
	EmailFrom           string
	PublicURL           string
	GroqAPIKey          string
	GroqModel           string
	StripeAPIKey        string
	StripeWebhookSecret string
}

type API struct {
	Store        *Store
	Config       ServerConfig
	mux          chi.Router
	limitMu      sync.Mutex
	limits       map[string]rateWindow
	agentMu      sync.Mutex
	agentCancels map[string]context.CancelFunc
	stripe       stripeCheckoutSessionCreator
}
type rateWindow struct {
	start time.Time
	count int
}

type apiError struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func NewAPI(store *Store, config ServerConfig) http.Handler {
	return newAPI(store, config).mux
}

func newAPI(store *Store, config ServerConfig) *API {
	a := &API{Store: store, Config: config, limits: map[string]rateWindow{}, agentCancels: map[string]context.CancelFunc{}}
	a.stripe = stripeCheckoutSessionCreatorHTTP{apiKey: config.StripeAPIKey, endpoint: stripeCheckoutSessionsEndpoint, client: &http.Client{Timeout: 15 * time.Second}}
	r := chi.NewRouter()
	r.Use(a.securityHeaders)
	r.Get("/api/v1/health", a.health)
	r.Post("/api/v1/auth/signup", a.signup)
	r.Post("/api/v1/auth/login", a.login)
	r.Post("/api/v1/auth/verify-email", a.verifyEmail)
	r.Post("/api/v1/auth/logout", a.protected(true, a.logout))
	r.Post("/api/v1/auth/password-reset", a.passwordResetRequest)
	r.Post("/api/v1/auth/password-reset/complete", a.passwordResetComplete)
	r.Get("/api/v1/me", a.protected(false, a.me))
	r.Get("/api/v1/portal/me", a.protected(false, a.portalMe))
	for _, resource := range []string{"products", "plans", "customers", "quotations", "subscriptions", "invoices", "payments", "discounts", "taxes", "staff", "exports", "agent/approvals", "agent/runs", "mcp/connections"} {
		path := "/api/v1/" + resource
		if resource == "agent/approvals" {
			r.Get(path, a.protected(false, a.listApprovals))
		} else if resource == "agent/runs" {
			r.Get(path, a.protected(false, a.listAgentRuns))
		} else if resource == "mcp/connections" {
			r.Get(path, a.protected(false, a.listMCPConnections))
		} else {
			r.Get(path, a.protected(false, a.listCollection(resource)))
		}
	}
	for _, resource := range []string{"products", "plans", "customers", "quotations", "subscriptions", "discounts", "taxes", "staff"} {
		path := "/api/v1/" + resource
		if resource == "products" {
			r.Post(path, a.protected(true, a.createProduct))
		}
		if resource == "plans" {
			r.Post(path, a.protected(true, a.createPlan))
		}
		if resource == "customers" {
			r.Post(path, a.protected(true, a.createCustomer))
		}
		if resource == "quotations" {
			r.Post(path, a.protected(true, a.createQuotation))
		}
		if resource == "subscriptions" {
			r.Post(path, a.protected(true, a.createSubscription))
		}
		if resource == "discounts" {
			r.Post(path, a.protected(true, a.createDiscount))
		}
		if resource == "taxes" {
			r.Post(path, a.protected(true, a.createTax))
		}
		if resource == "staff" {
			r.Post(path, a.protected(true, a.inviteStaff))
		}
	}
	r.Post("/api/v1/staff/invitations", a.protected(true, a.inviteStaff))
	r.Post("/api/v1/quotations/{id}/send", a.protected(true, a.sendQuotation))
	r.Post("/api/v1/quotations/{id}/accept", a.protected(true, a.acceptQuotation))
	r.Post("/api/v1/subscriptions/{id}/transition", a.protected(true, a.transitionSubscription))
	r.Post("/api/v1/billing/run", a.protected(true, a.runBilling))
	r.Get("/api/v1/invoices/{id}", a.protected(false, a.getInvoice))
	r.Post("/api/v1/invoices/{id}/confirm", a.protected(true, a.confirmInvoice))
	r.Post("/api/v1/invoices/{id}/cancel", a.protected(true, a.cancelInvoice))
	r.Post("/api/v1/invoices/{id}/send", a.protected(true, a.sendInvoice))
	r.Post("/api/v1/invoices/{id}/payments", a.protected(true, a.recordPayment))
	r.Post("/api/v1/portal/invoices/{id}/checkout", a.protected(true, a.createPortalCheckout))
	r.Get("/api/v1/reports/summary", a.protected(false, a.reportSummary))
	r.Post("/api/v1/exports", a.protected(true, a.requestExport))
	r.Get("/api/v1/exports/{id}", a.protected(false, a.getExport))
	r.Get("/api/v1/exports/{id}/download", a.protected(false, a.downloadExport))
	r.Post("/api/v1/agent/runs", a.protected(true, a.startAgentRun))
	r.Get("/api/v1/agent/runs/{id}", a.protected(false, a.getAgentRun))
	r.Post("/api/v1/agent/runs/{id}/cancel", a.protected(true, a.cancelAgentRun))
	r.Post("/api/v1/agent/approvals/{id}/decision", a.protected(true, a.decideApproval))
	r.Post("/api/v1/mcp/connections", a.protected(true, a.createMCPConnection))
	r.Delete("/api/v1/mcp/connections/{id}", a.protected(true, a.revokeMCPConnection))
	a.RegisterMCPRoutes(r)
	r.Post("/api/v1/webhooks/stripe", a.stripeWebhook)
	a.mux = r
	return a
}

func (a *API) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	database := "demo"
	if a.Store.IsPostgres() {
		database = "postgres"
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "database": database, "time": time.Now().UTC()})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeErr(w http.ResponseWriter, err error) {
	status := 500
	switch {
	case errors.Is(err, ErrUnauthorized):
		status = 401
	case errors.Is(err, ErrForbidden):
		status = 403
	case errors.Is(err, ErrNotFound):
		status = 404
	case errors.Is(err, ErrConflict):
		status = 409
	case errors.Is(err, ErrValidation), errors.Is(err, ErrInvalidMoney):
		status = 422
	}
	writeJSON(w, status, apiError{Message: err.Error()})
}
func decodeBody(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return fmt.Errorf("%w: invalid JSON request", ErrValidation)
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: request must contain one JSON object", ErrValidation)
	}
	return nil
}
func (a *API) protected(csrf bool, next func(http.ResponseWriter, *http.Request, Authenticated)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("orca_session")
		if err != nil {
			writeErr(w, ErrUnauthorized)
			return
		}
		auth, err := a.Store.ResolveSession(r.Context(), cookie.Value)
		if err != nil {
			writeErr(w, ErrUnauthorized)
			return
		}
		if csrf {
			if err := a.checkOrigin(r); err != nil {
				writeErr(w, err)
				return
			}
			given := r.Header.Get("X-CSRF-Token")
			sum := sha256.Sum256([]byte(given))
			if given == "" || fmt.Sprintf("%x", sum[:]) != auth.CSRFHash {
				writeJSON(w, 403, apiError{Message: "CSRF token is missing or invalid"})
				return
			}
		}
		next(w, r, auth)
	}
}
func (a *API) checkOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	expected := a.Config.AllowedOrigin
	if expected == "" {
		return fmt.Errorf("%w: cross-origin request is not allowed", ErrForbidden)
	}
	actual, e1 := url.Parse(origin)
	allowed, e2 := url.Parse(expected)
	if e1 != nil || e2 != nil || actual.Scheme != allowed.Scheme || actual.Host != allowed.Host {
		return fmt.Errorf("%w: request origin is not allowed", ErrForbidden)
	}
	return nil
}
func (a *API) rateLimit(r *http.Request, key string) bool {
	ip := r.RemoteAddr
	if host, _, err := strings.Cut(ip, ":"); err {
		ip = host
	}
	id := key + ":" + ip
	a.limitMu.Lock()
	defer a.limitMu.Unlock()
	now := time.Now()
	w := a.limits[id]
	if now.Sub(w.start) > time.Minute {
		w = rateWindow{start: now}
	}
	w.count++
	a.limits[id] = w
	return w.count <= 12
}
func (a *API) setSession(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: "orca_session", Value: token, Path: "/", HttpOnly: true, Secure: a.Config.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds())})
}
func clearSession(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: "orca_session", Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}
func (a *API) issueSession(ctx context.Context, user User, business Business, member Membership) (string, string, Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", "", Session{}, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return "", "", Session{}, err
	}
	now := time.Now().UTC()
	session := Session{UserID: user.ID, BusinessID: business.ID, CustomerID: member.CustomerID, Role: member.Role, CSRFToken: tokenDigest(csrf), CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)}
	if err := a.Store.InsertSession(ctx, session, tokenDigest(token)); err != nil {
		return "", "", Session{}, err
	}
	return token, csrf, session, nil
}
func (a *API) signup(w http.ResponseWriter, r *http.Request) {
	if !a.rateLimit(r, "signup") {
		writeJSON(w, 429, apiError{Message: "Too many requests"})
		return
	}
	var input struct {
		Email        string
		Password     string
		BusinessName string
		BusinessID   string
	}
	if err := decodeBody(r, &input); err != nil {
		writeErr(w, err)
		return
	}
	if strings.TrimSpace(input.BusinessID) != "" {
		a.signupCustomer(w, r, input.Email, input.Password, strings.TrimSpace(input.BusinessID))
		return
	}
	normalized, err := validateEmail(input.Email)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := validatePassword(input.Password); err != nil {
		writeErr(w, err)
		return
	}
	if strings.TrimSpace(input.BusinessName) == "" || len(input.BusinessName) > 120 {
		writeErr(w, fmt.Errorf("%w: businessName is required", ErrValidation))
		return
	}
	hash, err := passwordHash(input.Password)
	if err != nil {
		writeErr(w, err)
		return
	}
	uid, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	bid, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	now := time.Now().UTC()
	user := User{ID: uid, Email: strings.TrimSpace(input.Email), EmailNormalized: normalized, PasswordHash: hash, Verified: a.Config.Env == "development", CreatedAt: now}
	verificationToken, verificationHash := "", ""
	if !user.Verified {
		verificationToken, err = randomToken(32)
		if err != nil {
			writeErr(w, err)
			return
		}
		verificationHash = tokenDigest(verificationToken)
	}
	business := Business{ID: bid, Name: strings.TrimSpace(input.BusinessName), Currency: CurrencyINR, Timezone: "Asia/Kolkata", NextInvoiceNumber: 1}
	if err := a.Store.Update(r.Context(), func(st *State) error {
		for _, u := range st.Users {
			if u.EmailNormalized == normalized {
				return ErrConflict
			}
		}
		st.Users[user.ID] = user
		st.Businesses[business.ID] = business
		st.Memberships[membershipKey(business.ID, user.ID)] = Membership{BusinessID: business.ID, UserID: user.ID, Role: "admin"}
		if !user.Verified {
			st.EmailVerifications[verificationHash] = AuthToken{UserID: user.ID, ExpiresAt: now.Add(24 * time.Hour)}
		}
		return nil
	}); err != nil {
		writeErr(w, err)
		return
	}
	if !user.Verified {
		delivered := false
		if a.Config.ResendAPIKey != "" {
			link := strings.TrimRight(a.Config.PublicURL, "/") + "/?verify=" + url.QueryEscape(verificationToken)
			body := "<p>Verify your ORCA email address:</p><p><a href=\"" + html.EscapeString(link) + "\">Verify email</a></p>"
			delivered = a.sendResendEmail(r.Context(), user.Email, "Verify your ORCA account", body, "orca-verify:"+verificationHash) == nil
		}
		writeJSON(w, 201, map[string]any{"user": map[string]any{"id": user.ID, "email": user.Email, "verified": false}, "verificationRequired": true, "emailSent": delivered})
		return
	}
	member := Membership{BusinessID: business.ID, UserID: user.ID, Role: "admin"}
	token, csrf, session, err := a.issueSession(r.Context(), user, business, member)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.setSession(w, token, session.ExpiresAt)
	writeJSON(w, 201, map[string]any{"user": map[string]any{"id": user.ID, "email": user.Email, "verified": user.Verified}, "business": business, "csrfToken": csrf, "autoVerified": true})
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.rateLimit(r, "login") {
		writeJSON(w, 429, apiError{Message: "Too many requests"})
		return
	}
	var input struct {
		Email      string
		Password   string
		BusinessID string
	}
	if err := decodeBody(r, &input); err != nil {
		writeErr(w, err)
		return
	}
	email, err := validateEmail(input.Email)
	if err != nil {
		writeErr(w, ErrUnauthorized)
		return
	}
	user, members, err := a.Store.LookupUser(r.Context(), email)
	if err != nil || !verifyPassword(user.PasswordHash, input.Password) {
		writeErr(w, ErrUnauthorized)
		return
	}
	if !user.Verified {
		writeJSON(w, 403, apiError{Message: "Verify your email before signing in"})
		return
	}
	member := members[0]
	if input.BusinessID != "" {
		found := false
		for _, m := range members {
			if m.BusinessID == input.BusinessID {
				member = m
				found = true
				break
			}
		}
		if !found {
			writeErr(w, ErrForbidden)
			return
		}
	}
	var business Business
	err = a.Store.ReadFor(r.Context(), member.BusinessID, func(st *State) error {
		var ok bool
		business, ok = st.Businesses[member.BusinessID]
		if !ok {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	token, csrf, session, err := a.issueSession(r.Context(), user, business, member)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.setSession(w, token, session.ExpiresAt)
	writeJSON(w, 200, map[string]any{"user": map[string]any{"id": user.ID, "email": user.Email, "verified": user.Verified}, "business": business, "csrfToken": csrf})
}
func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct{ Token string }
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Token == "" {
		writeErr(w, fmt.Errorf("%w: token is required", ErrValidation))
		return
	}
	tokenHash := tokenDigest(req.Token)
	plainSession, err := randomToken(32)
	if err != nil {
		writeErr(w, err)
		return
	}
	csrf, err := randomToken(32)
	if err != nil {
		writeErr(w, err)
		return
	}
	now := time.Now().UTC()
	var user User
	var business Business
	var member Membership
	var session Session
	err = a.Store.Update(r.Context(), func(st *State) error {
		token, ok := st.EmailVerifications[tokenHash]
		if !ok || !token.ExpiresAt.After(now) {
			return ErrUnauthorized
		}
		user, ok = st.Users[token.UserID]
		if !ok {
			return ErrUnauthorized
		}
		user.Verified = true
		st.Users[user.ID] = user
		delete(st.EmailVerifications, tokenHash)
		for _, candidate := range st.Memberships {
			if candidate.UserID == user.ID && candidate.RevokedAt == nil {
				member = candidate
				break
			}
		}
		if member.BusinessID == "" {
			return ErrUnauthorized
		}
		business, ok = st.Businesses[member.BusinessID]
		if !ok {
			return ErrUnauthorized
		}
		session = Session{UserID: user.ID, BusinessID: business.ID, CustomerID: member.CustomerID, Role: member.Role, CSRFToken: tokenDigest(csrf), CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)}
		st.Sessions[tokenDigest(plainSession)] = session
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	a.setSession(w, plainSession, session.ExpiresAt)
	writeJSON(w, 200, map[string]any{"user": map[string]any{"id": user.ID, "email": user.Email, "verified": true}, "business": business, "csrfToken": csrf})
}
func (a *API) passwordResetRequest(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email string }
	if err := decodeBody(r, &req); err != nil {
		writeJSON(w, 202, map[string]any{"accepted": true})
		return
	}
	email, err := validateEmail(req.Email)
	if err != nil {
		writeJSON(w, 202, map[string]any{"accepted": true})
		return
	}
	if a.Config.ResendAPIKey == "" {
		writeJSON(w, 202, map[string]any{"accepted": true})
		return
	}
	user, _, err := a.Store.LookupUser(r.Context(), email)
	if err != nil {
		writeJSON(w, 202, map[string]any{"accepted": true})
		return
	}
	plain, err := randomToken(32)
	if err != nil {
		writeJSON(w, 202, map[string]any{"accepted": true})
		return
	}
	digest := tokenDigest(plain)
	expires := time.Now().UTC().Add(time.Hour)
	if err := a.Store.Update(r.Context(), func(st *State) error {
		st.ResetTokens[digest] = AuthToken{UserID: user.ID, ExpiresAt: expires}
		return nil
	}); err == nil {
		link := strings.TrimRight(a.Config.PublicURL, "/") + "/?reset=" + url.QueryEscape(plain)
		body := "<p>Reset your ORCA password:</p><p><a href=\"" + html.EscapeString(link) + "\">Choose a new password</a></p>"
		_ = a.sendResendEmail(r.Context(), user.Email, "Reset your ORCA password", body, "orca-reset:"+digest)
	}
	writeJSON(w, 202, map[string]any{"accepted": true})
}
func (a *API) passwordResetComplete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string
		Password string
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Token == "" {
		writeErr(w, ErrUnauthorized)
		return
	}
	if err := validatePassword(req.Password); err != nil {
		writeErr(w, err)
		return
	}
	hash, err := passwordHash(req.Password)
	if err != nil {
		writeErr(w, err)
		return
	}
	digest := tokenDigest(req.Token)
	now := time.Now().UTC()
	err = a.Store.Update(r.Context(), func(st *State) error {
		token, ok := st.ResetTokens[digest]
		if !ok || !token.ExpiresAt.After(now) {
			return ErrUnauthorized
		}
		user, ok := st.Users[token.UserID]
		if !ok {
			return ErrUnauthorized
		}
		user.PasswordHash = hash
		st.Users[user.ID] = user
		delete(st.ResetTokens, digest)
		for key, session := range st.Sessions {
			if session.UserID == user.ID {
				delete(st.Sessions, key)
			}
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}
func (a *API) logout(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	_ = a.Store.RevokeSession(r.Context(), auth.Actor.BusinessID, auth.TokenHash)
	clearSession(w, a.Config.CookieSecure)
	w.WriteHeader(204)
}
func (a *API) me(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	var user User
	var business Business
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		user = st.Users[auth.Actor.UserID]
		business = st.Businesses[auth.Actor.BusinessID]
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"actor": auth.Actor, "user": map[string]any{"id": user.ID, "email": user.Email, "verified": user.Verified}, "business": business})
}
func (a *API) portalMe(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if auth.Actor.Role != "customer" {
		writeErr(w, ErrForbidden)
		return
	}
	var customer Customer
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		var ok bool
		customer, ok = st.Customers[auth.Actor.CustomerID]
		if !ok || customer.UserID != auth.Actor.UserID {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"customer": customer, "businessId": auth.Actor.BusinessID})
}

func requireAdmin(auth Authenticated) error {
	if auth.Actor.Role != "admin" {
		return ErrForbidden
	}
	return nil
}
func requireStaff(auth Authenticated) error {
	if auth.Actor.Role != "admin" && auth.Actor.Role != "staff" {
		return ErrForbidden
	}
	return nil
}
func (a *API) listCollection(resource string) func(http.ResponseWriter, *http.Request, Authenticated) {
	return func(w http.ResponseWriter, r *http.Request, auth Authenticated) {
		items := []any{}
		err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
			items = collectionItems(st, resource, auth.Actor)
			if items == nil {
				items = []any{}
			}
			return nil
		})
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "nextCursor": nil})
	}
}
func collectionItems(st *State, resource string, actor Actor) []any {
	customerOnly := actor.Role == "customer"
	id := actor.CustomerID
	var values []any
	switch resource {
	case "products":
		for _, v := range st.Products {
			values = append(values, productWire(v))
		}
	case "plans":
		for _, v := range st.Plans {
			values = append(values, planWire(v))
		}
	case "customers":
		for _, v := range st.Customers {
			if !customerOnly || v.ID == id {
				values = append(values, v)
			}
		}
	case "quotations":
		for _, v := range st.Quotations {
			if !customerOnly || v.CustomerID == id {
				values = append(values, quoteWire(v))
			}
		}
	case "subscriptions":
		for _, v := range st.Subscriptions {
			if !customerOnly || v.CustomerID == id {
				values = append(values, subscriptionWire(v, st))
			}
		}
	case "invoices":
		for _, v := range st.Invoices {
			if !customerOnly || v.CustomerID == id {
				values = append(values, invoiceWire(v))
			}
		}
	case "payments":
		for _, v := range st.Payments {
			if !customerOnly || v.CustomerID == id {
				values = append(values, paymentWire(v))
			}
		}
	case "discounts":
		if !customerOnly {
			for _, v := range st.Discounts {
				values = append(values, discountWire(v))
			}
		}
	case "taxes":
		if !customerOnly {
			for _, v := range st.Taxes {
				values = append(values, taxWire(v))
			}
		}
	case "staff":
		if !customerOnly {
			for _, m := range st.Memberships {
				if m.BusinessID == actor.BusinessID && m.RevokedAt == nil {
					u := st.Users[m.UserID]
					values = append(values, map[string]any{"id": u.ID, "name": u.Email, "email": u.Email, "role": m.Role, "status": "Active"})
				}
			}
		}
	case "exports":
		for _, v := range st.Exports {
			if !customerOnly || v.RequestedBy == actor.UserID {
				values = append(values, v)
			}
		}
	}
	sort.SliceStable(values, func(i, j int) bool { return fmt.Sprint(values[i]) < fmt.Sprint(values[j]) })
	return values
}
func quoteWire(v Quotation) map[string]any {
	return map[string]any{"id": v.ID, "businessId": v.BusinessID, "customerId": v.CustomerID, "number": v.Number, "title": v.Title, "name": v.Title, "date": v.CreatedAt, "createdAt": v.CreatedAt, "validUntil": v.ValidUntil, "amount": money2(v.Amount), "status": v.Status, "lines": v.Lines}
}
func productWire(v Product) map[string]any {
	v.SalesPrice = money2(v.SalesPrice)
	v.CostPrice = money2(v.CostPrice)
	v.TaxRate = rate2(v.TaxRate)
	for i := range v.Variants {
		v.Variants[i].Adjustment = money2(v.Variants[i].Adjustment)
	}
	return map[string]any{"id": v.ID, "businessId": v.BusinessID, "name": v.Name, "type": v.Type, "salesPrice": v.SalesPrice, "costPrice": v.CostPrice, "notes": v.Notes, "variants": v.Variants, "taxRate": v.TaxRate, "version": v.Version, "createdAt": v.CreatedAt, "active": true}
}
func planWire(v Plan) map[string]any {
	return map[string]any{"id": v.ID, "businessId": v.BusinessID, "name": v.Name, "interval": v.Interval, "cadence": strings.Title(v.Interval), "salesPrice": money2(v.SalesPrice), "minimumQuantity": v.MinimumQuantity, "availableFrom": v.AvailableFrom, "availableUntil": v.AvailableUntil, "taxRate": rate2(v.TaxRate), "autoClose": v.AutoClose, "allowEarlyClose": v.AllowEarlyClose, "allowPause": v.AllowPause, "allowRenew": v.AllowRenew, "pauseAllowed": v.AllowPause, "closeAllowed": v.AllowEarlyClose, "renewAllowed": v.AllowRenew, "version": v.Version, "createdAt": v.CreatedAt, "active": true}
}
func subscriptionWire(v Subscription, st *State) map[string]any {
	amount := "0.00"
	if p, ok := st.Plans[v.PlanID]; ok {
		q := 1
		if len(v.Lines) > 0 && v.Lines[0].Quantity > 0 {
			q = v.Lines[0].Quantity
		}
		d, e := ParseDecimal(p.SalesPrice)
		if e == nil {
			amount = amountString(d.Mul(decimalFromInt(q)))
		}
	}
	return map[string]any{"id": v.ID, "businessId": v.BusinessID, "customerId": v.CustomerID, "planId": v.PlanID, "number": v.Number, "status": v.Status, "startDate": v.StartDate, "startedAt": v.StartDate, "expiryDate": v.ExpiryDate, "renewsAt": v.ExpiryDate, "paymentTermsDays": v.PaymentTermsDays, "lines": v.Lines, "amount": amount, "version": v.Version}
}
func invoiceWire(v Invoice) map[string]any {
	return map[string]any{"id": v.ID, "businessId": v.BusinessID, "subscriptionId": v.SubscriptionID, "customerId": v.CustomerID, "number": v.Number, "status": v.Status, "currency": v.Currency, "subtotal": money2(v.Subtotal), "discountTotal": money2(v.DiscountTotal), "taxTotal": money2(v.TaxTotal), "total": money2(v.Total), "outstanding": money2(v.Outstanding), "dueAt": v.DueAt, "dueDate": v.DueAt, "issueDate": v.CreatedAt, "createdAt": v.CreatedAt, "periodStart": v.PeriodStart, "periodEnd": v.PeriodEnd, "period": v.PeriodStart.Format("2006-01-02") + " – " + v.PeriodEnd.Format("2006-01-02"), "snapshot": v.Snapshot, "calculationHash": v.CalculationHash, "overdue": v.Status == "Confirmed" && v.Outstanding != "0.00" && time.Now().After(v.DueAt), "version": v.Version}
}
func paymentWire(v Payment) map[string]any {
	return map[string]any{"id": v.ID, "businessId": v.BusinessID, "invoiceId": v.InvoiceID, "customerId": v.CustomerID, "amount": money2(v.Amount), "method": v.Method, "date": v.ReceivedAt, "receivedAt": v.ReceivedAt, "createdAt": v.CreatedAt, "note": v.Note, "provider": v.Provider}
}
func discountWire(v Discount) map[string]any {
	return map[string]any{"id": v.ID, "name": v.Name, "kind": strings.Title(v.Kind), "value": v.Value, "scope": v.Scope, "minimumPurchase": money2(v.MinimumPurchase), "minimum": money2(v.MinimumPurchase), "minimumQuantity": v.MinimumQuantity, "minQuantity": v.MinimumQuantity, "useCount": v.UseCount, "maxUses": v.MaxUses, "limit": v.MaxUses, "endsAt": v.EndsAt, "active": true, "version": v.Version}
}
func taxWire(v TaxRule) map[string]any {
	return map[string]any{"id": v.ID, "name": v.Name, "region": v.Region, "rate": v.Rate, "active": v.Active, "version": v.Version}
}
func money2(value string) string {
	d, err := ParseDecimal(value)
	if err != nil {
		return "0.00"
	}
	return amountString(d)
}
func rate2(value string) string {
	d, err := ParseDecimal(value)
	if err != nil {
		return "0.00"
	}
	return d.Round(2).StringFixed(2)
}
func decimalFromInt(i int) decimal.Decimal { return decimal.NewFromInt(int64(i)) }
