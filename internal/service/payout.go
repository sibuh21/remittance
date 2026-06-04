package service

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"remittance-service/internal/domain"
)

type payoutService struct {
	boaClient domain.BoAClient
}

// NewPayoutService creates a new PayoutService backed by a BoA client (real or mock).
func NewPayoutService(boaClient domain.BoAClient) domain.PayoutService {
	return &payoutService{boaClient: boaClient}
}

// ValidateBeneficiary validates a beneficiary account/wallet before payout
// using the real BoA API.
func (s *payoutService) ValidateBeneficiary(payoutType domain.PayoutType, accountOrPhone, bankID string) (*domain.BeneficiaryCheckResponse, error) {
	log.Printf("INFO: Validating beneficiary %s (type: %s, bank: %s)", accountOrPhone, payoutType, bankID)

	switch payoutType {
	case domain.PayoutWithinBoA:
		info, err := s.boaClient.FetchAccountName(accountOrPhone)
		if err != nil {
			log.Printf("ERROR: BoA within-bank beneficiary validation failed: %v", err)
			return &domain.BeneficiaryCheckResponse{
				Valid:   false,
				Message: fmt.Sprintf("Account validation failed: %v", err),
			}, nil
		}
		return &domain.BeneficiaryCheckResponse{
			Valid:        true,
			Name:         info.CustomerName,
			CurrencyCode: info.AccountCurrency,
		}, nil

	case domain.PayoutOtherBank:
		info, err := s.boaClient.FetchAccountNameOtherBank(bankID, accountOrPhone)
		if err != nil {
			log.Printf("ERROR: BoA other-bank beneficiary validation failed: %v", err)
			return &domain.BeneficiaryCheckResponse{
				Valid:   false,
				Message: fmt.Sprintf("Account validation failed: %v", err),
			}, nil
		}
		return &domain.BeneficiaryCheckResponse{
			Valid:        true,
			Name:         info.CustomerName,
			CurrencyCode: info.AccountCurrency,
		}, nil

	case domain.PayoutTelebirr:
		info, err := s.boaClient.FetchNameTelebirr(accountOrPhone)
		if err != nil {
			log.Printf("ERROR: BoA Telebirr name validation failed: %v", err)
			return &domain.BeneficiaryCheckResponse{
				Valid:   false,
				Message: fmt.Sprintf("Telebirr validation failed: %v", err),
			}, nil
		}
		return &domain.BeneficiaryCheckResponse{
			Valid: true,
			Name:  info.CustomerName,
		}, nil

	case domain.PayoutMpesa:
		info, err := s.boaClient.FetchNameMpesa(accountOrPhone)
		if err != nil {
			log.Printf("ERROR: BoA Mpesa name validation failed: %v", err)
			return &domain.BeneficiaryCheckResponse{
				Valid:   false,
				Message: fmt.Sprintf("Mpesa validation failed: %v", err),
			}, nil
		}
		return &domain.BeneficiaryCheckResponse{
			Valid: true,
			Name:  info.CustomerName,
		}, nil

	default:
		return &domain.BeneficiaryCheckResponse{
			Valid:   false,
			Message: fmt.Sprintf("unsupported payout type: %s", payoutType),
		}, nil
	}
}

// TransferWithinBoA initiates a real fund transfer within Bank of Abyssinia.
func (s *payoutService) TransferWithinBoA(amount, accountNumber, reference string) (*domain.PayoutResult, error) {
	log.Printf("INFO: BoA within-bank transfer - Amount: %s, Account: %s, Ref: %s", amount, accountNumber, reference)

	// Shorten reference for BoA (often limited to 16 chars in T24)
	boaRef := reference
	if len(boaRef) > 15 {
		boaRef = boaRef[:15]
	}

	req := &domain.BoATransferWithinRequest{
		Amount:        amount,
		AccountNumber: accountNumber,
		Reference:     boaRef,
	}

	resp, err := s.boaClient.TransferWithin(req)
	if err != nil {
		return nil, fmt.Errorf("BoA within-bank transfer failed: %w", err)
	}

	return &domain.PayoutResult{
		PayoutID:     reference,
		Status:       resp.Header.Status,
		BoAReference: resp.Header.Reference,
		Amount:       amount,
		Currency:     "ETB",
		PayoutType:   domain.PayoutWithinBoA,
		Message:      fmt.Sprintf("Within-BoA transfer completed (status: %s)", resp.Header.Status),
		ProcessedAt:  time.Now().UTC(),
	}, nil
}

// TransferOtherBank initiates a real fund transfer to another bank via EthSwitch.
func (s *payoutService) TransferOtherBank(amount, bankID, accountNumber, receiverName, reference string) (*domain.PayoutResult, error) {
	log.Printf("INFO: BoA other-bank transfer - Amount: %s, Bank: %s, Account: %s, Ref: %s", amount, bankID, accountNumber, reference)

	// Shorten reference for BoA (often limited to 16 chars in T24)
	boaRef := reference
	if len(boaRef) > 15 {
		boaRef = boaRef[:15]
	}

	req := &domain.BoAOtherBankTransferRequest{
		Amount:        amount,
		BankCode:      bankID,
		AccountNumber: accountNumber,
		ReceiverName:  receiverName,
		Reference:     boaRef,
	}

	resp, err := s.boaClient.TransferOtherBank(req)
	if err != nil {
		return nil, fmt.Errorf("BoA other-bank transfer failed: %w", err)
	}

	return &domain.PayoutResult{
		PayoutID:     reference,
		Status:       resp.Header.Status,
		BoAReference: resp.Header.Reference,
		Amount:       amount,
		Currency:     "ETB",
		ReceiverName: receiverName,
		PayoutType:   domain.PayoutOtherBank,
		Message:      fmt.Sprintf("Other-bank transfer completed (status: %s)", resp.Header.Status),
		ProcessedAt:  time.Now().UTC(),
	}, nil
}

