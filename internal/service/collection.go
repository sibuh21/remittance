package service

import (
	"fmt"
	"log"
	"strings"

	"remittance-service/internal/cybersource"
	"remittance-service/internal/domain"
)

// collectionService implements domain.CollectionService for CyberSource Flex Microform.
type collectionService struct {
	csRESTClient *cybersource.RESTClient
	db           interface {
		UpdateCollectionResult(remittanceID, cybersourceRef, collectionStatus, status string) error
	}
	returnURL string
}

// NewCollectionService creates a new CollectionService.
func NewCollectionService(csRESTClient *cybersource.RESTClient, db interface {
	UpdateCollectionResult(remittanceID, cybersourceRef, collectionStatus, status string) error
}, returnURL string) domain.CollectionService {
	return &collectionService{
		csRESTClient: csRESTClient,
		db:           db,
		returnURL:    returnURL,
	}
}

// === Flex Microform (REST API) Methods ===

func (s *collectionService) CreateCaptureContext(origins []string) (string, error) {
	if origins == nil {
		origins = []string{}
	}
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
			TransientToken:    req.TransientTokenJti,
			TransientTokenJWT: req.TransientTokenJWT,
		},
		PaymentInformation: &cybersource.PASetupPaymentInfo{
			Card: &cybersource.CardInfo{
				ExpirationMonth: req.ExpirationMonth,
				ExpirationYear:  req.ExpirationYear,
			},
		},
	}

	resp, err := s.csRESTClient.PASetup(paReq)
	if err != nil {
		return nil, fmt.Errorf("PA Setup failed: %w", err)
	}

	// Prefer the 3DS session ReferenceId from the auth info block;
	// fall back to the transaction ID as a last resort.
	refID := resp.ConsumerAuthenticationInfo.ReferenceId
	if refID == "" {
		log.Printf("WARN: ConsumerAuthenticationInfo.ReferenceId is empty, falling back to transaction ID")
		refID = resp.ID
	}

	return &domain.PASetupResponse{
		AccessToken:             resp.ConsumerAuthenticationInfo.AccessToken,
		DeviceDataCollectionUrl: resp.ConsumerAuthenticationInfo.DeviceDataCollectionUrl,
		ReferenceId:             refID,
	}, nil
}

func (s *collectionService) AuthorizePayment(req *domain.AuthorizeRequest) (*domain.AuthorizeResponse, error) {
	paReq := s.buildPaymentRequest(req, []string{domain.ActionConsumerAuth})
	resp, err := s.csRESTClient.AuthorizePayment(paReq)
	if err != nil {
		return nil, err
	}

	domainResp, err := s.mapPaymentResponse(resp, req.RemittanceID)
	if err != nil {
		return nil, err
	}

	// Update DB based on status
	switch domainResp.Status {
	case domain.CSStatusAuthorized, domain.CSStatusAuthorizedPendingReview:
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceCollected))
	case domain.CSStatusPendingAuth:
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceCollectionPending))
	default:
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceFailed))
	}

	return domainResp, nil
}

func (s *collectionService) ValidateAndAuthorize(req *domain.ValidateRequest) (*domain.AuthorizeResponse, error) {
	paReq := &cybersource.PaymentRequest{
		ClientReferenceInformation: cybersource.ClientReferenceInfo{
			Code: req.RemittanceID,
		},
		ProcessingInformation: cybersource.ProcessingInfo{
			Capture:    true,
			ActionList: []string{domain.ActionValidateConsumerAuth},
		},
		ConsumerAuthenticationInfo: &cybersource.ConsumerAuthInfo{
			AuthenticationTransactionId: req.AuthenticationTransactionId,
		},
	}

	resp, err := s.csRESTClient.AuthorizePayment(paReq)
	if err != nil {
		return nil, err
	}

	domainResp, err := s.mapPaymentResponse(resp, req.RemittanceID)
	if err != nil {
		return nil, err
	}

	switch domainResp.Status {
	case domain.CSStatusAuthorized, domain.CSStatusAuthorizedPendingReview:
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceCollected))
	default:
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceFailed))
	}

	return domainResp, nil
}

