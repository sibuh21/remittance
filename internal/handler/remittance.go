package handler

import (
	"log"
	"net/http"

	"remittance-service/internal/domain"

	"github.com/labstack/echo/v4"
)

type remittanceHandler struct {
	svc domain.RemittanceService
}

// NewRemittanceHandler creates a new RemittanceHandler.
func NewRemittanceHandler(svc domain.RemittanceService) domain.RemittanceHandler {
	return &remittanceHandler{svc: svc}
}

// InitiateRemittance handles POST /api/remittance
// Validates beneficiary, fetches exchange rate, and returns CyberSource checkout fields.
func (h *remittanceHandler) InitiateRemittance(c echo.Context) error {
	var req domain.RemittanceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "invalid request body",
			"detail":  err.Error(),
		})
	}

	resp, err := h.svc.InitiateRemittance(&req)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return c.JSON(appErr.Code, appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "failed to initiate remittance",
			"detail":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// TriggerPayout handles POST /api/remittance/payout
func (h *remittanceHandler) TriggerPayout(c echo.Context) error {
	var req domain.ManualPayoutRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "invalid request body",
			"detail":  err.Error(),
		})
	}

	result, err := h.svc.TriggerManualPayout(&req)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return c.JSON(appErr.Code, appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "failed to trigger payout",
			"detail":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *remittanceHandler) GetStatus(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "id is required"})
	}

	rem, err := h.svc.GetRemittanceStatus(id)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return c.JSON(http.StatusNotFound, map[string]string{"message": "remittance not found"})
		}
		// Log the actual error for the developer
		log.Printf("ERROR: Database error in GetStatus: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "failed to retrieve remittance status",
			"detail":  err.Error(),
		})
	}

	return h.formatRemittanceResponse(c, rem)
}

// ListSenderRemittances handles GET /api/remittance/sender/:email
func (h *remittanceHandler) ListSenderRemittances(c echo.Context) error {
	email := c.Param("email")
	status := domain.RemittanceStatus(c.QueryParam("status"))
	
	rems, err := h.svc.GetSenderRemittances(email, status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "failed to list remittances", "detail": err.Error()})
	}

	return c.JSON(http.StatusOK, rems)
}

// ListReceiverRemittances handles GET /api/remittance/receiver/:phone
func (h *remittanceHandler) ListReceiverRemittances(c echo.Context) error {
	phone := c.Param("phone")
	status := domain.RemittanceStatus(c.QueryParam("status"))
	
	rems, err := h.svc.GetReceiverRemittances(phone, status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "failed to list remittances", "detail": err.Error()})
	}

	return c.JSON(http.StatusOK, rems)
}

func (h *remittanceHandler) formatRemittanceResponse(c echo.Context, t *domain.Remittance) error {
	return c.JSON(http.StatusOK, t)
}
