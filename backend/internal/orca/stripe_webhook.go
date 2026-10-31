package orca

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type stripeWebhookEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type stripeCheckoutSessionEvent struct {
	ID                string            `json:"id"`
	ClientReferenceID string            `json:"client_reference_id"`
	Metadata          map[string]string `json:"metadata"`
	AmountTotal       *int64            `json:"amount_total"`
	Currency          string            `json:"currency"`
	PaymentStatus     string            `json:"payment_status"`
	ExpiresAt         int64             `json:"expires_at"`
}

func (a *API) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if a.Config.StripeWebhookSecret == "" {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Message: "Stripe webhook is not configured"})
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, apiError{Message: "Webhook payload too large"})
		return
	}
	if !verifyStripeSignature(r.Header.Get("Stripe-Signature"), raw, a.Config.StripeWebhookSecret, time.Now()) {
		writeJSON(w, http.StatusBadRequest, apiError{Message: "Invalid Stripe signature"})
		return
	}
	var event stripeWebhookEvent
	if err := json.Unmarshal(raw, &event); err != nil || event.ID == "" || event.Type == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Message: "Invalid Stripe event"})
		return
	}
	businessID := ""
	var session stripeCheckoutSessionEvent
	if json.Unmarshal(event.Data.Object, &session) == nil {
		err = a.Store.Read(r.Context(), func(st *State) error {
			match := findStripeCheckoutAttempt(st, session)
			if match.Attempt != nil {
				businessID = match.Attempt.BusinessID
			}
			return nil
		})
		if err != nil {
			writeErr(w, err)
			return
		}
	}
	var duplicate bool
	update := func(st *State) error {
		key := "stripe:" + event.ID
		if st.WebhookIDs[key] {
			duplicate = true
			return nil
		}
		st.WebhookIDs[key] = true
		return reconcileStripeEvent(st, event)
	}
	if businessID != "" {
		err = a.Store.UpdateFor(r.Context(), businessID, update)
	} else {
		err = a.Store.Update(r.Context(), update)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": true, "duplicate": duplicate})
}

