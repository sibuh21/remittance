package service

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"remittance-service/internal/domain"

	"github.com/google/uuid"
)

type remittanceService struct {
	collectionSvc domain.CollectionService
	payoutSvc     domain.PayoutService
	db            interface {
		CreateTransaction(t *domain.Transaction) error
		UpdateCollectionResult(remittanceID, cybersourceRef, collectionStatus, status string) error
		UpdatePayoutResult(remittanceID, boaRef, payoutStatus, status string) error
		GetTransactionByRef(ref string) (*domain.Transaction, error)
		GetTransactionsBySender(email string, status string) ([]*domain.Transaction, error)
		GetTransactionsByReceiver(phone string, status string) ([]*domain.Transaction, error)
	}
}

// NewRemittanceService creates a new end-to-end remittance orchestrator.
func NewRemittanceService(
	collectionSvc domain.CollectionService,
	payoutSvc domain.PayoutService,
	db interface {
		CreateTransaction(t *domain.Transaction) error
		UpdateCollectionResult(remittanceID, cybersourceRef, collectionStatus, status string) error
		UpdatePayoutResult(remittanceID, boaRef, payoutStatus, status string) error
		GetTransactionByRef(ref string) (*domain.Transaction, error)
		GetTransactionsBySender(email string, status string) ([]*domain.Transaction, error)
		GetTransactionsByReceiver(phone string, status string) ([]*domain.Transaction, error)
	},
) domain.RemittanceService {
	return &remittanceService{
		collectionSvc: collectionSvc,
		payoutSvc:     payoutSvc,
		db:            db,
	}
}

