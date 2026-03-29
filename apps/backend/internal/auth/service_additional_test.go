package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestService_ListUsers_WrapperAndErrors(t *testing.T) {
	ctx := context.Background()

	store := &Store{backend: &fakeBackend{
		listUsersFn: func(_ context.Context, limit, offset int) ([]User, int, error) {
			if limit != 7 || offset != 3 {
				t.Fatalf("unexpected list users args limit=%d offset=%d", limit, offset)
			}
			return []User{
				{ID: "usr_1", FullName: "User One", Email: "one@example.com", Role: RoleUser, Plan: PlanFree, CreatedAt: time.Now().UTC()},
				{ID: "usr_2", FullName: "User Two", Email: "two@example.com", Role: RoleAdmin, Plan: PlanWeekly, CreatedAt: time.Now().UTC()},
			}, 2, nil
		},
	}}
	svc := NewService(store, Options{})

	users, total, err := svc.ListUsers(ctx, 7, 3)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if total != 2 || len(users) != 2 {
		t.Fatalf("expected 2 users from wrapper, total=%d len=%d", total, len(users))
	}
	if users[0].Email != "one@example.com" || users[1].Role != RoleAdmin {
		t.Fatalf("unexpected mapped public users: %+v", users)
	}

	expectedErr := errors.New("list users failed")
	svc = NewService(&Store{backend: &fakeBackend{listUsersFn: func(context.Context, int, int) ([]User, int, error) {
		return nil, 0, expectedErr
	}}}, Options{})
	if _, _, err := svc.ListUsers(ctx, 10, 0); !errors.Is(err, expectedErr) {
		t.Fatalf("expected list users error propagation, got %v", err)
	}

	var nilSvc *Service
	if _, _, err := nilSvc.ListUsers(ctx, 10, 0); err == nil {
		t.Fatalf("expected nil service ListUsers error")
	}
}

func TestService_GetUserStatsAndGetUserAdditionalBranches(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 29, 9, 10, 0, 0, time.UTC)

	store := &Store{backend: &fakeBackend{
		getUserStatsFn: func(_ context.Context, seen time.Time) (UserStats, error) {
			if !seen.Equal(now) {
				t.Fatalf("unexpected now in GetUserStats: %s", seen)
			}
			return UserStats{TotalUsers: 10, AdminUsers: 1}, nil
		},
		getUserByIDFn: func(_ context.Context, userID string) (User, error) {
			if userID != "usr_1" {
				t.Fatalf("unexpected user id for GetUser: %q", userID)
			}
			return User{ID: userID, FullName: "User One", Email: "one@example.com", CreatedAt: now}, nil
		},
	}}
	svc := NewService(store, Options{})
	svc.now = func() time.Time { return now }

	stats, err := svc.GetUserStats(ctx)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}
	if stats.TotalUsers != 10 || stats.AdminUsers != 1 {
		t.Fatalf("unexpected stats payload: %+v", stats)
	}

	user, err := svc.GetUser(ctx, "  usr_1  ")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user.Email != "one@example.com" {
		t.Fatalf("unexpected GetUser payload: %+v", user)
	}

	if _, err := svc.GetUser(ctx, "   "); err == nil {
		t.Fatalf("expected user_id validation error")
	}
}

func TestAuthHelperFunctions_AdditionalCoverage(t *testing.T) {
	now := time.Date(2026, 3, 29, 9, 20, 0, 0, time.UTC)
	later := now.Add(time.Minute)
	if !timesEqual(nil, nil) {
		t.Fatalf("expected nil times to be equal")
	}
	if timesEqual(&now, nil) || timesEqual(nil, &later) {
		t.Fatalf("expected nil/non-nil times to be non-equal")
	}
	if !timesEqual(&now, &now) {
		t.Fatalf("expected identical times to be equal")
	}
	if timesEqual(&now, &later) {
		t.Fatalf("expected different times to be non-equal")
	}

	if defaultPlanDuration(PlanDaily) != 24*time.Hour {
		t.Fatalf("unexpected daily default plan duration")
	}
	if defaultPlanDuration(PlanWeekly) != 7*24*time.Hour {
		t.Fatalf("unexpected weekly default plan duration")
	}
	if defaultPlanDuration(PlanMonthly) != 30*24*time.Hour {
		t.Fatalf("unexpected monthly default plan duration")
	}
	if defaultPlanDuration(Plan("unknown")) != 0 {
		t.Fatalf("expected unknown plan default duration to be 0")
	}

	name, err := normalizeGoogleDisplayName("  Google   User  ", "user@example.com")
	if err != nil || name != "Google User" {
		t.Fatalf("unexpected normalized google display name with explicit raw name: %q err=%v", name, err)
	}
	name, err = normalizeGoogleDisplayName("", "first.last-123@example.com")
	if err != nil || name != "first last 123" {
		t.Fatalf("unexpected fallback display name from email local part: %q err=%v", name, err)
	}
	name, err = normalizeGoogleDisplayName("", "@@")
	if err != nil || name != "Google User" {
		t.Fatalf("unexpected fallback display name for invalid local part: %q err=%v", name, err)
	}
}
