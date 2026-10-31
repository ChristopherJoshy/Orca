package orca

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

func (a *API) readTenant(ctx context.Context, actor Actor) (*State, error) {
	var state *State
	err := a.Store.ReadFor(ctx, actor.BusinessID, func(s *State) error { state = s; return nil })
	return state, err
}
func checkActorStaff(auth Authenticated) error { return requireStaff(auth) }
func validateRate(value string) error {
	d, err := ParseDecimal(value)
	if err != nil || d.IsNegative() || d.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("%w: percentage must be between 0 and 100", ErrValidation)
	}
	return nil
}
func validatePrice(value string) error {
	d, err := ParseDecimal(value)
	if err != nil || d.IsNegative() {
		return fmt.Errorf("%w: amount must be a non-negative decimal string", ErrValidation)
	}
	return nil
}
func (a *API) createProduct(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req struct {
		Product
		Currency string
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	v := req.Product
	// Archive state is controlled only by the archive endpoint, not client create payloads.
	v.Archived = false
	if strings.TrimSpace(v.Name) == "" {
		writeErr(w, fmt.Errorf("%w: name is required", ErrValidation))
		return
	}
	if v.SalesPrice == "" {
		v.SalesPrice = "0.00"
	}
	if v.CostPrice == "" {
		v.CostPrice = "0.00"
	}
	if v.TaxRate == "" {
		v.TaxRate = "0"
	}
	for _, m := range []string{v.SalesPrice, v.CostPrice} {
		if err := validatePrice(m); err != nil {
			writeErr(w, err)
			return
		}
	}
	v.SalesPrice = money2(v.SalesPrice)
	v.CostPrice = money2(v.CostPrice)
	if err := validateRate(v.TaxRate); err != nil {
		writeErr(w, err)
		return
	}
	v.TaxRate = rate2(v.TaxRate)
	for i := range v.Variants {
		if v.Variants[i].Adjustment == "" {
			v.Variants[i].Adjustment = "0.00"
		}
		if err := validatePrice(v.Variants[i].Adjustment); err != nil {
			writeErr(w, err)
			return
		}
		v.Variants[i].Adjustment = money2(v.Variants[i].Adjustment)
	}
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	v.ID = id
	v.BusinessID = auth.Actor.BusinessID
	v.Version = 1
	v.CreatedAt = time.Now().UTC()
	if v.Variants == nil {
		v.Variants = []Variant{}
	}
	err = a.Store.UpdateFor(r.Context(), v.BusinessID, func(st *State) error { st.Products[v.ID] = v; return nil })
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, productWire(v))
}
func (a *API) createPlan(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var v Plan
	if err := decodeBody(r, &v); err != nil {
		writeErr(w, err)
		return
	}
	// Archive state is controlled only by the archive endpoint, not client create payloads.
	v.Archived = false
	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" {
		writeErr(w, fmt.Errorf("%w: name is required", ErrValidation))
		return
	}
	if v.Interval == "" {
		v.Interval = stringCadence(v.Interval)
	}
	if !oneOf(v.Interval, "daily", "weekly", "monthly", "yearly") {
		writeErr(w, fmt.Errorf("%w: interval must be daily, weekly, monthly, or yearly", ErrValidation))
		return
	}
	if v.SalesPrice == "" {
		v.SalesPrice = "0.00"
	}
	if err := validatePrice(v.SalesPrice); err != nil {
		writeErr(w, err)
		return
	}
	v.SalesPrice = money2(v.SalesPrice)
	if v.TaxRate == "" {
		v.TaxRate = "0"
	}
	if err := validateRate(v.TaxRate); err != nil {
		writeErr(w, err)
		return
	}
	v.TaxRate = rate2(v.TaxRate)
	if v.AvailableFrom != "" {
		if _, err := time.Parse("2006-01-02", v.AvailableFrom); err != nil {
			writeErr(w, fmt.Errorf("%w: availableFrom must use YYYY-MM-DD", ErrValidation))
			return
		}
	}
	if v.AvailableUntil != "" {
		if _, err := time.Parse("2006-01-02", v.AvailableUntil); err != nil {
			writeErr(w, fmt.Errorf("%w: availableUntil must use YYYY-MM-DD", ErrValidation))
			return
		}
	}
	if v.AvailableFrom != "" && v.AvailableUntil != "" && v.AvailableUntil < v.AvailableFrom {
		writeErr(w, fmt.Errorf("%w: availableUntil must not precede availableFrom", ErrValidation))
		return
	}
	if v.MinimumQuantity < 1 {
		v.MinimumQuantity = 1
	}
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	v.ID = id
	v.BusinessID = auth.Actor.BusinessID
	v.Version = 1
	v.CreatedAt = time.Now().UTC()
	if v.AllowPause == false && v.AllowEarlyClose == false && v.AllowRenew == false && v.AutoClose == false { // zero-valued policy flags are deliberate
	}
	err = a.Store.UpdateFor(r.Context(), v.BusinessID, func(st *State) error { st.Plans[v.ID] = v; return nil })
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, planWire(v))
}
func stringCadence(v string) string { return strings.ToLower(v) }
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func (a *API) createCustomer(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := checkActorStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	var v Customer
	if err := decodeBody(r, &v); err != nil {
		writeErr(w, err)
		return
	}
	v.Name = strings.TrimSpace(v.Name)
	email, err := validateEmail(v.Email)
	if err != nil {
		writeErr(w, err)
		return
	}
	if v.Name == "" {
		writeErr(w, fmt.Errorf("%w: name is required", ErrValidation))
		return
	}
	v.Email = email
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	v.ID = id
	v.BusinessID = auth.Actor.BusinessID
	v.CreatedAt = time.Now().UTC()
	if v.Address == nil {
		v.Address = map[string]string{}
	}
	err = a.Store.UpdateFor(r.Context(), v.BusinessID, func(st *State) error {
		for _, c := range st.Customers {
			if strings.EqualFold(c.Email, v.Email) {
				return ErrConflict
			}
		}
		st.Customers[v.ID] = v
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, v)
}
func (a *API) createDiscount(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var v Discount
	if err := decodeBody(r, &v); err != nil {
		writeErr(w, err)
		return
	}
	v.Name = strings.TrimSpace(v.Name)
	v.Kind = strings.ToLower(v.Kind)
	v.Scope = strings.ToLower(v.Scope)
	if v.Name == "" || !oneOf(v.Kind, "fixed", "percentage") {
		writeErr(w, fmt.Errorf("%w: name and fixed or percentage kind are required", ErrValidation))
		return
	}
	if err := validatePrice(v.Value); err != nil {
		writeErr(w, err)
		return
	}
	if v.Kind == "percentage" {
		if err := validateRate(v.Value); err != nil {
			writeErr(w, err)
			return
		}
	}
	if v.MinimumPurchase == "" {
		v.MinimumPurchase = "0.00"
	}
	if err := validatePrice(v.MinimumPurchase); err != nil {
		writeErr(w, err)
		return
	}
	if v.Scope == "" {
		v.Scope = "subscription"
	}
	if !oneOf(v.Scope, "product", "subscription") {
		writeErr(w, fmt.Errorf("%w: scope must be product or subscription", ErrValidation))
		return
	}
	if v.MaxUses < 0 {
		writeErr(w, fmt.Errorf("%w: maxUses cannot be negative", ErrValidation))
		return
	}
	if v.MinimumQuantity < 1 {
		v.MinimumQuantity = 1
	}
	if v.StartsAt != nil && v.EndsAt != nil && v.EndsAt.Before(*v.StartsAt) {
		writeErr(w, fmt.Errorf("%w: endsAt must not precede startsAt", ErrValidation))
		return
	}
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	v.ID = id
	v.BusinessID = auth.Actor.BusinessID
	v.Version = 1
	v.UseCount = 0
	if v.ProductIDs == nil {
		v.ProductIDs = []string{}
	}
	err = a.Store.UpdateFor(r.Context(), v.BusinessID, func(st *State) error { st.Discounts[v.ID] = v; return nil })
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, discountWire(v))
}
func (a *API) createTax(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var v TaxRule
	if err := decodeBody(r, &v); err != nil {
		writeErr(w, err)
		return
	}
	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" {
		writeErr(w, fmt.Errorf("%w: name is required", ErrValidation))
		return
	}
	if v.Rate == "" {
		v.Rate = "0"
	}
	if err := validateRate(v.Rate); err != nil {
		writeErr(w, err)
		return
	}
	v.Rate = rate2(v.Rate)
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	v.ID = id
	v.BusinessID = auth.Actor.BusinessID
	v.Version = 1
	v.CreatedAt = time.Now().UTC()
	err = a.Store.UpdateFor(r.Context(), v.BusinessID, func(st *State) error { st.Taxes[v.ID] = v; return nil })
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, taxWire(v))
}

