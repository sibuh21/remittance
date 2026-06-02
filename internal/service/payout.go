package service

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"remittance-service/internal/boa"
	"remittance-service/internal/domain"

	"github.com/google/uuid"
)

type payoutService struct {
	boaClient *boa.Client
}

// NewPayoutService creates a new PayoutService backed by the BoA client.
func NewPayoutService(boaClient *boa.Client) domain.PayoutService {
	return &payoutService{boaClient: boaClient}
}

// ValidateBeneficiary validates a beneficiary account/wallet before payout.
func (s *payoutService) ValidateBeneficiary(payoutType domain.PayoutType, accountOrPhone, bankID string) (*domain.BeneficiaryCheckResponse, error) {
	switch payoutType {
	case domain.PayoutWithinBoA:
		info, err := s.boaClient.FetchAccountName(accountOrPhone)
		if err != nil {
			return &domain.BeneficiaryCheckResponse{
				Valid:   false,
				Message: err.Error(),
			}, nil
		}
		if info.EnquiryStatus == 0 {
			desc := domain.BoAErrorCodes[info.ErrorCode]
			return &domain.BeneficiaryCheckResponse{
				Valid:   false,
				Message: fmt.Sprintf("Account validation failed: %s (%s)", desc, info.ErrorCode),
			}, nil
		}
		return &domain.BeneficiaryCheckResponse{
			Valid:        true,
			Name:         info.AccountHolderName,
			CurrencyCode: info.CurrencyCode,
		}, nil

	case domain.PayoutOtherBank:
		if bankID == "" {
			return nil, domain.NewAppError(http.StatusBadRequest, "validation failed", "bank_id is required for other bank transfers")
		}
		info, err := s.boaClient.FetchAccountNameOtherBank(bankID, accountOrPhone)
		if err != nil {
			return &domain.BeneficiaryCheckResponse{
				Valid:   false,
				Message: err.Error(),
			}, nil
		}
		return &domain.BeneficiaryCheckResponse{
			Valid:        true,
			Name:         info.AccountHolderName,
			CurrencyCode: info.CurrencyCode,
		}, nil

	case domain.PayoutTelebirr:
		resp, err := s.boaClient.FetchNameTelebirr(accountOrPhone)
		if err != nil {
			return &domain.BeneficiaryCheckResponse{
				Valid:   false,
				Message: err.Error(),
			}, nil
		}
		return &domain.BeneficiaryCheckResponse{
			Valid: true,
			Name:  resp.CustomerName,
		}, nil

	case domain.PayoutMpesa:
		resp, err := s.boaClient.FetchNameMpesa(accountOrPhone)
		if err != nil {
			return &domain.BeneficiaryCheckResponse{
				Valid:   false,
				Message: err.Error(),
			}, nil
		}
		return &domain.BeneficiaryCheckResponse{
			Valid: true,
			Name:  resp.CustomerName,
		}, nil

	default:
		return nil, domain.NewAppError(http.StatusBadRequest, "invalid payout type", string(payoutType))
	}
}

// TransferWithinBoA initiates a fund transfer within Bank of Abyssinia.
func (s *payoutService) TransferWithinBoA(amount, accountNumber, reference string) (*domain.PayoutResult, error) {
	if reference == "" {
		reference = uuid.New().String()
	}

	req := &domain.BoATransferWithinRequest{
		Amount:        amount,
		AccountNumber: accountNumber,
		Reference:     reference,
	}

	resp, err := s.boaClient.TransferWithin(req)
	if err != nil {
		log.Printf("ERROR: BoA within-bank transfer failed: %v", err)
		return nil, fmt.Errorf("payout failed: %w", err)
	}

	return &domain.PayoutResult{
		PayoutID:     reference,
		Status:       resp.Header.Status,
		BoAReference: resp.Header.Reference,
		Amount:       amount,
		Currency:     "ETB",
		PayoutType:   domain.PayoutWithinBoA,
		Message:      "Transfer within BoA initiated",
		ProcessedAt:  time.Now().UTC(),
	}, nil
}

