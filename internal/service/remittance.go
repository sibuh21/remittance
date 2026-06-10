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
	targetOrigins []string
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
	targetOrigins []string,
) domain.RemittanceService {
	return &remittanceService{
		collectionSvc: collectionSvc,
		payoutSvc:     payoutSvc,
		db:            db,
		targetOrigins: targetOrigins,
	}
}

// InitiateRemittance starts the remittance flow:
// 1. Validate the request
// 2. Validate the beneficiary via BoA (MOCKED)
// 3. Fetch exchange rate (MOCKED)
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

	// 4. Generate Capture Context for Flex Microform
	captureContext, err := s.collectionSvc.CreateCaptureContext(s.targetOrigins)
	if err != nil {
		log.Printf("ERROR: Failed to generate CyberSource capture context: %v", err)
		return nil, domain.NewAppError(http.StatusInternalServerError, "payment system error", "unable to initialize secure card entry")
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
		RemittanceID:   remittanceID,
		Status:         domain.RemittanceCollectionPending,
		SendAmount:     req.SendAmount,
		SendCurrency:   req.SendCurrency,
		ExchangeRate:   exchangeRate,
		ReceiveAmount:  receiveAmount,
		CaptureContext: captureContext,
		Message:        fmt.Sprintf("Remittance initiated. Beneficiary: %s. Proceed to payment.", beneficiary.Name),
		CreatedAt:      time.Now().UTC(),
	}, nil
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

	// 3. Update status to PROCESSING
	_ = s.db.UpdatePayoutResult(remittanceID, "", "PROCESSING", string(domain.RemittancePayoutProcessing))

	// 4. Dispatch payout (calls mocked service)
	var payoutResult *domain.PayoutResult
	var payoutErr error

	switch txn.PayoutType {
	case domain.PayoutWithinBoA:
		payoutResult, payoutErr = s.payoutSvc.TransferWithinBoA(txn.TargetAmount, txn.AccountNumber, remittanceID)
	case domain.PayoutOtherBank:
		payoutResult, payoutErr = s.payoutSvc.TransferOtherBank(txn.TargetAmount, txn.BankID, txn.AccountNumber, txn.ReceiverName, remittanceID)
	case domain.PayoutTelebirr, domain.PayoutMpesa:
		provider := string(txn.PayoutType)
		payoutResult, payoutErr = s.payoutSvc.TransferWallet(txn.TargetAmount, txn.ReceiverPhone, provider, txn.ReceiverName, txn.SenderName, txn.ReceiverPhone, remittanceID)
	default:
		payoutErr = fmt.Errorf("unknown payout type: %s", txn.PayoutType)
	}

	if payoutErr != nil {
		log.Printf("ERROR: Payout failed for %s: %v", remittanceID, payoutErr)
		_ = s.db.UpdatePayoutResult(remittanceID, "", "FAILED", string(domain.RemittanceFailed))
		return nil, payoutErr
	}

	// 5. Update status to COMPLETED
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
		txn, err = s.db.GetTransactionByRef(req.RemittanceID)
		if err != nil {
			return nil, domain.NewAppError(http.StatusNotFound, "not found", "remittance transaction not found")
		}
		if req.Phone != "" && txn.ReceiverPhone != req.Phone {
			return nil, domain.NewAppError(http.StatusUnauthorized, "verification failed", "phone number does not match record")
		}
	} else if req.Phone != "" {
		txns, err := s.db.GetTransactionsByReceiver(req.Phone, string(domain.RemittanceCollected))
		if err != nil {
			return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to lookup transactions")
		}
		if len(txns) == 0 {
			return nil, domain.NewAppError(http.StatusNotFound, "not found", "no collected remittances found")
		}
		txn = txns[0]
	} else {
		return nil, domain.NewAppError(http.StatusBadRequest, "invalid request", "either remittance_id or phone is required")
	}

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
