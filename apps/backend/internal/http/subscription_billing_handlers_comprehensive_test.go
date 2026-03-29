package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/billing"
)

func TestSubscriptionHandlers_ComprehensiveBranches(t *testing.T) {
	t.Run("subscription get auth service unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)
		server.authService = nil

		req := httptest.NewRequest(http.MethodGet, "/v1/subscription", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when auth service is nil, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "subscription_unavailable" {
			t.Fatalf("expected subscription_unavailable code, got %#v", payload["code"])
		}
	})

	t.Run("subscription get dashboard failure from billing service", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)
		server.billingService = billing.NewService(&billing.Store{}) // non-nil but broken backend

		req := httptest.NewRequest(http.MethodGet, "/v1/subscription", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when billing dashboard call fails, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "subscription_unavailable" {
			t.Fatalf("expected subscription_unavailable code, got %#v", payload["code"])
		}
	})

	t.Run("subscription patch free-plan branch after paid plan", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)

		toPaidReq := httptest.NewRequest(http.MethodPatch, "/v1/subscription", bytes.NewBufferString(`{"subscription":{"plan":"monthly"}}`))
		toPaidReq.Header.Set("Authorization", "Bearer "+token)
		toPaidReq.Header.Set("Content-Type", "application/json")
		toPaidRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(toPaidRec, toPaidReq)
		if toPaidRec.Code != http.StatusOK {
			t.Fatalf("expected 200 when upgrading to monthly before free-plan patch, got %d body=%s", toPaidRec.Code, toPaidRec.Body.String())
		}

		toFreeReq := httptest.NewRequest(http.MethodPatch, "/v1/subscription", bytes.NewBufferString(`{"subscription":{"plan":"free"}}`))
		toFreeReq.Header.Set("Authorization", "Bearer "+token)
		toFreeReq.Header.Set("Content-Type", "application/json")
		toFreeRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(toFreeRec, toFreeReq)
		if toFreeRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for patch back to free plan, got %d body=%s", toFreeRec.Code, toFreeRec.Body.String())
		}

		payload := decodeJSONMap(t, toFreeRec.Body.Bytes())
		subscription := payload["subscription"].(map[string]any)
		if subscription["plan"] != "free" {
			t.Fatalf("expected free plan after patch, got %#v", subscription["plan"])
		}
	})

	t.Run("subscription patch dashboard reload failure", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)
		server.billingService = billing.NewService(&billing.Store{})

		req := httptest.NewRequest(http.MethodPatch, "/v1/subscription", bytes.NewBufferString(`{"subscription":{"plan":"daily"}}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 when subscription dashboard reload fails, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "failed to load subscription" {
			t.Fatalf("unexpected patch failure payload: %#v", payload)
		}
	})

	t.Run("subscription cancel service unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)
		server.billingService = nil

		req := httptest.NewRequest(http.MethodPost, "/v1/subscription/cancel", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 for cancel when billing service is nil, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("subscription cancel schedule failure", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)

		upgradeReq := httptest.NewRequest(http.MethodPatch, "/v1/subscription", bytes.NewBufferString(`{"subscription":{"plan":"daily"}}`))
		upgradeReq.Header.Set("Authorization", "Bearer "+token)
		upgradeReq.Header.Set("Content-Type", "application/json")
		upgradeRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(upgradeRec, upgradeReq)
		if upgradeRec.Code != http.StatusOK {
			t.Fatalf("expected 200 when upgrading before cancel schedule test, got %d body=%s", upgradeRec.Code, upgradeRec.Body.String())
		}

		server.billingService = billing.NewService(&billing.Store{})
		cancelReq := httptest.NewRequest(http.MethodPost, "/v1/subscription/cancel", nil)
		cancelReq.Header.Set("Authorization", "Bearer "+token)
		cancelRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(cancelRec, cancelReq)

		if cancelRec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 when schedule cancel fails, got %d body=%s", cancelRec.Code, cancelRec.Body.String())
		}
		payload := decodeJSONMap(t, cancelRec.Body.Bytes())
		if payload["error"] != "failed to cancel subscription" {
			t.Fatalf("unexpected cancel failure payload: %#v", payload)
		}
	})

	t.Run("subscription cancel immediate dashboard failure", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)

		upgradeReq := httptest.NewRequest(http.MethodPatch, "/v1/subscription", bytes.NewBufferString(`{"subscription":{"plan":"weekly"}}`))
		upgradeReq.Header.Set("Authorization", "Bearer "+token)
		upgradeReq.Header.Set("Content-Type", "application/json")
		upgradeRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(upgradeRec, upgradeReq)
		if upgradeRec.Code != http.StatusOK {
			t.Fatalf("expected 200 when upgrading before immediate cancel test, got %d body=%s", upgradeRec.Code, upgradeRec.Body.String())
		}

		server.billingService = billing.NewService(&billing.Store{})
		cancelReq := httptest.NewRequest(http.MethodPost, "/v1/subscription/cancel", bytes.NewBufferString(`{"immediate":true}`))
		cancelReq.Header.Set("Authorization", "Bearer "+token)
		cancelReq.Header.Set("Content-Type", "application/json")
		cancelRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(cancelRec, cancelReq)

		if cancelRec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 when immediate cancel dashboard reload fails, got %d body=%s", cancelRec.Code, cancelRec.Body.String())
		}
		payload := decodeJSONMap(t, cancelRec.Body.Bytes())
		if payload["error"] != "failed to load subscription" {
			t.Fatalf("unexpected immediate cancel failure payload: %#v", payload)
		}
	})

	t.Run("subscription cancel on free plan keeps inactive status", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)

		req := httptest.NewRequest(http.MethodPost, "/v1/subscription/cancel", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for cancel on free plan, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		subscription := payload["subscription"].(map[string]any)
		if subscription["status"] != "inactive" {
			t.Fatalf("expected inactive status for free-plan cancel, got %#v", subscription["status"])
		}
	})

	t.Run("subscription reactivate service unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)
		server.billingService = nil

		req := httptest.NewRequest(http.MethodPost, "/v1/subscription/reactivate", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 for reactivate when billing service is nil, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("subscription reactivate clear schedule failure", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)
		server.billingService = billing.NewService(&billing.Store{})

		req := httptest.NewRequest(http.MethodPost, "/v1/subscription/reactivate", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for reactivate clear schedule failure, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "failed to reactivate subscription" {
			t.Fatalf("unexpected reactivate failure payload: %#v", payload)
		}
	})
}

func TestBillingHandlers_ComprehensiveServiceErrorBranches(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)

	now := timeNowUTC()
	invoice := billing.Invoice{
		ID:          "inv_billing_branch_1",
		UserID:      userID,
		AmountCents: 1999,
		Currency:    "USD",
		Status:      billing.InvoiceStatusPaid,
		IssuedAt:    now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := server.billingStore.CreateInvoice(serverCtx(), invoice); err != nil {
		t.Fatalf("failed to seed invoice for billing branch tests: %v", err)
	}

	server.billingService = billing.NewService(&billing.Store{})

	historyReq := httptest.NewRequest(http.MethodGet, "/v1/billing/history", nil)
	historyReq.Header.Set("Authorization", "Bearer "+token)
	historyRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(historyRec, historyReq)
	if historyRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for billing history service error, got %d body=%s", historyRec.Code, historyRec.Body.String())
	}
	historyPayload := decodeJSONMap(t, historyRec.Body.Bytes())
	if historyPayload["code"] != "billing_unavailable" {
		t.Fatalf("expected billing_unavailable code for history service error, got %#v", historyPayload["code"])
	}

	invoiceGetReq := httptest.NewRequest(http.MethodGet, "/v1/billing/invoices/inv_billing_branch_1", nil)
	invoiceGetReq.Header.Set("Authorization", "Bearer "+token)
	invoiceGetRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(invoiceGetRec, invoiceGetReq)
	if invoiceGetRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for invoice get service error, got %d body=%s", invoiceGetRec.Code, invoiceGetRec.Body.String())
	}
	invoiceGetPayload := decodeJSONMap(t, invoiceGetRec.Body.Bytes())
	if invoiceGetPayload["code"] != "billing_unavailable" {
		t.Fatalf("expected billing_unavailable code for invoice get service error, got %#v", invoiceGetPayload["code"])
	}

	invoiceReceiptReq := httptest.NewRequest(http.MethodGet, "/v1/billing/invoices/inv_billing_branch_1/receipt", nil)
	invoiceReceiptReq.Header.Set("Authorization", "Bearer "+token)
	invoiceReceiptRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(invoiceReceiptRec, invoiceReceiptReq)
	if invoiceReceiptRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for invoice receipt service error, got %d body=%s", invoiceReceiptRec.Code, invoiceReceiptRec.Body.String())
	}
	invoiceReceiptPayload := decodeJSONMap(t, invoiceReceiptRec.Body.Bytes())
	if invoiceReceiptPayload["code"] != "billing_unavailable" {
		t.Fatalf("expected billing_unavailable code for invoice receipt service error, got %#v", invoiceReceiptPayload["code"])
	}
}

func TestSubscriptionParseUserPlan_WhitespaceAndCase(t *testing.T) {
	plan, err := parseUserPlan("  MoNtHlY  ")
	if err != nil {
		t.Fatalf("unexpected parseUserPlan error: %v", err)
	}
	if plan != auth.PlanMonthly {
		t.Fatalf("expected parsed monthly plan, got %s", plan)
	}
}

func serverCtx() context.Context {
	return context.Background()
}

func timeNowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}
