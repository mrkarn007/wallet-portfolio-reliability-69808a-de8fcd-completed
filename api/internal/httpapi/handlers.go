package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"keyrail.local/wallet-portfolio/internal/models"
	"keyrail.local/wallet-portfolio/internal/provider"
	"keyrail.local/wallet-portfolio/internal/service"
)

type portfolioService interface {
	Summary(ctx context.Context, address string) (models.PortfolioResponse, error)
}

type Handler struct {
	portfolio portfolioService
}

func NewHandler(portfolio portfolioService) *Handler {
	return &Handler{portfolio: portfolio}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET requests are supported")
		return
	}

	address, ok := portfolioAddress(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "endpoint_not_found", "Portfolio endpoint not found")
		return
	}

	portfolio, err := h.portfolio.Summary(r.Context(), address)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, portfolio)
}

func portfolioAddress(path string) (string, bool) {
	const prefix = "/api/wallets/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}

	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "portfolio" {
		return "", false
	}
	return parts[0], true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidAddress):
		writeError(w, http.StatusBadRequest, "invalid_address", "Invalid wallet address")
	case errors.Is(err, provider.ErrWalletNotFound):
		writeError(w, http.StatusNotFound, "wallet_not_found", "Wallet not found")
	case errors.Is(err, service.ErrProviderUnavailable), errors.Is(err, provider.ErrProviderTemporary):
		writeError(w, http.StatusServiceUnavailable, "provider_unavailable", "Market data provider is temporarily unavailable")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "request_timeout", "Portfolio request timed out")
	case errors.Is(err, context.Canceled):
		writeError(w, http.StatusRequestTimeout, "request_cancelled", "Portfolio request was cancelled")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Unable to load portfolio")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, models.ErrorBody{
		Error: models.ErrorDetail{Code: code, Message: message},
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