func reconcileStripeEvent(st *State, event stripeWebhookEvent) error {
	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded", "checkout.session.expired", "checkout.session.async_payment_failed":
	default:
		return nil
	}
	var session stripeCheckoutSessionEvent
	if err := json.Unmarshal(event.Data.Object, &session); err != nil {
		if stripeSuccessEvent(event.Type) {
			return appendStripeReconciliation(st, event, stripeCheckoutSessionEvent{}, "invalid Stripe Checkout session object", "", "")
		}
		return nil
	}
	if event.Type == "checkout.session.completed" && session.PaymentStatus != "paid" {
		if session.PaymentStatus != "unpaid" {
			return nil
		}
		match := findStripeCheckoutAttempt(st, session)
		if match.Attempt == nil || match.Reason != "" || !strings.HasPrefix(session.ID, "cs_test_") {
			return nil
		}
		attempt := *match.Attempt
		if attempt.Status == "Completed" || attempt.Status == "ReconciliationRequired" {
			return nil
		}
		if attempt.ProviderSessionID != "" && attempt.ProviderSessionID != session.ID {
			return nil
		}
		attempt.ProviderSessionID = session.ID
		attempt.Status = "Processing"
		if session.ExpiresAt > 0 {
			expires := time.Unix(session.ExpiresAt, 0).UTC()
			attempt.ExpiresAt = &expires
		}
		attempt.UpdatedAt = time.Now().UTC()
		st.StripeCheckouts[attempt.ID] = attempt
		appendStripeBusinessOutbox(st, attempt.BusinessID, "stripe.checkout_processing", "stripe.webhook:"+event.ID, map[string]any{"eventId": event.ID, "checkoutId": attempt.ID, "sessionId": session.ID})
		return nil
	}

	if event.Type == "checkout.session.expired" || event.Type == "checkout.session.async_payment_failed" {
		match := findStripeCheckoutAttempt(st, session)
		if match.Attempt == nil || match.Reason != "" || !strings.HasPrefix(session.ID, "cs_test_") {
			return nil
		}
		attempt := *match.Attempt
		if attempt.Status == "Completed" || attempt.Status == "ReconciliationRequired" || attempt.ProviderSessionID != "" && attempt.ProviderSessionID != session.ID {
			return nil
		}
		status, outboxType := "Expired", "stripe.checkout_expired"
		if event.Type == "checkout.session.async_payment_failed" {
			status, outboxType = "Failed", "stripe.checkout_async_payment_failed"
		}
		attempt.ProviderSessionID = session.ID
		attempt.Status = status
		if session.ExpiresAt > 0 {
			expires := time.Unix(session.ExpiresAt, 0).UTC()
			attempt.ExpiresAt = &expires
		}
		attempt.UpdatedAt = time.Now().UTC()
		st.StripeCheckouts[attempt.ID] = attempt
		appendStripeBusinessOutbox(st, attempt.BusinessID, outboxType, "stripe.webhook:"+event.ID, map[string]any{"eventId": event.ID, "checkoutId": attempt.ID, "sessionId": session.ID})
		return nil
	}

	if !stripeSuccessEvent(event.Type) {
		return nil
	}
	match := findStripeCheckoutAttempt(st, session)
	if match.Attempt == nil {
		return appendStripeReconciliation(st, event, session, "local checkout attempt not found", "", "")
	}
	attempt := *match.Attempt
	if attempt.Status == "Completed" {
		return nil
	}
	if match.Reason != "" {
		return markStripeReconciliation(st, event, session, attempt, match.Reason)
	}
	if !strings.HasPrefix(session.ID, "cs_test_") {
		return markStripeReconciliation(st, event, session, attempt, "provider session id is missing or is not a test session")
	}
	if attempt.ProviderSessionID != "" && attempt.ProviderSessionID != session.ID {
		return markStripeReconciliation(st, event, session, attempt, "provider session id does not match the stored checkout")
	}
	if checkoutMetadataMismatch(session, attempt) {
		return markStripeReconciliation(st, event, session, attempt, "provider metadata does not match the stored checkout")
	}
	if session.AmountTotal == nil {
		return markStripeReconciliation(st, event, session, attempt, "provider amount is missing")
	}
	attemptAmount, err := ParseDecimal(attempt.Amount)
	if err != nil {
		return markStripeReconciliation(st, event, session, attempt, "stored checkout amount is invalid")
	}
	amountMinor, err := paiseAmount(attemptAmount)
	if err != nil || *session.AmountTotal != amountMinor {
		return markStripeReconciliation(st, event, session, attempt, "provider amount does not match the stored checkout")
	}
	if !strings.EqualFold(session.Currency, attempt.Currency) || attempt.Currency != CurrencyINR {
		return markStripeReconciliation(st, event, session, attempt, "provider currency does not match the stored checkout")
	}
	invoice, ok := st.Invoices[attempt.InvoiceID]
	if !ok || invoice.BusinessID != attempt.BusinessID || invoice.CustomerID != attempt.CustomerID {
		return markStripeReconciliation(st, event, session, attempt, "stored checkout invoice is missing or mismatched")
	}
	if invoice.Status != "Confirmed" || invoice.Currency != CurrencyINR {
		return markStripeReconciliation(st, event, session, attempt, "invoice is not a confirmed INR receivable")
	}
	outstanding, err := ParseDecimal(invoice.Outstanding)
	if err != nil || !outstanding.Equal(attemptAmount) {
		return markStripeReconciliation(st, event, session, attempt, "invoice outstanding balance changed after checkout creation")
	}
	return applyStripePayment(st, event, session, attempt, invoice)
}

func stripeSuccessEvent(eventType string) bool {
	return eventType == "checkout.session.async_payment_succeeded" || eventType == "checkout.session.completed"
}

type stripeCheckoutAttemptMatch struct {
	Attempt *StripeCheckout
	Reason  string
}

