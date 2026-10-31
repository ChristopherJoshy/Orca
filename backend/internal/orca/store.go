package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type Store struct {
	pool  *pgxpool.Pool
	mu    sync.Mutex
	state *State
}

func OpenStore(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required; ORCA requires PostgreSQL")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	s := &Store{pool: pool, state: NewState()}
	if err := s.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
func (s *Store) IsPostgres() bool { return s.pool != nil }

func (s *Store) Migrate(ctx context.Context) error {
	if s.pool == nil {
		return errors.New("PostgreSQL is not configured")
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	db := stdlib.OpenDBFromPool(s.pool)
	defer db.Close()
	migrationDir := "migrations"
	if _, err := os.Stat(migrationDir); err != nil {
		if _, fallbackErr := os.Stat(filepath.Join("..", "..", "migrations")); fallbackErr == nil {
			migrationDir = filepath.Join("..", "..", "migrations")
		}
	}
	if err := goose.UpContext(ctx, db, migrationDir); err != nil {
		return fmt.Errorf("apply PostgreSQL migrations: %w", err)
	}
	return nil
}

func (s *Store) Read(ctx context.Context, fn func(*State) error) error {
	if s.pool != nil {
		return s.ReadFor(ctx, "", fn)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(s.state)
}

func (s *Store) ReadFor(ctx context.Context, businessID string, fn func(*State) error) error {
	if s.pool == nil {
		if businessID == "" || businessID == "global" {
			return s.Read(ctx, fn)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		scoped, err := stateForBusiness(s.state, businessID)
		if err != nil {
			return err
		}
		return fn(scoped)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	state, err := loadState(ctx, tx, businessID)
	if err != nil {
		return err
	}
	if err := fn(state); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Update commits a business-scoped change atomically to PostgreSQL. A pool-less
// in-memory store is used by unit tests only; OpenStore never creates one.
func (s *Store) Update(ctx context.Context, fn func(*State) error) error {
	if s.pool != nil {
		return updatePostgres(ctx, s.pool, "global", fn)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copy, err := cloneState(s.state)
	if err != nil {
		return err
	}
	if err := fn(copy); err != nil {
		return err
	}
	s.state = copy
	return nil
}

func (s *Store) UpdateFor(ctx context.Context, businessID string, fn func(*State) error) error {
	if s.pool == nil {
		if businessID == "" || businessID == "global" {
			return s.Update(ctx, fn)
		}
		return s.Update(ctx, func(all *State) error {
			scoped, err := stateForBusiness(all, businessID)
			if err != nil {
				return err
			}
			if err := fn(scoped); err != nil {
				return err
			}
			mergeBusinessState(all, scoped, businessID)
			return nil
		})
	}
	return updatePostgres(ctx, s.pool, businessID, fn)
}

func stateForBusiness(source *State, businessID string) (*State, error) {
	out := NewState()
	if business, ok := source.Businesses[businessID]; ok {
		out.Businesses[businessID] = business
	}
	userIDs := map[string]bool{}
	for key, member := range source.Memberships {
		if member.BusinessID == businessID {
			out.Memberships[key] = member
			userIDs[member.UserID] = true
		}
	}
	for id := range userIDs {
		if user, ok := source.Users[id]; ok {
			out.Users[id] = user
		}
	}
	for id, value := range source.Customers {
		if value.BusinessID == businessID {
			out.Customers[id] = value
		}
	}
	for id, value := range source.Products {
		if value.BusinessID == businessID {
			out.Products[id] = value
		}
	}
	for id, value := range source.Plans {
		if value.BusinessID == businessID {
			out.Plans[id] = value
		}
	}
	for id, value := range source.Discounts {
		if value.BusinessID == businessID {
			out.Discounts[id] = value
		}
	}
	for id, value := range source.QuoteTemplates {
		if value.BusinessID == businessID {
			out.QuoteTemplates[id] = value
		}
	}
	for id, value := range source.Quotations {
		if value.BusinessID == businessID {
			out.Quotations[id] = value
		}
	}
	for id, value := range source.Taxes {
		if value.BusinessID == businessID {
			out.Taxes[id] = value
		}
	}
	for id, value := range source.Subscriptions {
		if value.BusinessID == businessID {
			out.Subscriptions[id] = value
		}
	}
	for id, value := range source.Invoices {
		if value.BusinessID == businessID {
			out.Invoices[id] = value
		}
	}
	for id, value := range source.Payments {
		if value.BusinessID == businessID {
			out.Payments[id] = value
		}
	}
	for key, value := range source.Allocations {
		if value.BusinessID == businessID {
			out.Allocations[key] = value
		}
	}
	for id, value := range source.StripeCheckouts {
		if value.BusinessID == businessID {
			out.StripeCheckouts[id] = value
		}
	}
	for id, value := range source.StripeReconciliations {
		if value.BusinessID == businessID {
			out.StripeReconciliations[id] = value
		}
	}
	for _, value := range source.Journal {
		if value.BusinessID == businessID {
			out.Journal = append(out.Journal, value)
		}
	}
	for id, value := range source.Approvals {
		if value.BusinessID == businessID {
			out.Approvals[id] = value
		}
	}
	for key, value := range source.Sessions {
		if value.BusinessID == businessID {
			out.Sessions[key] = value
		}
	}
	for _, value := range source.Audit {
		if value.BusinessID == businessID {
			out.Audit = append(out.Audit, value)
		}
	}
	for _, value := range source.Outbox {
		if value.BusinessID == businessID {
			out.Outbox = append(out.Outbox, value)
		}
	}
	for id, value := range source.AgentRuns {
		if value.BusinessID == businessID {
			out.AgentRuns[id] = value
		}
	}
	for id, value := range source.OAuthGrants {
		if value.BusinessID == businessID {
			out.OAuthGrants[id] = value
		}
	}
	for id, value := range source.Exports {
		if value.BusinessID == businessID {
			out.Exports[id] = value
		}
	}
	for key, value := range source.EmailVerifications {
		if userIDs[value.UserID] {
			out.EmailVerifications[key] = value
		}
	}
	for key, value := range source.ResetTokens {
		if userIDs[value.UserID] {
			out.ResetTokens[key] = value
		}
	}
	for key, value := range source.WebhookIDs {
		out.WebhookIDs[key] = value
	}
	return out, nil
}

// mergeBusinessState updates only the selected business projection while
// preserving globally unique user accounts and all other workspaces.
func mergeBusinessState(all, scoped *State, businessID string) {
	if value, ok := scoped.Businesses[businessID]; ok {
		all.Businesses[businessID] = value
	}
	for key, value := range all.Memberships {
		if value.BusinessID == businessID {
			delete(all.Memberships, key)
		}
	}
	for key, value := range scoped.Memberships {
		all.Memberships[key] = value
	}
	for id, value := range scoped.Users {
		all.Users[id] = value
	}
	mergeByBusiness(all.Customers, scoped.Customers, businessID, func(v Customer) string { return v.BusinessID })
	mergeByBusiness(all.Products, scoped.Products, businessID, func(v Product) string { return v.BusinessID })
	mergeByBusiness(all.Plans, scoped.Plans, businessID, func(v Plan) string { return v.BusinessID })
	mergeByBusiness(all.Discounts, scoped.Discounts, businessID, func(v Discount) string { return v.BusinessID })
	mergeByBusiness(all.QuoteTemplates, scoped.QuoteTemplates, businessID, func(v QuoteTemplate) string { return v.BusinessID })
	mergeByBusiness(all.Quotations, scoped.Quotations, businessID, func(v Quotation) string { return v.BusinessID })
	mergeByBusiness(all.Taxes, scoped.Taxes, businessID, func(v TaxRule) string { return v.BusinessID })
	mergeByBusiness(all.Subscriptions, scoped.Subscriptions, businessID, func(v Subscription) string { return v.BusinessID })
	mergeByBusiness(all.Invoices, scoped.Invoices, businessID, func(v Invoice) string { return v.BusinessID })
	mergeByBusiness(all.Payments, scoped.Payments, businessID, func(v Payment) string { return v.BusinessID })
	mergeByBusiness(all.Allocations, scoped.Allocations, businessID, func(v PaymentAllocation) string { return v.BusinessID })
	mergeByBusiness(all.StripeCheckouts, scoped.StripeCheckouts, businessID, func(v StripeCheckout) string { return v.BusinessID })
	mergeByBusiness(all.StripeReconciliations, scoped.StripeReconciliations, businessID, func(v StripeReconciliation) string { return v.BusinessID })
	mergeByBusiness(all.Approvals, scoped.Approvals, businessID, func(v Approval) string { return v.BusinessID })
	mergeByBusiness(all.Sessions, scoped.Sessions, businessID, func(v Session) string { return v.BusinessID })
	mergeByBusiness(all.AgentRuns, scoped.AgentRuns, businessID, func(v AgentRun) string { return v.BusinessID })
	mergeByBusiness(all.OAuthGrants, scoped.OAuthGrants, businessID, func(v OAuthGrant) string { return v.BusinessID })
	mergeByBusiness(all.Exports, scoped.Exports, businessID, func(v ExportJob) string { return v.BusinessID })
	all.Journal = mergeSlice(all.Journal, scoped.Journal, businessID, func(v JournalEntry) string { return v.BusinessID })
	all.Audit = mergeSlice(all.Audit, scoped.Audit, businessID, func(v AuditEvent) string { return v.BusinessID })
	all.Outbox = mergeSlice(all.Outbox, scoped.Outbox, businessID, func(v OutboxEvent) string { return v.BusinessID })
	for key, value := range scoped.EmailVerifications {
		all.EmailVerifications[key] = value
	}
	for key, value := range scoped.ResetTokens {
		all.ResetTokens[key] = value
	}
	for key, value := range scoped.WebhookIDs {
		if value {
			all.WebhookIDs[key] = true
		}
	}
}

func mergeByBusiness[T any](all, scoped map[string]T, businessID string, get func(T) string) {
	for key, value := range all {
		if get(value) == businessID {
			delete(all, key)
		}
	}
	for key, value := range scoped {
		all[key] = value
	}
}
func mergeSlice[T any](all, scoped []T, businessID string, get func(T) string) []T {
	out := make([]T, 0, len(all)+len(scoped))
	for _, value := range all {
		if get(value) != businessID {
			out = append(out, value)
		}
	}
	return append(out, scoped...)
}

func cloneState(state *State) (*State, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	copy := NewState()
	if err := json.Unmarshal(data, copy); err != nil {
		return nil, err
	}
	// PasswordHash is intentionally excluded from JSON so state snapshots and
	// API responses cannot expose it. Restore the private field explicitly in
	// this internal deep copy, or unrelated updates would erase stored hashes.
	for id, user := range state.Users {
		copy.Users[id] = user
	}
	normalizeState(copy)
	return copy, nil
}

func normalizeState(s *State) {
	if s.Users == nil {
		s.Users = map[string]User{}
	}
	if s.Businesses == nil {
		s.Businesses = map[string]Business{}
	}
	if s.Memberships == nil {
		s.Memberships = map[string]Membership{}
	}
	if s.Customers == nil {
		s.Customers = map[string]Customer{}
	}
	if s.Products == nil {
		s.Products = map[string]Product{}
	}
	if s.Plans == nil {
		s.Plans = map[string]Plan{}
	}
	if s.Discounts == nil {
		s.Discounts = map[string]Discount{}
	}
	if s.QuoteTemplates == nil {
		s.QuoteTemplates = map[string]QuoteTemplate{}
	}
	if s.Quotations == nil {
		s.Quotations = map[string]Quotation{}
	}
	if s.Taxes == nil {
		s.Taxes = map[string]TaxRule{}
	}
	if s.Subscriptions == nil {
		s.Subscriptions = map[string]Subscription{}
	}
	if s.Invoices == nil {
		s.Invoices = map[string]Invoice{}
	}
	if s.Payments == nil {
		s.Payments = map[string]Payment{}
	}
	if s.Allocations == nil {
		s.Allocations = map[string]PaymentAllocation{}
	}
	if s.StripeCheckouts == nil {
		s.StripeCheckouts = map[string]StripeCheckout{}
	}
	if s.StripeReconciliations == nil {
		s.StripeReconciliations = map[string]StripeReconciliation{}
	}
	if s.Approvals == nil {
		s.Approvals = map[string]Approval{}
	}
	if s.Sessions == nil {
		s.Sessions = map[string]Session{}
	}
	if s.AgentRuns == nil {
		s.AgentRuns = map[string]AgentRun{}
	}
	if s.OAuthGrants == nil {
		s.OAuthGrants = map[string]OAuthGrant{}
	}
	if s.Exports == nil {
		s.Exports = map[string]ExportJob{}
	}
	if s.WebhookIDs == nil {
		s.WebhookIDs = map[string]bool{}
	}
	if s.EmailVerifications == nil {
		s.EmailVerifications = map[string]AuthToken{}
	}
	if s.ResetTokens == nil {
		s.ResetTokens = map[string]AuthToken{}
	}
	if s.Journal == nil {
		s.Journal = []JournalEntry{}
	}
	if s.Audit == nil {
		s.Audit = []AuditEvent{}
	}
	if s.Outbox == nil {
		s.Outbox = []OutboxEvent{}
	}
}
