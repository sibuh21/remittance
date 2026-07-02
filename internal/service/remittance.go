package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"remittance-service/internal/database"
	"remittance-service/internal/domain"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type remittanceService struct {
	collectionSvc domain.CollectionService
	payoutSvc     domain.PayoutService
	db            database.Queries
	targetOrigins []string
}

// NewRemittanceService creates a new end-to-end remittance orchestrator.
func NewRemittanceService(
	collectionSvc domain.CollectionService,
	payoutSvc domain.PayoutService,
	db database.Queries,
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

	var exchangeRate decimal.Decimal
	var receiveAmount decimal.Decimal

	rateResp, err := s.payoutSvc.GetExchangeRate(req.SendCurrency)
	if err != nil {
		log.Printf("WARNING: Failed to fetch exchange rate: %v (continuing without rate)", err)
	} else {
		exchangeRate = decimal.NewFromFloat(rateResp.Rate)
		if exchangeRate.GreaterThan(decimal.Zero) {
			receiveAmount = req.SendAmount.Mul(exchangeRate)
		}
	}

	// 4. Generate Capture Context for Flex Microform
	captureContext, err := s.collectionSvc.CreateCaptureContext(s.targetOrigins)
	if err != nil {
		log.Printf("ERROR: Failed to generate CyberSource capture context: %v", err)
		return nil, domain.NewAppError(http.StatusInternalServerError, "payment system error", "unable to initialize secure card entry")
	}

	// ID := uuid.New().String()

	// Split name for CyberSource compatibility
	nameParts := strings.SplitN(req.SenderName, " ", 2)
	firstName := nameParts[0]
	lastName := ""
	if len(nameParts) > 1 {
		lastName = nameParts[1]
	}
	// split receiver name for CyberSource compatibility
	receiverNameParts := strings.SplitN(req.ReceiverName, " ", 2)
	receiverFirstName := receiverNameParts[0]
	receiverLastName := ""
	if len(receiverNameParts) > 1 {
		receiverLastName = receiverNameParts[1]
	}
	if len(receiverNameParts) < 2 || len(nameParts) < 2 {
		return nil, domain.NewAppError(http.StatusBadRequest, "validation failed", "invalid sender or receiver name")
	}
	// check if the sender already exists in the database
	user, err := s.db.GetUserByEmail(context.Background(), req.SenderEmail)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to fetch user record")
		}
		// User does not exist, create a new one
		user, err = s.db.CreateUser(context.Background(), database.CreateUserParams{
			FirstName: firstName,
			LastName:  lastName,
			Email:     req.SenderEmail,
			Phone:     req.SenderPhone,
		})
		if err != nil {
			return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to save user record")
		}
	}

	remittance, err := s.db.CreateRemittance(context.Background(), database.CreateRemittanceParams{
		ID:                 uuid.New(),
		Status:             string(domain.RemittanceCollectionPending),
		SenderUserID:       user.ID.String(),
		SenderAddress:      req.SenderAddress,
		SenderCity:         req.SenderCity,
		SenderState:        req.SenderState,
		SenderPostalCode:   req.SenderPostal,
		SenderCountry:      req.SenderCountry,
		SourceAmount:       req.SendAmount,
		SourceCurrency:     req.SendCurrency,
		ExchangeRate:       decimal.NullDecimal{Decimal: exchangeRate, Valid: true},
		TargetAmount:       decimal.NullDecimal{Decimal: receiveAmount, Valid: true},
		TargetCurrency:     sql.NullString{String: targetCurrency, Valid: true},
		ReceiverFirstName:  receiverFirstName,
		ReceiverLastName:   receiverLastName,
		ReceiverEmail:      req.ReceiverEmail,
		ReceiverPhone:      sql.NullString{String: req.ReceiverPhone, Valid: true},
		ReceiverAddress:    req.ReceiverAddress,
		ReceiverCity:       req.ReceiverCity,
		ReceiverState:      req.ReceiverState,
		ReceiverPostalCode: req.ReceiverPostalCode,
		ReceiverCountry:    req.ReceiverCountry,
		PayoutType:         string(req.PayoutType),
		AccountNumber:      sql.NullString{String: req.AccountNumber, Valid: true},
		BankID:             sql.NullString{String: req.BankID, Valid: true},
	})
	if err != nil {
		log.Printf("ERROR: Failed to record remittance: %v", err)
		return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to save remittance record")
	}

	return &domain.RemittanceResponse{
		ID:             remittance.ID.String(),
		Status:         domain.RemittanceCollectionPending,
		SendAmount:     remittance.SourceAmount,
		SendCurrency:   remittance.SourceCurrency,
		ExchangeRate:   remittance.ExchangeRate.Decimal,
		ReceiveAmount:  remittance.TargetAmount.Decimal,
		CaptureContext: captureContext,
		Message:        fmt.Sprintf("Remittance initiated (vID). Beneficiary: %s. Proceed to payment.", beneficiary.Name),
		CreatedAt:      time.Now().UTC(),
	}, nil
}

