package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
)

// authorizeRequest is the body of POST /v1/authorize.
type authorizeRequest struct {
	OrderID     string `json:"order_id"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

// authorizeResponse is the body returned for an approved authorization.
type authorizeResponse struct {
	AuthorizationID string `json:"authorization_id"`
	OrderID         string `json:"order_id"`
	Status          string `json:"status"`
	Amount          int64  `json:"amount"` // cents
	Currency        string `json:"currency"`
}

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

func (r authorizeRequest) validate() error {
	switch {
	case r.OrderID == "":
		return errors.New("order_id is required")
	case r.AmountCents <= 0:
		return errors.New("amount_cents must be positive")
	case !currencyCode.MatchString(r.Currency):
		return errors.New("currency must be an ISO 4217 code")
	}
	return nil
}

type authorizeHandler struct {
	logger *slog.Logger
}

func (h *authorizeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req authorizeRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := authorizeResponse{
		AuthorizationID: newAuthorizationID(),
		OrderID:         req.OrderID,
		Status:          "approved",
		Amount:          req.AmountCents,
		Currency:        req.Currency,
	}
	h.logger.InfoContext(r.Context(), "payment authorized",
		"order_id", resp.OrderID, "authorization_id", resp.AuthorizationID)
	writeJSON(w, http.StatusOK, resp)
}

func newAuthorizationID() string {
	return "auth_" + rand.Text()[:16]
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
