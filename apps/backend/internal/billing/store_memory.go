package billing

import (
	"context"
	"sync"
	"time"
)

type memoryBackend struct {
	mu                 sync.RWMutex
	profiles           map[string]Profile
	subscriptionStates map[string]SubscriptionState
	invoicesByUser     map[string][]Invoice
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{
		profiles:           map[string]Profile{},
		subscriptionStates: map[string]SubscriptionState{},
		invoicesByUser:     map[string][]Invoice{},
	}
}

func (m *memoryBackend) Close() error {
	return nil
}

func (m *memoryBackend) EnsureReady(_ context.Context) error {
	return nil
}

func (m *memoryBackend) GetOrCreateProfile(_ context.Context, userID string, now time.Time) (Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if profile, exists := m.profiles[userID]; exists {
		return normalizeProfile(profile)
	}

	profile := defaultProfile(userID, now.UTC())
	m.profiles[userID] = profile
	return normalizeProfile(profile)
}

func (m *memoryBackend) GetOrCreateSubscriptionState(_ context.Context, userID string, now time.Time) (SubscriptionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.subscriptionStates[userID]; exists {
		return normalizeSubscriptionState(state)
	}

	state := defaultSubscriptionState(userID, now.UTC())
	m.subscriptionStates[userID] = state
	return normalizeSubscriptionState(state)
}

func (m *memoryBackend) UpsertSubscriptionState(_ context.Context, state SubscriptionState) (SubscriptionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state = copySubscriptionState(state)
	m.subscriptionStates[state.UserID] = state
	return normalizeSubscriptionState(state)
}

func (m *memoryBackend) CreateInvoice(_ context.Context, invoice Invoice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := m.invoicesByUser[invoice.UserID]
	for _, existing := range items {
		if existing.ID == invoice.ID {
			return &ValidationError{Message: "invoice id already exists"}
		}
	}

	entry := copyInvoice(invoice)
	items = append(items, entry)
	sortInvoicesDesc(items)
	m.invoicesByUser[invoice.UserID] = items
	return nil
}

func (m *memoryBackend) ListInvoices(_ context.Context, userID string, limit, offset int) ([]Invoice, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := m.invoicesByUser[userID]
	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	result := make([]Invoice, 0, end-offset)
	for _, item := range items[offset:end] {
		result = append(result, copyInvoice(item))
	}
	return result, total, nil
}

func (m *memoryBackend) GetInvoice(_ context.Context, userID, invoiceID string) (Invoice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := m.invoicesByUser[userID]
	for _, item := range items {
		if item.ID == invoiceID {
			return normalizeInvoice(copyInvoice(item))
		}
	}
	return Invoice{}, ErrInvoiceNotFound
}

func copySubscriptionState(state SubscriptionState) SubscriptionState {
	copied := state
	if state.CanceledAt != nil {
		value := state.CanceledAt.UTC()
		copied.CanceledAt = &value
	}
	return copied
}

func copyInvoice(invoice Invoice) Invoice {
	copied := invoice
	if invoice.PeriodStart != nil {
		value := invoice.PeriodStart.UTC()
		copied.PeriodStart = &value
	}
	if invoice.PeriodEnd != nil {
		value := invoice.PeriodEnd.UTC()
		copied.PeriodEnd = &value
	}
	return copied
}
