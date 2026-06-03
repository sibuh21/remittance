package service

import (
	"fmt"

	"remittance-service/internal/cybersource"
	"remittance-service/internal/domain"
)

type collectionService struct {
	csRESTClient *cybersource.RESTClient // NEW
	returnURL    string
}

// NewCollectionService creates a new CollectionService.
func NewCollectionService(csRESTClient *cybersource.RESTClient, returnURL string) domain.CollectionService {
	return &collectionService{
		csRESTClient: csRESTClient,
		returnURL:    returnURL,
	}
}

// === Flex Microform (REST API) Methods ===

func (s *collectionService) CreateCaptureContext(origins []string) (string, error) {
	req := &cybersource.CaptureContextRequest{
		TargetOrigins: origins,
	}
	return s.csRESTClient.CreateCaptureContext(req)
}

func (s *collectionService) SetupPASetup(req *domain.PASetupRequest) (*domain.PASetupResponse, error) {
	paReq := &cybersource.PASetupRequest{
		ClientReferenceInformation: cybersource.ClientReferenceInfo{
			Code: req.RemittanceID,
		},
		TokenInformation: &cybersource.PASetupTokenInfo{
			TransientToken: req.TransientTokenJti,
		},
	}

	resp, err := s.csRESTClient.PASetup(paReq)
	if err != nil {
		return nil, fmt.Errorf("PA Setup failed: %w", err)
	}

	return &domain.PASetupResponse{
		AccessToken:             resp.ConsumerAuthenticationInfo.AccessToken,
		DeviceDataCollectionUrl: resp.ConsumerAuthenticationInfo.DeviceChannel, // This is sometimes returned here or in setupUrl depending on version
		ReferenceId:             resp.ID,
	}, nil
}

func (s *collectionService) AuthorizePayment(req *domain.AuthorizeRequest) (*domain.AuthorizeResponse, error) {
	paReq := s.buildPaymentRequest(req, []string{"CONSUMER_AUTHENTICATION", "TOKEN_CREATE"})
	resp, err := s.csRESTClient.AuthorizePayment(paReq)
	if err != nil {
		return nil, err
	}

	return s.mapPaymentResponse(resp, req.RemittanceID)
}

func (s *collectionService) ValidateAndAuthorize(req *domain.ValidateRequest) (*domain.AuthorizeResponse, error) {
	// For Step 7, we call the same payment endpoint but with a different action list and the auth transaction ID
	paReq := &cybersource.PaymentRequest{
		ClientReferenceInformation: cybersource.ClientReferenceInfo{
			Code: req.RemittanceID,
		},
		ProcessingInformation: cybersource.ProcessingInfo{
			Capture:    true,
			ActionList: []string{"VALIDATE_CONSUMER_AUTHENTICATION"},
		},
		ConsumerAuthenticationInfo: &cybersource.ConsumerAuthInfo{
			AuthenticationTransactionId: req.AuthenticationTransactionId,
		},
	}

	resp, err := s.csRESTClient.AuthorizePayment(paReq)
	if err != nil {
		return nil, err
	}

	return s.mapPaymentResponse(resp, req.RemittanceID)
}

func (s *collectionService) buildPaymentRequest(req *domain.AuthorizeRequest, actionList []string) *cybersource.PaymentRequest {
	return &cybersource.PaymentRequest{
		ClientReferenceInformation: cybersource.ClientReferenceInfo{
			Code: req.RemittanceID,
		},
		ProcessingInformation: cybersource.ProcessingInfo{
			Capture:           true,
			CommerceIndicator: "internet",
			ActionList:        actionList,
			ActionTokenTypes:  []string{"customer", "paymentInstrument", "instrumentIdentifier"},
			AuthorizationOptions: &cybersource.AuthOptions{
				AFTIndicator: "true",
				FundingOptions: &cybersource.FundingOptions{
					Initiator: &cybersource.FundingInitiator{Type: "S"},
				},
			},
		},
		OrderInformation: cybersource.OrderInfo{
			BillTo: cybersource.BillTo{
				FirstName:  req.Sender.FirstName,
				LastName:   req.Sender.LastName,
				Email:      req.Sender.Email,
				Address1:   req.Sender.Address,
				Locality:   req.Sender.City,
				Country:    req.Sender.Country,
				PostalCode: req.Sender.PostalCode,
			},
			AmountDetails: cybersource.AmountDetails{
				TotalAmount: req.Amount,
				Currency:    req.Currency,
			},
		},
		TokenInformation: &cybersource.TokenInfo{
			TransientTokenJWT: req.TransientTokenJWT,
		},
		ConsumerAuthenticationInfo: &cybersource.ConsumerAuthInfo{
			ReferenceId:   req.PAReferenceId,
			ReturnUrl:     s.returnURL,
			DeviceChannel: "Browser",
		},
		SenderInformation: &cybersource.SenderInfo{
			FirstName:   req.Sender.FirstName,
			LastName:    req.Sender.LastName,
			Address1:    req.Sender.Address,
			Locality:    req.Sender.City,
			CountryCode: req.Sender.CountryISO3,
		},
		RecipientInformation: &cybersource.RecipientInfo{
			FirstName: req.Recipient.FirstName,
			LastName:  req.Recipient.LastName,
			Country:   req.Recipient.CountryISO3,
		},
	}
}

func (s *collectionService) mapPaymentResponse(resp *cybersource.PaymentResponse, remittanceID string) (*domain.AuthorizeResponse, error) {
	domainResp := &domain.AuthorizeResponse{
		Status:       resp.Status,
		RemittanceID: remittanceID,
	}

	if resp.Status == "PENDING_AUTHENTICATION" && resp.ConsumerAuthenticationInfo != nil {
		domainResp.StepUpUrl = resp.ConsumerAuthenticationInfo.StepUpUrl
		domainResp.AccessToken = resp.ConsumerAuthenticationInfo.AccessToken
		domainResp.AuthenticationTransactionId = resp.ConsumerAuthenticationInfo.AuthenticationTransactionId
	}

	if resp.Status == "DECLINED" || resp.Status == "REJECTED" || resp.Status == "PARTIAL_AUTHORIZED" {
		if resp.ErrorInformation != nil {
			domainResp.Message = resp.ErrorInformation.Message
		}
	}

	return domainResp, nil
}
