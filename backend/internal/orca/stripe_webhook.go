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
)

func (a *API) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if a.Config.StripeWebhookSecret == "" {
		writeJSON(w, 503, apiError{Message: "Stripe webhook is not configured"})
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, 413, apiError{Message: "Webhook payload too large"})
		return
	}
	if !verifyStripeSignature(r.Header.Get("Stripe-Signature"), raw, a.Config.StripeWebhookSecret, time.Now()) {
		writeJSON(w, 400, apiError{Message: "Invalid Stripe signature"})
		return
	}
	var event struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &event); err != nil || event.ID == "" {
		writeJSON(w, 400, apiError{Message: "Invalid Stripe event"})
		return
	}
	duplicate := false
	key := "stripe:" + event.ID
	err = a.Store.Update(r.Context(), func(st *State) error {
		if st.WebhookIDs[key] {
			duplicate = true
			return nil
		}
		st.WebhookIDs[key] = true
		st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), Type: "stripe.event_received", IdempotencyKey: "stripe.event:" + event.ID, Payload: map[string]any{"eventId": event.ID, "type": event.Type, "object": json.RawMessage(event.Data.Object)}, CreatedAt: time.Now().UTC()})
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"received": true, "duplicate": duplicate})
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
