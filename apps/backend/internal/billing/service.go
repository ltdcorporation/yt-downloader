package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Benefit struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type SubscriptionSummary struct {
	Plan              string     `json:"plan"`
	Status            string     `json:"status"`
	Interval          string     `json:"interval"`
	PriceCents        int64      `json:"price_cents"`
	Currency          string     `json:"currency"`
	PlanExpiresAt     *time.Time `json:"plan_expires_at,omitempty"`
	NextBillingAt     *time.Time `json:"next_billing_at,omitempty"`
	CancelAtPeriodEnd bool       `json:"cancel_at_period_end"`
	Benefits          []Benefit  `json:"benefits"`
}

type Dashboard struct {
	Subscription  SubscriptionSummary `json:"subscription"`
	PaymentMethod PaymentMethod       `json:"payment_method"`
}

type PlanPricing struct {
	Interval   string
	PriceCents int64
	Currency   string
	Benefits   []Benefit
}

type Service struct {
	store *Store
	now   func() time.Time
}

func NewService(store *Store) *Service {
	return &Service{
		store: store,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) GetDashboard(ctx context.Context, userID string, plan string, planExpiresAt *time.Time) (Dashboard, error) {
	if s == nil || s.store == nil {
		return Dashboard{}, errors.New("billing service is not initialized")
	}
	if err := s.store.EnsureReady(ctx); err != nil {
		return Dashboard{}, err
	}

	profile, err := s.store.GetOrCreateProfile(ctx, userID, s.now())
	if err != nil {
		return Dashboard{}, err
	}
	state, err := s.store.GetOrCreateSubscriptionState(ctx, userID, s.now())
	if err != nil {
		return Dashboard{}, err
	}

	summary, err := buildSubscriptionSummary(plan, planExpiresAt, state, s.now())
	if err != nil {
		return Dashboard{}, err
	}

	return Dashboard{
		Subscription:  summary,
		PaymentMethod: profile.PaymentMethod,
	}, nil
}

func (s *Service) ListInvoices(ctx context.Context, userID string, limit, offset int) ([]Invoice, int, error) {
	if s == nil || s.store == nil {
		return nil, 0, errors.New("billing service is not initialized")
	}
	if err := s.store.EnsureReady(ctx); err != nil {
		return nil, 0, err
	}
	return s.store.ListInvoices(ctx, userID, limit, offset)
}

func (s *Service) GetInvoice(ctx context.Context, userID, invoiceID string) (Invoice, error) {
	if s == nil || s.store == nil {
		return Invoice{}, errors.New("billing service is not initialized")
	}
	if err := s.store.EnsureReady(ctx); err != nil {
		return Invoice{}, err
	}
	return s.store.GetInvoice(ctx, userID, invoiceID)
}

func (s *Service) ScheduleCancelAtPeriodEnd(ctx context.Context, userID string) (SubscriptionState, error) {
	if s == nil || s.store == nil {
		return SubscriptionState{}, errors.New("billing service is not initialized")
	}
	if err := s.store.EnsureReady(ctx); err != nil {
		return SubscriptionState{}, err
	}

	current, err := s.store.GetOrCreateSubscriptionState(ctx, userID, s.now())
	if err != nil {
		return SubscriptionState{}, err
	}
	now := s.now()
	current.CancelAtPeriodEnd = true
	current.CanceledAt = &now
	current.UpdatedAt = now

	return s.store.UpsertSubscriptionState(ctx, current)
}

func (s *Service) ClearCancelSchedule(ctx context.Context, userID string) (SubscriptionState, error) {
	if s == nil || s.store == nil {
		return SubscriptionState{}, errors.New("billing service is not initialized")
	}
	if err := s.store.EnsureReady(ctx); err != nil {
		return SubscriptionState{}, err
	}

	current, err := s.store.GetOrCreateSubscriptionState(ctx, userID, s.now())
	if err != nil {
		return SubscriptionState{}, err
	}
	current.CancelAtPeriodEnd = false
	current.CanceledAt = nil
	current.UpdatedAt = s.now()

	return s.store.UpsertSubscriptionState(ctx, current)
}

func (s *Service) CreatePlanInvoice(ctx context.Context, userID, plan string, planExpiresAt *time.Time) (Invoice, error) {
	if s == nil || s.store == nil {
		return Invoice{}, errors.New("billing service is not initialized")
	}
	if err := s.store.EnsureReady(ctx); err != nil {
		return Invoice{}, err
	}

	pricing, err := planPricing(plan)
	if err != nil {
		return Invoice{}, err
	}
	if pricing.PriceCents <= 0 {
		return Invoice{}, &ValidationError{Message: "cannot create paid invoice for free plan"}
	}

	now := s.now()
	invoice := Invoice{
		ID:          "inv_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		UserID:      strings.TrimSpace(userID),
		AmountCents: pricing.PriceCents,
		Currency:    pricing.Currency,
		Status:      InvoiceStatusPaid,
		IssuedAt:    now,
		PeriodStart: &now,
		PeriodEnd:   planExpiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.store.CreateInvoice(ctx, invoice); err != nil {
		return Invoice{}, err
	}
	return invoice, nil
}

func buildSubscriptionSummary(plan string, planExpiresAt *time.Time, state SubscriptionState, now time.Time) (SubscriptionSummary, error) {
	pricing, err := planPricing(plan)
	if err != nil {
		return SubscriptionSummary{}, err
	}

	summary := SubscriptionSummary{
		Plan:              normalizePlan(plan),
		Status:            "inactive",
		Interval:          pricing.Interval,
		PriceCents:        pricing.PriceCents,
		Currency:          pricing.Currency,
		CancelAtPeriodEnd: state.CancelAtPeriodEnd,
		Benefits:          pricing.Benefits,
	}
	if planExpiresAt != nil {
		value := planExpiresAt.UTC()
		summary.PlanExpiresAt = &value
		summary.NextBillingAt = &value
	}

	if summary.Plan == "free" {
		summary.Status = "inactive"
		summary.NextBillingAt = nil
		summary.PlanExpiresAt = nil
		summary.CancelAtPeriodEnd = false
		return summary, nil
	}

	if summary.PlanExpiresAt != nil && summary.PlanExpiresAt.Before(now.UTC()) {
		summary.Status = "expired"
		return summary, nil
	}

	if summary.CancelAtPeriodEnd {
		summary.Status = "cancel_scheduled"
	} else {
		summary.Status = "active"
	}

	return summary, nil
}

func planPricing(plan string) (PlanPricing, error) {
	switch normalizePlan(plan) {
	case "free":
		return PlanPricing{
			Interval:   "none",
			PriceCents: 0,
			Currency:   "USD",
			Benefits: []Benefit{
				{ID: "core-download", Label: "Standard quality downloads"},
				{ID: "history", Label: "Basic history access"},
			},
		}, nil
	case "daily":
		return PlanPricing{
			Interval:   "day",
			PriceCents: 300,
			Currency:   "USD",
			Benefits: []Benefit{
				{ID: "hq", Label: "High quality downloads"},
				{ID: "priority", Label: "Priority processing"},
			},
		}, nil
	case "weekly":
		return PlanPricing{
			Interval:   "week",
			PriceCents: 1200,
			Currency:   "USD",
			Benefits: []Benefit{
				{ID: "hq", Label: "High quality downloads"},
				{ID: "priority", Label: "Priority processing"},
				{ID: "storage", Label: "Cloud storage support"},
			},
		}, nil
	case "monthly":
		return PlanPricing{
			Interval:   "month",
			PriceCents: 1200,
			Currency:   "USD",
			Benefits: []Benefit{
				{ID: "4k", Label: "Unlimited 4K downloads"},
				{ID: "priority", Label: "Priority processing"},
				{ID: "storage", Label: "Cloud storage (50GB)"},
				{ID: "trim", Label: "Auto-trim & silence removal"},
			},
		}, nil
	default:
		return PlanPricing{}, &ValidationError{Message: fmt.Sprintf("unsupported plan: %s", plan)}
	}
}

func normalizePlan(plan string) string {
	return strings.ToLower(strings.TrimSpace(plan))
}