// TransferWallet initiates a real fund transfer to Telebirr or Mpesa wallet.
func (s *payoutService) TransferWallet(amount, phoneNumber, provider, receiverName, senderName, senderPhone, reference string) (*domain.PayoutResult, error) {
	log.Printf("INFO: BoA wallet transfer - Provider: %s, Amount: %s, Phone: %s, Ref: %s", provider, amount, phoneNumber, reference)

	// Shorten reference for BoA (often limited to 16 chars in T24)
	boaRef := reference
	if len(boaRef) > 15 {
		boaRef = boaRef[:15]
	}

	req := &domain.BoAWalletTransferRequest{
		Amount:              amount,
		ReceiverPhonenumber: phoneNumber,
		MMProvider:          provider,
		Reference:           boaRef,
		ReceiverName:        receiverName,
		RemitterName:        senderName,
		RemitterPhonenumber: senderPhone,
		SecretCode:          "123456",
	}

	resp, err := s.boaClient.TransferWallet(req)
	if err != nil {
		return nil, fmt.Errorf("BoA wallet transfer failed: %w", err)
	}

	return &domain.PayoutResult{
		PayoutID:     reference,
		Status:       resp.Header.Status,
		BoAReference: resp.Header.Reference,
		Amount:       amount,
		Currency:     "ETB",
		ReceiverName: receiverName,
		PayoutType:   domain.PayoutType(provider),
		Message:      fmt.Sprintf("Wallet transfer to %s completed (status: %s)", provider, resp.Header.Status),
		ProcessedAt:  time.Now().UTC(),
	}, nil
}

// CheckTransactionStatus checks the status of a previously initiated BoA transaction.
func (s *payoutService) CheckTransactionStatus(transactionID string) (*domain.TransactionStatusResponse, error) {
	log.Printf("INFO: Checking BoA transaction status: %s", transactionID)

	resp, err := s.boaClient.GetTransactionStatus(transactionID)
	if err != nil {
		return nil, fmt.Errorf("BoA transaction status check failed: %w", err)
	}

	return &domain.TransactionStatusResponse{
		TransactionID: transactionID,
		Status:        resp.Header.Status,
		Detail:        resp.Body,
	}, nil
}

// GetExchangeRate retrieves the current exchange rate from BoA for a given base currency.
func (s *payoutService) GetExchangeRate(baseCurrency string) (*domain.ExchangeRateResponse, error) {
	log.Printf("INFO: Fetching BoA exchange rate for %s", baseCurrency)

	resp, err := s.boaClient.GetExchangeRate(baseCurrency)
	if err != nil {
		return nil, fmt.Errorf("BoA exchange rate fetch failed: %w", err)
	}

	// Extract rate from BoA response body
	var rate float64
	if rateVal, ok := resp.Body["rate"]; ok {
		switch v := rateVal.(type) {
		case float64:
			rate = v
		case string:
			rate, _ = strconv.ParseFloat(v, 64)
		}
	}

	targetCurrency := "ETB"
	if tc, ok := resp.Body["targetCurrency"]; ok {
		if tcStr, ok := tc.(string); ok {
			targetCurrency = tcStr
		}
	}

	log.Printf("INFO: BoA exchange rate %s → %s: %.4f", baseCurrency, targetCurrency, rate)

	return &domain.ExchangeRateResponse{
		BaseCurrency:   baseCurrency,
		TargetCurrency: targetCurrency,
		Rate:           rate,
		Timestamp:      time.Now().UTC(),
	}, nil
}

// GetBalance retrieves the settlement account balance from BoA.
func (s *payoutService) GetBalance() (*domain.BoABalanceResponse, error) {
	log.Printf("INFO: Fetching BoA settlement balance")

	resp, err := s.boaClient.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("BoA balance fetch failed: %w", err)
	}

	var balance float64
	if balVal, ok := resp.Body["balance"]; ok {
		switch v := balVal.(type) {
		case float64:
			balance = v
		case string:
			balance, _ = strconv.ParseFloat(v, 64)
		}
	}

	currency := "ETB"
	if cur, ok := resp.Body["currency"]; ok {
		if curStr, ok := cur.(string); ok {
			currency = curStr
		}
	}

	return &domain.BoABalanceResponse{
		Currency: currency,
		Balance:  balance,
	}, nil
}

// GetBanks retrieves the list of available banks for other-bank transfers from BoA.
func (s *payoutService) GetBanks() ([]domain.BoABankInfo, error) {
	log.Printf("INFO: Fetching BoA bank list")

	banks, err := s.boaClient.GetBankIDs()
	if err != nil {
		return nil, fmt.Errorf("BoA bank list fetch failed: %w", err)
	}

	return banks, nil
}