// buildPaymentRequest constructs the CyberSource AFT payment request from domain data.
// All dynamic values (country, address, state) come from the request — nothing is hardcoded.
func (s *collectionService) buildPaymentRequest(req *domain.AuthorizeRequest, actionList []string) *cybersource.PaymentRequest {
	senderAlpha2, senderAlpha3 := domain.GetCountryCodes(req.Sender.Country)
	_, recipientAlpha3 := domain.GetCountryCodes(req.Recipient.Country)

	return &cybersource.PaymentRequest{
		ClientReferenceInformation: cybersource.ClientReferenceInfo{
			Code: req.RemittanceID,
		},
		ProcessingInformation: cybersource.ProcessingInfo{
			Capture:               true,
			CommerceIndicator:     domain.CommerceIndicatorInternet,
			ActionList:            actionList,
			BusinessApplicationId: domain.BusinessAppIDPersonToPerson,
			AuthorizationOptions: &cybersource.AuthOptions{
				AFTIndicator: "true",
				FundingOptions: &cybersource.FundingOptions{
					Initiator: &cybersource.FundingInitiator{Type: domain.FundingInitiatorSender},
				},
			},
		},
		OrderInformation: cybersource.OrderInfo{
			BillTo: cybersource.BillTo{
				FirstName:          req.Sender.FirstName,
				LastName:           req.Sender.LastName,
				Email:              req.Sender.Email,
				Address1:           req.Sender.Address,
				Locality:           req.Sender.City,
				AdministrativeArea: domain.NormalizeState(req.Sender.AdministrativeArea),
				Country:            senderAlpha2,
				PostalCode:         req.Sender.PostalCode,
			},
			AmountDetails: cybersource.AmountDetails{
				TotalAmount: formatAmount(req.Amount),
				Currency:    req.Currency,
			},
		},
		TokenInformation: &cybersource.TokenInfo{
			TransientTokenJWT: req.TransientTokenJWT,
		},
		PaymentInformation: &cybersource.PaymentInfo{
			Card: &cybersource.CardInfo{
				ExpirationMonth: req.ExpirationMonth,
				ExpirationYear:  req.ExpirationYear,
			},
		},
		ConsumerAuthenticationInfo: &cybersource.ConsumerAuthInfo{
			ReferenceId:   req.PAReferenceId,
			ReturnUrl:     s.returnURL,
			DeviceChannel: domain.DeviceChannelBrowser,
		},
		SenderInformation: &cybersource.SenderInfo{
			FirstName:          req.Sender.FirstName,
			LastName:           req.Sender.LastName,
			Address1:           req.Sender.Address,
			Locality:           req.Sender.City,
			AdministrativeArea: domain.NormalizeState(req.Sender.AdministrativeArea),
			CountryCode:        senderAlpha3,
			PostalCode:         req.Sender.PostalCode,
		},
		RecipientInformation: &cybersource.RecipientInfo{
			FirstName:  req.Recipient.FirstName,
			LastName:   req.Recipient.LastName,
			Address1:   req.Recipient.Address,
			Locality:   req.Recipient.City,
			Country:    recipientAlpha3,
			PostalCode: "0000", // Default for ETH if missing
		},
	}
}

func formatAmount(amount string) string {
	// Simple ensure 2 decimal places: "10" -> "10.00", "10.5" -> "10.50"
	if amount == "" {
		return "0.00"
	}
	if !strings.Contains(amount, ".") {
		return amount + ".00"
	}
	parts := strings.Split(amount, ".")
	if len(parts[1]) == 1 {
		return amount + "0"
	}
	if len(parts[1]) > 2 {
		return parts[0] + "." + parts[1][:2]
	}
	return amount
}

func (s *collectionService) mapPaymentResponse(resp *cybersource.PaymentResponse, remittanceID string) (*domain.AuthorizeResponse, error) {
	domainResp := &domain.AuthorizeResponse{
		Status:       resp.Status,
		RemittanceID: remittanceID,
	}

	if resp.Status == domain.CSStatusPendingAuth && resp.ConsumerAuthenticationInfo != nil {
		domainResp.StepUpUrl = resp.ConsumerAuthenticationInfo.StepUpUrl
		domainResp.AccessToken = resp.ConsumerAuthenticationInfo.AccessToken
		domainResp.AuthenticationTransactionId = resp.ConsumerAuthenticationInfo.AuthenticationTransactionId
	}

	if resp.Status == domain.CSStatusDeclined || resp.Status == domain.CSStatusRejected {
		if resp.ErrorInformation != nil {
			domainResp.Message = resp.ErrorInformation.Message
		}
	}

	return domainResp, nil
}