// InitiateRemittance starts the remittance flow:
// 1. Validate the request
// 2. Validate the beneficiary via BoA
// 3. Fetch exchange rate
// 4. Generate CyberSource signed fields for collection
// 5. Record the transaction in DB
func (s *remittanceService) InitiateRemittance(req *domain.RemittanceRequest) (*domain.RemittanceResponse, error) {
	// 1. Validate request
	if err := req.Validate(); err != nil {
		return nil, domain.NewAppError(http.StatusBadRequest, "validation failed", err.Error())
	}

	remittanceID := uuid.New().String()
	log.Printf("INFO: Initiating remittance %s - %s %s from %s to %s (%s)",
		remittanceID, req.SendAmount, req.SendCurrency, req.SenderName, req.ReceiverName, req.PayoutType)

	// 2. Validate the beneficiary
	accountOrPhone := req.AccountNumber
	if req.PayoutType == domain.PayoutTelebirr || req.PayoutType == domain.PayoutMpesa {
		accountOrPhone = req.ReceiverPhone
	}

	beneficiary, err := s.payoutSvc.ValidateBeneficiary(req.PayoutType, accountOrPhone, req.BankID)
	if err != nil {
		return nil, err
	}
	if !beneficiary.Valid {
		return nil, domain.NewAppError(
			http.StatusBadRequest,
			"beneficiary validation failed",
			beneficiary.Message,
		)
	}

	log.Printf("INFO: Beneficiary validated - Name: %s", beneficiary.Name)

	// 3. Fetch exchange rate
	targetCurrency := req.TargetCurrency
	if targetCurrency == "" {
		targetCurrency = "ETB"
	}

	var exchangeRate float64
	var receiveAmount string

	rateResp, err := s.payoutSvc.GetExchangeRate(req.SendCurrency)
	if err != nil {
		log.Printf("WARNING: Failed to fetch exchange rate: %v (continuing without rate)", err)
	} else {
		exchangeRate = rateResp.Rate
		if exchangeRate > 0 {
			sendFloat, _ := strconv.ParseFloat(req.SendAmount, 64)
			receiveFloat := sendFloat * exchangeRate
			receiveAmount = fmt.Sprintf("%.2f", receiveFloat)
		}
	}

	// 4. Generate CyberSource signed fields for inbound collection
	checkoutReq := &domain.CheckoutRequest{
		Amount:   req.SendAmount,
		Currency: req.SendCurrency,
		Locale:   "en",
	}

	signedFields, err := s.collectionSvc.GenerateSignedFields(checkoutReq)
	if err != nil {
		return nil, fmt.Errorf("failed to generate checkout fields: %w", err)
	}

	// 5. Record the transaction in DB
	txn := &domain.Transaction{
		ID:             uuid.New().String(),
		RemittanceID:   remittanceID,
		Status:         domain.RemittanceCollectionPending,
		SenderName:     req.SenderName,
		SenderEmail:    req.SenderEmail,
		SourceAmount:   req.SendAmount,
		SourceCurrency: req.SendCurrency,
		ExchangeRate:   exchangeRate,
		TargetAmount:   receiveAmount,
		TargetCurrency: targetCurrency,
		ReceiverName:   req.ReceiverName,
		ReceiverPhone:  req.ReceiverPhone,
		PayoutType:     req.PayoutType,
		AccountNumber:  req.AccountNumber,
		BankID:         req.BankID,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.db.CreateTransaction(txn); err != nil {
		log.Printf("ERROR: Failed to record transaction: %v", err)
		return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to save remittance record")
	}

	return &domain.RemittanceResponse{
		RemittanceID:  remittanceID,
		Status:        domain.RemittanceCollectionPending,
		SendAmount:    req.SendAmount,
		SendCurrency:  req.SendCurrency,
		ExchangeRate:  exchangeRate,
		ReceiveAmount: receiveAmount,
		CheckoutURL:   signedFields.CheckoutURL,
		SignedFields:  signedFields,
		Message:       fmt.Sprintf("Remittance initiated. Beneficiary: %s. Proceed to payment.", beneficiary.Name),
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// ProcessCollectionResult handles the CyberSource result from either the
// frontend redirect (HandleResponse) or the server-to-server callback (HandleWebhook).
// Whichever arrives first updates the DB; the second call is a no-op.
// Returns (result, alreadyProcessed, error).
func (s *remittanceService) ProcessCollectionResult(data map[string]string) (*domain.PaymentResult, bool, error) {
	result, err := s.collectionSvc.HandleWebhook(data)
	if err != nil {
		return nil, false, err
	}

	remittanceID := result.ReferenceNumber
	result.RemittanceID = remittanceID

	// Read the current transaction state
	txn, err := s.db.GetTransactionByRef(remittanceID)
	if err != nil {
		log.Printf("WARNING: Transaction not found for ref %s: %v", remittanceID, err)
		// Still return the result even without DB match
		return result, false, nil
	}

	// Only update if still COLLECTION_PENDING — first-one-wins
	if txn.Status != domain.RemittanceCollectionPending {
		log.Printf("INFO: Remittance %s already processed (status: %s), skipping update", remittanceID, txn.Status)
		return result, true, nil
	}

	if result.Status == domain.StatusAccepted {
		log.Printf("INFO: Collection successful for ref %s - updating to COLLECTED", remittanceID)

		err = s.db.UpdateCollectionResult(remittanceID, result.ID, string(result.Status), string(domain.RemittanceCollected))
		if err != nil {
			log.Printf("ERROR: Failed to update transaction on collection success: %v", err)
		}

		log.Printf("INFO: Collection successful for %s. Waiting for manual payout trigger.", remittanceID)
	} else {
		log.Printf("INFO: Collection not accepted for ref %s (status: %s)", remittanceID, result.Status)
		_ = s.db.UpdateCollectionResult(remittanceID, result.ID, string(result.Status), string(domain.RemittanceFailed))
	}

	return result, false, nil
}

// ExecutePayout executes the outbound payout leg for a completed collection.
func (s *remittanceService) ExecutePayout(remittanceID string) (*domain.PayoutResult, error) {
	log.Printf("INFO: Executing payout for remittance %s", remittanceID)

	// 1. Retrieve transaction from DB
	txn, err := s.db.GetTransactionByRef(remittanceID)
	if err != nil {
		log.Printf("ERROR: Payout failed - transaction not found: %s", remittanceID)
		return nil, domain.NewAppError(http.StatusNotFound, "not found", "remittance transaction not found")
	}

	// 2. Check if transaction is in COLLECTED state
	if txn.Status != domain.RemittanceCollected {
		log.Printf("ERROR: Payout failed - invalid status %s for %s. Only COLLECTED remittances can be paid out.", txn.Status, remittanceID)
		return nil, domain.NewAppError(http.StatusBadRequest, "invalid status", fmt.Sprintf("cannot payout remittance in %s state", txn.Status))
	}

	// 3. Execute the appropriate payout based on type
	var payoutResult *domain.PayoutResult
	var payoutErr error

	// Update status to PROCESSING
	_ = s.db.UpdatePayoutResult(remittanceID, "", "PROCESSING", string(domain.RemittancePayoutProcessing))

	switch txn.PayoutType {
	case domain.PayoutWithinBoA:
		payoutResult, payoutErr = s.payoutSvc.TransferWithinBoA(txn.TargetAmount, txn.AccountNumber, remittanceID)
	case domain.PayoutOtherBank:
		payoutResult, payoutErr = s.payoutSvc.TransferOtherBank(txn.TargetAmount, txn.BankID, txn.AccountNumber, txn.ReceiverName, remittanceID)
	case domain.PayoutTelebirr, domain.PayoutMpesa:
		provider := string(txn.PayoutType)
		payoutResult, payoutErr = s.payoutSvc.TransferWallet(txn.TargetAmount, txn.ReceiverPhone, provider, txn.ReceiverName, txn.SenderName, "", remittanceID)
	default:
		payoutErr = fmt.Errorf("unknown payout type: %s", txn.PayoutType)
	}

	if payoutErr != nil {
		log.Printf("ERROR: Payout processing failed for %s: %v", remittanceID, payoutErr)
		_ = s.db.UpdatePayoutResult(remittanceID, "", "FAILED", string(domain.RemittanceFailed))
		return nil, payoutErr
	}

	// 3. Update status to COMPLETED
	_ = s.db.UpdatePayoutResult(remittanceID, payoutResult.BoAReference, payoutResult.Status, string(domain.RemittanceCompleted))

	return payoutResult, nil
}

// TriggerManualPayout implements domain.RemittanceService.
func (s *remittanceService) TriggerManualPayout(req *domain.ManualPayoutRequest) (*domain.PayoutResult, error) {
	log.Printf("INFO: Manual payout trigger requested - ID: %s, Phone: %s", req.RemittanceID, req.Phone)

	var txn *domain.Transaction
	var err error

	// 1. Find the transaction
	if req.RemittanceID != "" {
		// Lookup by ID
		txn, err = s.db.GetTransactionByRef(req.RemittanceID)
		if err != nil {
			return nil, domain.NewAppError(http.StatusNotFound, "not found", "remittance transaction not found")
		}

		// If phone was also provided, verify it matches the specific transaction
		if req.Phone != "" && txn.ReceiverPhone != req.Phone {
			return nil, domain.NewAppError(http.StatusUnauthorized, "verification failed", "phone number does not match record for this remittance")
		}
	} else if req.Phone != "" {
		// Lookup by Phone - find the latest COLLECTED remittance for this phone
		txns, err := s.db.GetTransactionsByReceiver(req.Phone, string(domain.RemittanceCollected))
		if err != nil {
			return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to lookup transactions by phone")
		}
		if len(txns) == 0 {
			return nil, domain.NewAppError(http.StatusNotFound, "not found", "no collected remittances found for this phone number")
		}
		
		// Take the most recent one (sorted by created_at DESC in DB)
		txn = txns[0]
		log.Printf("INFO: Found collected remittance %s for phone %s", txn.RemittanceID, req.Phone)
	} else {
		return nil, domain.NewAppError(http.StatusBadRequest, "invalid request", "either remittance_id or phone is required")
	}

	// 2. Status Check (already handled in ExecutePayout, but good to check early)
	if txn.Status != domain.RemittanceCollected {
		return nil, domain.NewAppError(http.StatusBadRequest, "invalid status", fmt.Sprintf("remittance is in %s state, not COLLECTED", txn.Status))
	}

	// 3. Delegate to ExecutePayout
	return s.ExecutePayout(txn.RemittanceID)
}

func (s *remittanceService) GetTransactionStatus(id string) (*domain.Transaction, error) {
	return s.db.GetTransactionByRef(id)
}

func (s *remittanceService) GetSenderRemittances(email string, status domain.RemittanceStatus) ([]*domain.Transaction, error) {
	return s.db.GetTransactionsBySender(email, string(status))
}

func (s *remittanceService) GetReceiverRemittances(phone string, status domain.RemittanceStatus) ([]*domain.Transaction, error) {
	return s.db.GetTransactionsByReceiver(phone, string(status))
}
