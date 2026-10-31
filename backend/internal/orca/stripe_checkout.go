package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

const stripeCheckoutSessionsEndpoint = "https://api.stripe.com/v1/checkout/sessions"

type stripeCheckoutSessionCreator interface {
	CreateCheckoutSession(context.Context, string, stripeCheckoutSessionParams) (stripeCheckoutSession, error)
}

type stripeCheckoutSessionCreatorHTTP struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

type stripeCheckoutSessionParams struct {
	Attempt     StripeCheckout
	InvoiceNo   string
	SuccessURL  string
	CancelURL   string
	AmountMinor int64
}

type stripeCheckoutSession struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expires_at"`
}

func (p stripeCheckoutSessionCreatorHTTP) CreateCheckoutSession(ctx context.Context, idempotencyKey string, params stripeCheckoutSessionParams) (stripeCheckoutSession, error) {
	if !strings.HasPrefix(strings.TrimSpace(p.apiKey), "sk_test_") {
		return stripeCheckoutSession{}, errors.New("Stripe test secret key is not configured")
	}
	if p.endpoint == "" {
		p.endpoint = stripeCheckoutSessionsEndpoint
	}
	if p.client == nil {
		p.client = &http.Client{Timeout: 15 * time.Second}
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", params.SuccessURL)
	form.Set("cancel_url", params.CancelURL)
	form.Set("client_reference_id", params.Attempt.ID)
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", strings.ToLower(params.Attempt.Currency))
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(params.AmountMinor, 10))
	form.Set("line_items[0][price_data][product_data][name]", "ORCA invoice "+params.InvoiceNo)
	form.Set("metadata[orca_checkout_id]", params.Attempt.ID)
	form.Set("metadata[orca_invoice_id]", params.Attempt.InvoiceID)
	form.Set("metadata[orca_customer_id]", params.Attempt.CustomerID)
	form.Set("metadata[orca_business_id]", params.Attempt.BusinessID)
	form.Set("payment_intent_data[metadata][orca_checkout_id]", params.Attempt.ID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return stripeCheckoutSession{}, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.apiKey))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return stripeCheckoutSession{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return stripeCheckoutSession{}, fmt.Errorf("Stripe returned status %d", resp.StatusCode)
	}
	var result stripeCheckoutSession
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return stripeCheckoutSession{}, fmt.Errorf("decode Stripe Checkout response: %w", err)
	}
	return result, nil
}

