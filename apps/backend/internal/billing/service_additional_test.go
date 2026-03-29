package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBillingService_GuardsAndEnsureReadyErrors(t *testing.T) {
	ctx := context.Background()

	var nilSvc *Service
	if _, err := nilSvc.GetDashboard(ctx, "usr_1", "free", nil); err == nil {
		t.Fatalf("expected nil service GetDashboard error")
	}
	if _, _, err := nilSvc.ListInvoices(ctx, "usr_1", 10, 0); err == nil {
		t.Fatalf("expected nil service ListInvoices error")
	}
	if _, err := nilSvc.GetInvoice(ctx, "usr_1", "inv_1"); err == nil {
		t.Fatalf("expected nil service GetInvoice error")
	}
	if _, err := nilSvc.ScheduleCancelAtPeriodEnd(ctx, "usr_1"); err == nil {
		t.Fatalf("expected nil service ScheduleCancelAtPeriodEnd error")
	}
	if _, err := nilSvc.ClearCancelSchedule(ctx, "usr_1"); err == nil {
		t.Fatalf("expected nil service ClearCancelSchedule error")
	}
	if _, err := nilSvc.CreatePlanInvoice(ctx, "usr_1", "weekly", nil); err == nil {
		t.Fatalf("expected nil service CreatePlanInvoice error")
	}

	expectedErr := errors.New("ensure ready failed")
	store := &Store{backend: &fakeBillingStoreBackend{ensureReadyFn: func(context.Context) error { return expectedErr }}}
	svc := NewService(store)

	if _, err := svc.GetDashboard(ctx, "usr_1", "free", nil); !errors.Is(err, expectedErr) {
		t.Fatalf("expected ensureReady error from GetDashboard, got %v", err)
	}
	if _, _, err := svc.ListInvoices(ctx, "usr_1", 10, 0); !errors.Is(err, expectedErr) {
		t.Fatalf("expected ensureReady error from ListInvoices, got %v", err)
	}
	if _, err := svc.GetInvoice(ctx, "usr_1", "inv_1"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected ensureReady error from GetInvoice, got %v", err)
	}
	if _, err := svc.ScheduleCancelAtPeriodEnd(ctx, "usr_1"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected ensureReady error from ScheduleCancelAtPeriodEnd, got %v", err)
	}
	if _, err := svc.ClearCancelSchedule(ctx, "usr_1"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected ensureReady error from ClearCancelSchedule, got %v", err)
	}
	if _, err := svc.CreatePlanInvoice(ctx, "usr_1", "weekly", nil); !errors.Is(err, expectedErr) {
		t.Fatalf("expected ensureReady error from CreatePlanInvoice, got %v", err)
	}
}

func TestBillingService_ListGetAndCreateInvoiceForwarding(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 29, 8, 40, 0, 0, time.UTC)

	capturedListLimit := 0
	capturedListOffset := 0
	capturedGetUserID := ""
	capturedGetInvoiceID := ""
	capturedCreate := Invoice{}

	store := &Store{backend: &fakeBillingStoreBackend{
		listInvoicesFn: func(_ context.Context, userID string, limit, offset int) ([]Invoice, int, error) {
			capturedListLimit = limit
			capturedListOffset = offset
			return []Invoice{{ID: "inv_list_1", UserID: userID, AmountCents: 100, Currency: "USD", Status: InvoiceStatusPaid, IssuedAt: now, CreatedAt: now, UpdatedAt: now}}, 1, nil
		},
		getInvoiceFn: func(_ context.Context, userID, invoiceID string) (Invoice, error) {
			capturedGetUserID = userID
			capturedGetInvoiceID = invoiceID
			return Invoice{ID: invoiceID, UserID: userID, AmountCents: 100, Currency: "USD", Status: InvoiceStatusPaid, IssuedAt: now, CreatedAt: now, UpdatedAt: now}, nil
		},
		createInvoiceFn: func(_ context.Context, invoice Invoice) error {
			capturedCreate = invoice
			return nil
		},
	}}
	svc := NewService(store)
	svc.now = func() time.Time { return now }

	items, total, err := svc.ListInvoices(ctx, "usr_1", 7, 2)
	if err != nil {
		t.Fatalf("ListInvoices failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one invoice from list, total=%d len=%d", total, len(items))
	}
	if capturedListLimit != 7 || capturedListOffset != 2 {
		t.Fatalf("unexpected forwarded list pagination limit=%d offset=%d", capturedListLimit, capturedListOffset)
	}

	invoice, err := svc.GetInvoice(ctx, "usr_1", "inv_get_1")
	if err != nil {
		t.Fatalf("GetInvoice failed: %v", err)
	}
	if capturedGetUserID != "usr_1" || capturedGetInvoiceID != "inv_get_1" {
		t.Fatalf("unexpected forwarded get invoice args user=%q invoice=%q", capturedGetUserID, capturedGetInvoiceID)
	}
	if invoice.ID != "inv_get_1" {
		t.Fatalf("unexpected returned invoice id: %s", invoice.ID)
	}

	expiresAt := now.Add(7 * 24 * time.Hour)
	created, err := svc.CreatePlanInvoice(ctx, "  usr_1  ", "weekly", &expiresAt)
	if err != nil {
		t.Fatalf("CreatePlanInvoice failed: %v", err)
	}
	if created.ID == "" || created.UserID != "usr_1" {
		t.Fatalf("unexpected created invoice payload: %+v", created)
	}
	if capturedCreate.UserID != "usr_1" {
		t.Fatalf("expected trimmed user id on create invoice payload, got %q", capturedCreate.UserID)
	}
	if capturedCreate.PeriodEnd == nil || !capturedCreate.PeriodEnd.Equal(expiresAt) {
		t.Fatalf("expected period_end forwarded to store, got %+v", capturedCreate.PeriodEnd)
	}
}

func TestBillingService_BuildSubscriptionSummaryBranches(t *testing.T) {
	now := time.Date(2026, 3, 29, 9, 0, 0, 0, time.UTC)

	free, err := buildSubscriptionSummary("free", nil, SubscriptionState{UserID: "usr_1", CancelAtPeriodEnd: true}, now)
	if err != nil {
		t.Fatalf("buildSubscriptionSummary free failed: %v", err)
	}
	if free.Status != "inactive" || free.CancelAtPeriodEnd {
		t.Fatalf("expected free plan to be inactive without cancel flag, got %+v", free)
	}

	expiredAt := now.Add(-time.Hour)
	expired, err := buildSubscriptionSummary("monthly", &expiredAt, SubscriptionState{UserID: "usr_1"}, now)
	if err != nil {
		t.Fatalf("buildSubscriptionSummary expired failed: %v", err)
	}
	if expired.Status != "expired" {
		t.Fatalf("expected expired status, got %s", expired.Status)
	}

	activeAt := now.Add(24 * time.Hour)
	active, err := buildSubscriptionSummary("monthly", &activeAt, SubscriptionState{UserID: "usr_1", CancelAtPeriodEnd: false}, now)
	if err != nil {
		t.Fatalf("buildSubscriptionSummary active failed: %v", err)
	}
	if active.Status != "active" {
		t.Fatalf("expected active status, got %s", active.Status)
	}

	if _, err := buildSubscriptionSummary("unknown", nil, SubscriptionState{UserID: "usr_1"}, now); err == nil {
		t.Fatalf("expected invalid plan error")
	}
}
