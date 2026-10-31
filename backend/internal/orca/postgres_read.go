package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type pgQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func tenantQuery(query, businessID string) (string, []any) {
	if businessID == "" || businessID == "global" {
		return query, nil
	}
	return query + " WHERE business_id=$1", []any{businessID}
}

func loadState(ctx context.Context, q pgQueryer, businessID string) (*State, error) {
	s := NewState()
	query, args := "SELECT id::text,name,currency,timezone,next_invoice_number FROM businesses", []any(nil)
	if businessID != "" && businessID != "global" {
		query += " WHERE id=$1"
		args = []any{businessID}
	}
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Business
		if err := rows.Scan(&v.ID, &v.Name, &v.Currency, &v.Timezone, &v.NextInvoiceNumber); err != nil {
			rows.Close()
			return nil, err
		}
		s.Businesses[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	qs := "SELECT encode(s.token_hash,'hex'),s.user_id::text,s.business_id::text,encode(s.csrf_hash,'hex'),s.created_at,s.last_seen_at,s.expires_at,m.role,COALESCE(m.customer_id::text,'') FROM sessions s JOIN memberships m ON m.user_id=s.user_id AND m.business_id=s.business_id"
	if businessID != "" && businessID != "global" {
		qs += " WHERE s.business_id=$1"
	}
	rows, err = q.Query(ctx, qs, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tokenHash, csrfHash string
		var v Session
		if err := rows.Scan(&tokenHash, &v.UserID, &v.BusinessID, &csrfHash, &v.CreatedAt, &v.LastSeenAt, &v.ExpiresAt, &v.Role, &v.CustomerID); err != nil {
			rows.Close()
			return nil, err
		}
		v.CSRFToken = csrfHash
		s.Sessions[tokenHash] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	qt := "SELECT encode(t.token_hash,'hex'),t.user_id::text,t.token_type,t.expires_at FROM auth_tokens t JOIN memberships m ON m.user_id=t.user_id"
	if businessID != "" && businessID != "global" {
		qt += " WHERE m.business_id=$1"
	}
	rows, err = q.Query(ctx, qt, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tokenHash, userID, kind string
		var expires time.Time
		if err := rows.Scan(&tokenHash, &userID, &kind, &expires); err != nil {
			rows.Close()
			return nil, err
		}
		token := AuthToken{UserID: userID, ExpiresAt: expires}
		if kind == "verify_email" {
			s.EmailVerifications[tokenHash] = token
		}
		if kind == "password_reset" {
			s.ResetTokens[tokenHash] = token
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	qm := "SELECT u.id::text,u.email,u.email_normalized,u.password_hash,(u.email_verified_at IS NOT NULL),u.created_at,m.business_id::text,m.role,COALESCE(m.customer_id::text,''),m.revoked_at FROM memberships m JOIN users u ON u.id=m.user_id"
	if businessID != "" && businessID != "global" {
		qm += " WHERE m.business_id=$1"
	}
	rows, err = q.Query(ctx, qm, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var u User
		var m Membership
		if err := rows.Scan(&u.ID, &u.Email, &u.EmailNormalized, &u.PasswordHash, &u.Verified, &u.CreatedAt, &m.BusinessID, &m.Role, &m.CustomerID, &m.RevokedAt); err != nil {
			rows.Close()
			return nil, err
		}
		m.UserID = u.ID
		s.Users[u.ID] = u
		s.Memberships[membershipKey(m.BusinessID, m.UserID)] = m
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,COALESCE(user_id::text,''),name,email,company,phone,billing_address,created_at FROM customers", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Customer
		var raw []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.UserID, &v.Name, &v.Email, &v.Company, &v.Phone, &raw, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Address); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode customer address: %w", err)
		}
		s.Customers[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,name,product_type,sales_price::text,cost_price::text,notes,variants,tax_rate::text,version,created_at,archived FROM products", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Product
		var raw []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.Name, &v.Type, &v.SalesPrice, &v.CostPrice, &v.Notes, &raw, &v.TaxRate, &v.Version, &v.CreatedAt, &v.Archived); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Variants); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode product variants: %w", err)
		}
		s.Products[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,name,interval,sales_price::text,minimum_quantity,COALESCE(available_from::text,''),COALESCE(available_until::text,''),tax_rate::text,auto_close,allow_early_close,allow_pause,allow_renew,version,created_at,archived FROM plans", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Plan
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.Name, &v.Interval, &v.SalesPrice, &v.MinimumQuantity, &v.AvailableFrom, &v.AvailableUntil, &v.TaxRate, &v.AutoClose, &v.AllowEarlyClose, &v.AllowPause, &v.AllowRenew, &v.Version, &v.CreatedAt, &v.Archived); err != nil {
			rows.Close()
			return nil, err
		}
		s.Plans[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,name,kind,value::text,min_purchase::text,min_quantity,starts_at,ends_at,COALESCE(max_uses,0),use_count,scope,product_ids::text[],version FROM discounts", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Discount
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.Name, &v.Kind, &v.Value, &v.MinimumPurchase, &v.MinimumQuantity, &v.StartsAt, &v.EndsAt, &v.MaxUses, &v.UseCount, &v.Scope, &v.ProductIDs, &v.Version); err != nil {
			rows.Close()
			return nil, err
		}
		s.Discounts[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,name,COALESCE(plan_id::text,''),valid_days,lines,created_at FROM quotation_templates", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v QuoteTemplate
		var raw []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.Name, &v.PlanID, &v.ValidDays, &raw, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Lines); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode quotation template lines: %w", err)
		}
		s.QuoteTemplates[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,customer_id::text,COALESCE(plan_id::text,''),number,title,status,COALESCE(valid_until::text,''),lines,amount::text,created_at FROM quotations", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Quotation
		var raw []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.CustomerID, &v.PlanID, &v.Number, &v.Title, &v.Status, &v.ValidUntil, &raw, &v.Amount, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Lines); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode quotation lines: %w", err)
		}
		s.Quotations[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,name,region,rate::text,active,version,created_at FROM tax_rules", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v TaxRule
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.Name, &v.Region, &v.Rate, &v.Active, &v.Version, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.Taxes[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	webhookRows, err := q.Query(ctx, "SELECT provider,event_id FROM webhook_events")
	if err != nil {
		return nil, err
	}
	for webhookRows.Next() {
		var provider, eventID string
		if err := webhookRows.Scan(&provider, &eventID); err != nil {
			webhookRows.Close()
			return nil, err
		}
		s.WebhookIDs[provider+":"+eventID] = true
	}
	if err := webhookRows.Err(); err != nil {
		webhookRows.Close()
		return nil, err
	}
	webhookRows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,customer_id::text,plan_id::text,COALESCE(discount_id::text,''),number,status,start_date::text,COALESCE(expiry_date::text,''),billing_cursor,payment_terms_days,lines,pause_intervals,version,created_at FROM subscriptions", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Subscription
		var raw, pause []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.CustomerID, &v.PlanID, &v.DiscountID, &v.Number, &v.Status, &v.StartDate, &v.ExpiryDate, &v.BillingCursor, &v.PaymentTermsDays, &raw, &pause, &v.Version, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Lines); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode subscription lines: %w", err)
		}
		if err := json.Unmarshal(pause, &v.PauseIntervals); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode subscription pauses: %w", err)
		}
		s.Subscriptions[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,COALESCE(subscription_id::text,''),customer_id::text,number,period_start,period_end,status,currency,subtotal::text,discount_total::text,tax_total::text,total::text,due_at,snapshot,calculation_hash,version,created_at FROM invoices", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Invoice
		var raw []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.SubscriptionID, &v.CustomerID, &v.Number, &v.PeriodStart, &v.PeriodEnd, &v.Status, &v.Currency, &v.Subtotal, &v.DiscountTotal, &v.TaxTotal, &v.Total, &v.DueAt, &raw, &v.CalculationHash, &v.Version, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Snapshot); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode snapshot: %w", err)
		}
		v.Outstanding = v.Total
		s.Invoices[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,customer_id::text,method,amount::text,received_at,note,provider,provider_operation_key,created_at FROM payments", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Payment
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.CustomerID, &v.Method, &v.Amount, &v.ReceivedAt, &v.Note, &v.Provider, &v.OperationKey, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.Payments[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT business_id::text,payment_id::text,invoice_id::text,amount::text,created_at FROM payment_allocations", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	allocated := map[string]decimal.Decimal{}
	for rows.Next() {
		var v PaymentAllocation
		if err := rows.Scan(&v.BusinessID, &v.PaymentID, &v.InvoiceID, &v.Amount, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.Allocations[v.PaymentID+":"+v.InvoiceID] = v
		d, _ := ParseDecimal(v.Amount)
		allocated[v.InvoiceID] = allocated[v.InvoiceID].Add(d)
		p := s.Payments[v.PaymentID]
		if p.InvoiceID == "" {
			p.InvoiceID = v.InvoiceID
			s.Payments[v.PaymentID] = p
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for id, inv := range s.Invoices {
		paid := allocated[id]
		total, _ := ParseDecimal(inv.Total)
		inv.Outstanding = amountString(maxZero(total.Sub(paid)))
		s.Invoices[id] = inv
	}

	query, args = tenantQuery("SELECT id::text,business_id::text,invoice_id::text,customer_id::text,amount::text,currency,idempotency_key,COALESCE(provider_session_id,''),checkout_url,status,expires_at,created_at,updated_at FROM stripe_checkouts", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v StripeCheckout
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.InvoiceID, &v.CustomerID, &v.Amount, &v.Currency, &v.IdempotencyKey, &v.ProviderSessionID, &v.URL, &v.Status, &v.ExpiresAt, &v.CreatedAt, &v.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.StripeCheckouts[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,provider_event_id,event_type,session_id,COALESCE(checkout_id::text,''),COALESCE(business_id::text,''),reason,payload,created_at FROM stripe_reconciliation_outbox", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v StripeReconciliation
		if err := rows.Scan(&v.ID, &v.ProviderEventID, &v.EventType, &v.SessionID, &v.CheckoutID, &v.BusinessID, &v.Reason, &v.Payload, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.StripeReconciliations[v.ProviderEventID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,operation_key,source_type,source_id::text,account,debit::text,credit::text,currency,created_at FROM journal_entries", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v JournalEntry
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.OperationKey, &v.SourceType, &v.SourceID, &v.Account, &v.Debit, &v.Credit, &v.Currency, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.Journal = append(s.Journal, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,actor_user_id::text,action,resource_type,COALESCE(resource_id::text,''),inputs_hash,resource_version,status,expires_at,COALESCE(decided_by::text,''),decided_at,created_at,inputs,execution_result FROM approvals", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v Approval
		var raw, result []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.ActorUserID, &v.Action, &v.ResourceType, &v.ResourceID, &v.InputsHash, &v.ResourceVersion, &v.Status, &v.ExpiresAt, &v.DecidedBy, &v.DecidedAt, &v.CreatedAt, &raw, &result); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Inputs); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode approval inputs: %w", err)
		}
		if err := json.Unmarshal(result, &v.ExecutionResult); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode approval execution result: %w", err)
		}
		s.Approvals[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,COALESCE(actor_user_id::text,''),action,resource_type,COALESCE(resource_id::text,''),details,created_at FROM audit_events", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v AuditEvent
		var raw []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.ActorID, &v.Action, &v.Resource, &v.ResourceID, &raw, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Details); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode audit details: %w", err)
		}
		s.Audit = append(s.Audit, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,event_type,idempotency_key,payload,created_at,delivered_at,attempts,COALESCE(last_error,'') FROM outbox_events", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v OutboxEvent
		var raw []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.Type, &v.IdempotencyKey, &raw, &v.CreatedAt, &v.DeliveredAt, &v.Attempts, &v.LastError); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Payload); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode outbox payload: %w", err)
		}
		s.Outbox = append(s.Outbox, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,user_id::text,conversation_id::text,status,message,transcript,outcome,created_at,updated_at FROM agent_runs", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v AgentRun
		var transcript, outcome []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.UserID, &v.ConversationID, &v.Status, &v.Message, &transcript, &outcome, &v.CreatedAt, &v.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(transcript, &v.Transcript); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode agent transcript: %w", err)
		}
		if err := json.Unmarshal(outcome, &v.Outcome); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode agent outcome: %w", err)
		}
		s.AgentRuns[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,user_id::text,client_id,scopes,encode(COALESCE(token_hash,''::bytea),'hex'),expires_at,revoked_at FROM oauth_grants", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v OAuthGrant
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.UserID, &v.ClientID, &v.Scopes, &v.TokenHash, &v.ExpiresAt, &v.RevokedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.OAuthGrants[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	query, args = tenantQuery("SELECT id::text,business_id::text,requested_by::text,status,filters,snapshot_at,manifest,COALESCE(archive_path,''),expires_at,created_at FROM export_jobs", businessID)
	rows, err = q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v ExportJob
		var filters, manifest []byte
		if err := rows.Scan(&v.ID, &v.BusinessID, &v.RequestedBy, &v.Status, &filters, &v.SnapshotAt, &manifest, &v.ArchivePath, &v.ExpiresAt, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(filters, &v.Filters); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode export filters: %w", err)
		}
		if err := json.Unmarshal(manifest, &v.Manifest); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode export manifest: %w", err)
		}
		s.Exports[v.ID] = v
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	normalizeState(s)
	return s, nil
}

func rawJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
