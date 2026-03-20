package auth

import (
	"errors"
	"testing"

	"google.golang.org/api/idtoken"
)

func TestNewGoogleTokenVerifier(t *testing.T) {
	if verifier := NewGoogleTokenVerifier(GoogleTokenVerifierOptions{}); verifier != nil {
		t.Fatalf("expected nil verifier when no client IDs provided")
	}

	verifier := NewGoogleTokenVerifier(GoogleTokenVerifierOptions{ClientIDs: []string{"", "client-1", "client-1", " client-2 "}})
	if verifier == nil {
		t.Fatalf("expected non-nil verifier with valid client IDs")
	}
}

func TestExtractGoogleTokenClaims(t *testing.T) {
	payload := &idtoken.Payload{
		Issuer:  "https://accounts.google.com",
		Subject: "sub_1",
		Claims: map[string]interface{}{
			"email":          "user@example.com",
			"email_verified": true,
			"name":           "User Name",
			"picture":        "https://example.com/u.png",
		},
	}

	claims, err := extractGoogleTokenClaims(payload)
	if err != nil {
		t.Fatalf("extractGoogleTokenClaims failed: %v", err)
	}
	if claims.Subject != "sub_1" || claims.Email != "user@example.com" || !claims.EmailVerified {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.FullName != "User Name" || claims.PictureURL != "https://example.com/u.png" {
		t.Fatalf("unexpected optional claims: %+v", claims)
	}
}

func TestExtractGoogleTokenClaims_Errors(t *testing.T) {
	tests := []struct {
		name    string
		payload *idtoken.Payload
	}{
		{name: "nil payload", payload: nil},
		{name: "invalid issuer", payload: &idtoken.Payload{Issuer: "https://issuer.invalid", Subject: "sub", Claims: map[string]interface{}{"email": "x@example.com"}}},
		{name: "missing subject", payload: &idtoken.Payload{Issuer: "accounts.google.com", Claims: map[string]interface{}{"email": "x@example.com"}}},
		{name: "missing email", payload: &idtoken.Payload{Issuer: "accounts.google.com", Subject: "sub", Claims: map[string]interface{}{}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := extractGoogleTokenClaims(tc.payload)
			if !errors.Is(err, ErrGoogleTokenInvalid) {
				t.Fatalf("expected ErrGoogleTokenInvalid, got %v", err)
			}
		})
	}
}

func TestClaimHelpers(t *testing.T) {
	claims := map[string]interface{}{
		"string":          "value",
		"bool_true":       true,
		"bool_string":     "true",
		"bool_string_bad": "oops",
		"bool_number":     float64(1),
	}

	if got := claimString(claims, "string"); got != "value" {
		t.Fatalf("unexpected claimString value: %q", got)
	}
	if got := claimString(claims, "missing"); got != "" {
		t.Fatalf("expected empty claimString for missing key, got %q", got)
	}

	if !claimBool(claims, "bool_true") {
		t.Fatalf("expected bool_true to parse true")
	}
	if !claimBool(claims, "bool_string") {
		t.Fatalf("expected bool_string to parse true")
	}
	if !claimBool(claims, "bool_number") {
		t.Fatalf("expected bool_number to parse true")
	}
	if claimBool(claims, "bool_string_bad") {
		t.Fatalf("expected bool_string_bad to parse false")
	}
	if claimBool(claims, "missing") {
		t.Fatalf("expected missing bool to parse false")
	}
}

func TestGoogleHelperFunctions(t *testing.T) {
	if !isGoogleIssuer("accounts.google.com") || !isGoogleIssuer("https://accounts.google.com") {
		t.Fatalf("expected accepted google issuers")
	}
	if isGoogleIssuer("https://example.com") {
		t.Fatalf("unexpected issuer accepted")
	}

	sanitized := sanitizeClientIDs([]string{"", "client-1", "client-1", " client-2 "})
	if len(sanitized) != 2 || sanitized[0] != "client-1" || sanitized[1] != "client-2" {
		t.Fatalf("unexpected sanitized client IDs: %#v", sanitized)
	}

	if err := wrapGoogleVerifyError(nil); err != nil {
		t.Fatalf("expected nil wrapped error")
	}
	if err := wrapGoogleVerifyError(ErrGoogleTokenInvalid); !errors.Is(err, ErrGoogleTokenInvalid) {
		t.Fatalf("expected token invalid passthrough, got %v", err)
	}
	wrapped := wrapGoogleVerifyError(errors.New("boom"))
	if wrapped == nil || wrapped.Error() == "boom" {
		t.Fatalf("expected wrapped generic error, got %v", wrapped)
	}
}
