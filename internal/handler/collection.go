package handler

import (
	"encoding/json"
	"fmt"
	"io"
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
	byt, _ := io.ReadAll(c.Request().Body)
	// Rewind the body so c.Bind can read it
	c.Request().Body = io.NopCloser(strings.NewReader(string(byt)))

	var req domain.PASetupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid request"})
	}

	if req.ID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "id is required in pa-setup request"})
	}
	if req.TransientTokenJWT == "" && req.PermanentTokenID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "either transient_token_jwt or permanent_token_id is required"})
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

	if req.ID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "id is required"})
	}
	if req.TransientTokenJWT == "" && req.PermanentTokenID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "either transient_token_jwt or permanent_token_id is required"})
	}

	// Capture IP for CyberSource fraud scoring
	req.IPAddress = c.RealIP()

	resp, err := h.collectionSvc.AuthorizePayment(&req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}
	marshal, _ := json.Marshal(resp)
	fmt.Println("marshal", string(marshal))
	return c.JSON(http.StatusOK, resp)
}

func (h *collectionHandler) CheckIfAuthorized(c echo.Context) error {
	var req domain.ValidateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid request"})
	}

	resp, err := h.collectionSvc.CheckIfAuthorized(&req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *collectionHandler) ReviewPayment(c echo.Context) error {
	var req struct {
		ID      string `json:"id"`
		Approve bool   `json:"approve"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid request"})
	}

	if req.ID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "id is required"})
	}

	err := h.collectionSvc.ReviewPayment(req.ID, req.Approve)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	status := "approved"
	if !req.Approve {
		status = "rejected"
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message": fmt.Sprintf("Remittance %s has been %s", req.ID, status),
		"id":      req.ID,
		"status":  status,
	})
}

func (h *collectionHandler) HandleWebhook(c echo.Context) error {
	eventType := c.Request().Header.Get("v-c-event-type")

	// Handle Token Status Update (TSU) Webhooks from Token Management Service
	if eventType == "tms.networktoken.updated" || strings.Contains(eventType, "tms.") {
		var tsu domain.TSUWebhookPayload
		if err := c.Bind(&tsu); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid json for tsu webhook"})
		}

		err := h.collectionSvc.ProcessTSUWebhook(&tsu)
		if err != nil {
			log.Printf("ERROR: TSU Webhook processing failed: %v", err)
			return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, echo.Map{"status": "received"})
	}

	// Handle Case Management Decision Webhooks
	if strings.HasPrefix(eventType, "risk.casemanagement.decision.") {
		var cm domain.CaseManagementWebhookPayload
		if err := c.Bind(&cm); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"message": "invalid json for casemanagement webhook"})
		}

		// Ensure EventType in payload is set correctly from header if missing in payload body
		if cm.EventType == "" {
			cm.EventType = eventType
		}

		err := h.collectionSvc.ProcessCaseManagementWebhook(&cm)
		if err != nil {
			log.Printf("ERROR: Case Management Webhook processing failed: %v", err)
			return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, echo.Map{"status": "received"})
	}

	// Legacy / Standard DM Webhooks
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

func (h *collectionHandler) GetSenderCards(c echo.Context) error {
	email := c.QueryParam("email")
	if email == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "email query parameter is required"})
	}

	cards, err := h.collectionSvc.GetSenderCards(email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	return c.JSON(http.StatusOK, cards)
}

func (h *collectionHandler) Handle3DSReturn(c echo.Context) error {
	transactionID := c.FormValue("transactionId")
	if transactionID == "" {
		transactionID = c.FormValue("TransactionId")
	}

	if transactionID == "" {
		log.Printf("ERROR: 3DS return missing transactionId")
		return c.Redirect(http.StatusSeeOther, "/checkout/error?message=missing_transaction_id")
	}

	// 1. Fetch remittance from DB to get the original Remittance ID
	rem, err := h.collectionSvc.GetRemittanceByCSAuthenticationID(transactionID)
	if err != nil {
		log.Printf("ERROR: Failed to find remittance %s on 3DS return: %v", transactionID, err)
		return c.Redirect(http.StatusSeeOther, "/checkout/error?message=remittance_not_found")
	}

	// 2. Send validation request to CyberSource
	req := &domain.ValidateRequest{
		ID:                          rem.ID,
		AuthenticationTransactionId: transactionID,
	}
	fmt.Println("req:===>", req)

	resp, err := h.collectionSvc.CheckIfAuthorized(req)
	if err != nil {
		log.Printf("ERROR: 3DS validation failed for %s: %v", rem.ID, err)
		return c.Redirect(http.StatusSeeOther, "/checkout/error?message="+err.Error())
	}

	// 3. Redirect based on response status
	switch resp.Status {
	case domain.CSStatusAuthorized:
		return c.Redirect(http.StatusSeeOther, "/checkout/success?ref="+rem.ID)
	case domain.CSStatusAuthorizedPendingReview:
		return c.Redirect(http.StatusSeeOther, "/checkout/review?ref="+rem.ID)
	case domain.CSStatusDeclined, domain.CSStatusRejected:
		return c.Redirect(http.StatusSeeOther, "/checkout/declined?message="+resp.Message)
	default:
		return c.Redirect(http.StatusSeeOther, "/checkout/error?message=payment_failed")
	}
}

func (h *collectionHandler) GetConfig(c echo.Context) error {
	cfg, err := h.collectionSvc.GetConfig()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}
	return c.JSON(http.StatusOK, cfg)
}