func findStripeCheckoutAttempt(st *State, session stripeCheckoutSessionEvent) stripeCheckoutAttemptMatch {
	metadataID := strings.TrimSpace(session.Metadata["orca_checkout_id"])
	clientReferenceID := strings.TrimSpace(session.ClientReferenceID)
	byMetadata, metadataFound := st.StripeCheckouts[metadataID]
	byClientReference, clientReferenceFound := st.StripeCheckouts[clientReferenceID]
	var bySession StripeCheckout
	var sessionFound bool
	for _, attempt := range st.StripeCheckouts {
		if session.ID != "" && attempt.ProviderSessionID == session.ID {
			if sessionFound && bySession.ID != attempt.ID {
				return stripeCheckoutAttemptMatch{Reason: "provider session id maps to multiple local checkouts"}
			}
			bySession, sessionFound = attempt, true
		}
	}
	if metadataID != "" && clientReferenceID != "" && metadataID != clientReferenceID {
		switch {
		case metadataFound:
			return stripeCheckoutAttemptMatch{Attempt: &byMetadata, Reason: "provider checkout references conflict"}
		case clientReferenceFound:
			return stripeCheckoutAttemptMatch{Attempt: &byClientReference, Reason: "provider checkout references conflict"}
		case sessionFound:
			return stripeCheckoutAttemptMatch{Attempt: &bySession, Reason: "provider checkout references conflict"}
		default:
			return stripeCheckoutAttemptMatch{}
		}
	}
	if metadataFound && clientReferenceFound && byMetadata.ID != byClientReference.ID {
		return stripeCheckoutAttemptMatch{Attempt: &byMetadata, Reason: "provider checkout references map to different local attempts"}
	}
	byCheckout, checkoutFound := byMetadata, metadataFound
	if !checkoutFound {
		byCheckout, checkoutFound = byClientReference, clientReferenceFound
	}
	if checkoutFound && sessionFound && byCheckout.ID != bySession.ID {
		return stripeCheckoutAttemptMatch{Attempt: &byCheckout, Reason: "provider session and checkout reference map to different local attempts"}
	}
	if checkoutFound {
		return stripeCheckoutAttemptMatch{Attempt: &byCheckout}
	}
	if sessionFound {
		return stripeCheckoutAttemptMatch{Attempt: &bySession}
	}
	return stripeCheckoutAttemptMatch{}
}

func checkoutMetadataMismatch(session stripeCheckoutSessionEvent, attempt StripeCheckout) bool {
	if value := strings.TrimSpace(session.Metadata["orca_invoice_id"]); value != "" && value != attempt.InvoiceID {
		return true
	}
	if value := strings.TrimSpace(session.Metadata["orca_customer_id"]); value != "" && value != attempt.CustomerID {
		return true
	}
	if value := strings.TrimSpace(session.Metadata["orca_business_id"]); value != "" && value != attempt.BusinessID {
		return true
	}
	return false
}

func applyStripePayment(st *State, event stripeWebhookEvent, session stripeCheckoutSessionEvent, attempt StripeCheckout, invoice Invoice) error {
	operationKey := "stripe.session:" + session.ID
	for _, old := range st.Payments {
		if old.BusinessID != attempt.BusinessID || old.OperationKey != operationKey {
			continue
		}
		if old.InvoiceID != invoice.ID || old.Amount != attempt.Amount || old.Provider != "stripe" {
			return markStripeReconciliation(st, event, session, attempt, "existing Stripe payment operation conflicts with the checkout")
		}
		attempt.ProviderSessionID = session.ID
		attempt.Status = "Completed"
		attempt.UpdatedAt = time.Now().UTC()
		st.StripeCheckouts[attempt.ID] = attempt
		return nil
	}
	paymentID, err := newUUID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	payment := Payment{ID: paymentID, BusinessID: invoice.BusinessID, CustomerID: invoice.CustomerID, Method: "Stripe Checkout", Amount: attempt.Amount, ReceivedAt: now, Note: "Stripe Checkout session " + session.ID, Provider: "stripe", OperationKey: operationKey, InvoiceID: invoice.ID, CreatedAt: now}
	allocation := PaymentAllocation{BusinessID: invoice.BusinessID, PaymentID: payment.ID, InvoiceID: invoice.ID, Amount: payment.Amount, CreatedAt: now}
	st.Payments[payment.ID] = payment
	st.Allocations[payment.ID+":"+invoice.ID] = allocation
	paid := decimal.Zero
	for _, row := range st.Allocations {
		if row.InvoiceID != invoice.ID {
			continue
		}
		amount, err := ParseDecimal(row.Amount)
		if err != nil {
			return err
		}
		paid = paid.Add(amount)
	}
	total, err := ParseDecimal(invoice.Total)
	if err != nil {
		return err
	}
	invoice.Outstanding = amountString(maxZero(total.Sub(paid)))
	if invoice.Outstanding == "0.00" {
		invoice.Status = "Paid"
	}
	invoice.Version++
	st.Invoices[invoice.ID] = invoice
	if err := postBalanced(st, Actor{BusinessID: invoice.BusinessID}, "payment:"+operationKey, "payment", payment.ID, []posting{{"cash", payment.Amount, "0.00"}, {"accounts_receivable", "0.00", payment.Amount}}); err != nil {
		return err
	}
	st.Audit = append(st.Audit, auditEvent(Actor{BusinessID: invoice.BusinessID}, "payment.allocated", "payment", payment.ID, map[string]any{"invoiceId": invoice.ID, "amount": payment.Amount, "provider": "stripe", "sessionId": session.ID, "checkoutId": attempt.ID}))
	appendStripeBusinessOutbox(st, invoice.BusinessID, "payment.recorded", "payment.recorded:"+operationKey, map[string]any{"paymentId": payment.ID, "invoiceId": invoice.ID, "provider": "stripe", "sessionId": session.ID})
	attempt.ProviderSessionID = session.ID
	attempt.Status = "Completed"
	attempt.UpdatedAt = now
	st.StripeCheckouts[attempt.ID] = attempt
	return nil
}