// TransferOtherBank initiates a fund transfer to another bank via EthSwitch.
func (s *payoutService) TransferOtherBank(amount, bankID, accountNumber, receiverName, reference string) (*domain.PayoutResult, error) {
	if reference == "" {
		reference = uuid.New().String()
	}

	req := &domain.BoAOtherBankTransferRequest{
		Amount:        amount,
		Reference:     reference,
		BankID:        bankID,
		AccountNumber: accountNumber,
		ReceiverName:  receiverName,
	}

	resp, err := s.boaClient.TransferOtherBank(req)
	if err != nil {
		log.Printf("ERROR: BoA other-bank transfer failed: %v", err)
		return nil, fmt.Errorf("payout failed: %w", err)
	}

	return &domain.PayoutResult{
		PayoutID:     reference,
		Status:       resp.Header.Status,
		BoAReference: resp.Header.Reference,
		Amount:       amount,
		Currency:     "ETB",
		ReceiverName: receiverName,
		PayoutType:   domain.PayoutOtherBank,
		Message:      "Other bank transfer initiated via EthSwitch",
		ProcessedAt:  time.Now().UTC(),
	}, nil
}

// TransferWallet initiates a fund transfer to Telebirr or Mpesa wallet.
func (s *payoutService) TransferWallet(amount, phoneNumber, provider, receiverName, senderName, senderPhone, reference string) (*domain.PayoutResult, error) {
	if reference == "" {
		reference = uuid.New().String()
	}

	req := &domain.BoAWalletTransferRequest{
		Amount:           amount,
		PhoneNumber:      phoneNumber,
		MMProvider:       provider,
		Reference:        reference,
		ReceiverName:     receiverName,
		OrderingCustomer: senderName,
		SenderPhone:      senderPhone,
	}

	resp, err := s.boaClient.TransferWallet(req)
	if err != nil {
		log.Printf("ERROR: BoA wallet transfer failed (provider: %s): %v", provider, err)
		return nil, fmt.Errorf("payout failed: %w", err)
	}

	return &domain.PayoutResult{
		PayoutID:     reference,
		Status:       resp.Header.Status,
		BoAReference: resp.Header.Reference,
		Amount:       amount,
		Currency:     "ETB",
		ReceiverName: receiverName,
		PayoutType:   domain.PayoutType(provider),
		Message:      fmt.Sprintf("Wallet transfer to %s initiated", provider),
		ProcessedAt:  time.Now().UTC(),
	}, nil
}

// CheckTransactionStatus checks the status of a previously initiated BoA transaction.
func (s *payoutService) CheckTransactionStatus(transactionID string) (*domain.TransactionStatusResponse, error) {
	resp, err := s.boaClient.GetTransactionStatus(transactionID)
	if err != nil {
		return nil, fmt.Errorf("check transaction status: %w", err)
	}

	return &domain.TransactionStatusResponse{
		TransactionID: transactionID,
		Status:        resp.Header.Status,
		Detail:        resp.Body,
	}, nil
}

// GetExchangeRate retrieves the current exchange rate for a given base currency.
func (s *payoutService) GetExchangeRate(baseCurrency string) (*domain.ExchangeRateResponse, error) {
	resp, err := s.boaClient.GetExchangeRate(baseCurrency)
	if err != nil {
		return nil, fmt.Errorf("get exchange rate: %w", err)
	}

	rate := 0.0
	if r, ok := resp.Body["rate"]; ok {
		if f, ok := r.(float64); ok {
			rate = f
		}
	}

	return &domain.ExchangeRateResponse{
		BaseCurrency:   baseCurrency,
		TargetCurrency: "ETB",
		Rate:           rate,
		Timestamp:      time.Now().UTC(),
	}, nil
}

// GetBalance retrieves the settlement account balance.
func (s *payoutService) GetBalance() (*domain.BoABalanceResponse, error) {
	resp, err := s.boaClient.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	balance := &domain.BoABalanceResponse{}
	if c, ok := resp.Body["currency"]; ok {
		if s, ok := c.(string); ok {
			balance.Currency = s
		}
	}
	if b, ok := resp.Body["balance"]; ok {
		if f, ok := b.(float64); ok {
			balance.Balance = f
		}
	}

	return balance, nil
}

// GetBanks retrieves the list of available banks for other-bank transfers.
func (s *payoutService) GetBanks() ([]domain.BoABankInfo, error) {
	return s.boaClient.GetBankIDs()
}
