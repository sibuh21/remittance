package handler

import (
	"log"
	"net/http"

	"remittance-service/internal/domain"

	"github.com/labstack/echo/v4"
)

type collectionHandler struct {
	collectionSvc  domain.CollectionService
	remittanceSvc  domain.RemittanceService
}

// NewCollectionHandler creates a new CollectionHandler.
// It needs both services: CollectionService for generating checkout fields,
// and RemittanceService for processing collection results with DB persistence.
func NewCollectionHandler(collectionSvc domain.CollectionService, remittanceSvc domain.RemittanceService) domain.CollectionHandler {
	return &collectionHandler{
		collectionSvc:  collectionSvc,
		remittanceSvc:  remittanceSvc,
	}
}

// GenerateSignedFields handles POST /api/checkout
// The frontend calls this to get signed form fields, then POSTs them to CyberSource.
func (h *collectionHandler) GenerateSignedFields(c echo.Context) error {
	var req domain.CheckoutRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "invalid request body",
			"detail":  err.Error(),
		})
	}

	resp, err := h.collectionSvc.GenerateSignedFields(&req)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return c.JSON(appErr.Code, appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "internal server error",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// HandleResponse handles POST /api/response
// CyberSource redirects the customer here after payment (frontend redirect).
// This is the user-facing path — it updates the DB (if still COLLECTION_PENDING)
// and then redirects to the appropriate result page.
func (h *collectionHandler) HandleResponse(c echo.Context) error {
	data := parseFormValues(c)

	result, _, err := h.remittanceSvc.ProcessCollectionResult(data)
	if err != nil {
		log.Printf("ERROR: Failed to process CyberSource response: %v", err)
		return c.Redirect(http.StatusFound, "/checkout/error")
	}

	// Redirect the user to the appropriate result page
	switch result.Status {
	case domain.StatusAccepted:
		return c.Redirect(http.StatusFound, "/checkout/success?ref="+result.ReferenceNumber)
	case domain.StatusDeclined:
		return c.Redirect(http.StatusFound, "/checkout/declined?ref="+result.ReferenceNumber)
	case domain.StatusReview:
		return c.Redirect(http.StatusFound, "/checkout/review?ref="+result.ReferenceNumber)
	case domain.StatusCancel:
		return c.Redirect(http.StatusFound, "/checkout/cancelled")
	default:
		return c.Redirect(http.StatusFound, "/checkout/error")
	}
}

// HandleWebhook handles POST /api/webhook
// CyberSource sends a silent POST notification with the payment result (server-to-server).
// This is the callback path — it updates the DB (if still COLLECTION_PENDING)
// and returns only the status as JSON.
func (h *collectionHandler) HandleWebhook(c echo.Context) error {
	data := parseFormValues(c)

	result, alreadyProcessed, err := h.remittanceSvc.ProcessCollectionResult(data)
	if err != nil {
		log.Printf("ERROR: Webhook processing failed: %v", err)
		if appErr, ok := err.(*domain.AppError); ok {
			return c.JSON(appErr.Code, appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "webhook processing failed",
		})
	}

	if alreadyProcessed {
		log.Printf("INFO: Webhook for %s - already processed via redirect", result.ReferenceNumber)
	} else {
		log.Printf("INFO: Webhook processed - Status: %s, Ref: %s", result.Status, result.ReferenceNumber)
	}

	// CyberSource only checks the HTTP status code — no body needed
	return c.NoContent(http.StatusOK)
}

// parseFormValues extracts all form values from the request into a map.
func parseFormValues(c echo.Context) map[string]string {
	data := make(map[string]string)

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("WARNING: Failed to parse form data: %v", err)
		return data
	}

	for key, values := range c.Request().PostForm {
		if len(values) > 0 {
			data[key] = values[0]
		}
	}

	return data
}