// ExecutePayout executes the outbound payout leg for a completed collection.
func (s *remittanceService) ExecutePayout(id uuid.UUID) (*domain.PayoutResult, error) {
	log.Printf("INFO: Executing payout for remittance %s", id)

	// 1. Retrieve remittance from DB
	rem, err := s.db.GetRemittanceByID(context.Background(), id)
	if err != nil {
		log.Printf("ERROR: Payout failed - remittance not found: %s", id)
		return nil, domain.NewAppError(http.StatusNotFound, "not found", "remittance not found")
	}
	//fetch user
	user, err := s.db.GetUserByID(context.Background(), database.GetUserByIDParams{
		ID: uuid.MustParse(rem.SenderUserID),
	})
	if err != nil {
		log.Printf("ERROR: Failed to fetch user: %v", err)
		return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to fetch user record")
	}
	// 2. Check if remittance is in COLLECTED state
	if rem.Status != string(domain.RemittanceCollected) {
		log.Printf("ERROR: Payout failed - invalid status %s for %s. Only COLLECTED remittances can be paid out.", rem.Status, id)
		return nil, domain.NewAppError(http.StatusBadRequest, "invalid status", fmt.Sprintf("cannot payout remittance in %s state", rem.Status))
	}

	// 3. Update status to PROCESSING
	_, err = s.db.UpdatePayoutResult(context.Background(), database.UpdatePayoutResultParams{
		PayoutStatus: sql.NullString{String: string(domain.RemittancePayoutProcessing), Valid: true},
		IDOrRef:      id.String(),
		Status:       string(domain.RemittancePayoutProcessing),
	})
	if err != nil {
		log.Printf("ERROR: Failed to update payout result: %v", err)
		return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to update payout result")
	}

	// 4. Dispatch payout (calls mocked service)
	var payoutResult *domain.PayoutResult
	var payoutErr error

	switch rem.PayoutType {
	case string(domain.PayoutWithinBoA):
		payoutResult, payoutErr = s.payoutSvc.TransferWithinBoA(rem.TargetAmount.Decimal.String(), rem.AccountNumber.String, id.String())
	case string(domain.PayoutOtherBank):
		payoutResult, payoutErr = s.payoutSvc.TransferOtherBank(rem.TargetAmount.Decimal.String(), rem.BankID.String, rem.AccountNumber.String, rem.ReceiverFirstName+" "+rem.ReceiverLastName, id.String())
	case string(domain.PayoutTelebirr), string(domain.PayoutMpesa):
		provider := string(rem.PayoutType)
		payoutResult, payoutErr = s.payoutSvc.TransferWallet(rem.TargetAmount.Decimal.String(), rem.ReceiverPhone.String, provider, rem.ReceiverFirstName+" "+rem.ReceiverLastName, user.FirstName+" "+user.LastName, rem.ReceiverPhone.String, id.String())
	default:
		payoutErr = fmt.Errorf("unknown payout type: %s", rem.PayoutType)
	}

	if payoutErr != nil {
		log.Printf("ERROR: Payout failed for %s: %v", id, payoutErr)
		_, err = s.db.UpdatePayoutResult(context.Background(), database.UpdatePayoutResultParams{
			PayoutStatus: sql.NullString{String: string(domain.RemittanceFailed), Valid: true},
			IDOrRef:      id.String(),
			Status:       string(domain.RemittanceFailed),
		})
		if err != nil {
			log.Printf("ERROR: Failed to update payout result: %v", err)
			return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to update payout result")
		}
		return nil, payoutErr
	}

	// 5. Update status to COMPLETED
	_, err = s.db.UpdatePayoutResult(context.Background(), database.UpdatePayoutResultParams{
		PayoutStatus: sql.NullString{String: string(domain.RemittanceCompleted), Valid: true},
		IDOrRef:      id.String(),
		Status:       string(domain.RemittanceCompleted),
	})
	if err != nil {
		log.Printf("ERROR: Failed to update payout result: %v", err)
		return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to update payout result")
	}

	return payoutResult, nil
}

