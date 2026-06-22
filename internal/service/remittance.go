package service

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"remittance-service/internal/domain"

	"github.com/google/uuid"
)

type remittanceService struct {
	collectionSvc domain.CollectionService
	payoutSvc     domain.PayoutService
	db            interface {
		CreateRemittance(t *domain.Remittance) error
		UpdateCollectionResult(id, csTransactionID, csAuthTransactionID, collectionStatus, status, paymentTokenID, transientTokenJWT string) error
		UpdatePayoutResult(id, boaRef, payoutStatus, status string) error
		GetRemittanceByID(id string) (*domain.Remittance, error)
		GetRemittancesBySender(email string, status string) ([]*domain.Remittance, error)
		GetRemittancesByReceiver(phone string, status string) ([]*domain.Remittance, error)
	}
	targetOrigins []string
}

// NewRemittanceService creates a new end-to-end remittance orchestrator.
func NewRemittanceService(
	collectionSvc domain.CollectionService,
	payoutSvc domain.PayoutService,
	db interface {
		CreateRemittance(t *domain.Remittance) error
		UpdateCollectionResult(id, csTransactionID, csAuthTransactionID, collectionStatus, status, paymentTokenID, transientTokenJWT string) error
		UpdatePayoutResult(id, boaRef, payoutStatus, status string) error
		GetRemittanceByID(id string) (*domain.Remittance, error)
		GetRemittancesBySender(email string, status string) ([]*domain.Remittance, error)
		GetRemittancesByReceiver(phone string, status string) ([]*domain.Remittance, error)
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
// 5. Record the remittance in DB
func (s *remittanceService) InitiateRemittance(req *domain.RemittanceRequest) (*domain.RemittanceResponse, error) {
	// 1. Validate request
	if err := req.Validate(); err != nil {
		return nil, domain.NewAppError(http.StatusBadRequest, "validation failed", err.Error())
	}

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

	ID := uuid.New().String()

	// Split name for CyberSource compatibility
	nameParts := strings.SplitN(req.SenderName, " ", 2)
	firstName := nameParts[0]
	lastName := ""
	if len(nameParts) > 1 {
		lastName = nameParts[1]
	}

	// 5. Record the remittance in DB
	rem := &domain.Remittance{
		ID:               ID,
		Status:           domain.RemittanceCollectionPending,
		SenderName:       req.SenderName,
		SenderFirstName:  firstName,
		SenderLastName:   lastName,
		SenderEmail:      req.SenderEmail,
		SenderAddress:    req.SenderAddress,
		SenderCity:       req.SenderCity,
		SenderState:      req.SenderState,
		SenderPostalCode: req.SenderPostal,
		SenderCountry:    req.SenderCountry,
		SourceAmount:     req.SendAmount,
		SourceCurrency:   req.SendCurrency,
		ExchangeRate:     exchangeRate,
		TargetAmount:     receiveAmount,
		TargetCurrency:   targetCurrency,
		ReceiverName:     req.ReceiverName,
		ReceiverPhone:    req.ReceiverPhone,
		PayoutType:       req.PayoutType,
		AccountNumber:    req.AccountNumber,
		BankID:           req.BankID,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := s.db.CreateRemittance(rem); err != nil {
		log.Printf("ERROR: Failed to record remittance: %v", err)
		return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to save remittance record")
	}

	return &domain.RemittanceResponse{
		ID:             ID,
		Status:         domain.RemittanceCollectionPending,
		SendAmount:     req.SendAmount,
		SendCurrency:   req.SendCurrency,
		ExchangeRate:   exchangeRate,
		ReceiveAmount:  receiveAmount,
		CaptureContext: captureContext,
		Message:        fmt.Sprintf("Remittance initiated (vID). Beneficiary: %s. Proceed to payment.", beneficiary.Name),
		CreatedAt:      time.Now().UTC(),
	}, nil
}

// ExecutePayout executes the outbound payout leg for a completed collection.
func (s *remittanceService) ExecutePayout(id string) (*domain.PayoutResult, error) {
	log.Printf("INFO: Executing payout for remittance %s", id)

	// 1. Retrieve remittance from DB
	rem, err := s.db.GetRemittanceByID(id)
	if err != nil {
		log.Printf("ERROR: Payout failed - remittance not found: %s", id)
		return nil, domain.NewAppError(http.StatusNotFound, "not found", "remittance not found")
	}

	// 2. Check if remittance is in COLLECTED state
	if rem.Status != domain.RemittanceCollected {
		log.Printf("ERROR: Payout failed - invalid status %s for %s. Only COLLECTED remittances can be paid out.", rem.Status, id)
		return nil, domain.NewAppError(http.StatusBadRequest, "invalid status", fmt.Sprintf("cannot payout remittance in %s state", rem.Status))
	}

	// 3. Update status to PROCESSING
	_ = s.db.UpdatePayoutResult(id, "", "PROCESSING", string(domain.RemittancePayoutProcessing))

	// 4. Dispatch payout (calls mocked service)
	var payoutResult *domain.PayoutResult
	var payoutErr error

	switch rem.PayoutType {
	case domain.PayoutWithinBoA:
		payoutResult, payoutErr = s.payoutSvc.TransferWithinBoA(rem.TargetAmount, rem.AccountNumber, id)
	case domain.PayoutOtherBank:
		payoutResult, payoutErr = s.payoutSvc.TransferOtherBank(rem.TargetAmount, rem.BankID, rem.AccountNumber, rem.ReceiverName, id)
	case domain.PayoutTelebirr, domain.PayoutMpesa:
		provider := string(rem.PayoutType)
		payoutResult, payoutErr = s.payoutSvc.TransferWallet(rem.TargetAmount, rem.ReceiverPhone, provider, rem.ReceiverName, rem.SenderName, rem.ReceiverPhone, id)
	default:
		payoutErr = fmt.Errorf("unknown payout type: %s", rem.PayoutType)
	}

	if payoutErr != nil {
		log.Printf("ERROR: Payout failed for %s: %v", id, payoutErr)
		_ = s.db.UpdatePayoutResult(id, "", "FAILED", string(domain.RemittanceFailed))
		return nil, payoutErr
	}

	// 5. Update status to COMPLETED
	_ = s.db.UpdatePayoutResult(id, payoutResult.BoAReference, payoutResult.Status, string(domain.RemittanceCompleted))

	return payoutResult, nil
}

// TriggerManualPayout implements domain.RemittanceService.
func (s *remittanceService) TriggerManualPayout(req *domain.ManualPayoutRequest) (*domain.PayoutResult, error) {
	log.Printf("INFO: Manual payout trigger requested - ID: %s, Phone: %s", req.ID, req.Phone)

	var rem *domain.Remittance
	var err error

	// 1. Find the remittance
	if req.ID != "" {
		rem, err = s.db.GetRemittanceByID(req.ID)
		if err != nil {
			return nil, domain.NewAppError(http.StatusNotFound, "not found", "remittance not found")
		}
		if req.Phone != "" && rem.ReceiverPhone != req.Phone {
			return nil, domain.NewAppError(http.StatusUnauthorized, "verification failed", "phone number does not match record")
		}
	} else if req.Phone != "" {
		rems, err := s.db.GetRemittancesByReceiver(req.Phone, string(domain.RemittanceCollected))
		if err != nil {
			return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to lookup remittances")
		}
		if len(rems) == 0 {
			return nil, domain.NewAppError(http.StatusNotFound, "not found", "no collected remittances found")
		}
		rem = rems[0]
	} else {
		return nil, domain.NewAppError(http.StatusBadRequest, "invalid request", "either remittance_id or phone is required")
	}

	return s.ExecutePayout(rem.ID)
}

func (s *remittanceService) GetRemittanceStatus(id string) (*domain.Remittance, error) {
	return s.db.GetRemittanceByID(id)
}

func (s *remittanceService) GetSenderRemittances(email string, status domain.RemittanceStatus) ([]*domain.Remittance, error) {
	return s.db.GetRemittancesBySender(email, string(status))
}

func (s *remittanceService) GetReceiverRemittances(phone string, status domain.RemittanceStatus) ([]*domain.Remittance, error) {
	return s.db.GetRemittancesByReceiver(phone, string(status))
}
