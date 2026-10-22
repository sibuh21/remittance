package cybersource

// import (
// 	"context"
// 	"remittance-service/internal/cybersource"
// 	"remittance-service/internal/domain"
// )

// type MockCybersource struct {
// 	ResponseStatus              string
// 	InstrumentIdentifierID      string
// 	StepUpUrl                   string
// 	AccessToken                 string
// 	AuthenticationTransactionId string
// 	ReferenceId                 string
// }

// func NewMockCybersource() *MockCybersource {
// 	return &MockCybersource{
// 		ResponseStatus:         domain.CSStatusAuthorized,
// 		InstrumentIdentifierID: "mock-perm-token",
// 		AccessToken:            "mock-access-token",
// 		ReferenceId:            "mock-ref-id",
// 	}
// }

// var _ cybersource.Cybersource = (*MockCybersource)(nil)

// func (m *MockCybersource) CreateCaptureContext(ctx context.Context, req *domain.CaptureContextRequest) (string, error) {
// 	return "mock-capture-context-jwt", nil
// }

// func (m *MockCybersource) PASetup(ctx context.Context, req *domain.PASetupRequest) (*domain.PASetupResponse, error) {
// 	refID := m.ReferenceId
// 	if refID == "" {
// 		refID = "mock-pa-ref-id"
// 	}
// 	return &dto.PASetupResponse{
// 		ID:     "mock-pa-setup-id",
// 		Status: "COMPLETED",
// 		ConsumerAuthenticationInfo: dto.ConsumerAuthResponse{
// 			AccessToken:             m.AccessToken,
// 			DeviceDataCollectionUrl: "https://mock.cybersource.com/collect",
// 			ReferenceId:             refID,
// 		},
// 	}, nil
// }

// func (m *MockCybersource) AuthorizePayment(ctx context.Context, req *dto.AFTRequest) (*dto.PaymentResponse, error) {
// 	status := m.ResponseStatus
// 	if status == "" {
// 		status = dto.CSStatusAuthorized
// 	}

// 	resp := &dto.PaymentResponse{
// 		ID:     "mock-cs-txn-id",
// 		Status: status,
// 		ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{
// 			StepUpUrl:                   m.StepUpUrl,
// 			AccessToken:                 m.AccessToken,
// 			AuthenticationTransactionId: m.AuthenticationTransactionId,
// 		},
// 	}

// 	if m.InstrumentIdentifierID != "" {
// 		resp.TokenInformation = &dto.TokenInfoResponse{
// 			InstrumentIdentifier: &dto.TMSTokenRef{ID: m.InstrumentIdentifierID},
// 		}
// 	}

// 	return resp, nil
// }

// func (m *MockCybersource) ReverseAuthorization(ctx context.Context, paymentID string) error {
// 	return nil
// }

// func (m *MockCybersource) GenerateHTTPSignature(
// 	ctx context.Context, method, path string, body []byte) (map[string]string, error) {
// 	return map[string]string{"Signature": "mock-signature"}, nil
// }
