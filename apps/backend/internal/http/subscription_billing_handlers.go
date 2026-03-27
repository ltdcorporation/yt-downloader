package http

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/billing"
)

type subscriptionPatchRequest struct {
	Subscription *subscriptionPatchPayload `json:"subscription"`
}

type subscriptionPatchPayload struct {
	Plan string `json:"plan"`
}

type subscriptionCancelRequest struct {
	Immediate bool `json:"immediate"`
}

type billingHistoryResponse struct {
	Items []billingInvoiceResponse `json:"items"`
	Page  billingHistoryPage       `json:"page"`
}

type billingHistoryPage struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type billingInvoiceResponse struct {
	ID          string  `json:"id"`
	IssuedAt    string  `json:"issued_at"`
	AmountCents int64   `json:"amount_cents"`
	Amount      string  `json:"amount"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	ReceiptURL  string  `json:"receipt_url,omitempty"`
	PeriodStart *string `json:"period_start,omitempty"`
	PeriodEnd   *string `json:"period_end,omitempty"`
}

func (s *Server) handleSubscriptionGet(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil || s.billingService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "subscription service unavailable",
			"code":  "subscription_unavailable",
		})
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	user, err := s.authService.GetUser(r.Context(), identity.User.ID)
	if err != nil {
		s.logger.Printf("subscription get failed user_id=%s err=%v", identity.User.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}

	dashboard, err := s.billingService.GetDashboard(r.Context(), user.ID, string(user.Plan), user.PlanExpiresAt)
	if err != nil {
		s.logger.Printf("subscription dashboard failed user_id=%s err=%v", identity.User.ID, err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "subscription service unavailable",
			"code":  "subscription_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) handleSubscriptionPatch(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil || s.billingService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "subscription service unavailable",
			"code":  "subscription_unavailable",
		})
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	var req subscriptionPatchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid JSON body",
			"code":  "subscription_invalid_request",
		})
		return
	}
	if req.Subscription == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "subscription is required",
			"code":  "subscription_invalid_request",
		})
		return
	}

	targetPlan, err := parseUserPlan(req.Subscription.Plan)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
			"code":  "subscription_invalid_request",
		})
		return
	}

	currentUser, err := s.authService.GetUser(r.Context(), identity.User.ID)
	if err != nil {
		s.logger.Printf("subscription patch get current user failed user_id=%s err=%v", identity.User.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}

	updatedUser, err := s.authService.UpdateUserByAdmin(r.Context(), identity.User.ID, identity.User.ID, auth.AdminUpdateUserInput{
		Plan: &targetPlan,
	})
	if err != nil {
		var validationErr *auth.ValidationError
		if errors.As(err, &validationErr) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": validationErr.Error(),
				"code":  "subscription_invalid_request",
			})
			return
		}

		s.logger.Printf("subscription patch failed user_id=%s err=%v", identity.User.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}

	if updatedUser.Plan == auth.PlanFree {
		if _, err := s.billingService.ClearCancelSchedule(r.Context(), updatedUser.ID); err != nil {
			s.logger.Printf("subscription clear cancel schedule failed user_id=%s err=%v", updatedUser.ID, err)
		}
	} else {
		if _, err := s.billingService.ClearCancelSchedule(r.Context(), updatedUser.ID); err != nil {
			s.logger.Printf("subscription clear cancel schedule failed user_id=%s err=%v", updatedUser.ID, err)
		}
		if currentUser.Plan != updatedUser.Plan {
			if _, err := s.billingService.CreatePlanInvoice(r.Context(), updatedUser.ID, string(updatedUser.Plan), updatedUser.PlanExpiresAt); err != nil {
				s.logger.Printf("subscription create invoice best-effort failed user_id=%s plan=%s err=%v", updatedUser.ID, updatedUser.Plan, err)
			}
		}
	}

	dashboard, err := s.billingService.GetDashboard(r.Context(), updatedUser.ID, string(updatedUser.Plan), updatedUser.PlanExpiresAt)
	if err != nil {
		s.logger.Printf("subscription dashboard reload failed user_id=%s err=%v", updatedUser.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}

	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) handleSubscriptionCancel(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil || s.billingService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "subscription service unavailable",
			"code":  "subscription_unavailable",
		})
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	req := subscriptionCancelRequest{}
	if r.ContentLength > 0 {
		if err := decodeJSONBody(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid JSON body",
				"code":  "subscription_invalid_request",
			})
			return
		}
	}

	user, err := s.authService.GetUser(r.Context(), identity.User.ID)
	if err != nil {
		s.logger.Printf("subscription cancel get user failed user_id=%s err=%v", identity.User.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to cancel subscription")
		return
	}

	if req.Immediate {
		freePlan := auth.PlanFree
		user, err = s.authService.UpdateUserByAdmin(r.Context(), identity.User.ID, identity.User.ID, auth.AdminUpdateUserInput{
			Plan: &freePlan,
		})
		if err != nil {
			s.logger.Printf("subscription immediate cancel failed user_id=%s err=%v", identity.User.ID, err)
			writeError(w, http.StatusInternalServerError, "failed to cancel subscription")
			return
		}
		if _, err := s.billingService.ClearCancelSchedule(r.Context(), identity.User.ID); err != nil {
			s.logger.Printf("subscription clear cancel schedule failed user_id=%s err=%v", identity.User.ID, err)
		}
	} else {
		if user.Plan != auth.PlanFree {
			if _, err := s.billingService.ScheduleCancelAtPeriodEnd(r.Context(), identity.User.ID); err != nil {
				s.logger.Printf("subscription schedule cancel failed user_id=%s err=%v", identity.User.ID, err)
				writeError(w, http.StatusInternalServerError, "failed to cancel subscription")
				return
			}
		}
	}

	dashboard, err := s.billingService.GetDashboard(r.Context(), user.ID, string(user.Plan), user.PlanExpiresAt)
	if err != nil {
		s.logger.Printf("subscription cancel dashboard reload failed user_id=%s err=%v", identity.User.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}

	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) handleSubscriptionReactivate(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil || s.billingService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "subscription service unavailable",
			"code":  "subscription_unavailable",
		})
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	if _, err := s.billingService.ClearCancelSchedule(r.Context(), identity.User.ID); err != nil {
		s.logger.Printf("subscription reactivate failed user_id=%s err=%v", identity.User.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to reactivate subscription")
		return
	}

	user, err := s.authService.GetUser(r.Context(), identity.User.ID)
	if err != nil {
		s.logger.Printf("subscription reactivate get user failed user_id=%s err=%v", identity.User.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}

	dashboard, err := s.billingService.GetDashboard(r.Context(), user.ID, string(user.Plan), user.PlanExpiresAt)
	if err != nil {
		s.logger.Printf("subscription reactivate dashboard failed user_id=%s err=%v", identity.User.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}

	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) handleBillingHistoryList(w http.ResponseWriter, r *http.Request) {
	if s.billingService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "billing service unavailable",
			"code":  "billing_unavailable",
		})
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	items, total, err := s.billingService.ListInvoices(r.Context(), identity.User.ID, limit, offset)
	if err != nil {
		s.logger.Printf("billing history list failed user_id=%s err=%v", identity.User.ID, err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "billing service unavailable",
			"code":  "billing_unavailable",
		})
		return
	}

	responseItems := make([]billingInvoiceResponse, 0, len(items))
	for _, invoice := range items {
		responseItems = append(responseItems, toBillingInvoiceResponse(r, invoice))
	}

	writeJSON(w, http.StatusOK, billingHistoryResponse{
		Items: responseItems,
		Page:  billingHistoryPage{Total: total, Limit: limit, Offset: offset},
	})
}

func (s *Server) handleBillingInvoiceGet(w http.ResponseWriter, r *http.Request) {
	if s.billingService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "billing service unavailable",
			"code":  "billing_unavailable",
		})
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	invoiceID := strings.TrimSpace(chi.URLParam(r, "id"))
	if invoiceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invoice id is required",
			"code":  "billing_invalid_request",
		})
		return
	}

	invoice, err := s.billingService.GetInvoice(r.Context(), identity.User.ID, invoiceID)
	if err != nil {
		if errors.Is(err, billing.ErrInvoiceNotFound) {
			writeError(w, http.StatusNotFound, "invoice not found")
			return
		}
		s.logger.Printf("billing invoice get failed user_id=%s invoice_id=%s err=%v", identity.User.ID, invoiceID, err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "billing service unavailable",
			"code":  "billing_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, toBillingInvoiceResponse(r, invoice))
}

func (s *Server) handleBillingInvoiceReceipt(w http.ResponseWriter, r *http.Request) {
	if s.billingService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "billing service unavailable",
			"code":  "billing_unavailable",
		})
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	invoiceID := strings.TrimSpace(chi.URLParam(r, "id"))
	if invoiceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invoice id is required",
			"code":  "billing_invalid_request",
		})
		return
	}

	invoice, err := s.billingService.GetInvoice(r.Context(), identity.User.ID, invoiceID)
	if err != nil {
		if errors.Is(err, billing.ErrInvoiceNotFound) {
			writeError(w, http.StatusNotFound, "invoice not found")
			return
		}
		s.logger.Printf("billing invoice receipt failed user_id=%s invoice_id=%s err=%v", identity.User.ID, invoiceID, err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "billing service unavailable",
			"code":  "billing_unavailable",
		})
		return
	}

	if invoice.ReceiptURL != "" {
		http.Redirect(w, r, invoice.ReceiptURL, http.StatusFound)
		return
	}

	filename := fmt.Sprintf("invoice-%s.txt", invoice.ID)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)

	_, _ = fmt.Fprintf(
		w,
		"Invoice ID: %s\nIssued At: %s\nAmount: %s\nStatus: %s\n",
		invoice.ID,
		invoice.IssuedAt.UTC().Format(time.RFC3339),
		billingAmountDisplay(invoice.AmountCents, invoice.Currency),
		strings.ToUpper(string(invoice.Status)),
	)
}

func parseUserPlan(rawPlan string) (auth.Plan, error) {
	switch auth.Plan(strings.ToLower(strings.TrimSpace(rawPlan))) {
	case auth.PlanFree:
		return auth.PlanFree, nil
	case auth.PlanDaily:
		return auth.PlanDaily, nil
	case auth.PlanWeekly:
		return auth.PlanWeekly, nil
	case auth.PlanMonthly:
		return auth.PlanMonthly, nil
	default:
		return "", errors.New("plan must be one of: free, daily, weekly, monthly")
	}
}

func toBillingInvoiceResponse(r *http.Request, invoice billing.Invoice) billingInvoiceResponse {
	response := billingInvoiceResponse{
		ID:          invoice.ID,
		IssuedAt:    invoice.IssuedAt.UTC().Format(time.RFC3339),
		AmountCents: invoice.AmountCents,
		Amount:      billingAmountDisplay(invoice.AmountCents, invoice.Currency),
		Currency:    strings.ToUpper(strings.TrimSpace(invoice.Currency)),
		Status:      string(invoice.Status),
		ReceiptURL:  invoiceReceiptURL(r, invoice.ID),
	}
	if invoice.PeriodStart != nil {
		value := invoice.PeriodStart.UTC().Format(time.RFC3339)
		response.PeriodStart = &value
	}
	if invoice.PeriodEnd != nil {
		value := invoice.PeriodEnd.UTC().Format(time.RFC3339)
		response.PeriodEnd = &value
	}
	if invoice.ReceiptURL != "" {
		response.ReceiptURL = invoice.ReceiptURL
	}
	return response
}

func invoiceReceiptURL(r *http.Request, invoiceID string) string {
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return fmt.Sprintf("/v1/billing/invoices/%s/receipt", invoiceID)
	}
	return fmt.Sprintf("%s://%s/v1/billing/invoices/%s/receipt", scheme, host, invoiceID)
}

func billingAmountDisplay(amountCents int64, currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "USD"
	}
	return fmt.Sprintf("%s %.2f", currency, float64(amountCents)/100.0)
}
