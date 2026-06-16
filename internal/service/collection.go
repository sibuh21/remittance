package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"remittance-service/internal/cybersource"
	"remittance-service/internal/domain"

	"github.com/google/uuid"
)

// collectionService implements domain.CollectionService for CyberSource Flex Microform.
type collectionService struct {
	csRESTClient *cybersource.RESTClient
	db           interface {
		CreateTransaction(t *domain.Transaction) error
		UpdateCollectionResult(remittanceID, cybersourceRef, collectionStatus, status string) error
		UpdatePayoutResult(remittanceID, boaRef, payoutStatus, status string) error
		GetTransactionByRef(ref string) (*domain.Transaction, error)
		GetTransactionsBySender(email string, status string) ([]*domain.Transaction, error)
		GetTransactionsByReceiver(phone string, status string) ([]*domain.Transaction, error)
		SaveSenderCard(card *domain.SenderCard) error
		GetCardsBySenderEmail(email string) ([]*domain.SenderCard, error)
	}
	returnURL   string
	onCollected func(remittanceID string)
}

// NewCollectionService creates a new CollectionService.
func NewCollectionService(csRESTClient *cybersource.RESTClient, db interface {
	CreateTransaction(t *domain.Transaction) error
	UpdateCollectionResult(remittanceID, cybersourceRef, collectionStatus, status string) error
	UpdatePayoutResult(remittanceID, boaRef, payoutStatus, status string) error
	GetTransactionByRef(ref string) (*domain.Transaction, error)
	GetTransactionsBySender(email string, status string) ([]*domain.Transaction, error)
	GetTransactionsByReceiver(phone string, status string) ([]*domain.Transaction, error)
	SaveSenderCard(card *domain.SenderCard) error
	GetCardsBySenderEmail(email string) ([]*domain.SenderCard, error)
}, returnURL string, onCollected func(string)) domain.CollectionService {
	return &collectionService{
		csRESTClient: csRESTClient,
		db:           db,
		returnURL:    returnURL,
		onCollected:  onCollected,
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
	}

	if req.PermanentTokenID != "" {
		paReq.PaymentInformation = &cybersource.PASetupPaymentInfo{
			InstrumentIdentifier: &cybersource.TMSReference{ID: req.PermanentTokenID},
			Card: &cybersource.CardInfo{
				ExpirationMonth: req.ExpirationMonth,
				ExpirationYear:  req.ExpirationYear,
			},
		}
	} else {
		paReq.TokenInformation = &cybersource.PASetupTokenInfo{
			TransientToken:    req.TransientTokenJti,
			TransientTokenJWT: req.TransientTokenJWT,
		}
		paReq.PaymentInformation = &cybersource.PASetupPaymentInfo{
			Card: &cybersource.CardInfo{
				ExpirationMonth: req.ExpirationMonth,
				ExpirationYear:  req.ExpirationYear,
			},
		}
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
	// Step 1: Build the CyberSource REST request
	actionList := []string{domain.ActionAuthorize, domain.ActionConsumerAuth}
	
	// If we're validating a 3DS challenge, we must use VALIDATE_CONSUMER_AUTHENTICATION instead
	if req.AuthenticationTransactionId != "" {
		actionList = []string{domain.ActionAuthorize, domain.ActionValidateConsumerAuth}
	}

	paReq := s.buildPaymentRequest(req, actionList)
	resp, err := s.csRESTClient.AuthorizePayment(paReq)
	if err != nil {
		return nil, err
	}

	domainResp, err := s.mapPaymentResponse(resp, req.RemittanceID)
	if err != nil {
		return nil, err
	}

	log.Printf("DEBUG: AuthorizePayment request: %v", req)
	log.Printf("DEBUG: AuthorizePayment response: %v", resp)

	// Update DB based on status
	paymentToken := ""
	if resp.TokenInformation != nil && resp.TokenInformation.InstrumentIdentifier != nil {
		paymentToken = resp.TokenInformation.InstrumentIdentifier.ID
		domainResp.PaymentTokenID = paymentToken
	}

	// AUTO-SAVE CARD: Ensure tokens are saved even if transaction is flagged for review
	if paymentToken != "" && req.Sender.Email != "" && (domainResp.Status == domain.CSStatusAuthorized || domainResp.Status == domain.CSStatusAuthorizedPendingReview) {
		cardInfo := &domain.SenderCard{
			ID:              uuid.New().String(),
			SenderEmail:     req.Sender.Email,
			TokenID:         paymentToken,
			ExpirationMonth: req.ExpirationMonth,
			ExpirationYear:  req.ExpirationYear,
		}
		if resp.PaymentInformation != nil && resp.PaymentInformation.Card != nil {
			cardInfo.CardBIN = resp.PaymentInformation.Card.Bin
			cardInfo.CardSuffix = resp.PaymentInformation.Card.Suffix
			if cardInfo.CardSuffix == "" && req.TransientTokenJWT != "" {
				cardInfo.CardSuffix = extractSuffixFromJWT(req.TransientTokenJWT)
			}
			cardInfo.CardBrand = resp.PaymentInformation.Card.Type
		} else if req.TransientTokenJWT != "" {
			cardInfo.CardSuffix = extractSuffixFromJWT(req.TransientTokenJWT)
		}
		_ = s.db.SaveSenderCard(cardInfo)
	}

	switch domainResp.Status {
	case domain.CSStatusAuthorized:
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceCollected))

		if s.onCollected != nil {
			s.onCollected(req.RemittanceID)
		}
	case domain.CSStatusAuthorizedPendingReview:
		log.Printf("INFO: Remittance %s flagged for manual review", req.RemittanceID)
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceReviewPending))
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

	paymentToken := ""
	if resp.TokenInformation != nil && resp.TokenInformation.InstrumentIdentifier != nil {
		paymentToken = resp.TokenInformation.InstrumentIdentifier.ID
		domainResp.PaymentTokenID = paymentToken
	}

	// AUTO-SAVE CARD (Post-3DS): Ensure tokens are saved even if transaction is flagged for review
	t, _ := s.db.GetTransactionByRef(req.RemittanceID)
	if t != nil && t.SenderEmail != "" && paymentToken != "" && (domainResp.Status == domain.CSStatusAuthorized || domainResp.Status == domain.CSStatusAuthorizedPendingReview) {
		cardInfo := &domain.SenderCard{
			ID:          uuid.New().String(),
			SenderEmail: t.SenderEmail,
			TokenID:     paymentToken,
		}
		if resp.PaymentInformation != nil && resp.PaymentInformation.Card != nil {
			cardInfo.CardBIN = resp.PaymentInformation.Card.Bin
			cardInfo.CardSuffix = resp.PaymentInformation.Card.Suffix
			cardInfo.CardBrand = resp.PaymentInformation.Card.Type
		}
		_ = s.db.SaveSenderCard(cardInfo)
	}

	switch domainResp.Status {
	case domain.CSStatusAuthorized:
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceCollected))

		if s.onCollected != nil {
			s.onCollected(req.RemittanceID)
		}
	case domain.CSStatusAuthorizedPendingReview:
		log.Printf("INFO: Remittance %s flagged for manual review (post-3DS)", req.RemittanceID)
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceReviewPending))
	case domain.CSStatusPendingAuth:
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceCollectionPending))
	default:
		_ = s.db.UpdateCollectionResult(req.RemittanceID, resp.ID, domainResp.Status, string(domain.RemittanceFailed))
	}

	return domainResp, nil
}

