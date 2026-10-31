package orca

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"sort"
	"strings"
	"time"
)

const emailOutboxBatchSize = 50

func (a *API) emailDeliveryConfigured() bool {
	return a != nil && strings.TrimSpace(a.Config.ResendAPIKey) != "" && strings.TrimSpace(a.Config.EmailFrom) != ""
}

type emailOutboxLine struct {
	Description string
	Quantity    int
	UnitPrice   string
	Amount      string
}

type emailOutboxMessage struct {
	To            string
	Subject       string
	BusinessName  string
	CustomerName  string
	DocumentLabel string
	DocumentNo    string
	Status        string
	DateLabel     string
	DateValue     string
	Total         string
	Balance       string
	Lines         []emailOutboxLine
	PortalURL     string
}

var invoiceEmailTemplate = template.Must(template.New("invoice-email").Parse(`<!doctype html>
<html lang="en"><body style="margin:0;background:#f7f6f9;color:#272433;font:15px/1.5 Arial,sans-serif">
<main style="max-width:600px;margin:24px auto;padding:28px;background:#fff;border:1px solid #e8e5ec;border-radius:12px">
  <p style="margin:0 0 8px;color:#625d70">{{.BusinessName}}</p>
  <h1 style="font-size:22px;margin:0 0 18px">Invoice {{.DocumentNo}}</h1>
  <p>Hello {{.CustomerName}},</p>
  <p>Invoice {{.DocumentNo}} is {{.Status}}. {{.DateLabel}}: {{.DateValue}}.</p>
  {{if .Lines}}<table style="width:100%;border-collapse:collapse;margin:20px 0"><thead><tr><th align="left">Item</th><th align="right">Qty</th><th align="right">Amount</th></tr></thead><tbody>{{range .Lines}}<tr><td style="padding:8px 0;border-top:1px solid #eee">{{.Description}}</td><td align="right" style="padding:8px 0;border-top:1px solid #eee">{{.Quantity}}</td><td align="right" style="padding:8px 0;border-top:1px solid #eee">INR {{.Amount}}</td></tr>{{end}}</tbody></table>{{end}}
  <p>Total: <strong>INR {{.Total}}</strong></p><p>Outstanding: <strong>INR {{.Balance}}</strong></p>
  {{if .PortalURL}}<p style="margin-top:24px"><a href="{{.PortalURL}}">Open customer portal</a></p>{{end}}
</main></body></html>`))

var quotationEmailTemplate = template.Must(template.New("quotation-email").Parse(`<!doctype html>
<html lang="en"><body style="margin:0;background:#f7f6f9;color:#272433;font:15px/1.5 Arial,sans-serif">
<main style="max-width:600px;margin:24px auto;padding:28px;background:#fff;border:1px solid #e8e5ec;border-radius:12px">
  <p style="margin:0 0 8px;color:#625d70">{{.BusinessName}}</p>
  <h1 style="font-size:22px;margin:0 0 18px">Offer {{.DocumentNo}}</h1>
  <p>Hello {{.CustomerName}},</p>
  <p>{{.BusinessName}} prepared “{{.DocumentLabel}}” for your review. This offer is valid through {{.DateValue}}.</p>
  {{if .Lines}}<table style="width:100%;border-collapse:collapse;margin:20px 0"><thead><tr><th align="left">Item</th><th align="right">Qty</th><th align="right">Unit price</th></tr></thead><tbody>{{range .Lines}}<tr><td style="padding:8px 0;border-top:1px solid #eee">{{.Description}}</td><td align="right" style="padding:8px 0;border-top:1px solid #eee">{{.Quantity}}</td><td align="right" style="padding:8px 0;border-top:1px solid #eee">INR {{.UnitPrice}}</td></tr>{{end}}</tbody></table>{{end}}
  <p>Offer total: <strong>INR {{.Total}}</strong></p>
  {{if .PortalURL}}<p style="margin-top:24px"><a href="{{.PortalURL}}">Review offer in customer portal</a></p>{{end}}
</main></body></html>`))

