package handler

import (
	"net/http"

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
