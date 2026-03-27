package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryBackend_ProfileStateAndInvoices(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC()

	profile, err := backend.GetOrCreateProfile(ctx, "usr_1", now)
	if err != nil {
		t.Fatalf("GetOrCreateProfile failed: %v", err)
	}
	if profile.UserID != "usr_1" {
		t.Fatalf("unexpected profile user id: %s", profile.UserID)
	}

	state, err := backend.GetOrCreateSubscriptionState(ctx, "usr_1", now)
	if err != nil {
		t.Fatalf("GetOrCreateSubscriptionState failed: %v", err)
	}
	if state.CancelAtPeriodEnd {
		t.Fatalf("expected default cancel_at_period_end=false")
	}

	cancelAt := now.Add(time.Minute)
	state.CancelAtPeriodEnd = true
	state.CanceledAt = &cancelAt
	state.UpdatedAt = cancelAt
	updatedState, err := backend.UpsertSubscriptionState(ctx, state)
	if err != nil {
		t.Fatalf("UpsertSubscriptionState failed: %v", err)
	}
	if !updatedState.CancelAtPeriodEnd || updatedState.CanceledAt == nil {
		t.Fatalf("expected updated subscription state, got %+v", updatedState)
	}

	invoices := []Invoice{
		{ID: "inv_1", UserID: "usr_1", AmountCents: 1200, Currency: "USD", Status: InvoiceStatusPaid, IssuedAt: now.Add(-2 * time.Hour), CreatedAt: now, UpdatedAt: now},
		{ID: "inv_2", UserID: "usr_1", AmountCents: 900, Currency: "USD", Status: InvoiceStatusPaid, IssuedAt: now.Add(-1 * time.Hour), CreatedAt: now, UpdatedAt: now},
	}
	for _, inv := range invoices {
		if err := backend.CreateInvoice(ctx, inv); err != nil {
			t.Fatalf("CreateInvoice failed: %v", err)
		}
	}

	items, total, err := backend.ListInvoices(ctx, "usr_1", 10, 0)
	if err != nil {
		t.Fatalf("ListInvoices failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected 2 invoices, total=%d len=%d", total, len(items))
	}
	if items[0].ID != "inv_2" {
		t.Fatalf("expected latest invoice first, got %s", items[0].ID)
	}

	fetched, err := backend.GetInvoice(ctx, "usr_1", "inv_1")
	if err != nil {
		t.Fatalf("GetInvoice failed: %v", err)
	}
	if fetched.ID != "inv_1" {
		t.Fatalf("unexpected fetched invoice id: %s", fetched.ID)
	}

	if _, err := backend.GetInvoice(ctx, "usr_1", "missing"); !errors.Is(err, ErrInvoiceNotFound) {
		t.Fatalf("expected ErrInvoiceNotFound, got %v", err)
	}
}