// TriggerManualPayout implements domain.RemittanceService.
func (s *remittanceService) TriggerManualPayout(req *domain.ManualPayoutRequest) (*domain.PayoutResult, error) {

	var rem database.Remittance
	var err error

	// 1. Find the remittance
	if req.ID != "" {
		id := uuid.MustParse(req.ID)
		rem, err = s.db.GetRemittanceByID(context.Background(), id)
		if err != nil {
			return nil, domain.NewAppError(http.StatusNotFound, "not found", "remittance not found")
		}
		if req.Phone != "" && rem.ReceiverPhone.String != req.Phone {
			return nil, domain.NewAppError(http.StatusUnauthorized, "verification failed", "phone number does not match record")
		}
	} else if req.Phone != "" {
		rems, err := s.db.GetRemittancesByReceiver(context.Background(), database.GetRemittancesByReceiverParams{
			ReceiverPhone: sql.NullString{String: req.Phone, Valid: true},
			Status:        sql.NullString{String: string(domain.RemittanceCollected), Valid: true},
		})
		if err != nil {
			return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to lookup remittances")
		}
		if len(rems) == 0 {
			return nil, domain.NewAppError(http.StatusNotFound, "not found", "no collected remittances found")
		}
		rem = database.Remittance{
			ID:                            rems[0].ID,
			CsTransactionID:               rems[0].CsTransactionID,
			CsAuthenticationTransactionID: rems[0].CsAuthenticationTransactionID,
			SenderUserID:                  rems[0].SenderUserID,
			Status:                        rems[0].Status,
		}
	} else {
		return nil, domain.NewAppError(http.StatusBadRequest, "invalid request", "either remittance_id or phone is required")
	}

	return s.ExecutePayout(rem.ID)
}

func (s *remittanceService) GetRemittanceStatus(id string) (domain.Remittance, error) {
	result, err := s.db.GetRemittanceByID(context.Background(), uuid.MustParse(id))
	if err != nil {
		return domain.Remittance{}, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to lookup remittance")
	}
	return domain.Remittance{
		ID:     result.ID.String(),
		Status: domain.RemittanceStatus(result.Status),
	}, nil
}

