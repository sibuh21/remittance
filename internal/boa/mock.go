package boa

import (
	"fmt"

	"remittance-service/internal/domain"

	"github.com/google/uuid"
)

// MockClient is a mock implementation of the BoAClient interface.
type MockClient struct{}

func NewMockClient() domain.BoAClient {
	return &MockClient{}
}

func (m *MockClient) FetchAccountName(accountID string) (*domain.BoAAccountInfo, error) {
	return &domain.BoAAccountInfo{
		CustomerName:    "Mock Recipient (BoA Account)",
		AccountCurrency: "ETB",
	}, nil
}

func (m *MockClient) FetchAccountNameOtherBank(bankID, accountID string) (*domain.BoAAccountInfo, error) {
	return &domain.BoAAccountInfo{
		CustomerName:    "Mock Recipient (Other Bank)",
		AccountCurrency: "ETB",
	}, nil
}

func (m *MockClient) FetchNameTelebirr(phoneNumber string) (*domain.BoANameCheckResponse, error) {
	return &domain.BoANameCheckResponse{
		Status:       "SUCCESS",
		CustomerName: "Mock Telebirr User",
	}, nil
}

func (m *MockClient) FetchNameMpesa(phoneNumber string) (*domain.BoANameCheckResponse, error) {
	return &domain.BoANameCheckResponse{
		Status:       "SUCCESS",
		CustomerName: "Mock Mpesa User",
	}, nil
}

func (m *MockClient) TransferWithin(req *domain.BoATransferWithinRequest) (*domain.BoAAPIResponse, error) {
	return &domain.BoAAPIResponse{
		Header: domain.BoAResponseHeader{
			Status:    "SUCCESS",
			Reference: "MOCK-BOA-" + uuid.New().String()[:8],
		},
		Body: map[string]any{"message": "Mock within-bank transfer successful"},
	}, nil
}

func (m *MockClient) TransferOtherBank(req *domain.BoAOtherBankTransferRequest) (*domain.BoAAPIResponse, error) {
	return &domain.BoAAPIResponse{
		Header: domain.BoAResponseHeader{
			Status:    "SUCCESS",
			Reference: "MOCK-ETH-" + uuid.New().String()[:8],
		},
		Body: map[string]any{"message": "Mock other-bank transfer successful"},
	}, nil
}

func (m *MockClient) TransferWallet(req *domain.BoAWalletTransferRequest) (*domain.BoAAPIResponse, error) {
	return &domain.BoAAPIResponse{
		Header: domain.BoAResponseHeader{
			Status:    "SUCCESS",
			Reference: "MOCK-WALLET-" + uuid.New().String()[:8],
		},
		Body: map[string]any{"message": fmt.Sprintf("Mock wallet transfer via %s successful", req.MMProvider)},
	}, nil
}

func (m *MockClient) GetBankIDs() ([]domain.BoABankInfo, error) {
	return []domain.BoABankInfo{
		{BankID: "BOA", BankName: "Bank of Abyssinia"},
		{BankID: "CBE", BankName: "Commercial Bank of Ethiopia"},
		{BankID: "AWASH", BankName: "Awash International Bank"},
		{BankID: "DASHP", BankName: "Dashen Bank"},
	}, nil
}

func (m *MockClient) GetTransactionStatus(transactionID string) (*domain.BoAAPIResponse, error) {
	return &domain.BoAAPIResponse{
		Header: domain.BoAResponseHeader{
			Status: "SUCCESS",
		},
		Body: map[string]any{"note": "Mocked transaction status"},
	}, nil
}

func (m *MockClient) GetExchangeRate(baseCurrency string) (*domain.BoAAPIResponse, error) {
	return &domain.BoAAPIResponse{
		Body: map[string]any{
			"baseCurrency":   baseCurrency,
			"targetCurrency": "ETB",
			"rate":           135.5,
		},
	}, nil
}

func (m *MockClient) GetBalance() (*domain.BoAAPIResponse, error) {
	return &domain.BoAAPIResponse{
		Body: map[string]any{
			"currency": "ETB",
			"balance":  1000000.0,
		},
	}, nil
}
