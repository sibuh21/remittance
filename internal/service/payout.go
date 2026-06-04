package service

import (
	"fmt"
	"log"
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
	log.Printf("MOCK: Validating beneficiary %s (%s)", accountOrPhone, payoutType)

	// Always valid for testing
	return &domain.BeneficiaryCheckResponse{
		Valid:        true,
		Name:         "Test Recipient",
		CurrencyCode: "ETB",
	}, nil
}

// TransferWithinBoA initiates a fund transfer within Bank of Abyssinia.
func (s *payoutService) TransferWithinBoA(amount, accountNumber, reference string) (*domain.PayoutResult, error) {
	log.Printf("MOCK: Transfer within BoA initiated - Amount: %s", amount)
	return &domain.PayoutResult{
		PayoutID:     reference,
		Status:       "SUCCESS",
		BoAReference: "MOCK-BOA-" + uuid.New().String()[:8],
		Amount:       amount,
		Currency:     "ETB",
		PayoutType:   domain.PayoutWithinBoA,
		Message:      "Transfer within BoA (MOCKED)",
		ProcessedAt:  time.Now().UTC(),
	}, nil
}

// TransferOtherBank initiates a fund transfer to another bank via EthSwitch.
func (s *payoutService) TransferOtherBank(amount, bankID, accountNumber, receiverName, reference string) (*domain.PayoutResult, error) {
	log.Printf("MOCK: Other bank transfer initiated - Amount: %s", amount)
	return &domain.PayoutResult{
		PayoutID:     reference,
		Status:       "SUCCESS",
		BoAReference: "MOCK-ETH-" + uuid.New().String()[:8],
		Amount:       amount,
		Currency:     "ETB",
		ReceiverName: receiverName,
		PayoutType:   domain.PayoutOtherBank,
		Message:      "Other bank transfer (MOCKED)",
		ProcessedAt:  time.Now().UTC(),
	}, nil
}

// TransferWallet initiates a fund transfer to Telebirr or Mpesa wallet.
func (s *payoutService) TransferWallet(amount, phoneNumber, provider, receiverName, senderName, senderPhone, reference string) (*domain.PayoutResult, error) {
	log.Printf("MOCK: Wallet transfer initiated (%s) - Amount: %s", provider, amount)
	return &domain.PayoutResult{
		PayoutID:     reference,
		Status:       "SUCCESS",
		BoAReference: "MOCK-WALLET-" + uuid.New().String()[:8],
		Amount:       amount,
		Currency:     "ETB",
		ReceiverName: receiverName,
		PayoutType:   domain.PayoutType(provider),
		Message:      fmt.Sprintf("Wallet transfer to %s (MOCKED)", provider),
		ProcessedAt:  time.Now().UTC(),
	}, nil
}

// CheckTransactionStatus checks the status of a previously initiated BoA transaction.
func (s *payoutService) CheckTransactionStatus(transactionID string) (*domain.TransactionStatusResponse, error) {
	return &domain.TransactionStatusResponse{
		TransactionID: transactionID,
		Status:        "SUCCESS",
		Detail:        map[string]any{"note": "Mocked response"},
	}, nil
}

// GetExchangeRate retrieves the current exchange rate for a given base currency.
func (s *payoutService) GetExchangeRate(baseCurrency string) (*domain.ExchangeRateResponse, error) {
	return &domain.ExchangeRateResponse{
		BaseCurrency:   baseCurrency,
		TargetCurrency: "ETB",
		Rate:           135.5,
		Timestamp:      time.Now().UTC(),
	}, nil
}

// GetBalance retrieves the settlement account balance.
func (s *payoutService) GetBalance() (*domain.BoABalanceResponse, error) {
	return &domain.BoABalanceResponse{
		Currency: "ETB",
		Balance:  1000000.0,
	}, nil
}

// GetBanks retrieves the list of available banks for other-bank transfers.
func (s *payoutService) GetBanks() ([]domain.BoABankInfo, error) {
	return []domain.BoABankInfo{
		{BankID: "BOA", BankName: "Bank of Abyssinia"},
		{BankID: "CBE", BankName: "Commercial Bank of Ethiopia"},
		{BankID: "AWASH", BankName: "Awash International Bank"},
		{BankID: "DASHP", BankName: "Dashen Bank"},
	}, nil
}
