package maintenance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PatchInput struct {
	ExpectedVersion int64
	Patch           Patch
	ActorUserID     string
	RequestID       string
	Source          string
}

type VersionConflictError struct {
	CurrentVersion int64
}

func (e *VersionConflictError) Error() string {
	if e == nil {
		return "maintenance version conflict"
	}
	if e.CurrentVersion > 0 {
		return fmt.Sprintf("maintenance version conflict (current=%d)", e.CurrentVersion)
	}
	return "maintenance version conflict"
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

func (s *Service) Get(ctx context.Context) (Snapshot, error) {
	if s == nil || s.store == nil {
		return Snapshot{}, errors.New("maintenance service is not initialized")
	}
	if err := s.store.EnsureReady(ctx); err != nil {
		return Snapshot{}, err
	}
	return s.store.GetOrCreateSnapshot(ctx, s.now())
}

func (s *Service) Patch(ctx context.Context, input PatchInput) (Snapshot, error) {
	if s == nil || s.store == nil {
		return Snapshot{}, errors.New("maintenance service is not initialized")
	}
	if err := s.store.EnsureReady(ctx); err != nil {
		return Snapshot{}, err
	}
	if input.ExpectedVersion < 1 {
		return Snapshot{}, &ValidationError{Message: "meta.version must be >= 1"}
	}

	current, err := s.store.GetOrCreateSnapshot(ctx, s.now())
	if err != nil {
		return Snapshot{}, err
	}
	if current.Version != input.ExpectedVersion {
		return Snapshot{}, &VersionConflictError{CurrentVersion: current.Version}
	}

	next, changedFields, err := mergePatch(current, input.Patch)
	if err != nil {
		return Snapshot{}, err
	}
	if len(changedFields) == 0 {
		return current, nil
	}

	now := s.now()
	next.Version = current.Version + 1
	next.CreatedAt = current.CreatedAt
	next.UpdatedAt = now
	next.UpdatedByUserID = strings.TrimSpace(input.ActorUserID)

	updated, err := s.store.ApplyPatch(ctx, ApplyPatchParams{
		Before:        current,
		After:         next,
		ChangedFields: changedFields,
		ActorUserID:   strings.TrimSpace(input.ActorUserID),
		RequestID:     strings.TrimSpace(input.RequestID),
		Source:        strings.TrimSpace(strings.ToLower(input.Source)),
		ChangedAt:     now,
	})
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			latest, latestErr := s.store.GetOrCreateSnapshot(ctx, s.now())
			if latestErr == nil {
				return Snapshot{}, &VersionConflictError{CurrentVersion: latest.Version}
			}
			return Snapshot{}, &VersionConflictError{}
		}
		return Snapshot{}, err
	}

	return updated, nil
}

func mergePatch(current Snapshot, patch Patch) (Snapshot, []string, error) {
	next := current
	changedFields := make([]string, 0, 8)

	if !patchHasAnyField(patch) {
		return Snapshot{}, nil, &ValidationError{Message: "maintenance patch must include at least one field"}
	}

	if patch.Enabled != nil && next.Data.Enabled != *patch.Enabled {
		next.Data.Enabled = *patch.Enabled
		changedFields = append(changedFields, "enabled")
	}

	if patch.EstimatedDowntime != nil {
		value := strings.TrimSpace(*patch.EstimatedDowntime)
		if value == "" {
			value = defaultEstimatedDowntime
		}
		if value != next.Data.EstimatedDowntime {
			next.Data.EstimatedDowntime = value
			changedFields = append(changedFields, "estimated_downtime")
		}
	}

	if patch.PublicMessage != nil {
		value := strings.TrimSpace(*patch.PublicMessage)
		if value == "" {
			value = defaultPublicMessage
		}
		if value != next.Data.PublicMessage {
			next.Data.PublicMessage = value
			changedFields = append(changedFields, "public_message")
		}
	}

	if len(patch.Services) > 0 {
		index := make(map[ServiceKey]ServiceOverride, len(next.Data.Services))
		for _, service := range next.Data.Services {
			index[service.Key] = service
		}

		for _, servicePatch := range patch.Services {
			key := ServiceKey(strings.TrimSpace(strings.ToLower(string(servicePatch.Key))))
			service, exists := index[key]
			if !exists {
				return Snapshot{}, nil, &ValidationError{Message: fmt.Sprintf("unsupported service key: %s", key)}
			}
			if servicePatch.Status != nil {
				status := ServiceStatus(strings.TrimSpace(strings.ToLower(string(*servicePatch.Status))))
				if !IsValidServiceStatus(status) {
					return Snapshot{}, nil, &ValidationError{Message: fmt.Sprintf("unsupported service status: %s", status)}
				}
				if service.Status != status {
					service.Status = status
					changedFields = append(changedFields, fmt.Sprintf("services.%s.status", key))
				}
			}
			if servicePatch.Enabled != nil && service.Enabled != *servicePatch.Enabled {
				service.Enabled = *servicePatch.Enabled
				changedFields = append(changedFields, fmt.Sprintf("services.%s.enabled", key))
			}
			index[key] = service
		}

		reordered := make([]ServiceOverride, 0, len(defaultServiceOverrides()))
		for _, base := range defaultServiceOverrides() {
			reordered = append(reordered, index[base.Key])
		}
		next.Data.Services = reordered
	}

	normalized, err := normalizeSnapshot(next)
	if err != nil {
		return Snapshot{}, nil, err
	}

	return normalized, normalizeChangedFields(changedFields), nil
}

func patchHasAnyField(patch Patch) bool {
	if patch.Enabled != nil || patch.EstimatedDowntime != nil || patch.PublicMessage != nil {
		return true
	}
	if len(patch.Services) > 0 {
		return true
	}
	return false
}
