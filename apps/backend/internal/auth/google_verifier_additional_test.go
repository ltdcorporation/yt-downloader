package auth

import (
	"context"
	"errors"
	"testing"
)

func TestGoogleTokenVerifier_VerifyGuardBranches(t *testing.T) {
	ctx := context.Background()

	var nilVerifier *googleTokenVerifier
	if _, err := nilVerifier.Verify(ctx, "token"); !errors.Is(err, ErrGoogleAuthDisabled) {
		t.Fatalf("expected ErrGoogleAuthDisabled for nil verifier, got %v", err)
	}

	verifier := &googleTokenVerifier{clientIDs: []string{"client-1"}}
	if _, err := verifier.Verify(ctx, "   "); !errors.Is(err, ErrGoogleTokenInvalid) {
		t.Fatalf("expected ErrGoogleTokenInvalid for empty id token, got %v", err)
	}

	// For malformed non-JWT token, validator should fail and verifier must map it to ErrGoogleTokenInvalid.
	if _, err := verifier.Verify(ctx, "not-a-jwt"); !errors.Is(err, ErrGoogleTokenInvalid) {
		t.Fatalf("expected ErrGoogleTokenInvalid for malformed id token, got %v", err)
	}
}
