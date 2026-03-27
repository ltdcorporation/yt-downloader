package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresBackend struct {
	db *sql.DB

	schemaMu    sync.Mutex
	schemaReady bool
}

func newPostgresBackend(dsn string) *postgresBackend {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(fmt.Sprintf("billing postgres open failed: %v", err))
	}
	return &postgresBackend{db: db}
}

func (p *postgresBackend) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *postgresBackend) EnsureReady(ctx context.Context) error {
	return p.ensureSchema(ctx)
}

func (p *postgresBackend) GetOrCreateProfile(ctx context.Context, userID string, now time.Time) (Profile, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Profile{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	defaultValue := defaultProfile(userID, now.UTC())
	_, err := p.db.ExecContext(
		ctx,
		`INSERT INTO billing_profiles (
			user_id,
			payment_brand,
			payment_last4,
			payment_exp_month,
			payment_exp_year,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (user_id) DO NOTHING`,
		defaultValue.UserID,
		defaultValue.PaymentMethod.Brand,
		defaultValue.PaymentMethod.Last4,
		defaultValue.PaymentMethod.ExpMonth,
		defaultValue.PaymentMethod.ExpYear,
		defaultValue.CreatedAt.UTC(),
		defaultValue.UpdatedAt.UTC(),
	)
	if err != nil {
		return Profile{}, fmt.Errorf("create default billing profile: %w", err)
	}

	row := p.db.QueryRowContext(
		ctx,
		`SELECT user_id, payment_brand, payment_last4, payment_exp_month, payment_exp_year, created_at, updated_at
		 FROM billing_profiles
		 WHERE user_id = $1`,
		userID,
	)

	profile := Profile{}
	if err := row.Scan(
		&profile.UserID,
		&profile.PaymentMethod.Brand,
		&profile.PaymentMethod.Last4,
		&profile.PaymentMethod.ExpMonth,
		&profile.PaymentMethod.ExpYear,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	); err != nil {
		return Profile{}, fmt.Errorf("select billing profile: %w", err)
	}
	profile.PaymentMethod.UpdatedAt = profile.UpdatedAt

	return normalizeProfile(profile)
}

func (p *postgresBackend) GetOrCreateSubscriptionState(ctx context.Context, userID string, now time.Time) (SubscriptionState, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return SubscriptionState{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	defaultState := defaultSubscriptionState(userID, now.UTC())
	_, err := p.db.ExecContext(
		ctx,
		`INSERT INTO billing_subscription_states (
			user_id,
			cancel_at_period_end,
			canceled_at,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id) DO NOTHING`,
		defaultState.UserID,
		defaultState.CancelAtPeriodEnd,
		nil,
		defaultState.CreatedAt.UTC(),
		defaultState.UpdatedAt.UTC(),
	)
	if err != nil {
		return SubscriptionState{}, fmt.Errorf("create default subscription state: %w", err)
	}

	return p.loadSubscriptionState(ctx, userID)
}

func (p *postgresBackend) UpsertSubscriptionState(ctx context.Context, state SubscriptionState) (SubscriptionState, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return SubscriptionState{}, err
	}

	_, err := p.db.ExecContext(
		ctx,
		`INSERT INTO billing_subscription_states (
			user_id,
			cancel_at_period_end,
			canceled_at,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id) DO UPDATE
		SET cancel_at_period_end = EXCLUDED.cancel_at_period_end,
		    canceled_at = EXCLUDED.canceled_at,
		    updated_at = EXCLUDED.updated_at`,
		state.UserID,
		state.CancelAtPeriodEnd,
		nullableTime(state.CanceledAt),
		state.CreatedAt.UTC(),
		state.UpdatedAt.UTC(),
	)
	if err != nil {
		return SubscriptionState{}, fmt.Errorf("upsert subscription state: %w", err)
	}

	return p.loadSubscriptionState(ctx, state.UserID)
}

func (p *postgresBackend) CreateInvoice(ctx context.Context, invoice Invoice) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	_, err := p.db.ExecContext(
		ctx,
		`INSERT INTO billing_invoices (
			id,
			user_id,
			amount_cents,
			currency,
			status,
			issued_at,
			period_start,
			period_end,
			receipt_url,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		invoice.ID,
		invoice.UserID,
		invoice.AmountCents,
		invoice.Currency,
		invoice.Status,
		invoice.IssuedAt.UTC(),
		nullableTime(invoice.PeriodStart),
		nullableTime(invoice.PeriodEnd),
		nullableString(invoice.ReceiptURL),
		invoice.CreatedAt.UTC(),
		invoice.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("create billing invoice: %w", err)
	}
	return nil
}

func (p *postgresBackend) ListInvoices(ctx context.Context, userID string, limit, offset int) ([]Invoice, int, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return nil, 0, err
	}

	var total int
	if err := p.db.QueryRowContext(
		ctx,
		`SELECT COUNT(1) FROM billing_invoices WHERE user_id = $1`,
		userID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count billing invoices: %w", err)
	}

	rows, err := p.db.QueryContext(
		ctx,
		`SELECT id, user_id, amount_cents, currency, status, issued_at, period_start, period_end, COALESCE(receipt_url, ''), created_at, updated_at
		 FROM billing_invoices
		 WHERE user_id = $1
		 ORDER BY issued_at DESC, id DESC
		 LIMIT $2 OFFSET $3`,
		userID,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list billing invoices: %w", err)
	}
	defer rows.Close()

	invoices := make([]Invoice, 0, limit)
	for rows.Next() {
		invoice, err := scanInvoice(rows)
		if err != nil {
			return nil, 0, err
		}
		invoices = append(invoices, invoice)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate billing invoices: %w", err)
	}

	return invoices, total, nil
}

func (p *postgresBackend) GetInvoice(ctx context.Context, userID, invoiceID string) (Invoice, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Invoice{}, err
	}

	row := p.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, amount_cents, currency, status, issued_at, period_start, period_end, COALESCE(receipt_url, ''), created_at, updated_at
		 FROM billing_invoices
		 WHERE user_id = $1 AND id = $2`,
		userID,
		invoiceID,
	)

	invoice, err := scanInvoice(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Invoice{}, ErrInvoiceNotFound
		}
		return Invoice{}, err
	}

	return normalizeInvoice(invoice)
}