// buildPaymentRequest constructs the CyberSource AFT payment request from domain data.
// All dynamic values (country, address, state) come from the request — nothing is hardcoded.
func (s *collectionService) ReviewPayment(remittanceID string, approve bool) error {
	log.Printf("INFO: Manual review update for remittance %s - Approved: %v", remittanceID, approve)

	// Fetch transaction to get the CyberSource reference for voiding if rejected
	t, _ := s.db.GetTransactionByRef(remittanceID)

	if !approve {
		// If rejected, we ideally reverse the authorization hold
		if t != nil && t.CybersourceRef != "" {
			_ = s.csRESTClient.ReverseAuthorization(t.CybersourceRef)
		}
		return s.db.UpdateCollectionResult(remittanceID, "", "REVIEW_REJECTED", string(domain.RemittanceFailed))
	}

	// Update to COLLECTED
	err := s.db.UpdateCollectionResult(remittanceID, "", "REVIEW_APPROVED", string(domain.RemittanceCollected))
	if err != nil {
		return err
	}

	// Trigger automatic payout
	if s.onCollected != nil {
		go s.onCollected(remittanceID)
	}

	return nil
}

func (s *collectionService) GetSenderCards(email string) ([]*domain.SenderCard, error) {
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	return s.db.GetCardsBySenderEmail(email)
}
func (s *collectionService) UpdateCollectionResult(remittanceID, cybersourceRef, collectionStatus, status string) error {
	return s.db.UpdateCollectionResult(remittanceID, cybersourceRef, collectionStatus, status)
}

