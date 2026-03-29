package billing

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

type fakeBillingStoreBackend struct {
	closeFn              func() error
	ensureReadyFn        func(context.Context) error
	getProfileFn         func(context.Context, string, time.Time) (Profile, error)
	getSubscriptionFn    func(context.Context, string, time.Time) (SubscriptionState, error)
	upsertSubscriptionFn func(context.Context, SubscriptionState) (SubscriptionState, error)
	createInvoiceFn      func(context.Context, Invoice) error
	listInvoicesFn       func(context.Context, string, int, int) ([]Invoice, int, error)
	getInvoiceFn         func(context.Context, string, string) (Invoice, error)
}

func (f *fakeBillingStoreBackend) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func (f *fakeBillingStoreBackend) EnsureReady(ctx context.Context) error {
	if f.ensureReadyFn != nil {
		return f.ensureReadyFn(ctx)
	}
	return nil
}

func (f *fakeBillingStoreBackend) GetOrCreateProfile(ctx context.Context, userID string, now time.Time) (Profile, error) {
	if f.getProfileFn != nil {
		return f.getProfileFn(ctx, userID, now)
	}
	return defaultProfile(userID, now), nil
}

func (f *fakeBillingStoreBackend) GetOrCreateSubscriptionState(ctx context.Context, userID string, now time.Time) (SubscriptionState, error) {
	if f.getSubscriptionFn != nil {
		return f.getSubscriptionFn(ctx, userID, now)
	}
	return defaultSubscriptionState(userID, now), nil
}

func (f *fakeBillingStoreBackend) UpsertSubscriptionState(ctx context.Context, state SubscriptionState) (SubscriptionState, error) {
	if f.upsertSubscriptionFn != nil {
		return f.upsertSubscriptionFn(ctx, state)
	}
	return state, nil
}

func (f *fakeBillingStoreBackend) CreateInvoice(ctx context.Context, invoice Invoice) error {
	if f.createInvoiceFn != nil {
		return f.createInvoiceFn(ctx, invoice)
	}
	return nil
}

func (f *fakeBillingStoreBackend) ListInvoices(ctx context.Context, userID string, limit, offset int) ([]Invoice, int, error) {
	if f.listInvoicesFn != nil {
		return f.listInvoicesFn(ctx, userID, limit, offset)
	}
	return nil, 0, nil
}

func (f *fakeBillingStoreBackend) GetInvoice(ctx context.Context, userID, invoiceID string) (Invoice, error) {
	if f.getInvoiceFn != nil {
		return f.getInvoiceFn(ctx, userID, invoiceID)
	}
	return Invoice{}, ErrInvoiceNotFound
}

func TestBillingValidationError_ErrorBranches(t *testing.T) {
	if got := (&ValidationError{}).Error(); got != "invalid billing input" {
		t.Fatalf("unexpected default validation message: %q", got)
	}
	if got := (&ValidationError{Message: "boom"}).Error(); got != "boom" {
		t.Fatalf("unexpected custom validation message: %q", got)
	}
}

func TestBillingStore_NewStoreSelectsBackend(t *testing.T) {
	memoryStore := NewStore(config.Config{}, nil)
	if memoryStore == nil {
		t.Fatalf("expected non-nil memory store")
	}
	if _, ok := memoryStore.backend.(*memoryBackend); !ok {
		t.Fatalf("expected memory backend when POSTGRES_DSN empty")
	}

	pgStore := NewStore(config.Config{PostgresDSN: "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"}, nil)
	if pgStore == nil {
		t.Fatalf("expected non-nil postgres store")
	}
	if _, ok := pgStore.backend.(*postgresBackend); !ok {
		t.Fatalf("expected postgres backend when POSTGRES_DSN is set")
	}
	_ = pgStore.Close()
}

func TestBillingStore_Guards(t *testing.T) {
	var nilStore *Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("nil store Close should not error, got %v", err)
	}

	s := &Store{}
	ctx := context.Background()
	if err := s.EnsureReady(ctx); err == nil {
		t.Fatalf("expected EnsureReady error on uninitialized store")
	}
	if _, err := s.GetOrCreateProfile(ctx, "usr_1", time.Now()); err == nil {
		t.Fatalf("expected GetOrCreateProfile error on uninitialized store")
	}
	if _, err := s.GetOrCreateSubscriptionState(ctx, "usr_1", time.Now()); err == nil {
		t.Fatalf("expected GetOrCreateSubscriptionState error on uninitialized store")
	}
	if _, err := s.UpsertSubscriptionState(ctx, SubscriptionState{UserID: "usr_1"}); err == nil {
		t.Fatalf("expected UpsertSubscriptionState error on uninitialized store")
	}
	if err := s.CreateInvoice(ctx, Invoice{ID: "inv_1", UserID: "usr_1", AmountCents: 100, Currency: "USD", Status: InvoiceStatusPaid}); err == nil {
		t.Fatalf("expected CreateInvoice error on uninitialized store")
	}
	if _, _, err := s.ListInvoices(ctx, "usr_1", 10, 0); err == nil {
		t.Fatalf("expected ListInvoices error on uninitialized store")
	}
	if _, err := s.GetInvoice(ctx, "usr_1", "inv_1"); err == nil {
		t.Fatalf("expected GetInvoice error on uninitialized store")
	}
}

