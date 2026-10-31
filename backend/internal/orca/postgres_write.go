package orca

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func updatePostgres(ctx context.Context, pool *pgxpool.Pool, businessID string, fn func(*State) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	lockKey := businessID
	if lockKey == "" {
		lockKey = "global"
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", lockKey); err != nil {
		return err
	}
	before, err := loadState(ctx, tx, businessID)
	if err != nil {
		return err
	}
	after, err := cloneState(before)
	if err != nil {
		return err
	}
	if err = fn(after); err != nil {
		return err
	}
	if err = persistStateDiff(ctx, tx, before, after); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func persistStateDiff(ctx context.Context, tx pgx.Tx, before, after *State) error {
	for _, v := range after.Businesses {
		old, ok := before.Businesses[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO businesses(id,name,currency,timezone,next_invoice_number) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,currency=EXCLUDED.currency,timezone=EXCLUDED.timezone,next_invoice_number=EXCLUDED.next_invoice_number", v.ID, v.Name, v.Currency, v.Timezone, v.NextInvoiceNumber); err != nil {
			return err
		}
	}
	for _, v := range after.Users {
		old, ok := before.Users[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		var verified any
		if v.Verified {
			verified = v.CreatedAt
		}
		if _, err := tx.Exec(ctx, "INSERT INTO users(id,email,email_normalized,password_hash,email_verified_at,created_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO UPDATE SET email=EXCLUDED.email,email_normalized=EXCLUDED.email_normalized,password_hash=EXCLUDED.password_hash,email_verified_at=EXCLUDED.email_verified_at", v.ID, v.Email, v.EmailNormalized, v.PasswordHash, verified, v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Memberships {
		old, ok := before.Memberships[membershipKey(v.BusinessID, v.UserID)]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO memberships(business_id,user_id,role,customer_id,revoked_at) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (business_id,user_id) DO UPDATE SET role=EXCLUDED.role,customer_id=EXCLUDED.customer_id,revoked_at=EXCLUDED.revoked_at", v.BusinessID, v.UserID, v.Role, nullString(v.CustomerID), v.RevokedAt); err != nil {
			return err
		}
	}
	for tokenHash, v := range after.Sessions {
		if old, ok := before.Sessions[tokenHash]; ok && reflect.DeepEqual(old, v) {
			continue
		}
		hash, err := hex.DecodeString(tokenHash)
		if err != nil {
			return err
		}
		csrf, err := hex.DecodeString(v.CSRFToken)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,business_id,csrf_hash,created_at,last_seen_at,expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (token_hash) DO UPDATE SET csrf_hash=EXCLUDED.csrf_hash,last_seen_at=EXCLUDED.last_seen_at,expires_at=EXCLUDED.expires_at", hash, v.UserID, v.BusinessID, csrf, v.CreatedAt, v.LastSeenAt, v.ExpiresAt); err != nil {
			return err
		}
	}
	for tokenHash := range before.Sessions {
		if _, ok := after.Sessions[tokenHash]; !ok {
			hash, err := hex.DecodeString(tokenHash)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", hash); err != nil {
				return err
			}
		}
	}
	for tokenHash, v := range after.EmailVerifications {
		if old, ok := before.EmailVerifications[tokenHash]; ok && reflect.DeepEqual(old, v) {
			continue
		}
		if err := persistAuthToken(ctx, tx, tokenHash, v, "verify_email"); err != nil {
			return err
		}
	}
	for tokenHash, v := range after.ResetTokens {
		if old, ok := before.ResetTokens[tokenHash]; ok && reflect.DeepEqual(old, v) {
			continue
		}
		if err := persistAuthToken(ctx, tx, tokenHash, v, "password_reset"); err != nil {
			return err
		}
	}
	for tokenHash := range before.EmailVerifications {
		if _, ok := after.EmailVerifications[tokenHash]; !ok {
			if err := deleteAuthToken(ctx, tx, tokenHash); err != nil {
				return err
			}
		}
	}
	for tokenHash := range before.ResetTokens {
		if _, ok := after.ResetTokens[tokenHash]; !ok {
			if err := deleteAuthToken(ctx, tx, tokenHash); err != nil {
				return err
			}
		}
	}
	for _, v := range after.Customers {
		old, ok := before.Customers[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO customers(id,business_id,user_id,name,email,company,phone,billing_address,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9) ON CONFLICT (id) DO UPDATE SET user_id=EXCLUDED.user_id,name=EXCLUDED.name,email=EXCLUDED.email,company=EXCLUDED.company,phone=EXCLUDED.phone,billing_address=EXCLUDED.billing_address", v.ID, v.BusinessID, nullString(v.UserID), v.Name, v.Email, v.Company, v.Phone, jsonBytes(v.Address), v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Products {
		old, ok := before.Products[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO products(id,business_id,name,product_type,sales_price,cost_price,notes,variants,tax_rate,version,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,product_type=EXCLUDED.product_type,sales_price=EXCLUDED.sales_price,cost_price=EXCLUDED.cost_price,notes=EXCLUDED.notes,variants=EXCLUDED.variants,tax_rate=EXCLUDED.tax_rate,version=EXCLUDED.version", v.ID, v.BusinessID, v.Name, v.Type, v.SalesPrice, v.CostPrice, v.Notes, jsonBytes(v.Variants), v.TaxRate, v.Version, v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Plans {
		old, ok := before.Plans[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO plans(id,business_id,name,interval,sales_price,minimum_quantity,available_from,available_until,tax_rate,auto_close,allow_early_close,allow_pause,allow_renew,version,created_at) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::date,NULLIF($8,'')::date,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,interval=EXCLUDED.interval,sales_price=EXCLUDED.sales_price,minimum_quantity=EXCLUDED.minimum_quantity,available_from=EXCLUDED.available_from,available_until=EXCLUDED.available_until,tax_rate=EXCLUDED.tax_rate,auto_close=EXCLUDED.auto_close,allow_early_close=EXCLUDED.allow_early_close,allow_pause=EXCLUDED.allow_pause,allow_renew=EXCLUDED.allow_renew,version=EXCLUDED.version", v.ID, v.BusinessID, v.Name, v.Interval, v.SalesPrice, v.MinimumQuantity, v.AvailableFrom, v.AvailableUntil, v.TaxRate, v.AutoClose, v.AllowEarlyClose, v.AllowPause, v.AllowRenew, v.Version, v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Discounts {
		old, ok := before.Discounts[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		var maxUses any
		if v.MaxUses > 0 {
			maxUses = v.MaxUses
		}
		if _, err := tx.Exec(ctx, "INSERT INTO discounts(id,business_id,name,kind,value,min_purchase,min_quantity,starts_at,ends_at,max_uses,use_count,scope,product_ids,version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,kind=EXCLUDED.kind,value=EXCLUDED.value,min_purchase=EXCLUDED.min_purchase,min_quantity=EXCLUDED.min_quantity,starts_at=EXCLUDED.starts_at,ends_at=EXCLUDED.ends_at,max_uses=EXCLUDED.max_uses,use_count=EXCLUDED.use_count,scope=EXCLUDED.scope,product_ids=EXCLUDED.product_ids,version=EXCLUDED.version", v.ID, v.BusinessID, v.Name, v.Kind, v.Value, v.MinimumPurchase, v.MinimumQuantity, v.StartsAt, v.EndsAt, maxUses, v.UseCount, v.Scope, v.ProductIDs, v.Version); err != nil {
			return err
		}
	}
	for _, v := range after.QuoteTemplates {
		old, ok := before.QuoteTemplates[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO quotation_templates(id,business_id,name,valid_days,lines,created_at) VALUES ($1,$2,$3,$4,$5::jsonb,$6) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,valid_days=EXCLUDED.valid_days,lines=EXCLUDED.lines", v.ID, v.BusinessID, v.Name, v.ValidDays, jsonBytes(v.Lines), v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Quotations {
		old, ok := before.Quotations[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO quotations(id,business_id,customer_id,plan_id,number,title,status,valid_until,lines,amount,created_at) VALUES ($1,$2,$3,NULLIF($4,'')::uuid,$5,$6,$7,NULLIF($8,'')::date,$9::jsonb,$10,$11) ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title,status=EXCLUDED.status,valid_until=EXCLUDED.valid_until,lines=EXCLUDED.lines,amount=EXCLUDED.amount,plan_id=EXCLUDED.plan_id", v.ID, v.BusinessID, v.CustomerID, v.PlanID, v.Number, v.Title, v.Status, v.ValidUntil, jsonBytes(v.Lines), money2(v.Amount), v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Taxes {
		old, ok := before.Taxes[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO tax_rules(id,business_id,name,region,rate,active,version,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,region=EXCLUDED.region,rate=EXCLUDED.rate,active=EXCLUDED.active,version=EXCLUDED.version", v.ID, v.BusinessID, v.Name, v.Region, v.Rate, v.Active, v.Version, v.CreatedAt); err != nil {
			return err
		}
	}
	for key, present := range after.WebhookIDs {
		if !present || before.WebhookIDs[key] {
			continue
		}
		provider, eventID, ok := strings.Cut(key, ":")
		if !ok {
			return fmt.Errorf("invalid webhook event key")
		}
		if _, err := tx.Exec(ctx, "INSERT INTO webhook_events(provider,event_id,payload,processed_at) VALUES ($1,$2,'{}'::jsonb,now()) ON CONFLICT (provider,event_id) DO NOTHING", provider, eventID); err != nil {
			return err
		}
	}
	for _, v := range after.Subscriptions {
		old, ok := before.Subscriptions[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO subscriptions(id,business_id,customer_id,plan_id,discount_id,number,status,start_date,expiry_date,billing_cursor,payment_terms_days,lines,pause_intervals,version,created_at) VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,$8::date,NULLIF($9,'')::date,$10,$11,$12::jsonb,$13::jsonb,$14,$15) ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status,billing_cursor=EXCLUDED.billing_cursor,payment_terms_days=EXCLUDED.payment_terms_days,lines=EXCLUDED.lines,pause_intervals=EXCLUDED.pause_intervals,version=EXCLUDED.version,expiry_date=EXCLUDED.expiry_date,discount_id=EXCLUDED.discount_id", v.ID, v.BusinessID, v.CustomerID, v.PlanID, v.DiscountID, v.Number, v.Status, v.StartDate, v.ExpiryDate, v.BillingCursor, v.PaymentTermsDays, jsonBytes(v.Lines), jsonBytes(v.PauseIntervals), v.Version, v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Invoices {
		old, ok := before.Invoices[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO invoices(id,business_id,subscription_id,customer_id,number,period_start,period_end,status,currency,subtotal,discount_total,tax_total,total,due_at,snapshot,calculation_hash,calculation_engine,version,created_at) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::jsonb,$16,$17,$18,$19) ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status,version=EXCLUDED.version", v.ID, v.BusinessID, v.SubscriptionID, v.CustomerID, v.Number, v.PeriodStart, v.PeriodEnd, v.Status, v.Currency, v.Subtotal, v.DiscountTotal, v.TaxTotal, v.Total, v.DueAt, jsonBytes(v.Snapshot), v.CalculationHash, v.Snapshot.EngineVersion, v.Version, v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Payments {
		old, ok := before.Payments[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO payments(id,business_id,customer_id,method,amount,received_at,note,provider,provider_operation_key,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (id) DO NOTHING", v.ID, v.BusinessID, v.CustomerID, v.Method, v.Amount, v.ReceivedAt, v.Note, v.Provider, v.OperationKey, v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Allocations {
		old, ok := before.Allocations[v.PaymentID+":"+v.InvoiceID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO payment_allocations(business_id,payment_id,invoice_id,amount,created_at) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (business_id,payment_id,invoice_id) DO NOTHING", v.BusinessID, v.PaymentID, v.InvoiceID, v.Amount, v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.StripeCheckouts {
		old, ok := before.StripeCheckouts[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO stripe_checkouts(id,business_id,invoice_id,customer_id,amount,currency,idempotency_key,provider_session_id,checkout_url,status,expires_at,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11,$12,$13) ON CONFLICT (id) DO UPDATE SET provider_session_id=NULLIF(EXCLUDED.provider_session_id,''),checkout_url=EXCLUDED.checkout_url,status=EXCLUDED.status,expires_at=EXCLUDED.expires_at,updated_at=EXCLUDED.updated_at", v.ID, v.BusinessID, v.InvoiceID, v.CustomerID, v.Amount, v.Currency, v.IdempotencyKey, v.ProviderSessionID, v.URL, v.Status, v.ExpiresAt, v.CreatedAt, v.UpdatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.StripeReconciliations {
		if _, ok := before.StripeReconciliations[v.ProviderEventID]; ok {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO stripe_reconciliation_outbox(id,provider_event_id,event_type,session_id,checkout_id,business_id,reason,payload,created_at) VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8::jsonb,$9) ON CONFLICT (provider_event_id) DO NOTHING", v.ID, v.ProviderEventID, v.EventType, v.SessionID, v.CheckoutID, v.BusinessID, v.Reason, string(v.Payload), v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Journal {
		if containsJournal(before.Journal, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO journal_entries(id,business_id,operation_key,source_type,source_id,account,debit,credit,currency,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (business_id,operation_key,account) DO NOTHING", v.ID, v.BusinessID, v.OperationKey, v.SourceType, v.SourceID, v.Account, v.Debit, v.Credit, v.Currency, v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Approvals {
		old, ok := before.Approvals[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO approvals(id,business_id,actor_user_id,action,resource_type,resource_id,inputs_hash,resource_version,status,expires_at,decided_by,decided_at,created_at,inputs,execution_result) VALUES ($1,$2,$3,$4,$5,NULLIF($6,'')::uuid,$7,$8,$9,$10,NULLIF($11,'')::uuid,$12,$13,$14::jsonb,$15::jsonb) ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status,decided_by=EXCLUDED.decided_by,decided_at=EXCLUDED.decided_at,execution_result=EXCLUDED.execution_result", v.ID, v.BusinessID, v.ActorUserID, v.Action, v.ResourceType, v.ResourceID, v.InputsHash, v.ResourceVersion, v.Status, v.ExpiresAt, v.DecidedBy, v.DecidedAt, v.CreatedAt, jsonBytes(v.Inputs), jsonBytes(v.ExecutionResult)); err != nil {
			return err
		}
	}
	for _, v := range after.Audit {
		if containsAudit(before.Audit, v.ID) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO audit_events(id,business_id,actor_user_id,action,resource_type,resource_id,details,created_at) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,NULLIF($6,'')::uuid,$7::jsonb,$8) ON CONFLICT (id) DO NOTHING", v.ID, v.BusinessID, v.ActorID, v.Action, v.Resource, v.ResourceID, jsonBytes(v.Details), v.CreatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Outbox {
		old, ok := findOutbox(before.Outbox, v.ID)
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO outbox_events(id,business_id,event_type,idempotency_key,payload,created_at,delivered_at,attempts,last_error) VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8,NULLIF($9,'')) ON CONFLICT (business_id,idempotency_key) DO UPDATE SET delivered_at=EXCLUDED.delivered_at,attempts=EXCLUDED.attempts,last_error=EXCLUDED.last_error", v.ID, v.BusinessID, v.Type, v.IdempotencyKey, jsonBytes(v.Payload), v.CreatedAt, v.DeliveredAt, v.Attempts, v.LastError); err != nil {
			return err
		}
	}
	for _, v := range after.AgentRuns {
		old, ok := before.AgentRuns[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO agent_runs(id,business_id,user_id,conversation_id,status,message,transcript,outcome,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10) ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status,transcript=EXCLUDED.transcript,outcome=EXCLUDED.outcome,updated_at=EXCLUDED.updated_at", v.ID, v.BusinessID, v.UserID, v.ConversationID, v.Status, v.Message, jsonBytes(v.Transcript), jsonBytes(v.Outcome), v.CreatedAt, v.UpdatedAt); err != nil {
			return err
		}
	}
	for _, v := range after.Exports {
		old, ok := before.Exports[v.ID]
		if ok && reflect.DeepEqual(old, v) {
			continue
		}
		if _, err := tx.Exec(ctx, "INSERT INTO export_jobs(id,business_id,requested_by,status,filters,snapshot_at,manifest,archive_path,expires_at,created_at) VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7::jsonb,NULLIF($8,''),$9,$10) ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status,snapshot_at=EXCLUDED.snapshot_at,manifest=EXCLUDED.manifest,archive_path=EXCLUDED.archive_path,expires_at=EXCLUDED.expires_at", v.ID, v.BusinessID, v.RequestedBy, v.Status, jsonBytes(v.Filters), v.SnapshotAt, jsonBytes(v.Manifest), v.ArchivePath, v.ExpiresAt, v.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func persistAuthToken(ctx context.Context, tx pgx.Tx, hash string, v AuthToken, kind string) error {
	b, err := hex.DecodeString(hash)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO auth_tokens(token_hash,user_id,token_type,expires_at) VALUES ($1,$2,$3,$4) ON CONFLICT (token_hash) DO UPDATE SET expires_at=EXCLUDED.expires_at,consumed_at=NULL", b, v.UserID, kind, v.ExpiresAt)
	return err
}

func deleteAuthToken(ctx context.Context, tx pgx.Tx, hash string) error {
	b, err := hex.DecodeString(hash)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM auth_tokens WHERE token_hash=$1", b)
	return err
}

func jsonBytes(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
func containsAudit(list []AuditEvent, id string) bool {
	for _, v := range list {
		if v.ID == id {
			return true
		}
	}
	return false
}
func containsJournal(list []JournalEntry, target JournalEntry) bool {
	for _, v := range list {
		if v.OperationKey == target.OperationKey && v.Account == target.Account {
			return true
		}
	}
	return false
}
func findOutbox(list []OutboxEvent, id string) (OutboxEvent, bool) {
	for _, v := range list {
		if v.ID == id {
			return v, true
		}
	}
	return OutboxEvent{}, false
}