func (s *collectionService) ProcessWebhook(n *domain.CyberSourceNotification) error {
	log.Printf("INFO: Received CyberSource Webhook - Ref: %s, Decision: %s", n.MerchantReferenceCode, n.Decision)

	if n.MerchantReferenceCode == "" {
		return fmt.Errorf("missing merchant_reference_code in webhook")
	}

	approve := strings.ToUpper(n.Decision) == "ACCEPT"
	return s.ReviewPayment(n.MerchantReferenceCode, approve)
}

func (s *collectionService) validatedState(country, state string) string {
	if country != "US" && country != "CA" {
		return "" // Avoid sending region as "State" for UK/ETH etc.
	}
	return domain.NormalizeState(state)
}

func (s *collectionService) buildPaymentRequest(req *domain.AuthorizeRequest, actionList []string) *cybersource.PaymentRequest {
	senderAlpha2, senderAlpha3 := domain.GetCountryCodes(req.Sender.Country)
	_, recipientAlpha3 := domain.GetCountryCodes(req.Recipient.Country)

	// Only create a new token if we're using a new card (not a permanent token)
	if req.PermanentTokenID == "" {
		actionList = append(actionList, domain.ActionTokenCreate)
	}

	creq := &cybersource.PaymentRequest{
		ClientReferenceInformation: cybersource.ClientReferenceInfo{
			Code: req.RemittanceID,
		},
		ProcessingInformation: cybersource.ProcessingInfo{
			Capture:               true,
			CommerceIndicator:     domain.CommerceIndicatorInternet,
			ActionList:            s.enrichActionList(actionList, req),
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
				FirstName:          strings.TrimSpace(req.Sender.FirstName),
				LastName:           strings.TrimSpace(req.Sender.LastName),
				Email:              strings.TrimSpace(req.Sender.Email),
				Address1:           strings.TrimSpace(req.Sender.Address),
				Locality:           strings.TrimSpace(req.Sender.City),
				AdministrativeArea: s.validatedState(senderAlpha2, req.Sender.AdministrativeArea),
				Country:            senderAlpha2,
				PostalCode:         strings.TrimSpace(req.Sender.PostalCode),
			},
			AmountDetails: cybersource.AmountDetails{
				TotalAmount: formatAmount(req.Amount),
				Currency:    req.Currency,
			},
		},
		DeviceInformation: &cybersource.DeviceInfo{
			FingerprintSessionId: req.FingerprintID,
			IPAddress:            req.IPAddress,
		},
		ConsumerAuthenticationInfo: &cybersource.ConsumerAuthInfo{
			ReferenceId:                 req.PAReferenceId,
			ReturnUrl:                   s.returnURL,
			DeviceChannel:               domain.DeviceChannelBrowser,
			AuthenticationTransactionId: req.AuthenticationTransactionId,
		},
		SenderInformation: &cybersource.SenderInfo{
			FirstName:          strings.TrimSpace(req.Sender.FirstName),
			LastName:           strings.TrimSpace(req.Sender.LastName),
			Address1:           strings.TrimSpace(req.Sender.Address),
			Locality:           strings.TrimSpace(req.Sender.City),
			AdministrativeArea: s.validatedState(senderAlpha2, req.Sender.AdministrativeArea),
			CountryCode:        senderAlpha3,
			PostalCode:         strings.TrimSpace(req.Sender.PostalCode),
		},
		RecipientInformation: &cybersource.RecipientInfo{
			FirstName:  strings.TrimSpace(req.Recipient.FirstName),
			LastName:   strings.TrimSpace(req.Recipient.LastName),
			Address1:   strings.TrimSpace(req.Recipient.Address),
			Locality:   strings.TrimSpace(req.Recipient.City),
			Country:    recipientAlpha3,
			PostalCode: "1000", // Default for ETH if missing
		},
	}

	// Token handling: saved card vs new card
	if req.PermanentTokenID != "" {
		// Use stored permanent token (Card-on-File)
		// CyberSource requires expiration even with instrumentIdentifier
		creq.PaymentInformation = &cybersource.PaymentInfo{
			InstrumentIdentifier: &cybersource.TMSReference{ID: req.PermanentTokenID},
			Card: &cybersource.CardInfo{
				ExpirationMonth: req.ExpirationMonth,
				ExpirationYear:  req.ExpirationYear,
			},
		}
		// Flag as stored credential for Card-on-File compliance
		creq.ProcessingInformation.AuthorizationOptions.Initiator = &cybersource.Initiator{
			Type:                 "customer",
			StoredCredentialUsed: "true",
		}
	} else {
		// Use transient token from Flex Microform
		creq.TokenInformation = &cybersource.TokenInfo{
			TransientTokenJWT: req.TransientTokenJWT,
		}
		creq.PaymentInformation = &cybersource.PaymentInfo{
			Card: &cybersource.CardInfo{
				ExpirationMonth: req.ExpirationMonth,
				ExpirationYear:  req.ExpirationYear,
			},
		}
		// Include ActionTokenTypes only for new cards
		creq.ProcessingInformation.ActionTokenTypes = []string{"paymentInstrument", "instrumentIdentifier"}
	}

	// Final JSON Debug
	jsonBytes, _ := json.MarshalIndent(creq, "", "  ")
	log.Printf("DEBUG: Final CyberSource Request JSON:\n%s", string(jsonBytes))

	return creq
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

