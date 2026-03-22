package history

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type memoryBackend struct {
	mu sync.RWMutex

	itemsByID            map[string]Item
	activeItemByUserHash map[string]string
	attemptsByID         map[string]Attempt
	attemptIDByJobID     map[string]string
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{
		itemsByID:            make(map[string]Item),
		activeItemByUserHash: make(map[string]string),
		attemptsByID:         make(map[string]Attempt),
		attemptIDByJobID:     make(map[string]string),
	}
}

func (m *memoryBackend) Close() error {
	return nil
}

func (m *memoryBackend) EnsureReady(_ context.Context) error {
	return nil
}

func (m *memoryBackend) UpsertItem(_ context.Context, item Item) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}

	key := buildUserHashKey(item.UserID, item.SourceURLHash)
	if existingID, ok := m.activeItemByUserHash[key]; ok {
		existing, ok := m.itemsByID[existingID]
		if ok && existing.DeletedAt == nil {
			existing.Platform = item.Platform
			existing.SourceURL = item.SourceURL
			if item.Title != "" {
				existing.Title = item.Title
			}
			if item.ThumbnailURL != "" {
				existing.ThumbnailURL = item.ThumbnailURL
			}
			if item.LastAttemptAt != nil {
				t := item.LastAttemptAt.UTC()
				existing.LastAttemptAt = &t
			}
			if item.LastSuccessAt != nil {
				t := item.LastSuccessAt.UTC()
				existing.LastSuccessAt = &t
			}
			increment := item.AttemptCount
			if increment <= 0 {
				increment = 1
			}
			existing.AttemptCount += increment
			existing.UpdatedAt = item.UpdatedAt
			m.itemsByID[existing.ID] = copyItem(existing)
			return copyItem(existing), nil
		}
	}

	item.DeletedAt = nil
	m.itemsByID[item.ID] = copyItem(item)
	m.activeItemByUserHash[key] = item.ID
	return copyItem(item), nil
}

func (m *memoryBackend) GetItemByID(_ context.Context, userID, itemID string) (Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.itemsByID[itemID]
	if !ok || item.UserID != userID || item.DeletedAt != nil {
		return Item{}, ErrItemNotFound
	}
	return copyItem(item), nil
}

func (m *memoryBackend) SoftDeleteItem(_ context.Context, userID, itemID string, deletedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.itemsByID[itemID]
	if !ok || item.UserID != userID || item.DeletedAt != nil {
		return ErrItemNotFound
	}

	t := deletedAt.UTC()
	item.DeletedAt = &t
	item.UpdatedAt = t
	m.itemsByID[itemID] = copyItem(item)
	delete(m.activeItemByUserHash, buildUserHashKey(item.UserID, item.SourceURLHash))
	return nil
}

func (m *memoryBackend) MarkItemSuccess(_ context.Context, userID, itemID string, succeededAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.itemsByID[itemID]
	if !ok || item.UserID != userID || item.DeletedAt != nil {
		return ErrItemNotFound
	}

	t := succeededAt.UTC()
	item.LastSuccessAt = &t
	item.UpdatedAt = t
	m.itemsByID[itemID] = copyItem(item)
	return nil
}

func (m *memoryBackend) CreateAttempt(_ context.Context, attempt Attempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.itemsByID[attempt.HistoryItemID]
	if !ok || item.UserID != attempt.UserID || item.DeletedAt != nil {
		return ErrItemNotFound
	}
	if _, exists := m.attemptsByID[attempt.ID]; exists {
		return ErrConflict
	}
	if attempt.JobID != "" {
		if existingID, exists := m.attemptIDByJobID[attempt.JobID]; exists && existingID != attempt.ID {
			return fmt.Errorf("%w: job_id already exists", ErrConflict)
		}
	}

	m.attemptsByID[attempt.ID] = copyAttempt(attempt)
	if attempt.JobID != "" {
		m.attemptIDByJobID[attempt.JobID] = attempt.ID
	}
	return nil
}

func (m *memoryBackend) UpdateAttempt(_ context.Context, attempt Attempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.attemptsByID[attempt.ID]
	if !ok || existing.UserID != attempt.UserID {
		return ErrAttemptNotFound
	}

	if existing.JobID != "" && existing.JobID != attempt.JobID {
		delete(m.attemptIDByJobID, existing.JobID)
	}
	if attempt.JobID != "" {
		if existingID, exists := m.attemptIDByJobID[attempt.JobID]; exists && existingID != attempt.ID {
			return fmt.Errorf("%w: job_id already exists", ErrConflict)
		}
		m.attemptIDByJobID[attempt.JobID] = attempt.ID
	}

	m.attemptsByID[attempt.ID] = copyAttempt(attempt)
	return nil
}

func (m *memoryBackend) GetAttemptByID(_ context.Context, userID, attemptID string) (Attempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	attempt, ok := m.attemptsByID[attemptID]
	if !ok || attempt.UserID != userID {
		return Attempt{}, ErrAttemptNotFound
	}
	return copyAttempt(attempt), nil
}

func (m *memoryBackend) GetAttemptByJobID(_ context.Context, jobID string) (Attempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	attemptID, ok := m.attemptIDByJobID[jobID]
	if !ok {
		return Attempt{}, ErrAttemptNotFound
	}
	attempt, ok := m.attemptsByID[attemptID]
	if !ok {
		return Attempt{}, ErrAttemptNotFound
	}
	return copyAttempt(attempt), nil
}

