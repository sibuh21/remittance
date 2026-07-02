package handler

import (
	"net/http"

	"remittance-service/internal/domain"

	"github.com/labstack/echo/v4"
)

type payoutHandler struct {
	svc domain.PayoutService
}

// NewPayoutHandler creates a new PayoutHandler.
func NewPayoutHandler(svc domain.PayoutService) domain.PayoutHandler {
	return &payoutHandler{svc: svc}
}

// ValidateBeneficiary handles POST /api/payout/validate
// Validates a beneficiary account/wallet before initiating a payout.
func (h *payoutHandler) ValidateBeneficiary(c echo.Context) error {
	var req struct {
		PayoutType     string `json:"payout_type"`
		AccountOrPhone string `json:"account_or_phone"`
		BankID         string `json:"bank_id"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "invalid request body",
			"detail":  err.Error(),
		})
	}

	if req.PayoutType == "" || req.AccountOrPhone == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "payout_type and account_or_phone are required",
		})
	}

	result, err := h.svc.ValidateBeneficiary(domain.PayoutType(req.PayoutType), req.AccountOrPhone, req.BankID)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return c.JSON(appErr.Code, appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "beneficiary validation failed",
			"detail":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, result)
}

// GetExchangeRate handles GET /api/payout/rate/:currency
// Returns the current exchange rate for a given base currency.
func (h *payoutHandler) GetExchangeRate(c echo.Context) error {
	currency := c.Param("currency")
	if currency == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "currency parameter is required",
		})
	}
	rate, err := h.svc.GetExchangeRate(currency)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "failed to fetch exchange rate",
			"detail":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, rate)
}

// GetBanks handles GET /api/payout/banks
// Returns the list of available banks for other-bank transfers.
func (h *payoutHandler) GetBanks(c echo.Context) error {
	banks, err := h.svc.GetBanks()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "failed to fetch bank list",
			"detail":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"banks": banks,
	})
}

// GetBalance handles GET /api/payout/balance
// Returns the current settlement account balance.
func (h *payoutHandler) GetBalance(c echo.Context) error {
	balance, err := h.svc.GetBalance()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "failed to fetch balance",
			"detail":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, balance)
}

// CheckRemittanceStatus handles GET /api/payout/status/:id
// Checks the status of a previously initiated payout transaction.
func (h *payoutHandler) CheckRemittanceStatus(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "remittance id parameter is required",
		})
	}

	status, err := h.svc.CheckRemittanceStatus(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "failed to check remittance status",
			"detail":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, status)
}
