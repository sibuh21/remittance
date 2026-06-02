package service

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"remittance-service/internal/cybersource"
	"remittance-service/internal/domain"
)

type collectionService struct {
	csClient *cybersource.Client
}

// NewCollectionService creates a new CollectionService backed by the CyberSource client.
func NewCollectionService(csClient *cybersource.Client) domain.CollectionService {
	return &collectionService{csClient: csClient}
}

// GenerateSignedFields creates the signed form fields for the CyberSource hosted checkout POST.
func (s *collectionService) GenerateSignedFields(req *domain.CheckoutRequest) (*domain.SignedFieldsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, domain.NewAppError(http.StatusBadRequest, "validation failed", err.Error())
	}

	locale := req.Locale
	if locale == "" {
		locale = "en"
	}
	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = "card"
	}

	fields := s.csClient.GenerateSignedFields(req.Amount, req.Currency, locale, paymentMethod)

	return &domain.SignedFieldsResponse{
		AccessKey:          fields.AccessKey,
		Amount:             fields.Amount,
		Currency:           fields.Currency,
		Locale:             fields.Locale,
		PaymentMethod:      fields.PaymentMethod,
		ProfileID:          fields.ProfileID,
		ReferenceNumber:    fields.ReferenceNumber,
		SignedDateTime:     fields.SignedDateTime,
		SignedFieldNames:   fields.SignedFieldNames,
		TransactionType:    fields.TransactionType,
		TransactionUUID:    fields.TransactionUUID,
		UnsignedFieldNames: fields.UnsignedFieldNames,
		Signature:          fields.Signature,
		CheckoutURL:        s.csClient.CheckoutURL(),
	}, nil
}

// HandleWebhook processes the response/webhook from CyberSource after payment.
func (s *collectionService) HandleWebhook(data map[string]string) (*domain.PaymentResult, error) {
	// Verify the signature to ensure the data is from CyberSource
	if !s.csClient.VerifySignature(data) {
		log.Printf("WARNING: Invalid signature received from CyberSource webhook")
		return nil, domain.NewAppError(
			http.StatusForbidden,
			"invalid signature",
			"the webhook signature verification failed",
		)
	}

	decision := data["decision"]
	reasonCode := data["reason_code"]
	transactionID := data["transaction_id"]
	referenceNumber := data["req_reference_number"]
	amount := data["req_amount"]
	currency := data["req_currency"]
	authCode := data["auth_code"]
	message := data["message"]

	result := &domain.PaymentResult{
		ID:              transactionID,
		Amount:          amount,
		Currency:        currency,
		AuthCode:        authCode,
		ReferenceNumber: referenceNumber,
		ProcessedAt:     time.Now().UTC(),
	}

	switch decision {
	case "ACCEPT":
		result.Status = domain.StatusAccepted
		result.Message = "Payment accepted successfully"
		log.Printf("INFO: Payment ACCEPTED - TxnID: %s, Ref: %s, Amount: %s %s",
			transactionID, referenceNumber, amount, currency)
	case "DECLINE":
		result.Status = domain.StatusDeclined
		result.Message = fmt.Sprintf("Payment declined: %s (reason: %s)", message, reasonCode)
		log.Printf("WARNING: Payment DECLINED - TxnID: %s, Reason: %s, Message: %s",
			transactionID, reasonCode, message)
	case "REVIEW":
		result.Status = domain.StatusReview
		result.Message = fmt.Sprintf("Payment under review: %s", message)
		log.Printf("INFO: Payment REVIEW - TxnID: %s, Reason: %s", transactionID, reasonCode)
	case "CANCEL":
		result.Status = domain.StatusCancel
		result.Message = "Payment was cancelled"
		log.Printf("INFO: Payment CANCELLED - Ref: %s", referenceNumber)
	case "ERROR":
		result.Status = domain.StatusError
		result.Message = fmt.Sprintf("CyberSource Error: %s (Reason Code: %s)", message, reasonCode)
		log.Printf("ERROR: CyberSource returned ERROR - Reason: %s, Message: %s, TxnID: %s",
			reasonCode, message, transactionID)
	default:
		result.Status = domain.StatusError
		result.Message = fmt.Sprintf("Unexpected decision: %s", decision)
		log.Printf("ERROR: Unknown payment decision '%s' - TxnID: %s, Code: %s, Msg: %s",
			decision, transactionID, reasonCode, message)
	}

	return result, nil
}