func (m *memoryBackend) GetLatestAttemptByItem(_ context.Context, userID, itemID string) (Attempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.itemsByID[itemID]
	if !ok || item.UserID != userID || item.DeletedAt != nil {
		return Attempt{}, ErrItemNotFound
	}

	attempt, ok := m.latestAttemptForItemLocked(userID, itemID)
	if !ok {
		return Attempt{}, ErrAttemptNotFound
	}
	return copyAttempt(attempt), nil
}

func (m *memoryBackend) ListItems(_ context.Context, userID string, filter ListFilter) (ListPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]ListEntry, 0, len(m.itemsByID))
	query := strings.ToLower(strings.TrimSpace(filter.Query))

	for _, item := range m.itemsByID {
		if item.UserID != userID || item.DeletedAt != nil {
			continue
		}
		if filter.Platform != "" && item.Platform != filter.Platform {
			continue
		}
		if query != "" {
			title := strings.ToLower(item.Title)
			sourceURL := strings.ToLower(item.SourceURL)
			if !strings.Contains(title, query) && !strings.Contains(sourceURL, query) {
				continue
			}
		}

		latestAttempt, hasAttempt := m.latestAttemptForItemLocked(userID, item.ID)
		if filter.Status != "" {
			if !hasAttempt || latestAttempt.Status != filter.Status {
				continue
			}
		}

		entry := ListEntry{Item: copyItem(item)}
		if hasAttempt {
			attemptCopy := copyAttempt(latestAttempt)
			entry.LatestAttempt = &attemptCopy
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		leftSortAt := itemSortAt(entries[i].Item)
		rightSortAt := itemSortAt(entries[j].Item)
		if leftSortAt.Equal(rightSortAt) {
			return entries[i].Item.ID > entries[j].Item.ID
		}
		return leftSortAt.After(rightSortAt)
	})

	filtered := make([]ListEntry, 0, len(entries))
	for _, entry := range entries {
		if filter.Cursor != nil {
			sortAt := itemSortAt(entry.Item)
			if sortAt.After(filter.Cursor.SortAt) {
				continue
			}
			if sortAt.Equal(filter.Cursor.SortAt) && entry.Item.ID >= filter.Cursor.ItemID {
				continue
			}
		}
		filtered = append(filtered, entry)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}

	page := ListPage{Entries: make([]ListEntry, 0, limit)}
	for i, entry := range filtered {
		if i >= limit {
			page.HasMore = true
			break
		}
		page.Entries = append(page.Entries, entry)
	}
	if page.HasMore && len(page.Entries) > 0 {
		last := page.Entries[len(page.Entries)-1].Item
		page.NextCursor = &ListCursor{SortAt: itemSortAt(last), ItemID: last.ID}
	}

	return page, nil
}

func (m *memoryBackend) GetStats(_ context.Context, userID string) (Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := Stats{}
	activeItemIDs := make(map[string]struct{})
	monthStart := time.Now().UTC().Truncate(24 * time.Hour)
	monthStart = time.Date(monthStart.Year(), monthStart.Month(), 1, 0, 0, 0, 0, time.UTC)

	for _, item := range m.itemsByID {
		if item.UserID != userID || item.DeletedAt != nil {
			continue
		}
		stats.TotalItems++
		activeItemIDs[item.ID] = struct{}{}
	}

	for _, attempt := range m.attemptsByID {
		if attempt.UserID != userID {
			continue
		}
		if _, ok := activeItemIDs[attempt.HistoryItemID]; !ok {
			continue
		}

		stats.TotalAttempts++
		switch attempt.Status {
		case StatusDone:
			stats.SuccessCount++
			if attempt.SizeBytes != nil {
				stats.TotalBytesDownloaded += *attempt.SizeBytes
			}
		case StatusFailed:
			stats.FailedCount++
		}
		if !attempt.CreatedAt.Before(monthStart) {
			stats.ThisMonthAttempts++
		}
	}

	return stats, nil
}

func (m *memoryBackend) latestAttemptForItemLocked(userID, itemID string) (Attempt, bool) {
	var (
		latest Attempt
		found  bool
	)
	for _, attempt := range m.attemptsByID {
		if attempt.UserID != userID || attempt.HistoryItemID != itemID {
			continue
		}
		if !found {
			latest = attempt
			found = true
			continue
		}
		if attempt.CreatedAt.After(latest.CreatedAt) || (attempt.CreatedAt.Equal(latest.CreatedAt) && attempt.ID > latest.ID) {
			latest = attempt
		}
	}
	return latest, found
}

func itemSortAt(item Item) time.Time {
	if item.LastAttemptAt != nil {
		return item.LastAttemptAt.UTC()
	}
	return item.CreatedAt.UTC()
}

func buildUserHashKey(userID, sourceURLHash string) string {
	return userID + "|" + sourceURLHash
}

func copyItem(input Item) Item {
	out := input
	if input.LastAttemptAt != nil {
		t := input.LastAttemptAt.UTC()
		out.LastAttemptAt = &t
	}
	if input.LastSuccessAt != nil {
		t := input.LastSuccessAt.UTC()
		out.LastSuccessAt = &t
	}
	if input.DeletedAt != nil {
		t := input.DeletedAt.UTC()
		out.DeletedAt = &t
	}
	return out
}

func copyAttempt(input Attempt) Attempt {
	out := input
	if input.SizeBytes != nil {
		value := *input.SizeBytes
		out.SizeBytes = &value
	}
	if input.ExpiresAt != nil {
		t := input.ExpiresAt.UTC()
		out.ExpiresAt = &t
	}
	if input.CompletedAt != nil {
		t := input.CompletedAt.UTC()
		out.CompletedAt = &t
	}
	return out
}