func (p *postgresBackend) loadSubscriptionState(ctx context.Context, userID string) (SubscriptionState, error) {
	row := p.db.QueryRowContext(
		ctx,
		`SELECT user_id, cancel_at_period_end, canceled_at, created_at, updated_at
		 FROM billing_subscription_states
		 WHERE user_id = $1`,
		userID,
	)

	state := SubscriptionState{}
	var canceledAt sql.NullTime
	if err := row.Scan(&state.UserID, &state.CancelAtPeriodEnd, &canceledAt, &state.CreatedAt, &state.UpdatedAt); err != nil {
		return SubscriptionState{}, fmt.Errorf("select subscription state: %w", err)
	}
	if canceledAt.Valid {
		value := canceledAt.Time.UTC()
		state.CanceledAt = &value
	}

	return normalizeSubscriptionState(state)
}

func scanInvoice(row interface{ Scan(dest ...any) error }) (Invoice, error) {
	invoice := Invoice{}
	var status string
	var periodStart sql.NullTime
	var periodEnd sql.NullTime
	if err := row.Scan(
		&invoice.ID,
		&invoice.UserID,
		&invoice.AmountCents,
		&invoice.Currency,
		&status,
		&invoice.IssuedAt,
		&periodStart,
		&periodEnd,
		&invoice.ReceiptURL,
		&invoice.CreatedAt,
		&invoice.UpdatedAt,
	); err != nil {
		return Invoice{}, err
	}
	if periodStart.Valid {
		value := periodStart.Time.UTC()
		invoice.PeriodStart = &value
	}
	if periodEnd.Valid {
		value := periodEnd.Time.UTC()
		invoice.PeriodEnd = &value
	}
	invoice.Status = InvoiceStatus(status)

	return normalizeInvoice(invoice)
}

func (p *postgresBackend) ensureSchema(ctx context.Context) error {
	p.schemaMu.Lock()
	defer p.schemaMu.Unlock()

	if p.schemaReady {
		return nil
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS billing_profiles (
			user_id TEXT PRIMARY KEY REFERENCES auth_users(id) ON DELETE CASCADE,
			payment_brand TEXT NOT NULL,
			payment_last4 TEXT NOT NULL,
			payment_exp_month INT NOT NULL,
			payment_exp_year INT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT chk_billing_profile_last4 CHECK (char_length(payment_last4) = 4),
			CONSTRAINT chk_billing_profile_exp_month CHECK (payment_exp_month >= 1 AND payment_exp_month <= 12)
		)`,
		`CREATE TABLE IF NOT EXISTS billing_subscription_states (
			user_id TEXT PRIMARY KEY REFERENCES auth_users(id) ON DELETE CASCADE,
			cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
			canceled_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS billing_invoices (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
			amount_cents BIGINT NOT NULL,
			currency TEXT NOT NULL,
			status TEXT NOT NULL,
			issued_at TIMESTAMPTZ NOT NULL,
			period_start TIMESTAMPTZ,
			period_end TIMESTAMPTZ,
			receipt_url TEXT,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT chk_billing_invoice_amount CHECK (amount_cents >= 0),
			CONSTRAINT chk_billing_invoice_status CHECK (status IN ('paid','pending','failed'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_invoices_user_issued ON billing_invoices (user_id, issued_at DESC, id DESC)`,
	}

	for _, stmt := range statements {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("billing ensure schema failed: %w", err)
		}
	}

	p.schemaReady = true
	return nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
