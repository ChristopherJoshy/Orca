package orca

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type productMutation struct {
	ExpectedVersion int64     `json:"expectedVersion"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	SalesPrice      string    `json:"salesPrice"`
	CostPrice       string    `json:"costPrice"`
	Notes           string    `json:"notes"`
	Variants        []Variant `json:"variants"`
	TaxRate         string    `json:"taxRate"`
}

type planMutation struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Name            string `json:"name"`
	Interval        string `json:"interval"`
	SalesPrice      string `json:"salesPrice"`
	MinimumQuantity int    `json:"minimumQuantity"`
	AvailableFrom   string `json:"availableFrom"`
	AvailableUntil  string `json:"availableUntil"`
	TaxRate         string `json:"taxRate"`
	AutoClose       bool   `json:"autoClose"`
	AllowEarlyClose bool   `json:"allowEarlyClose"`
	AllowPause      bool   `json:"allowPause"`
	AllowRenew      bool   `json:"allowRenew"`
}

type archiveRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

func (a *API) updateProduct(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req productMutation
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if err := validateExpectedVersion(req.ExpectedVersion); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var result Product
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		current, ok := st.Products[id]
		if !ok {
			return ErrNotFound
		}
		if current.Archived {
			return fmt.Errorf("%w: archived products cannot be edited", ErrConflict)
		}
		if req.ExpectedVersion != current.Version {
			return ErrConflict
		}
		updated, err := normalizeProductMutation(req)
		if err != nil {
			return err
		}
		updated.ID = current.ID
		updated.BusinessID = current.BusinessID
		updated.Version = current.Version + 1
		updated.CreatedAt = current.CreatedAt
		st.Products[id] = updated
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "product.updated", "product", id, map[string]any{"version": updated.Version}))
		result = updated
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productWire(result))
}

func (a *API) updatePlan(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req planMutation
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if err := validateExpectedVersion(req.ExpectedVersion); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var result Plan
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		current, ok := st.Plans[id]
		if !ok {
			return ErrNotFound
		}
		if current.Archived {
			return fmt.Errorf("%w: archived plans cannot be edited", ErrConflict)
		}
		if req.ExpectedVersion != current.Version {
			return ErrConflict
		}
		updated, err := normalizePlanMutation(req)
		if err != nil {
			return err
		}
		updated.ID = current.ID
		updated.BusinessID = current.BusinessID
		updated.Version = current.Version + 1
		updated.CreatedAt = current.CreatedAt
		st.Plans[id] = updated
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "plan.updated", "plan", id, map[string]any{"version": updated.Version}))
		result = updated
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, planWire(result))
}

func (a *API) archiveProduct(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req archiveRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if err := validateExpectedVersion(req.ExpectedVersion); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var result Product
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		current, ok := st.Products[id]
		if !ok {
			return ErrNotFound
		}
		if current.Archived {
			result = current
			return nil
		}
		if req.ExpectedVersion != current.Version {
			return ErrConflict
		}
		current.Archived = true
		current.Version++
		st.Products[id] = current
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "product.archived", "product", id, map[string]any{"version": current.Version}))
		result = current
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productWire(result))
}

func (a *API) archivePlan(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req archiveRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if err := validateExpectedVersion(req.ExpectedVersion); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var result Plan
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		current, ok := st.Plans[id]
		if !ok {
			return ErrNotFound
		}
		if current.Archived {
			result = current
			return nil
		}
		if req.ExpectedVersion != current.Version {
			return ErrConflict
		}
		current.Archived = true
		current.Version++
		st.Plans[id] = current
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "plan.archived", "plan", id, map[string]any{"version": current.Version}))
		result = current
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, planWire(result))
}

func validateExpectedVersion(version int64) error {
	if version < 1 {
		return fmt.Errorf("%w: expectedVersion is required", ErrValidation)
	}
	return nil
}

func normalizeProductMutation(req productMutation) (Product, error) {
	v := Product{Name: strings.TrimSpace(req.Name), Type: strings.TrimSpace(req.Type), Notes: strings.TrimSpace(req.Notes), Variants: req.Variants, SalesPrice: req.SalesPrice, CostPrice: req.CostPrice, TaxRate: req.TaxRate}
	if v.Name == "" || v.Type == "" {
		return Product{}, fmt.Errorf("%w: name and type are required", ErrValidation)
	}
	if v.SalesPrice == "" || v.CostPrice == "" || v.TaxRate == "" {
		return Product{}, fmt.Errorf("%w: salesPrice, costPrice, and taxRate are required", ErrValidation)
	}
	if err := validatePrice(v.SalesPrice); err != nil {
		return Product{}, err
	}
	if err := validatePrice(v.CostPrice); err != nil {
		return Product{}, err
	}
	if err := validateRate(v.TaxRate); err != nil {
		return Product{}, err
	}
	v.SalesPrice, v.CostPrice, v.TaxRate = money2(v.SalesPrice), money2(v.CostPrice), rate2(v.TaxRate)
	seen := map[string]bool{}
	for i := range v.Variants {
		variant := &v.Variants[i]
		variant.Name, variant.Value = strings.TrimSpace(variant.Name), strings.TrimSpace(variant.Value)
		if variant.Name == "" && variant.Value == "" {
			return Product{}, fmt.Errorf("%w: each variant needs a name or value", ErrValidation)
		}
		key := variant.Value
		if key == "" {
			key = variant.Name
		}
		if seen[key] {
			return Product{}, fmt.Errorf("%w: variant values must be unique", ErrValidation)
		}
		seen[key] = true
		if variant.Adjustment == "" {
			variant.Adjustment = "0.00"
		}
		if err := validatePrice(variant.Adjustment); err != nil {
			return Product{}, err
		}
		variant.Adjustment = money2(variant.Adjustment)
	}
	if v.Variants == nil {
		v.Variants = []Variant{}
	}
	return v, nil
}

func normalizePlanMutation(req planMutation) (Plan, error) {
	v := Plan{Name: strings.TrimSpace(req.Name), Interval: strings.ToLower(strings.TrimSpace(req.Interval)), SalesPrice: req.SalesPrice, MinimumQuantity: req.MinimumQuantity, AvailableFrom: req.AvailableFrom, AvailableUntil: req.AvailableUntil, TaxRate: req.TaxRate, AutoClose: req.AutoClose, AllowEarlyClose: req.AllowEarlyClose, AllowPause: req.AllowPause, AllowRenew: req.AllowRenew}
	if v.Name == "" || !oneOf(v.Interval, "daily", "weekly", "monthly", "yearly") {
		return Plan{}, fmt.Errorf("%w: name and a supported interval are required", ErrValidation)
	}
	if v.SalesPrice == "" || v.TaxRate == "" {
		return Plan{}, fmt.Errorf("%w: salesPrice and taxRate are required", ErrValidation)
	}
	if err := validatePrice(v.SalesPrice); err != nil {
		return Plan{}, err
	}
	if err := validateRate(v.TaxRate); err != nil {
		return Plan{}, err
	}
	if v.MinimumQuantity < 1 {
		return Plan{}, fmt.Errorf("%w: minimumQuantity must be positive", ErrValidation)
	}
	for _, date := range []string{v.AvailableFrom, v.AvailableUntil} {
		if date != "" {
			parsed, err := time.Parse("2006-01-02", date)
			if err != nil || parsed.Format("2006-01-02") != date {
				return Plan{}, fmt.Errorf("%w: availability dates must use YYYY-MM-DD", ErrValidation)
			}
		}
	}
	if v.AvailableFrom != "" && v.AvailableUntil != "" && v.AvailableUntil < v.AvailableFrom {
		return Plan{}, fmt.Errorf("%w: availableUntil must not precede availableFrom", ErrValidation)
	}
	v.SalesPrice, v.TaxRate = money2(v.SalesPrice), rate2(v.TaxRate)
	return v, nil
}