func (a *API) createPortalCheckout(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if auth.Actor.Role != "customer" {
		writeErr(w, ErrForbidden)
		return
	}
	if !strings.HasPrefix(strings.TrimSpace(a.Config.StripeAPIKey), "sk_test_") {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Message: "Stripe test checkout is not configured"})
		return
	}
	invoiceID := chi.URLParam(r, "id")
	now := time.Now().UTC()
	var attempt StripeCheckout
	var invoiceNumber string
	var needsProvider bool
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		customer, ok := st.Customers[auth.Actor.CustomerID]
		if !ok || customer.BusinessID != auth.Actor.BusinessID || customer.UserID != auth.Actor.UserID {
			return ErrNotFound
		}
		inv, ok := st.Invoices[invoiceID]
		if !ok || inv.BusinessID != auth.Actor.BusinessID || inv.CustomerID != customer.ID {
			return ErrNotFound
		}
		if inv.Status != "Confirmed" {
			return ErrConflict
		}
		if inv.Currency != CurrencyINR {
			return fmt.Errorf("%w: Stripe Checkout supports INR invoices only", ErrValidation)
		}
		outstanding, err := ParseDecimal(inv.Outstanding)
		if err != nil || !outstanding.IsPositive() {
			return fmt.Errorf("%w: invoice has no payable outstanding balance", ErrConflict)
		}
		if _, err := paiseAmount(outstanding); err != nil {
			return err
		}
		invoiceNumber = inv.Number

		var latest StripeCheckout
		for _, existing := range st.StripeCheckouts {
			if existing.BusinessID != auth.Actor.BusinessID || existing.InvoiceID != inv.ID {
				continue
			}
			if existing.CreatedAt.After(latest.CreatedAt) {
				latest = existing
			}
		}
		if latest.ID != "" {
			switch latest.Status {
			case "Creating", "Open", "Processing":
				if latest.Amount != amountString(outstanding) || latest.Currency != CurrencyINR {
					return fmt.Errorf("%w: invoice balance changed while a Stripe Checkout session is active", ErrConflict)
				}
				attempt = latest
				if latest.Status == "Open" || latest.Status == "Processing" {
					if latest.URL != "" && checkoutURLAllowed(latest.URL) {
						return nil
					}
				}
				needsProvider = true
				return nil
			case "ReconciliationRequired":
				return fmt.Errorf("%w: the prior Stripe Checkout session requires reconciliation", ErrConflict)
			case "Completed":
				return ErrConflict
			}
		}
		id, err := newUUID()
		if err != nil {
			return err
		}
		attempt = StripeCheckout{
			ID:             id,
			BusinessID:     inv.BusinessID,
			InvoiceID:      inv.ID,
			CustomerID:     customer.ID,
			Amount:         amountString(outstanding),
			Currency:       CurrencyINR,
			IdempotencyKey: "orca-checkout:" + id,
			Status:         "Creating",
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		st.StripeCheckouts[attempt.ID] = attempt
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "stripe.checkout_requested", "invoice", inv.ID, map[string]any{"checkoutId": attempt.ID, "amount": attempt.Amount, "currency": attempt.Currency}))
		needsProvider = true
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	if !needsProvider {
		writeCheckoutResponse(w, attempt)
		return
	}
	amount, err := ParseDecimal(attempt.Amount)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Message: "Unable to calculate the invoice balance"})
		return
	}
	minor, err := paiseAmount(amount)
	if err != nil {
		writeErr(w, err)
		return
	}
	successURL, cancelURL, err := stripePortalReturnURLs(a.Config.PublicURL, invoiceID)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Message: "Stripe Checkout return URLs are not configured"})
		return
	}
	provider := a.stripe
	if provider == nil {
		provider = stripeCheckoutSessionCreatorHTTP{apiKey: a.Config.StripeAPIKey, endpoint: stripeCheckoutSessionsEndpoint, client: &http.Client{Timeout: 15 * time.Second}}
	}
	session, err := provider.CreateCheckoutSession(r.Context(), attempt.IdempotencyKey, stripeCheckoutSessionParams{
		Attempt: attempt, InvoiceNo: invoiceNumber, SuccessURL: successURL, CancelURL: cancelURL, AmountMinor: minor,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, apiError{Message: "Stripe Checkout session could not be created; retry with the same invoice"})
		return
	}
	if !strings.HasPrefix(session.ID, "cs_test_") || !checkoutURLAllowed(session.URL) {
		writeJSON(w, http.StatusBadGateway, apiError{Message: "Stripe returned an invalid Checkout session"})
		return
	}
	if session.ExpiresAt > 0 {
		expires := time.Unix(session.ExpiresAt, 0).UTC()
		attempt.ExpiresAt = &expires
	}
	attempt.ProviderSessionID = session.ID
	attempt.URL = session.URL
	if err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		current, ok := st.StripeCheckouts[attempt.ID]
		if !ok || current.IdempotencyKey != attempt.IdempotencyKey {
			return ErrConflict
		}
		if current.ProviderSessionID != "" && current.ProviderSessionID != session.ID {
			return ErrConflict
		}
		current.ProviderSessionID = session.ID
		current.URL = session.URL
		current.ExpiresAt = attempt.ExpiresAt
		if current.Status != "Completed" && current.Status != "ReconciliationRequired" {
			current.Status = "Open"
		}
		current.UpdatedAt = time.Now().UTC()
		st.StripeCheckouts[current.ID] = current
		attempt = current
		return nil
	}); err != nil {
		writeJSON(w, http.StatusBadGateway, apiError{Message: "Stripe Checkout session was created but could not be saved; retry with the same invoice"})
		return
	}
	writeCheckoutResponse(w, attempt)
}

func writeCheckoutResponse(w http.ResponseWriter, attempt StripeCheckout) {
	writeJSON(w, http.StatusOK, map[string]string{
		"sessionId": attempt.ProviderSessionID,
		"url":       attempt.URL,
		"amount":    attempt.Amount,
		"currency":  attempt.Currency,
	})
}

func paiseAmount(amount decimal.Decimal) (int64, error) {
	paise := amount.Mul(decimal.NewFromInt(100))
	if !amount.IsPositive() || !paise.Equal(paise.Truncate(0)) || !paise.IsInteger() || !paise.BigInt().IsInt64() {
		return 0, fmt.Errorf("%w: invoice outstanding balance must be positive INR paise", ErrValidation)
	}
	return paise.IntPart(), nil
}

func checkoutURLAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Scheme != "https" || u.Hostname() != "checkout.stripe.com" || u.User != nil || u.Port() != "" || !strings.HasPrefix(u.Path, "/c/pay/") {
		return false
	}
	return true
}

func stripePortalReturnURLs(publicURL, invoiceID string) (string, string, error) {
	base, err := url.Parse(strings.TrimSpace(publicURL))
	if err != nil || !base.IsAbs() || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil {
		return "", "", errors.New("invalid public URL")
	}
	if base.RawQuery != "" || base.Fragment != "" {
		return "", "", errors.New("public URL cannot include query or fragment")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/portal/invoices/" + url.PathEscape(invoiceID)
	success := *base
	cancel := *base
	success.RawQuery = "checkout=success"
	cancel.RawQuery = "checkout=cancelled"
	return success.String(), cancel.String(), nil
}
