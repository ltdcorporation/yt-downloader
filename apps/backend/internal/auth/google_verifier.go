package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/api/idtoken"
)

type GoogleTokenClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	FullName      string
	PictureURL    string
}

type GoogleTokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (GoogleTokenClaims, error)
}

type GoogleTokenVerifierOptions struct {
	ClientIDs []string
}

type googleTokenVerifier struct {
	clientIDs []string
}

func NewGoogleTokenVerifier(opts GoogleTokenVerifierOptions) GoogleTokenVerifier {
	clientIDs := sanitizeClientIDs(opts.ClientIDs)
	if len(clientIDs) == 0 {
		return nil
	}
	return &googleTokenVerifier{clientIDs: clientIDs}
}

func (v *googleTokenVerifier) Verify(ctx context.Context, rawIDToken string) (GoogleTokenClaims, error) {
	if v == nil || len(v.clientIDs) == 0 {
		return GoogleTokenClaims{}, ErrGoogleAuthDisabled
	}

	idToken := strings.TrimSpace(rawIDToken)
	if idToken == "" {
		return GoogleTokenClaims{}, ErrGoogleTokenInvalid
	}

	var payload *idtoken.Payload
	for _, clientID := range v.clientIDs {
		validated, err := idtoken.Validate(ctx, idToken, clientID)
		if err == nil {
			payload = validated
			break
		}
	}
	if payload == nil {
		return GoogleTokenClaims{}, ErrGoogleTokenInvalid
	}

	claims, err := extractGoogleTokenClaims(payload)
	if err != nil {
		return GoogleTokenClaims{}, err
	}
	if !claims.EmailVerified {
		return GoogleTokenClaims{}, ErrGoogleEmailUnverified
	}

	return claims, nil
}

func extractGoogleTokenClaims(payload *idtoken.Payload) (GoogleTokenClaims, error) {
	if payload == nil {
		return GoogleTokenClaims{}, ErrGoogleTokenInvalid
	}
	if !isGoogleIssuer(payload.Issuer) {
		return GoogleTokenClaims{}, ErrGoogleTokenInvalid
	}

	subject := strings.TrimSpace(payload.Subject)
	if subject == "" {
		subject = strings.TrimSpace(claimString(payload.Claims, "sub"))
	}
	if subject == "" {
		return GoogleTokenClaims{}, ErrGoogleTokenInvalid
	}

	email := strings.TrimSpace(strings.ToLower(claimString(payload.Claims, "email")))
	if email == "" {
		return GoogleTokenClaims{}, ErrGoogleTokenInvalid
	}

	emailVerified := claimBool(payload.Claims, "email_verified")

	return GoogleTokenClaims{
		Subject:       subject,
		Email:         email,
		EmailVerified: emailVerified,
		FullName:      strings.TrimSpace(claimString(payload.Claims, "name")),
		PictureURL:    strings.TrimSpace(claimString(payload.Claims, "picture")),
	}, nil
}

func sanitizeClientIDs(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func isGoogleIssuer(rawIssuer string) bool {
	switch strings.TrimSpace(rawIssuer) {
	case "accounts.google.com", "https://accounts.google.com":
		return true
	default:
		return false
	}
}

func claimString(claims map[string]interface{}, key string) string {
	if claims == nil {
		return ""
	}
	value, ok := claims[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func claimBool(claims map[string]interface{}, key string) bool {
	if claims == nil {
		return false
	}
	value, ok := claims[key]
	if !ok || value == nil {
		return false
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false
		}
		return parsed
	case float64:
		return typed != 0
	default:
		return false
	}
}

func wrapGoogleVerifyError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrGoogleTokenInvalid) || errors.Is(err, ErrGoogleEmailUnverified) || errors.Is(err, ErrGoogleAuthDisabled) {
		return err
	}
	return fmt.Errorf("google token verification failed: %w", err)
}