// ProcessEmailOutbox sends only supported customer-facing events. The provider
// call is outside the database transaction; the event identity remains stable
// so a timeout after acceptance can be retried with Resend's idempotency key.
func (a *API) ProcessEmailOutbox(ctx context.Context) (int, error) {
	if strings.TrimSpace(a.Config.ResendAPIKey) == "" || strings.TrimSpace(a.Config.EmailFrom) == "" {
		return 0, nil
	}
	businessIDs, err := a.Store.BusinessIDs(ctx)
	if err != nil {
		return 0, err
	}
	delivered := 0
	var failures []error
	for _, businessID := range businessIDs {
		var pending []OutboxEvent
		err := a.Store.ReadFor(ctx, businessID, func(st *State) error {
			for _, event := range st.Outbox {
				if event.DeliveredAt == nil && supportedEmailEvent(event.Type) {
					pending = append(pending, event)
				}
			}
			return nil
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("read email outbox for business %s: %w", businessID, err))
			continue
		}
		sort.Slice(pending, func(i, j int) bool {
			if pending[i].CreatedAt.Equal(pending[j].CreatedAt) {
				return pending[i].ID < pending[j].ID
			}
			return pending[i].CreatedAt.Before(pending[j].CreatedAt)
		})
		if len(pending) > emailOutboxBatchSize {
			pending = pending[:emailOutboxBatchSize]
		}
		for _, event := range pending {
			if err := ctx.Err(); err != nil {
				return delivered, errors.Join(append(failures, err)...)
			}
			message, buildErr := a.buildEmailOutboxMessage(ctx, businessID, event)
			if buildErr != nil {
				if markErr := a.recordEmailOutboxFailure(ctx, businessID, event.ID, buildErr); markErr != nil {
					failures = append(failures, fmt.Errorf("record email event %s failure: %w", event.ID, markErr))
				}
				failures = append(failures, fmt.Errorf("prepare email event %s: %w", event.ID, buildErr))
				continue
			}
			if err := a.recordEmailOutboxAttempt(ctx, businessID, event.ID); err != nil {
				failures = append(failures, fmt.Errorf("record email event %s attempt: %w", event.ID, err))
				continue
			}
			htmlBody, err := renderEmailOutboxMessage(event.Type, message)
			if err == nil {
				err = a.sendResendEmail(ctx, message.To, message.Subject, htmlBody, resendOutboxIdempotencyKey(event))
			}
			if err != nil {
				if markErr := a.recordEmailOutboxFailure(ctx, businessID, event.ID, err); markErr != nil {
					failures = append(failures, fmt.Errorf("record email event %s failure: %w", event.ID, markErr))
				}
				failures = append(failures, fmt.Errorf("deliver email event %s: %w", event.ID, err))
				continue
			}
			if err := a.recordEmailOutboxDelivered(ctx, businessID, event.ID, time.Now().UTC()); err != nil {
				failures = append(failures, fmt.Errorf("record delivered email event %s: %w", event.ID, err))
				continue
			}
			delivered++
		}
	}
	return delivered, errors.Join(failures...)
}

func supportedEmailEvent(eventType string) bool {
	return eventType == "invoice.email" || eventType == "quotation.email"
}

func resendOutboxIdempotencyKey(event OutboxEvent) string {
	hash := sha256.Sum256([]byte(event.BusinessID + ":" + event.IdempotencyKey))
	return "orca-email:" + hex.EncodeToString(hash[:])
}

