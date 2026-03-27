package billing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"yt-downloader/backend/internal/config"
)

var (
	ErrInvalidInput    = errors.New("invalid billing input")
	ErrInvoiceNotFound = errors.New("invoice not found")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return "invalid billing input"
	}
	return e.Message
}

type InvoiceStatus string

const (
	InvoiceStatusPaid    InvoiceStatus = "paid"
	InvoiceStatusPending InvoiceStatus = "pending"
	InvoiceStatusFailed  InvoiceStatus = "failed"
)

type Invoice struct {
	ID          string
	UserID      string
	AmountCents int64
	Currency    string
	Status      InvoiceStatus
	IssuedAt    time.Time
	PeriodStart *time.Time
	PeriodEnd   *time.Time
	ReceiptURL  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PaymentMethod struct {
	Brand     string
	Last4     string
	ExpMonth  int
	ExpYear   int
	UpdatedAt time.Time
}

type Profile struct {
	UserID        string
	PaymentMethod PaymentMethod
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SubscriptionState struct {
	UserID            string
	CancelAtPeriodEnd bool
	CanceledAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type backend interface {
	Close() error
	EnsureReady(ctx context.Context) error
	GetOrCreateProfile(ctx context.Context, userID string, now time.Time) (Profile, error)
	GetOrCreateSubscriptionState(ctx context.Context, userID string, now time.Time) (SubscriptionState, error)
	UpsertSubscriptionState(ctx context.Context, state SubscriptionState) (SubscriptionState, error)
	CreateInvoice(ctx context.Context, invoice Invoice) error
	ListInvoices(ctx context.Context, userID string, limit, offset int) ([]Invoice, int, error)
	GetInvoice(ctx context.Context, userID, invoiceID string) (Invoice, error)
}

type Store struct {
	backend backend
}

func NewStore(cfg config.Config, logger *log.Logger) *Store {
	if strings.TrimSpace(cfg.PostgresDSN) != "" {
		if logger != nil {
			logger.Printf("billing store engine=postgres")
		}
		return &Store{backend: newPostgresBackend(cfg.PostgresDSN)}
	}

	if logger != nil {
		logger.Printf("billing store engine=memory (POSTGRES_DSN empty)")
	}
	return &Store{backend: newMemoryBackend()}
}

func (s *Store) Close() error {
	if s == nil || s.backend == nil {
		return nil
	}
	return s.backend.Close()
}

func (s *Store) EnsureReady(ctx context.Context) error {
	if s == nil || s.backend == nil {
		return errors.New("billing store is not initialized")
	}
	return s.backend.EnsureReady(ctx)
}

func (s *Store) GetOrCreateProfile(ctx context.Context, userID string, now time.Time) (Profile, error) {
	if s == nil || s.backend == nil {
		return Profile{}, errors.New("billing store is not initialized")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Profile{}, &ValidationError{Message: "user_id is required"}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	profile, err := s.backend.GetOrCreateProfile(ctx, userID, now.UTC())
	if err != nil {
		return Profile{}, err
	}
	return normalizeProfile(profile)
}

func (s *Store) GetOrCreateSubscriptionState(ctx context.Context, userID string, now time.Time) (SubscriptionState, error) {
	if s == nil || s.backend == nil {
		return SubscriptionState{}, errors.New("billing store is not initialized")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return SubscriptionState{}, &ValidationError{Message: "user_id is required"}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	state, err := s.backend.GetOrCreateSubscriptionState(ctx, userID, now.UTC())
	if err != nil {
		return SubscriptionState{}, err
	}
	return normalizeSubscriptionState(state)
}

func (s *Store) UpsertSubscriptionState(ctx context.Context, state SubscriptionState) (SubscriptionState, error) {
	if s == nil || s.backend == nil {
		return SubscriptionState{}, errors.New("billing store is not initialized")
	}

	normalized, err := normalizeSubscriptionState(state)
	if err != nil {
		return SubscriptionState{}, err
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = time.Now().UTC()
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = normalized.UpdatedAt
	}

	updated, err := s.backend.UpsertSubscriptionState(ctx, normalized)
	if err != nil {
		return SubscriptionState{}, err
	}
	return normalizeSubscriptionState(updated)
}

func (s *Store) CreateInvoice(ctx context.Context, invoice Invoice) error {
	if s == nil || s.backend == nil {
		return errors.New("billing store is not initialized")
	}

	normalized, err := normalizeInvoice(invoice)
	if err != nil {
		return err
	}
	if normalized.ID == "" {
		return &ValidationError{Message: "invoice id is required"}
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = time.Now().UTC()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}
	if normalized.IssuedAt.IsZero() {
		normalized.IssuedAt = normalized.CreatedAt
	}

	return s.backend.CreateInvoice(ctx, normalized)
}

func (s *Store) ListInvoices(ctx context.Context, userID string, limit, offset int) ([]Invoice, int, error) {
	if s == nil || s.backend == nil {
		return nil, 0, errors.New("billing store is not initialized")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, 0, &ValidationError{Message: "user_id is required"}
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	invoices, total, err := s.backend.ListInvoices(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	normalized := make([]Invoice, 0, len(invoices))
	for _, invoice := range invoices {
		entry, err := normalizeInvoice(invoice)
		if err != nil {
			return nil, 0, err
		}
		normalized = append(normalized, entry)
	}

	return normalized, total, nil
}

func (s *Store) GetInvoice(ctx context.Context, userID, invoiceID string) (Invoice, error) {
	if s == nil || s.backend == nil {
		return Invoice{}, errors.New("billing store is not initialized")
	}

	userID = strings.TrimSpace(userID)
	invoiceID = strings.TrimSpace(invoiceID)
	if userID == "" {
		return Invoice{}, &ValidationError{Message: "user_id is required"}
	}
	if invoiceID == "" {
		return Invoice{}, &ValidationError{Message: "invoice id is required"}
	}

	invoice, err := s.backend.GetInvoice(ctx, userID, invoiceID)
	if err != nil {
		return Invoice{}, err
	}

	return normalizeInvoice(invoice)
}

func normalizeProfile(profile Profile) (Profile, error) {
	profile.UserID = strings.TrimSpace(profile.UserID)
	if profile.UserID == "" {
		return Profile{}, &ValidationError{Message: "user_id is required"}
	}

	profile.PaymentMethod.Brand = strings.TrimSpace(strings.ToLower(profile.PaymentMethod.Brand))
	if profile.PaymentMethod.Brand == "" {
		profile.PaymentMethod.Brand = "visa"
	}
	profile.PaymentMethod.Last4 = strings.TrimSpace(profile.PaymentMethod.Last4)
	if len(profile.PaymentMethod.Last4) != 4 {
		return Profile{}, &ValidationError{Message: "payment_method.last4 must be 4 digits"}
	}
	if profile.PaymentMethod.ExpMonth < 1 || profile.PaymentMethod.ExpMonth > 12 {
		return Profile{}, &ValidationError{Message: "payment_method.exp_month must be between 1 and 12"}
	}
	if profile.PaymentMethod.ExpYear < 2000 || profile.PaymentMethod.ExpYear > 9999 {
		return Profile{}, &ValidationError{Message: "payment_method.exp_year is invalid"}
	}
	profile.CreatedAt = profile.CreatedAt.UTC()
	profile.UpdatedAt = profile.UpdatedAt.UTC()
	profile.PaymentMethod.UpdatedAt = profile.PaymentMethod.UpdatedAt.UTC()

	return profile, nil
}

func normalizeSubscriptionState(state SubscriptionState) (SubscriptionState, error) {
	state.UserID = strings.TrimSpace(state.UserID)
	if state.UserID == "" {
		return SubscriptionState{}, &ValidationError{Message: "user_id is required"}
	}
	state.CreatedAt = state.CreatedAt.UTC()
	state.UpdatedAt = state.UpdatedAt.UTC()
	if state.CanceledAt != nil {
		v := state.CanceledAt.UTC()
		state.CanceledAt = &v
	}
	return state, nil
}

func normalizeInvoice(invoice Invoice) (Invoice, error) {
	invoice.ID = strings.TrimSpace(invoice.ID)
	invoice.UserID = strings.TrimSpace(invoice.UserID)
	if invoice.UserID == "" {
		return Invoice{}, &ValidationError{Message: "user_id is required"}
	}
	if invoice.AmountCents < 0 {
		return Invoice{}, &ValidationError{Message: "amount_cents must be >= 0"}
	}
	invoice.Currency = strings.TrimSpace(strings.ToUpper(invoice.Currency))
	if invoice.Currency == "" {
		invoice.Currency = "USD"
	}

	invoice.Status = InvoiceStatus(strings.TrimSpace(strings.ToLower(string(invoice.Status))))
	if !IsValidInvoiceStatus(invoice.Status) {
		return Invoice{}, &ValidationError{Message: "invoice status must be one of: paid, pending, failed"}
	}

	invoice.ReceiptURL = strings.TrimSpace(invoice.ReceiptURL)
	invoice.IssuedAt = invoice.IssuedAt.UTC()
	invoice.CreatedAt = invoice.CreatedAt.UTC()
	invoice.UpdatedAt = invoice.UpdatedAt.UTC()
	if invoice.PeriodStart != nil {
		start := invoice.PeriodStart.UTC()
		invoice.PeriodStart = &start
	}
	if invoice.PeriodEnd != nil {
		end := invoice.PeriodEnd.UTC()
		invoice.PeriodEnd = &end
	}
	if invoice.PeriodStart != nil && invoice.PeriodEnd != nil && invoice.PeriodEnd.Before(*invoice.PeriodStart) {
		return Invoice{}, &ValidationError{Message: "period_end must be after period_start"}
	}

	return invoice, nil
}

func IsValidInvoiceStatus(status InvoiceStatus) bool {
	switch status {
	case InvoiceStatusPaid, InvoiceStatusPending, InvoiceStatusFailed:
		return true
	default:
		return false
	}
}

func defaultProfile(userID string, now time.Time) Profile {
	year := now.Year() + 2
	return Profile{
		UserID: userID,
		PaymentMethod: PaymentMethod{
			Brand:     "visa",
			Last4:     "4242",
			ExpMonth:  12,
			ExpYear:   year,
			UpdatedAt: now,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func defaultSubscriptionState(userID string, now time.Time) SubscriptionState {
	return SubscriptionState{
		UserID:            userID,
		CancelAtPeriodEnd: false,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func sortInvoicesDesc(invoices []Invoice) {
	sort.Slice(invoices, func(i, j int) bool {
		if invoices[i].IssuedAt.Equal(invoices[j].IssuedAt) {
			return invoices[i].ID > invoices[j].ID
		}
		return invoices[i].IssuedAt.After(invoices[j].IssuedAt)
	})
}

func formatAmount(amountCents int64, currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "USD"
	}
	return fmt.Sprintf("%s %.2f", currency, float64(amountCents)/100.0)
}