func TestBillingStore_ProfileStateAndInvoiceNormalization(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 29, 7, 0, 0, 0, time.UTC)

	capturedProfileUserID := ""
	capturedProfileNow := time.Time{}
	capturedState := SubscriptionState{}
	capturedInvoice := Invoice{}
	capturedListLimit := 0
	capturedListOffset := 0
	capturedGetUserID := ""
	capturedGetInvoiceID := ""

	store := &Store{backend: &fakeBillingStoreBackend{
		getProfileFn: func(_ context.Context, userID string, ts time.Time) (Profile, error) {
			capturedProfileUserID = userID
			capturedProfileNow = ts
			return Profile{
				UserID: userID,
				PaymentMethod: PaymentMethod{
					Brand:     "",
					Last4:     "4242",
					ExpMonth:  12,
					ExpYear:   2030,
					UpdatedAt: ts,
				},
				CreatedAt: ts,
				UpdatedAt: ts,
			}, nil
		},
		getSubscriptionFn: func(_ context.Context, userID string, ts time.Time) (SubscriptionState, error) {
			return SubscriptionState{UserID: userID, CreatedAt: ts, UpdatedAt: ts}, nil
		},
		upsertSubscriptionFn: func(_ context.Context, state SubscriptionState) (SubscriptionState, error) {
			capturedState = state
			return state, nil
		},
		createInvoiceFn: func(_ context.Context, invoice Invoice) error {
			capturedInvoice = invoice
			return nil
		},
		listInvoicesFn: func(_ context.Context, userID string, limit, offset int) ([]Invoice, int, error) {
			capturedListLimit = limit
			capturedListOffset = offset
			periodStart := now.Add(-24 * time.Hour)
			periodEnd := now
			return []Invoice{{
				ID:          "inv_1",
				UserID:      userID,
				AmountCents: 1999,
				Currency:    "usd",
				Status:      InvoiceStatusPaid,
				IssuedAt:    now,
				PeriodStart: &periodStart,
				PeriodEnd:   &periodEnd,
				CreatedAt:   now,
				UpdatedAt:   now,
			}}, 1, nil
		},
		getInvoiceFn: func(_ context.Context, userID, invoiceID string) (Invoice, error) {
			capturedGetUserID = userID
			capturedGetInvoiceID = invoiceID
			return Invoice{ID: invoiceID, UserID: userID, AmountCents: 100, Currency: "usd", Status: InvoiceStatusPaid, IssuedAt: now, CreatedAt: now, UpdatedAt: now}, nil
		},
	}}

	profile, err := store.GetOrCreateProfile(ctx, "  usr_1  ", time.Time{})
	if err != nil {
		t.Fatalf("GetOrCreateProfile failed: %v", err)
	}
	if capturedProfileUserID != "usr_1" {
		t.Fatalf("expected trimmed profile user id sent to backend, got %q", capturedProfileUserID)
	}
	if capturedProfileNow.IsZero() {
		t.Fatalf("expected non-zero now when caller passes zero")
	}
	if profile.PaymentMethod.Brand != "visa" {
		t.Fatalf("expected default brand=visa after normalization, got %q", profile.PaymentMethod.Brand)
	}

	stateInput := SubscriptionState{UserID: " usr_1 "}
	state, err := store.UpsertSubscriptionState(ctx, stateInput)
	if err != nil {
		t.Fatalf("UpsertSubscriptionState failed: %v", err)
	}
	if state.UserID != "usr_1" {
		t.Fatalf("expected normalized state user id, got %q", state.UserID)
	}
	if capturedState.CreatedAt.IsZero() || capturedState.UpdatedAt.IsZero() {
		t.Fatalf("expected auto-filled created_at/updated_at in upsert payload, got %+v", capturedState)
	}

	err = store.CreateInvoice(ctx, Invoice{ID: "inv_1", UserID: " usr_1 ", AmountCents: 2500, Currency: "", Status: InvoiceStatusPaid})
	if err != nil {
		t.Fatalf("CreateInvoice failed: %v", err)
	}
	if capturedInvoice.Currency != "USD" {
		t.Fatalf("expected default currency USD, got %q", capturedInvoice.Currency)
	}
	if capturedInvoice.CreatedAt.IsZero() || capturedInvoice.UpdatedAt.IsZero() || capturedInvoice.IssuedAt.IsZero() {
		t.Fatalf("expected invoice timestamps to be auto-filled, got %+v", capturedInvoice)
	}

	items, total, err := store.ListInvoices(ctx, "usr_1", 999, -10)
	if err != nil {
		t.Fatalf("ListInvoices failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected total=1 and one item, got total=%d len=%d", total, len(items))
	}
	if capturedListLimit != 100 || capturedListOffset != 0 {
		t.Fatalf("expected clamped limit=100 offset=0, got limit=%d offset=%d", capturedListLimit, capturedListOffset)
	}
	if items[0].Currency != "USD" {
		t.Fatalf("expected normalized invoice currency=USD, got %q", items[0].Currency)
	}

	invoice, err := store.GetInvoice(ctx, "  usr_1  ", "  inv_1  ")
	if err != nil {
		t.Fatalf("GetInvoice failed: %v", err)
	}
	if capturedGetUserID != "usr_1" || capturedGetInvoiceID != "inv_1" {
		t.Fatalf("expected trimmed get invoice args, got user=%q invoice=%q", capturedGetUserID, capturedGetInvoiceID)
	}
	if invoice.Currency != "USD" {
		t.Fatalf("expected normalized invoice currency=USD, got %q", invoice.Currency)
	}

	if _, err := store.GetOrCreateProfile(ctx, "   ", now); err == nil {
		t.Fatalf("expected user_id validation error for profile get")
	}
	if _, err := store.GetOrCreateSubscriptionState(ctx, "   ", now); err == nil {
		t.Fatalf("expected user_id validation error for subscription state get")
	}
}

