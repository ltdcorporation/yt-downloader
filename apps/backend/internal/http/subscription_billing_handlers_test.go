package http

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubscriptionAndBillingHandlers(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, _ := registerUserAndGetToken(t, server)

	subscriptionReq := httptest.NewRequest(http.MethodGet, "/v1/subscription", nil)
	subscriptionReq.Header.Set("Authorization", "Bearer "+token)
	subscriptionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(subscriptionRec, subscriptionReq)
	if subscriptionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for subscription get, got %d body=%s", subscriptionRec.Code, subscriptionRec.Body.String())
	}
	initialPayload := decodeJSONMap(t, subscriptionRec.Body.Bytes())
	initialSub, ok := initialPayload["subscription"].(map[string]any)
	if !ok {
		t.Fatalf("expected subscription object, got %#v", initialPayload["subscription"])
	}
	if initialSub["plan"] != "free" {
		t.Fatalf("expected initial free plan, got %#v", initialSub["plan"])
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/subscription", bytes.NewBufferString(`{"subscription":{"plan":"monthly"}}`))
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for subscription patch, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}
	patchPayload := decodeJSONMap(t, patchRec.Body.Bytes())
	patchSub := patchPayload["subscription"].(map[string]any)
	if patchSub["plan"] != "monthly" {
		t.Fatalf("expected monthly plan after patch, got %#v", patchSub["plan"])
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/v1/billing/history?limit=10", nil)
	historyReq.Header.Set("Authorization", "Bearer "+token)
	historyRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(historyRec, historyReq)
	if historyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for billing history, got %d body=%s", historyRec.Code, historyRec.Body.String())
	}
	historyPayload := decodeJSONMap(t, historyRec.Body.Bytes())
	items, ok := historyPayload["items"].([]any)
	if !ok {
		t.Fatalf("expected billing items array, got %#v", historyPayload["items"])
	}
	if len(items) < 1 {
		t.Fatalf("expected at least one invoice after monthly plan update, got %d", len(items))
	}
	firstInvoice, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected invoice object, got %#v", items[0])
	}
	invoiceID, _ := firstInvoice["id"].(string)
	if invoiceID == "" {
		t.Fatalf("expected invoice id in first invoice payload: %#v", firstInvoice)
	}

	invoiceReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/billing/invoices/%s", invoiceID), nil)
	invoiceReq.Header.Set("Authorization", "Bearer "+token)
	invoiceRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(invoiceRec, invoiceReq)
	if invoiceRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for billing invoice get, got %d body=%s", invoiceRec.Code, invoiceRec.Body.String())
	}

	receiptReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/billing/invoices/%s/receipt", invoiceID), nil)
	receiptReq.Header.Set("Authorization", "Bearer "+token)
	receiptRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(receiptRec, receiptReq)
	if receiptRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for billing invoice receipt, got %d body=%s", receiptRec.Code, receiptRec.Body.String())
	}
	if gotType := receiptRec.Header().Get("Content-Type"); gotType == "" || gotType[:10] != "text/plain" {
		t.Fatalf("expected text/plain receipt content type, got %q", gotType)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/subscription/cancel", nil)
	cancelReq.Header.Set("Authorization", "Bearer "+token)
	cancelRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for schedule cancel, got %d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	cancelPayload := decodeJSONMap(t, cancelRec.Body.Bytes())
	cancelSub := cancelPayload["subscription"].(map[string]any)
	if cancelSub["status"] != "cancel_scheduled" {
		t.Fatalf("expected cancel_scheduled status, got %#v", cancelSub["status"])
	}

	immediateCancelReq := httptest.NewRequest(http.MethodPost, "/v1/subscription/cancel", bytes.NewBufferString(`{"immediate":true}`))
	immediateCancelReq.Header.Set("Authorization", "Bearer "+token)
	immediateCancelReq.Header.Set("Content-Type", "application/json")
	immediateCancelRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(immediateCancelRec, immediateCancelReq)
	if immediateCancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for immediate cancel, got %d body=%s", immediateCancelRec.Code, immediateCancelRec.Body.String())
	}
	immediatePayload := decodeJSONMap(t, immediateCancelRec.Body.Bytes())
	immediateSub := immediatePayload["subscription"].(map[string]any)
	if immediateSub["plan"] != "free" {
		t.Fatalf("expected free plan after immediate cancel, got %#v", immediateSub["plan"])
	}
	if immediateSub["status"] != "inactive" {
		t.Fatalf("expected inactive status after immediate cancel, got %#v", immediateSub["status"])
	}

	reactivateReq := httptest.NewRequest(http.MethodPost, "/v1/subscription/reactivate", nil)
	reactivateReq.Header.Set("Authorization", "Bearer "+token)
	reactivateRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(reactivateRec, reactivateReq)
	if reactivateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for subscription reactivate, got %d body=%s", reactivateRec.Code, reactivateRec.Body.String())
	}
}
