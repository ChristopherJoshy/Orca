package orca

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresInvoicePeriodUniqueConstraint(t *testing.T) {
	dsn := os.Getenv("ORCA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ORCA_TEST_DATABASE_URL to run the isolated PostgreSQL constraint test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schemaID, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "orca_test_" + strings.ReplaceAll(schemaID, "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := &Store{pool: pool}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	businessID, customerID, planID, subscriptionID, invoiceID, duplicateID := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555", "66666666-6666-4666-8666-666666666666"
	if _, err := pool.Exec(ctx, "INSERT INTO businesses(id,name) VALUES($1,'Test')", businessID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO customers(id,business_id,name,email) VALUES($1,$2,'Customer','customer@example.test')", customerID, businessID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO plans(id,business_id,name,interval,sales_price) VALUES($1,$2,'Monthly','monthly',10)", planID, businessID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO subscriptions(id,business_id,customer_id,plan_id,number,status,start_date,lines) VALUES($1,$2,$3,$4,'SUB-1','Active','2026-01-01','[]')", subscriptionID, businessID, customerID, planID); err != nil {
		t.Fatal(err)
	}
	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)
	insert := func(id, number string) error {
		_, err := pool.Exec(ctx, `INSERT INTO invoices(id,business_id,subscription_id,customer_id,number,period_start,period_end,status,subtotal,discount_total,tax_total,total,due_at,snapshot,calculation_hash,calculation_engine)
		VALUES($1,$2,$3,$4,$5,$6,$7,'Draft',10,0,0,10,$7,'{}',repeat('a',64),'test')`, id, businessID, subscriptionID, customerID, number, periodStart, periodEnd)
		return err
	}
	if err := insert(invoiceID, "INV-1"); err != nil {
		t.Fatal(err)
	}
	err = insert(duplicateID, "INV-2")
	if err == nil {
		t.Fatal("expected duplicate subscription period to be rejected")
	}
	var pgErr *pgconn.PgError
	if !asPgError(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("duplicate period error = %v, expected unique constraint violation", err)
	}
}

func asPgError(err error, target **pgconn.PgError) bool {
	if pg, ok := err.(*pgconn.PgError); ok {
		*target = pg
		return true
	}
	return false
}