func (a *API) buildEmailOutboxMessage(ctx context.Context, businessID string, event OutboxEvent) (emailOutboxMessage, error) {
	var message emailOutboxMessage
	err := a.Store.ReadFor(ctx, businessID, func(st *State) error {
		business := st.Businesses[businessID]
		portalURL := customerPortalURL(a.Config.PublicURL)
		switch event.Type {
		case "invoice.email":
			id := payloadString(event.Payload, "invoiceId")
			invoice, ok := st.Invoices[id]
			if !ok || invoice.BusinessID != businessID {
				return ErrNotFound
			}
			customer, ok := st.Customers[invoice.CustomerID]
			if !ok || customer.BusinessID != businessID || strings.TrimSpace(customer.Email) == "" {
				return fmt.Errorf("invoice customer has no deliverable email address")
			}
			dateLabel, dateValue := "Due date", invoice.DueAt.UTC().Format("2 Jan 2006")
			if dateValue == "" {
				dateLabel, dateValue = "Billing period", invoice.PeriodStart.UTC().Format("2 Jan 2006")+" to "+invoice.PeriodEnd.UTC().Format("2 Jan 2006")
			}
			message = emailOutboxMessage{
				To: customer.Email, Subject: "Invoice " + invoice.Number + " from " + business.Name,
				BusinessName: business.Name, CustomerName: customer.Name, DocumentNo: invoice.Number,
				Status: invoice.Status, DateLabel: dateLabel, DateValue: dateValue,
				Total: invoice.Total, Balance: invoice.Outstanding, PortalURL: portalURL,
			}
			for _, line := range invoice.Snapshot.Lines {
				message.Lines = append(message.Lines, emailOutboxLine{Description: line.Description, Quantity: line.Quantity, UnitPrice: line.UnitPrice, Amount: line.LineTotal})
			}
		case "quotation.email":
			id := payloadString(event.Payload, "quotationId")
			quotation, ok := st.Quotations[id]
			if !ok || quotation.BusinessID != businessID {
				return ErrNotFound
			}
			customer, ok := st.Customers[quotation.CustomerID]
			if !ok || customer.BusinessID != businessID || strings.TrimSpace(customer.Email) == "" {
				return fmt.Errorf("quotation customer has no deliverable email address")
			}
			message = emailOutboxMessage{
				To: customer.Email, Subject: "Offer " + quotation.Number + " from " + business.Name,
				BusinessName: business.Name, CustomerName: customer.Name, DocumentLabel: quotation.Title,
				DocumentNo: quotation.Number, DateLabel: "Valid through", DateValue: quotation.ValidUntil,
				Total: quotation.Amount, PortalURL: portalURL,
			}
			for _, line := range quotation.Lines {
				message.Lines = append(message.Lines, emailOutboxLine{Description: line.Name, Quantity: line.Quantity, UnitPrice: line.UnitPrice, Amount: line.UnitPrice})
			}
		default:
			return fmt.Errorf("unsupported email outbox event %q", event.Type)
		}
		message.Subject = strings.Join(strings.Fields(message.Subject), " ")
		return nil
	})
	return message, err
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, _ := payload[key].(string)
	return value
}

func customerPortalURL(base string) string {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return ""
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/portal"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func renderEmailOutboxMessage(eventType string, message emailOutboxMessage) (string, error) {
	var body strings.Builder
	var tmpl *template.Template
	switch eventType {
	case "invoice.email":
		tmpl = invoiceEmailTemplate
	case "quotation.email":
		tmpl = quotationEmailTemplate
	default:
		return "", fmt.Errorf("unsupported email outbox event %q", eventType)
	}
	if err := tmpl.Execute(&body, message); err != nil {
		return "", err
	}
	return body.String(), nil
}

func (a *API) recordEmailOutboxAttempt(ctx context.Context, businessID, eventID string) error {
	return a.Store.UpdateFor(ctx, businessID, func(st *State) error {
		for i := range st.Outbox {
			if st.Outbox[i].ID == eventID {
				if st.Outbox[i].DeliveredAt != nil {
					return nil
				}
				st.Outbox[i].Attempts++
				st.Outbox[i].LastError = ""
				return nil
			}
		}
		return ErrNotFound
	})
}

func (a *API) recordEmailOutboxFailure(ctx context.Context, businessID, eventID string, cause error) error {
	message := cause.Error()
	if len(message) > 512 {
		message = message[:512]
	}
	return a.Store.UpdateFor(ctx, businessID, func(st *State) error {
		for i := range st.Outbox {
			if st.Outbox[i].ID == eventID {
				if st.Outbox[i].DeliveredAt != nil {
					return nil
				}
				st.Outbox[i].LastError = message
				return nil
			}
		}
		return ErrNotFound
	})
}

func (a *API) recordEmailOutboxDelivered(ctx context.Context, businessID, eventID string, at time.Time) error {
	return a.Store.UpdateFor(ctx, businessID, func(st *State) error {
		for i := range st.Outbox {
			if st.Outbox[i].ID == eventID {
				if st.Outbox[i].DeliveredAt == nil {
					st.Outbox[i].DeliveredAt = &at
				}
				st.Outbox[i].LastError = ""
				return nil
			}
		}
		return ErrNotFound
	})
}