func (s *remittanceService) GetSenderRemittances(email string, status domain.RemittanceStatus) ([]domain.Remittance, error) {
	user, err := s.db.GetUserByEmail(context.Background(), email)
	if err != nil {
		return nil, domain.NewAppError(http.StatusBadRequest, "user not found", "could not find user by email")
	}
	result, err := s.db.GetRemittancesBySender(context.Background(), database.GetRemittancesBySenderParams{
		SenderUserID: user.ID.String(),
		Status:       sql.NullString{String: string(status), Valid: true},
	})
	if err != nil {
		return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to lookup remittances")
	}
	var remittances []domain.Remittance
	for _, rem := range result {
		remittances = append(remittances, domain.Remittance{
			ID:                            rem.ID.String(),
			Status:                        domain.RemittanceStatus(rem.Status),
			CsTransactionID:               rem.CsTransactionID.String,
			CsAuthenticationTransactionID: rem.CsAuthenticationTransactionID.String,
			SenderCountry:                 rem.SenderCountry,
			SenderState:                   rem.SenderState,
			SenderCity:                    rem.SenderCity,
			SenderAddress:                 rem.SenderAddress,
			SenderPostalCode:              rem.SenderPostalCode,
			SourceAmount:                  rem.SourceAmount,
			SourceCurrency:                rem.SourceCurrency,
			CollectionStatus:              rem.CollectionStatus,
			ExchangeRate:                  rem.ExchangeRate.Decimal,
			TargetAmount:                  rem.TargetAmount,
			TargetCurrency:                rem.TargetCurrency,
			ReceiverName:                  rem.ReceiverFirstName + " " + rem.ReceiverLastName,
			ReceiverPhone:                 rem.ReceiverPhone,
			ReceiverEmail:                 rem.ReceiverEmail,
			ReceiverCountry:               rem.ReceiverCountry,
			ReceiverState:                 rem.ReceiverState,
			ReceiverCity:                  rem.ReceiverCity,
			ReceiverAddress:               rem.ReceiverAddress,
			ReceiverPostalCode:            rem.ReceiverPostalCode,
			PayoutType:                    domain.PayoutType(rem.PayoutType),
			AccountNumber:                 rem.AccountNumber,
			BankID:                        rem.BankID,
			PayoutStatus:                  rem.PayoutStatus,
			CreatedAt:                     rem.CreatedAt.Time,
			UpdatedAt:                     rem.UpdatedAt.Time,
		})
	}
	return remittances, nil
}

func (s *remittanceService) GetReceiverRemittances(phone string, status domain.RemittanceStatus) ([]domain.Remittance, error) {
	result, err := s.db.GetRemittancesByReceiver(context.Background(), database.GetRemittancesByReceiverParams{
		ReceiverPhone: sql.NullString{String: phone, Valid: true},
		Status:        sql.NullString{String: string(status), Valid: true},
	})
	if err != nil {
		return nil, domain.NewAppError(http.StatusInternalServerError, "database error", "failed to lookup remittances")
	}
	var remittances []domain.Remittance
	for _, rem := range result {
		remittances = append(remittances, domain.Remittance{
			ID:                            rem.ID.String(),
			Status:                        domain.RemittanceStatus(rem.Status),
			CsTransactionID:               rem.CsTransactionID.String,
			CsAuthenticationTransactionID: rem.CsAuthenticationTransactionID.String,
			SenderCountry:                 rem.SenderCountry,
			SenderState:                   rem.SenderState,
			SenderCity:                    rem.SenderCity,
			SenderAddress:                 rem.SenderAddress,
			SenderPostalCode:              rem.SenderPostalCode,
			SourceAmount:                  rem.SourceAmount,
			SourceCurrency:                rem.SourceCurrency,
			CollectionStatus:              rem.CollectionStatus,
			ExchangeRate:                  rem.ExchangeRate.Decimal,
			TargetAmount:                  rem.TargetAmount,
			TargetCurrency:                rem.TargetCurrency,
			ReceiverName:                  rem.ReceiverFirstName + " " + rem.ReceiverLastName,
			ReceiverPhone:                 rem.ReceiverPhone,
			ReceiverEmail:                 rem.ReceiverEmail,
			ReceiverCountry:               rem.ReceiverCountry,
			ReceiverState:                 rem.ReceiverState,
			ReceiverCity:                  rem.ReceiverCity,
			ReceiverAddress:               rem.ReceiverAddress,
			ReceiverPostalCode:            rem.ReceiverPostalCode,
			PayoutType:                    domain.PayoutType(rem.PayoutType),
			AccountNumber:                 rem.AccountNumber,
			BankID:                        rem.BankID,
			PayoutStatus:                  rem.PayoutStatus,
			CreatedAt:                     rem.CreatedAt.Time,
			UpdatedAt:                     rem.UpdatedAt.Time,
		})
	}
	return remittances, nil
}
