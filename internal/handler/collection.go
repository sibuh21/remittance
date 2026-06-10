package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"remittance-service/internal/domain"

	"github.com/labstack/echo/v4"
)

type collectionHandler struct {
	collectionSvc domain.CollectionService
}

// NewCollectionHandler creates a new CollectionHandler.
func NewCollectionHandler(collectionSvc domain.CollectionService) domain.CollectionHandler {
	return &collectionHandler{
		collectionSvc: collectionSvc,
	}
}

// === Flex Microform Handlers ===

func (h *collectionHandler) CreateCaptureContext(c echo.Context) error {
	var req domain.CaptureContextRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid request"})
	}

	jwt, err := h.collectionSvc.CreateCaptureContext(req.TargetOrigins)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"captureContext": jwt})
}

func (h *collectionHandler) SetupPayerAuth(c echo.Context) error {
	var req domain.PASetupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid request"})
	}

	if req.RemittanceID == "" || req.TransientTokenJWT == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "remittance_id and transient_token_jwt are required"})
	}

	resp, err := h.collectionSvc.SetupPASetup(&req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *collectionHandler) AuthorizePayment(c echo.Context) error {
	var req domain.AuthorizeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid request"})
	}

	if req.RemittanceID == "" || req.TransientTokenJWT == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "remittance_id and transient_token_jwt are required"})
	}

	resp, err := h.collectionSvc.AuthorizePayment(&req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *collectionHandler) ValidateAndAuthorize(c echo.Context) error {
	var req domain.ValidateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid request"})
	}

	resp, err := h.collectionSvc.ValidateAndAuthorize(&req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *collectionHandler) ReviewPayment(c echo.Context) error {
	var req struct {
		RemittanceID string `json:"remittance_id"`
		Approve      bool   `json:"approve"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid request"})
	}

	if req.RemittanceID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "remittance_id is required"})
	}

	err := h.collectionSvc.ReviewPayment(req.RemittanceID, req.Approve)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	status := "approved"
	if !req.Approve {
		status = "rejected"
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message":       fmt.Sprintf("Remittance %s has been %s", req.RemittanceID, status),
		"remittance_id": req.RemittanceID,
		"status":        status,
	})
}

func (h *collectionHandler) HandleWebhook(c echo.Context) error {
	var n domain.CyberSourceNotification

	// CyberSource can send as JSON or Form (URL Encoded)
	if strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
		if err := c.Bind(&n); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid json"})
		}
	} else {
		// Try binding as Form
		n.MerchantReferenceCode = c.FormValue("merchant_reference_code")
		n.Decision = c.FormValue("decision")
		n.RequestID = c.FormValue("request_id")
		n.ReasonCode = c.FormValue("reason_code")
	}

	err := h.collectionSvc.ProcessWebhook(&n)
	if err != nil {
		log.Printf("ERROR: Webhook processing failed: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"status": "received"})
}
