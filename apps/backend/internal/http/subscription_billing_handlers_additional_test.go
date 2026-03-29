package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/billing"
)

func TestParseUserPlan_Table(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    auth.Plan
		wantErr bool
	}{
		{name: "free", raw: "free", want: auth.PlanFree},
		{name: "daily uppercase trimmed", raw: "  DAILY  ", want: auth.PlanDaily},
		{name: "weekly", raw: "weekly", want: auth.PlanWeekly},
		{name: "monthly", raw: "monthly", want: auth.PlanMonthly},
		{name: "invalid", raw: "yearly", wantErr: true},
		{name: "empty", raw: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUserPlan(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for raw=%q", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected parse error for raw=%q err=%v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("unexpected parsed plan raw=%q got=%s want=%s", tt.raw, got, tt.want)
			}
		})
	}
}

func TestBillingResponseHelpers(t *testing.T) {
	now := time.Date(2026, 3, 29, 4, 30, 0, 0, time.UTC)
	periodStart := now.Add(-24 * time.Hour)
	periodEnd := now.Add(24 * time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/history", nil)
	req.Host = "billing.local"
	req.Header.Set("X-Forwarded-Proto", "https")

	invoice := billing.Invoice{
		ID:          "inv_helper_1",
		AmountCents: 12345,
		Currency:    "usd",
		Status:      billing.InvoiceStatusPaid,
		IssuedAt:    now,
		PeriodStart: &periodStart,
		PeriodEnd:   &periodEnd,
	}

	response := toBillingInvoiceResponse(req, invoice)
	if response.ID != invoice.ID {
		t.Fatalf("expected invoice id=%s, got %s", invoice.ID, response.ID)
	}
	if response.Amount != "USD 123.45" {
		t.Fatalf("unexpected amount display: %s", response.Amount)
	}
	if response.ReceiptURL != "https://billing.local/v1/billing/invoices/inv_helper_1/receipt" {
		t.Fatalf("unexpected generated receipt URL: %s", response.ReceiptURL)
	}
	if response.PeriodStart == nil || response.PeriodEnd == nil {
		t.Fatalf("expected period range in response, got start=%#v end=%#v", response.PeriodStart, response.PeriodEnd)
	}

	invoice.ReceiptURL = "https://receipt.example.com/inv_helper_1"
	response = toBillingInvoiceResponse(req, invoice)
	if response.ReceiptURL != invoice.ReceiptURL {
		t.Fatalf("expected explicit receipt URL override, got %s", response.ReceiptURL)
	}

	if got := invoiceReceiptURL(nil, "inv_nil"); got != "" {
		t.Fatalf("expected empty receipt URL when request is nil, got %q", got)
	}

	reqNoHost := httptest.NewRequest(http.MethodGet, "/v1/billing/history", nil)
	reqNoHost.Host = ""
	if got := invoiceReceiptURL(reqNoHost, "inv_relative"); got != "/v1/billing/invoices/inv_relative/receipt" {
		t.Fatalf("expected relative receipt URL for empty host, got %q", got)
	}

	if got := billingAmountDisplay(500, ""); got != "USD 5.00" {
		t.Fatalf("expected USD fallback amount display, got %q", got)
	}
}

func TestSubscriptionBillingHandlers_ErrorScenarios(t *testing.T) {
	t.Run("service unavailable short-circuit", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		server.billingService = nil

		req := httptest.NewRequest(http.MethodGet, "/v1/subscription", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when billing service is nil, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "subscription_unavailable" {
			t.Fatalf("expected subscription_unavailable code, got %#v", payload["code"])
		}
	})

	t.Run("validation branches for subscription patch and cancel", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)

		cases := []struct {
			name string
			path string
			body string
			want int
			code string
		}{
			{name: "patch invalid json", path: "/v1/subscription", body: `{`, want: http.StatusBadRequest, code: "subscription_invalid_request"},
			{name: "patch missing subscription", path: "/v1/subscription", body: `{"meta":{}}`, want: http.StatusBadRequest, code: "subscription_invalid_request"},
			{name: "patch invalid plan", path: "/v1/subscription", body: `{"subscription":{"plan":"yearly"}}`, want: http.StatusBadRequest, code: "subscription_invalid_request"},
			{name: "cancel invalid json", path: "/v1/subscription/cancel", body: `{`, want: http.StatusBadRequest, code: "subscription_invalid_request"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				method := http.MethodPatch
				if strings.Contains(tc.path, "/cancel") {
					method = http.MethodPost
				}
				req := httptest.NewRequest(method, tc.path, bytes.NewBufferString(tc.body))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				server.Handler().ServeHTTP(rec, req)
				if rec.Code != tc.want {
					t.Fatalf("unexpected status for %s got=%d want=%d body=%s", tc.name, rec.Code, tc.want, rec.Body.String())
				}
				payload := decodeJSONMap(t, rec.Body.Bytes())
				if payload["code"] != tc.code {
					t.Fatalf("unexpected error code for %s got=%#v want=%s", tc.name, payload["code"], tc.code)
				}
			})
		}
	})

	t.Run("billing history fallback pagination and invoice edge paths", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, userID := registerUserAndGetToken(t, server)

		historyReq := httptest.NewRequest(http.MethodGet, "/v1/billing/history?limit=999&offset=-5", nil)
		historyReq.Header.Set("Authorization", "Bearer "+token)
		historyRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(historyRec, historyReq)
		if historyRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for billing history fallback pagination, got %d body=%s", historyRec.Code, historyRec.Body.String())
		}
		historyPayload := decodeJSONMap(t, historyRec.Body.Bytes())
		page := historyPayload["page"].(map[string]any)
		if limit := int(page["limit"].(float64)); limit != 20 {
			t.Fatalf("expected default limit=20 for out-of-range query, got %d", limit)
		}
		if offset := int(page["offset"].(float64)); offset != 0 {
			t.Fatalf("expected offset fallback=0 for negative query, got %d", offset)
		}

		// Missing chi route param branch for invoice handlers.
		invoiceGetReq := httptest.NewRequest(http.MethodGet, "/unused", nil)
		invoiceGetReq.Header.Set("Authorization", "Bearer "+token)
		invoiceGetReq = invoiceGetReq.WithContext(context.WithValue(invoiceGetReq.Context(), chi.RouteCtxKey, chi.NewRouteContext()))
		invoiceGetRec := httptest.NewRecorder()
		server.handleBillingInvoiceGet(invoiceGetRec, invoiceGetReq)
		if invoiceGetRec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing invoice id, got %d body=%s", invoiceGetRec.Code, invoiceGetRec.Body.String())
		}

		invoiceReceiptReq := httptest.NewRequest(http.MethodGet, "/unused", nil)
		invoiceReceiptReq.Header.Set("Authorization", "Bearer "+token)
		invoiceReceiptReq = invoiceReceiptReq.WithContext(context.WithValue(invoiceReceiptReq.Context(), chi.RouteCtxKey, chi.NewRouteContext()))
		invoiceReceiptRec := httptest.NewRecorder()
		server.handleBillingInvoiceReceipt(invoiceReceiptRec, invoiceReceiptReq)
		if invoiceReceiptRec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing invoice id on receipt handler, got %d body=%s", invoiceReceiptRec.Code, invoiceReceiptRec.Body.String())
		}

		missingReq := httptest.NewRequest(http.MethodGet, "/v1/billing/invoices/inv_missing", nil)
		missingReq.Header.Set("Authorization", "Bearer "+token)
		missingRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(missingRec, missingReq)
		if missingRec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for unknown invoice, got %d body=%s", missingRec.Code, missingRec.Body.String())
		}

		now := time.Date(2026, 3, 29, 5, 0, 0, 0, time.UTC)
		invoice := billing.Invoice{
			ID:          "inv_receipt_redirect_test",
			UserID:      userID,
			AmountCents: 1999,
			Currency:    "USD",
			Status:      billing.InvoiceStatusPaid,
			IssuedAt:    now,
			ReceiptURL:  "https://receipt.example.com/inv_receipt_redirect_test",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := server.billingStore.CreateInvoice(context.Background(), invoice); err != nil {
			t.Fatalf("failed to seed invoice with receipt URL: %v", err)
		}

		receiptReq := httptest.NewRequest(http.MethodGet, "/v1/billing/invoices/inv_receipt_redirect_test/receipt", nil)
		receiptReq.Header.Set("Authorization", "Bearer "+token)
		receiptRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(receiptRec, receiptReq)
		if receiptRec.Code != http.StatusFound {
			t.Fatalf("expected 302 redirect when invoice has receipt_url, got %d body=%s", receiptRec.Code, receiptRec.Body.String())
		}
		if got := receiptRec.Header().Get("Location"); got != invoice.ReceiptURL {
			t.Fatalf("unexpected receipt redirect location got=%q want=%q", got, invoice.ReceiptURL)
		}
	})

	t.Run("billing history unavailable when service missing", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)
		server.billingService = nil

		req := httptest.NewRequest(http.MethodGet, "/v1/billing/history", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when billing service is nil, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "billing_unavailable" {
			t.Fatalf("expected billing_unavailable code, got %#v", payload["code"])
		}
	})

	t.Run("text receipt output branch via direct handler invocation", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, userID := registerUserAndGetToken(t, server)

		now := time.Date(2026, 3, 29, 5, 10, 0, 0, time.UTC)
		invoice := billing.Invoice{
			ID:          "inv_receipt_text_test",
			UserID:      userID,
			AmountCents: 300,
			Currency:    "USD",
			Status:      billing.InvoiceStatusPaid,
			IssuedAt:    now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := server.billingStore.CreateInvoice(context.Background(), invoice); err != nil {
			t.Fatalf("failed to seed invoice for text receipt branch: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/unused", nil)
		req.Host = "billing.local"
		req.Header.Set("Authorization", "Bearer "+token)
		chiCtx := chi.NewRouteContext()
		chiCtx.URLParams.Add("id", invoice.ID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

		rec := httptest.NewRecorder()
		server.handleBillingInvoiceReceipt(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for text receipt response, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
			t.Fatalf("expected text/plain content type, got %q", got)
		}
		if !strings.Contains(rec.Body.String(), "Invoice ID: inv_receipt_text_test") {
			t.Fatalf("unexpected text receipt body: %s", rec.Body.String())
		}
	})
}