func (a *API) createQuotationTemplate(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := checkActorStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req struct {
		Name            string
		PlanID          string
		ProductID       string
		ProductVariant  string
		Quantity        int
		ProductQuantity int
		ValidDays       int
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, fmt.Errorf("%w: name is required", ErrValidation))
		return
	}
	if req.ValidDays < 1 || req.ValidDays > 3650 {
		writeErr(w, fmt.Errorf("%w: validDays must be between 1 and 3650", ErrValidation))
		return
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}
	if req.ProductQuantity < 1 {
		req.ProductQuantity = 1
	}
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	now := time.Now().UTC()
	template := QuoteTemplate{ID: id, BusinessID: auth.Actor.BusinessID, Name: req.Name, PlanID: req.PlanID, ValidDays: req.ValidDays, CreatedAt: now, Lines: []SubscriptionLine{}}
	err = a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		plan, ok := st.Plans[req.PlanID]
		if !ok {
			return ErrNotFound
		}
		if req.Quantity < plan.MinimumQuantity {
			return fmt.Errorf("%w: quantity is below the plan minimum", ErrValidation)
		}
		lines := []SubscriptionLine{{Name: plan.Name, Quantity: req.Quantity}}
		if req.ProductID != "" {
			lines = append(lines, SubscriptionLine{ProductID: req.ProductID, Quantity: req.ProductQuantity, Variant: req.ProductVariant})
		} else if req.ProductVariant != "" {
			return fmt.Errorf("%w: productVariant requires productId", ErrValidation)
		}
		template.Lines, err = canonicalCommercialLines(st, plan, lines, req.Quantity)
		if err != nil {
			return err
		}
		st.QuoteTemplates[template.ID] = template
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, template)
}

