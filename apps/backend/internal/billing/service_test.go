package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	store := NewStore(config.Config{}, nil)
	svc := NewService(store)
	if svc == nil {
		t.Fatalf("expected non-nil billing service")
	}
	return svc
}

func TestService_GetDashboardAndCancelLifecycle(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	dashboard, err := svc.GetDashboard(ctx, "usr_1", "monthly", &expiresAt)
	if err != nil {
		t.Fatalf("GetDashboard failed: %v", err)
	}
	if dashboard.Subscription.Plan != "monthly" {
		t.Fatalf("unexpected plan: %s", dashboard.Subscription.Plan)
	}
	if dashboard.Subscription.Status != "active" {
		t.Fatalf("expected active status, got %s", dashboard.Subscription.Status)
	}
	if dashboard.PaymentMethod.Last4 != "4242" {
		t.Fatalf("unexpected default payment method: %+v", dashboard.PaymentMethod)
	}

	state, err := svc.ScheduleCancelAtPeriodEnd(ctx, "usr_1")
	if err != nil {
		t.Fatalf("ScheduleCancelAtPeriodEnd failed: %v", err)
	}
	if !state.CancelAtPeriodEnd || state.CanceledAt == nil {
		t.Fatalf("expected scheduled cancel state, got %+v", state)
	}

	dashboard, err = svc.GetDashboard(ctx, "usr_1", "monthly", &expiresAt)
	if err != nil {
		t.Fatalf("GetDashboard after cancel schedule failed: %v", err)
	}
	if dashboard.Subscription.Status != "cancel_scheduled" {
		t.Fatalf("expected cancel_scheduled status, got %s", dashboard.Subscription.Status)
	}

	state, err = svc.ClearCancelSchedule(ctx, "usr_1")
	if err != nil {
		t.Fatalf("ClearCancelSchedule failed: %v", err)
	}
	if state.CancelAtPeriodEnd || state.CanceledAt != nil {
		t.Fatalf("expected cleared cancel state, got %+v", state)
	}
}

func TestService_Invoices(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)

	created, err := svc.CreatePlanInvoice(ctx, "usr_2", "weekly", &expiresAt)
	if err != nil {
		t.Fatalf("CreatePlanInvoice failed: %v", err)
	}
	if created.Status != InvoiceStatusPaid {
		t.Fatalf("unexpected created invoice status: %s", created.Status)
	}

	items, total, err := svc.ListInvoices(ctx, "usr_2", 10, 0)
	if err != nil {
		t.Fatalf("ListInvoices failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 invoice, total=%d len=%d", total, len(items))
	}

	invoice, err := svc.GetInvoice(ctx, "usr_2", created.ID)
	if err != nil {
		t.Fatalf("GetInvoice failed: %v", err)
	}
	if invoice.ID != created.ID {
		t.Fatalf("unexpected invoice id: %s", invoice.ID)
	}

	if _, err := svc.GetInvoice(ctx, "usr_2", "missing"); !errors.Is(err, ErrInvoiceNotFound) {
		t.Fatalf("expected ErrInvoiceNotFound, got %v", err)
	}

	if _, err := svc.CreatePlanInvoice(ctx, "usr_2", "free", nil); err == nil {
		t.Fatalf("expected free-plan invoice creation to fail")
	}
}

func TestBuildSubscriptionSummaryValidation(t *testing.T) {
	_, err := buildSubscriptionSummary("unknown", nil, SubscriptionState{UserID: "usr_1"}, time.Now().UTC())
	if err == nil {
		t.Fatalf("expected invalid plan error")
	}
}
