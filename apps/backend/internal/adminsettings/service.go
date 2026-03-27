package adminsettings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
		return "admin settings version conflict"
	}
	if e.CurrentVersion > 0 {
		return fmt.Sprintf("admin settings version conflict (current=%d)", e.CurrentVersion)
	}
	return "admin settings version conflict"
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
		return Snapshot{}, errors.New("admin settings service is not initialized")
	}
	if err := s.store.EnsureReady(ctx); err != nil {
		return Snapshot{}, err
	}
	return s.store.GetOrCreateSnapshot(ctx, s.now())
}

func (s *Service) Patch(ctx context.Context, input PatchInput) (Snapshot, error) {
	if s == nil || s.store == nil {
		return Snapshot{}, errors.New("admin settings service is not initialized")
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

	source := strings.TrimSpace(strings.ToLower(input.Source))
	if source == "" {
		source = "admin_web"
	}

	updated, err := s.store.ApplyPatch(ctx, ApplyPatchParams{
		Before:        current,
		After:         next,
		ChangedFields: changedFields,
		ActorUserID:   strings.TrimSpace(input.ActorUserID),
		RequestID:     strings.TrimSpace(input.RequestID),
		Source:        source,
		ChangedAt:     now,
		AuditID:       "asa_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
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
	changedFields := make([]string, 0, 6)

	if !patchHasAnyField(patch) {
		return Snapshot{}, nil, &ValidationError{Message: "settings patch must include at least one field"}
	}

	if patch.Preferences != nil {
		if patch.Preferences.DefaultQuality != nil {
			quality, err := normalizeQuality(*patch.Preferences.DefaultQuality)
			if err != nil {
				return Snapshot{}, nil, &ValidationError{Message: "settings.preferences.default_quality is invalid"}
			}
			if next.Data.Preferences.DefaultQuality != quality {
				next.Data.Preferences.DefaultQuality = quality
				changedFields = append(changedFields, "preferences.default_quality")
			}
		}
		if patch.Preferences.AutoTrimSilence != nil && next.Data.Preferences.AutoTrimSilence != *patch.Preferences.AutoTrimSilence {
			next.Data.Preferences.AutoTrimSilence = *patch.Preferences.AutoTrimSilence
			changedFields = append(changedFields, "preferences.auto_trim_silence")
		}
		if patch.Preferences.ThumbnailGeneration != nil && next.Data.Preferences.ThumbnailGeneration != *patch.Preferences.ThumbnailGeneration {
			next.Data.Preferences.ThumbnailGeneration = *patch.Preferences.ThumbnailGeneration
			changedFields = append(changedFields, "preferences.thumbnail_generation")
		}
	}

	if patch.Notifications != nil && patch.Notifications.Email != nil {
		emailPatch := patch.Notifications.Email
		if emailPatch.Processing != nil && next.Data.Notifications.Email.Processing != *emailPatch.Processing {
			next.Data.Notifications.Email.Processing = *emailPatch.Processing
			changedFields = append(changedFields, "notifications.email.processing")
		}
		if emailPatch.Storage != nil && next.Data.Notifications.Email.Storage != *emailPatch.Storage {
			next.Data.Notifications.Email.Storage = *emailPatch.Storage
			changedFields = append(changedFields, "notifications.email.storage")
		}
		if emailPatch.Summary != nil && next.Data.Notifications.Email.Summary != *emailPatch.Summary {
			next.Data.Notifications.Email.Summary = *emailPatch.Summary
			changedFields = append(changedFields, "notifications.email.summary")
		}
	}

	return next, changedFields, nil
}

func patchHasAnyField(patch Patch) bool {
	if patch.Preferences != nil {
		if patch.Preferences.DefaultQuality != nil ||
			patch.Preferences.AutoTrimSilence != nil ||
			patch.Preferences.ThumbnailGeneration != nil {
			return true
		}
	}

	if patch.Notifications != nil && patch.Notifications.Email != nil {
		emailPatch := patch.Notifications.Email
		if emailPatch.Processing != nil || emailPatch.Storage != nil || emailPatch.Summary != nil {
			return true
		}
	}

	return false
}