func (a *API) createQuotation(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := checkActorStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req struct {
		CustomerID   string
		PlanID       string
		TemplateID   string
		Title        string
		Name         string
		ValidDays    int
		ValidityDays int
		Quantity     int
		Lines        []SubscriptionLine
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Title == "" {
		req.Title = req.Name
	}
	if req.ValidityDays > 0 {
		req.ValidDays = req.ValidityDays
	}
	if req.ValidDays < 0 {
		writeErr(w, fmt.Errorf("%w: validDays must be positive", ErrValidation))
		return
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	now := time.Now().UTC()
	v := Quotation{ID: id, BusinessID: auth.Actor.BusinessID, CustomerID: req.CustomerID, PlanID: req.PlanID, Title: strings.TrimSpace(req.Title), Status: "Draft", CreatedAt: now, Lines: req.Lines}
	err = a.Store.UpdateFor(r.Context(), v.BusinessID, func(st *State) error {
		if _, ok := st.Customers[v.CustomerID]; !ok {
			return ErrNotFound
		}
		if req.TemplateID != "" {
			template, ok := st.QuoteTemplates[req.TemplateID]
			if !ok {
				return ErrNotFound
			}
			templatePlan, exists := st.Plans[template.PlanID]
			if !exists || templatePlan.Archived {
				return ErrNotFound
			}
			for _, line := range template.Lines {
				if line.ProductID == "" {
					continue
				}
				product, exists := st.Products[line.ProductID]
				if !exists || product.BusinessID != auth.Actor.BusinessID || product.Archived {
					return ErrNotFound
				}
			}
			if req.PlanID != "" && req.PlanID != template.PlanID {
				return fmt.Errorf("%w: template plan cannot be overridden", ErrValidation)
			}
			v.PlanID = template.PlanID
			if req.ValidDays == 0 {
				req.ValidDays = template.ValidDays
			}
			if v.Title == "" {
				v.Title = template.Name
			}
			// A template is a saved offer. Ignore client-supplied line prices and
			// always copy its server-stored terms into the quotation.
			v.Lines = append([]SubscriptionLine(nil), template.Lines...)
		}
		if req.ValidDays == 0 {
			req.ValidDays = 30
		}
		if req.ValidDays < 1 || req.ValidDays > 3650 {
			return fmt.Errorf("%w: validDays must be between 1 and 3650", ErrValidation)
		}
		v.ValidUntil = now.AddDate(0, 0, req.ValidDays).Format("2006-01-02")
		if len(v.Lines) == 0 && v.PlanID != "" {
			plan, ok := st.Plans[v.PlanID]
			if !ok {
				return ErrNotFound
			}
			if req.Quantity < plan.MinimumQuantity {
				return fmt.Errorf("%w: quantity is below the plan minimum", ErrValidation)
			}
			v.Lines = []SubscriptionLine{{Name: plan.Name, Quantity: req.Quantity, UnitPrice: plan.SalesPrice, TaxRate: plan.TaxRate}}
		}
		if req.TemplateID == "" {
			plan, ok := st.Plans[v.PlanID]
			if !ok {
				return ErrNotFound
			}
			planQuantity := req.Quantity
			for _, line := range v.Lines {
				if line.ProductID == "" {
					planQuantity = line.Quantity
					break
				}
			}
			v.Lines, err = canonicalCommercialLines(st, plan, v.Lines, planQuantity)
			if err != nil {
				return err
			}
		}
		if v.Title == "" {
			if plan, ok := st.Plans[v.PlanID]; ok {
				v.Title = fmt.Sprintf("%s · %d", plan.Name, req.Quantity)
			} else {
				return fmt.Errorf("%w: title is required", ErrValidation)
			}
		}
		next := len(st.Quotations) + 1
		v.Number = fmt.Sprintf("QUO-%04d-%04d", now.Year(), next)
		if len(v.Lines) == 0 {
			return fmt.Errorf("%w: quotation requires at least one line", ErrValidation)
		}
		plan, ok := st.Plans[v.PlanID]
		if !ok || plan.Archived {
			return ErrNotFound
		}
		calculationLines := make([]CalculationInputLine, 0, len(v.Lines))
		for _, line := range v.Lines {
			if line.Quantity < 1 {
				return fmt.Errorf("%w: line quantity must be positive", ErrValidation)
			}
			if err := validatePrice(line.UnitPrice); err != nil {
				return err
			}
			if err := validateRate(line.TaxRate); err != nil {
				return err
			}
			product := st.Products[line.ProductID]
			if line.ProductID != "" && (product.ID == "" || product.BusinessID != auth.Actor.BusinessID || product.Archived) {
				return ErrNotFound
			}
			calculationLines = append(calculationLines, CalculationInputLine{
				Product: product, Quantity: line.Quantity, UnitPrice: line.UnitPrice, TaxRate: line.TaxRate,
				Description: line.Name, Variant: line.Variant, PlanID: v.PlanID, PlanVersion: plan.Version,
			})
		}
		business := st.Businesses[auth.Actor.BusinessID]
		snapshot, _, err := Calculate(calculationLines, nil, v.BusinessID, v.ID, now, now, business.Timezone)
		if err != nil {
			return fmt.Errorf("%w: quotation lines could not be calculated", ErrValidation)
		}
		v.Amount = snapshot.Total
		st.Quotations[v.ID] = v
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, quoteWire(v))
}

func (a *API) sendQuotation(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var quote Quotation
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		var ok bool
		quote, ok = st.Quotations[id]
		if !ok || (auth.Actor.Role == "customer" && quote.CustomerID != auth.Actor.CustomerID) {
			return ErrNotFound
		}
		if quote.Status != "Draft" {
			return ErrConflict
		}
		quote.Status = "Sent"
		st.Quotations[id] = quote
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "quotation.send_requested", "quotation", id, nil))
		st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: auth.Actor.BusinessID, Type: "quotation.email", IdempotencyKey: "quotation.email:" + id, Payload: map[string]any{"quotationId": id}, CreatedAt: time.Now().UTC()})
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 202, map[string]any{"queued": true, "quotation": quoteWire(quote)})
}