func TestBillingStore_ValidationAndHelperBranches(t *testing.T) {
	ctx := context.Background()
	store := &Store{backend: &fakeBillingStoreBackend{}}

	if err := store.CreateInvoice(ctx, Invoice{ID: "", UserID: "usr_1", AmountCents: 100, Currency: "USD", Status: InvoiceStatusPaid}); err == nil {
		t.Fatalf("expected invoice id validation error")
	}
	if _, _, err := store.ListInvoices(ctx, "   ", 10, 0); err == nil {
		t.Fatalf("expected user_id validation error for list invoices")
	}
	if _, err := store.GetInvoice(ctx, "usr_1", "   "); err == nil {
		t.Fatalf("expected invoice id validation error for get invoice")
	}

	_, err := normalizeProfile(Profile{UserID: "usr_1", PaymentMethod: PaymentMethod{Last4: "12", ExpMonth: 12, ExpYear: 2030}})
	if err == nil {
		t.Fatalf("expected profile last4 validation error")
	}

	periodStart := time.Date(2026, 3, 29, 8, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(-time.Hour)
	_, err = normalizeInvoice(Invoice{ID: "inv_1", UserID: "usr_1", AmountCents: 100, Currency: "USD", Status: InvoiceStatusPaid, IssuedAt: periodStart, CreatedAt: periodStart, UpdatedAt: periodStart, PeriodStart: &periodStart, PeriodEnd: &periodEnd})
	if err == nil {
		t.Fatalf("expected period range validation error")
	}

	if !IsValidInvoiceStatus(InvoiceStatusPaid) || IsValidInvoiceStatus(InvoiceStatus("x")) {
		t.Fatalf("unexpected invoice status validation behavior")
	}

	if got := formatAmount(12345, "usd"); got != "USD 123.45" {
		t.Fatalf("unexpected formatAmount output: %q", got)
	}

	invoices := []Invoice{{ID: "b", IssuedAt: periodStart}, {ID: "a", IssuedAt: periodStart}, {ID: "c", IssuedAt: periodStart.Add(time.Hour)}}
	sortInvoicesDesc(invoices)
	order := []string{invoices[0].ID, invoices[1].ID, invoices[2].ID}
	want := []string{"c", "b", "a"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("unexpected invoice sort order got=%#v want=%#v", order, want)
	}

	expectedErr := errors.New("backend failed")
	store = &Store{backend: &fakeBillingStoreBackend{
		listInvoicesFn: func(context.Context, string, int, int) ([]Invoice, int, error) {
			return nil, 0, expectedErr
		},
	}}
	if _, _, err := store.ListInvoices(ctx, "usr_1", 10, 0); !errors.Is(err, expectedErr) {
		t.Fatalf("expected list backend error propagation, got %v", err)
	}
}