func (s *collectionService) enrichActionList(base []string, req *domain.AuthorizeRequest) []string {
	if req.ShouldTokenize {
		base = append(base, domain.ActionTokenCreate)
	}
	return base
}

func (s *collectionService) buildTokenInfo(req *domain.AuthorizeRequest) *cybersource.TokenInfo {
	if req.PermanentTokenID != "" {
		return nil // No transient token needed if using permanent token
	}
	return &cybersource.TokenInfo{
		TransientTokenJWT: req.TransientTokenJWT,
	}
}

func (s *collectionService) buildPaymentInfo(req *domain.AuthorizeRequest) *cybersource.PaymentInfo {
	pi := &cybersource.PaymentInfo{}

	if req.PermanentTokenID != "" {
		pi.PaymentInstrument = &cybersource.TMSReference{ID: req.PermanentTokenID}
	} else {
		pi.Card = &cybersource.CardInfo{
			ExpirationMonth: req.ExpirationMonth,
			ExpirationYear:  req.ExpirationYear,
		}
	}
	return pi
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

// extractSuffixFromJWT parses the Flex Microform transient token to extract the last 4 digits (suffix)
// since CyberSource responses occasionally omit the suffix on AFT approvals.
func extractSuffixFromJWT(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var data struct {
		Data struct {
			Number string `json:"number"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &data); err == nil && len(data.Data.Number) > 4 {
		return data.Data.Number[len(data.Data.Number)-4:]
	}
	return ""
}