func (a *API) acceptQuotation(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if auth.Actor.Role != "customer" {
		writeErr(w, ErrForbidden)
		return
	}
	id := chi.URLParam(r, "id")
	var result Quotation
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		quote, ok := st.Quotations[id]
		if !ok || quote.CustomerID != auth.Actor.CustomerID {
			return ErrNotFound
		}
		if quote.Status != "Sent" {
			return ErrConflict
		}
		if quote.ValidUntil != "" {
			business := st.Businesses[auth.Actor.BusinessID]
			location, err := time.LoadLocation(business.Timezone)
			if err != nil {
				location = time.UTC
			}
			if quote.ValidUntil < time.Now().In(location).Format("2006-01-02") {
				return ErrConflict
			}
		}
		quote.Status = "Accepted"
		st.Quotations[id] = quote
		result = quote
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "quotation.accepted", "quotation", id, map[string]any{"number": quote.Number}))
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, quoteWire(result))
}
func (a *API) inviteStaff(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req struct {
		Email string
		Name  string
		Role  string
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	email, err := validateEmail(req.Email)
	if err != nil {
		writeErr(w, err)
		return
	}
	role := strings.ToLower(req.Role)
	if role == "" {
		role = "staff"
	}
	if role != "staff" {
		writeErr(w, fmt.Errorf("%w: invitations may only grant staff role", ErrValidation))
		return
	}
	var member Membership
	err = a.Store.Update(r.Context(), func(st *State) error {
		adminKey := membershipKey(auth.Actor.BusinessID, auth.Actor.UserID)
		admin, ok := st.Memberships[adminKey]
		if !ok || admin.RevokedAt != nil || admin.Role != "admin" {
			return ErrForbidden
		}
		var user User
		for _, u := range st.Users {
			if u.EmailNormalized == email {
				user = u
				break
			}
		}
		if user.ID == "" || !user.Verified {
			return fmt.Errorf("%w: team members must first create and verify an ORCA account", ErrValidation)
		}
		key := membershipKey(auth.Actor.BusinessID, user.ID)
		if old, ok := st.Memberships[key]; ok && old.RevokedAt == nil {
			return ErrConflict
		}
		member = Membership{BusinessID: auth.Actor.BusinessID, UserID: user.ID, Role: "staff"}
		st.Memberships[key] = member
		// Memberships use a composite map key, while audit_events.resource_id is a UUID.
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "staff.added", "membership", user.ID, map[string]any{"userId": user.ID, "role": "staff"}))
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": member.UserID, "email": email, "name": req.Name, "role": "staff", "status": "Active"})
}

func (a *API) updateResource(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	writeJSON(w, 501, apiError{Message: "Updates are not available on this endpoint yet"})
}
func (a *API) deleteResource(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	writeJSON(w, 501, apiError{Message: "Delete is not available on this endpoint yet"})
}

var _ = context.Background
