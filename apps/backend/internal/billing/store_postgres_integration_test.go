package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStorePostgresIntegration_ProfileSubscriptionAndInvoices(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	t.Cleanup(func() { _ = backend.Close() })

	ensureAuthUsersTable(t, backend.db)
	seedAuthUser(t, backend.db, "usr_billing_1")

	store := &Store{backend: backend}
	ctx := context.Background()

	if err := store.EnsureReady(ctx); err != nil {
		t.Fatalf("EnsureReady failed: %v", err)
	}
	if err := store.EnsureReady(ctx); err != nil {
		t.Fatalf("EnsureReady should be idempotent, got: %v", err)
	}

	profile, err := store.GetOrCreateProfile(ctx, "usr_billing_1", time.Time{})
	if err != nil {
		t.Fatalf("GetOrCreateProfile failed: %v", err)
	}
	if profile.UserID != "usr_billing_1" {
		t.Fatalf("unexpected profile user_id: %q", profile.UserID)
	}
	if profile.PaymentMethod.Last4 != "4242" {
		t.Fatalf("unexpected default payment method: %+v", profile.PaymentMethod)
	}

	state, err := store.GetOrCreateSubscriptionState(ctx, "usr_billing_1", time.Time{})
	if err != nil {
		t.Fatalf("GetOrCreateSubscriptionState failed: %v", err)
	}
	if state.CancelAtPeriodEnd {
		t.Fatalf("expected default cancel_at_period_end=false")
	}

	cancelAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	updatedState, err := store.UpsertSubscriptionState(ctx, SubscriptionState{
		UserID:            "usr_billing_1",
		CancelAtPeriodEnd: true,
		CanceledAt:        &cancelAt,
	})
	if err != nil {
		t.Fatalf("UpsertSubscriptionState failed: %v", err)
	}
	if !updatedState.CancelAtPeriodEnd || updatedState.CanceledAt == nil {
		t.Fatalf("expected canceled state after upsert, got %+v", updatedState)
	}

	now := time.Date(2026, 3, 29, 4, 20, 0, 0, time.UTC)
	invoices := []Invoice{
		{
			ID:          "inv_pg_1",
			UserID:      "usr_billing_1",
			AmountCents: 1999,
			Currency:    "usd",
			Status:      InvoiceStatusPaid,
			IssuedAt:    now.Add(2 * time.Hour),
			ReceiptURL:  "https://example.com/r/inv_pg_1",
			CreatedAt:   now.Add(2 * time.Hour),
			UpdatedAt:   now.Add(2 * time.Hour),
		},
		{
			ID:          "inv_pg_2",
			UserID:      "usr_billing_1",
			AmountCents: 999,
			Currency:    "usd",
			Status:      InvoiceStatusPending,
			IssuedAt:    now.Add(1 * time.Hour),
			CreatedAt:   now.Add(1 * time.Hour),
			UpdatedAt:   now.Add(1 * time.Hour),
		},
		{
			ID:          "inv_pg_3",
			UserID:      "usr_billing_1",
			AmountCents: 0,
			Currency:    "usd",
			Status:      InvoiceStatusFailed,
			IssuedAt:    now,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	for _, invoice := range invoices {
		if err := store.CreateInvoice(ctx, invoice); err != nil {
			t.Fatalf("CreateInvoice failed for %s: %v", invoice.ID, err)
		}
	}

	listed, total, err := store.ListInvoices(ctx, "usr_billing_1", 999, -10)
	if err != nil {
		t.Fatalf("ListInvoices failed: %v", err)
	}
	if total != 3 || len(listed) != 3 {
		t.Fatalf("expected total=3 and len=3, got total=%d len=%d", total, len(listed))
	}
	if listed[0].ID != "inv_pg_1" || listed[1].ID != "inv_pg_2" || listed[2].ID != "inv_pg_3" {
		t.Fatalf("unexpected invoice ordering: %#v", []string{listed[0].ID, listed[1].ID, listed[2].ID})
	}

	paged, total, err := store.ListInvoices(ctx, "usr_billing_1", 2, 1)
	if err != nil {
		t.Fatalf("ListInvoices paged failed: %v", err)
	}
	if total != 3 || len(paged) != 2 {
		t.Fatalf("expected paged total=3 len=2, got total=%d len=%d", total, len(paged))
	}
	if paged[0].ID != "inv_pg_2" || paged[1].ID != "inv_pg_3" {
		t.Fatalf("unexpected paged invoice order: %#v", []string{paged[0].ID, paged[1].ID})
	}

	gotInvoice, err := store.GetInvoice(ctx, "usr_billing_1", "inv_pg_1")
	if err != nil {
		t.Fatalf("GetInvoice failed: %v", err)
	}
	if gotInvoice.ID != "inv_pg_1" || gotInvoice.Status != InvoiceStatusPaid {
		t.Fatalf("unexpected invoice payload: %+v", gotInvoice)
	}

	_, err = store.GetInvoice(ctx, "usr_billing_1", "inv_missing")
	if !errors.Is(err, ErrInvoiceNotFound) {
		t.Fatalf("expected ErrInvoiceNotFound, got %v", err)
	}
}

func TestStorePostgresIntegration_CreateInvoiceValidation(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	t.Cleanup(func() { _ = backend.Close() })

	ensureAuthUsersTable(t, backend.db)
	seedAuthUser(t, backend.db, "usr_billing_2")

	store := &Store{backend: backend}
	ctx := context.Background()

	err := store.CreateInvoice(ctx, Invoice{
		ID:          "",
		UserID:      "usr_billing_2",
		AmountCents: 100,
		Currency:    "usd",
		Status:      InvoiceStatusPaid,
	})
	if err == nil {
		t.Fatalf("expected validation error for empty invoice id")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T %v", err, err)
	}
}