func markStripeReconciliation(st *State, event stripeWebhookEvent, session stripeCheckoutSessionEvent, attempt StripeCheckout, reason string) error {
	if attempt.ProviderSessionID == "" && session.ID != "" {
		attempt.ProviderSessionID = session.ID
	}
	attempt.Status = "ReconciliationRequired"
	attempt.UpdatedAt = time.Now().UTC()
	st.StripeCheckouts[attempt.ID] = attempt
	return appendStripeReconciliation(st, event, session, reason, attempt.ID, attempt.BusinessID)
}

func appendStripeReconciliation(st *State, event stripeWebhookEvent, session stripeCheckoutSessionEvent, reason, checkoutID, businessID string) error {
	if _, exists := st.StripeReconciliations[event.ID]; exists {
		return nil
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	metadata := map[string]string{}
	for _, key := range []string{"orca_checkout_id", "orca_invoice_id", "orca_customer_id", "orca_business_id"} {
		if value := strings.TrimSpace(session.Metadata[key]); value != "" {
			metadata[key] = value
		}
	}
	payload, err := json.Marshal(map[string]any{
		"eventId":   event.ID,
		"eventType": event.Type,
		"session": map[string]any{
			"id": session.ID, "clientReferenceId": session.ClientReferenceID, "metadata": metadata,
			"amountTotal": session.AmountTotal, "currency": session.Currency, "paymentStatus": session.PaymentStatus,
		},
	})
	if err != nil {
		return err
	}
	record := StripeReconciliation{ID: id, ProviderEventID: event.ID, EventType: event.Type, SessionID: session.ID, CheckoutID: checkoutID, BusinessID: businessID, Reason: reason, Payload: payload, CreatedAt: time.Now().UTC()}
	st.StripeReconciliations[event.ID] = record
	if businessID != "" {
		appendStripeBusinessOutbox(st, businessID, "stripe.reconciliation_required", "stripe.reconciliation:"+event.ID, map[string]any{"eventId": event.ID, "sessionId": session.ID, "checkoutId": checkoutID, "reason": reason})
	}
	return nil
}

func appendStripeBusinessOutbox(st *State, businessID, eventType, idempotencyKey string, payload map[string]any) {
	if businessID == "" {
		return
	}
	for _, old := range st.Outbox {
		if old.BusinessID == businessID && old.IdempotencyKey == idempotencyKey {
			return
		}
	}
	st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: businessID, Type: eventType, IdempotencyKey: idempotencyKey, Payload: payload, CreatedAt: time.Now().UTC()})
}

func verifyStripeSignature(header string, payload []byte, secret string, now time.Time) bool {
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		if key == "t" {
			timestamp = value
		}
		if key == "v1" {
			signatures = append(signatures, value)
		}
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || len(signatures) == 0 {
		return false
	}
	when := time.Unix(unix, 0)
	if d := now.Sub(when); d > 5*time.Minute || d < -5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%s.", timestamp)
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	for _, signature := range signatures {
		provided, e := hex.DecodeString(signature)
		if e == nil && hmac.Equal(provided, expected) {
			return true
		}
	}
	return false
}
