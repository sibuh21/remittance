package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"remittance-service/internal/cybersource"
	"remittance-service/internal/database"
	"remittance-service/internal/domain"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
)

// collectionService implements domain.CollectionService for CyberSource Flex Microform.
type collectionService struct {
	csRESTClient *cybersource.RESTClient
	redisClient  *redis.Client
	db           database.Queries
	returnURL    string
	merchantID   string
	onCollected  func(id uuid.UUID)
}

// NewCollectionService creates a new CollectionService.
func NewCollectionService(csRESTClient *cybersource.RESTClient, rc *redis.Client, db database.Queries, returnURL string, merchantID string, onCollected func(uuid.UUID)) domain.CollectionService {
	return &collectionService{
		csRESTClient: csRESTClient,
		redisClient:  rc,
		db:           db,
		returnURL:    returnURL,
		merchantID:   merchantID,
		onCollected:  onCollected,
	}
}

// === Flex Microform (REST API) Methods ===
func (s *collectionService) GetConfig() (map[string]string, error) {
	return map[string]string{
		"merchant_id": s.merchantID,
	}, nil
}

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
	// store trasient token to redis
	if req.TransientTokenJWT != "" {
		err := s.redisClient.Set("transient_token_"+req.ID, req.TransientTokenJWT, 15*60*time.Second).Err()
		if err != nil {
			return nil, fmt.Errorf("failed to save transient token to redis")
		}
	}
	paReq := &cybersource.PASetupRequest{
		ClientReferenceInformation: cybersource.ClientReferenceInfo{
			Code: req.ID,
		},
	}

	if req.PermanentTokenID != "" {
		card, err := s.db.GetCardByToken(context.Background(), sql.NullString{String: req.PermanentTokenID, Valid: true})
		if err != nil {
			return nil, err
		}
		paReq.PaymentInformation = &cybersource.PASetupPaymentInfo{
			InstrumentIdentifier: &cybersource.TMSReference{ID: req.PermanentTokenID},
			Card: &cybersource.CardInfo{
				ExpirationMonth: card.ExpirationMonth,
				ExpirationYear:  card.ExpirationYear,
			},
		}
	} else {
		paReq.TokenInformation = &cybersource.PASetupTokenInfo{
			TransientToken:    req.TransientTokenJti,
			TransientTokenJWT: req.TransientTokenJWT,
		}
	}

	resp, err := s.csRESTClient.PASetup(paReq)
	if err != nil {
		return nil, fmt.Errorf("PA Setup failed: %w", err)
	}

	// Prefer the 3DS session ReferenceId from the auth info block;
	// fall back to the remittance ID as a last resort.
	refID := resp.ConsumerAuthenticationInfo.ReferenceId
	if refID == "" {
		log.Printf("WARN: ConsumerAuthenticationInfo.ReferenceId is empty, falling back to remittance ID")
		refID = resp.ID
	}

	return &domain.PASetupResponse{
		ID:                      req.ID,
		AccessToken:             resp.ConsumerAuthenticationInfo.AccessToken,
		DeviceDataCollectionUrl: resp.ConsumerAuthenticationInfo.DeviceDataCollectionUrl,
		ReferenceId:             refID,
	}, nil
}

func (s *collectionService) AuthorizePayment(req *domain.AuthorizeRequest) (*domain.AuthorizeResponse, error) {
	remID, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid remittance ID")
	}
	rem, err := s.db.GetRemittanceByID(context.Background(), remID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remittance: %v", err)
	}

	user, err := s.db.GetUserByID(context.Background(),
		database.GetUserByIDParams{ID: uuid.MustParse(rem.SenderUserID)})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %v", err)
	}

	actionList := []string{domain.ActionAuthorize, domain.ActionConsumerAuth}

	// Only create a new token if we're using a new card (not a permanent token)
	if req.PermanentTokenID == "" {
		actionList = append(actionList, domain.ActionTokenCreate)
	}

	var deviceInfo *cybersource.DeviceInfo
	if req.BrowserInfo != nil {
		deviceInfo = &cybersource.DeviceInfo{
			FingerprintSessionId: req.FingerprintID,
			IPAddress: func() string {
				if req.IPAddress == "::1" || req.IPAddress == "127.0.0.1" {
					return "8.8.8.8" // Use a public IP for testing if localhost
				}
				return req.IPAddress
			}(),
			UserAgentBrowserValue:        req.BrowserInfo.UserAgentBrowserValue,
			HttpBrowserColorDepth:        req.BrowserInfo.HttpBrowserColorDepth,
			HttpBrowserScreenWidth:       req.BrowserInfo.HttpBrowserScreenWidth,
			HttpBrowserScreenHeight:      req.BrowserInfo.HttpBrowserScreenHeight,
			HttpBrowserLanguage:          req.BrowserInfo.HttpBrowserLanguage,
			HttpBrowserTimeDifference:    req.BrowserInfo.HttpBrowserTimeDifference,
			HttpAcceptBrowserValue:       req.BrowserInfo.HttpAcceptBrowserValue,
			HttpBrowserJavaEnabled:       req.BrowserInfo.HttpBrowserJavaEnabled,
			HttpBrowserJavaScriptEnabled: req.BrowserInfo.HttpBrowserJavaScriptEnabled,
		}
	} else {
		deviceInfo = &cybersource.DeviceInfo{
			FingerprintSessionId: req.FingerprintID,
			IPAddress: func() string {
				if req.IPAddress == "::1" || req.IPAddress == "127.0.0.1" {
					return "8.8.8.8"
				}
				return req.IPAddress
			}(),
		}
	}

	creq := &cybersource.PaymentRequest{
		ClientReferenceInformation: cybersource.ClientReferenceInfo{
			Code: req.ID,
		},
		ProcessingInformation: cybersource.ProcessingInfo{
			Capture:               true,
			CommerceIndicator:     domain.CommerceIndicatorInternet,
			ActionList:            actionList,
			BusinessApplicationId: domain.BusinessAppIDPersonToPerson,
			ActionTokenTypes: func() []string {
				if req.PermanentTokenID == "" {
					return []string{"instrumentIdentifier", "paymentInstrument"}
				}
				return nil
			}(),
			AuthorizationOptions: &cybersource.AuthOptions{
				AFTIndicator: "true",
				FundingOptions: &cybersource.FundingOptions{
					Initiator: &cybersource.FundingInitiator{Type: domain.FundingInitiatorSender},
				},
			},
		},
		OrderInformation: cybersource.OrderInfo{
			BillTo: &cybersource.BillTo{
				FirstName:          user.FirstName,
				LastName:           user.LastName,
				Email:              user.Email,
				Address1:           rem.SenderAddress,
				Locality:           strings.TrimSpace(rem.SenderCity),
				AdministrativeArea: s.validatedState(rem.SenderCountry, rem.SenderState),
				Country: func() string {
					senderAlpha2, _ := domain.GetCountryCodes(rem.SenderCountry)
					return senderAlpha2
				}(),
				PostalCode: strings.TrimSpace(rem.SenderPostalCode),
			},
			AmountDetails: cybersource.AmountDetails{
				TotalAmount: formatAmount(rem.SourceAmount.String()),
				Currency:    rem.SourceCurrency,
			},
		},
		DeviceInformation: deviceInfo,
		ConsumerAuthenticationInfo: &cybersource.ConsumerAuthInfo{
			ReferenceId:                 req.PAReferenceId,
			ReturnUrl:                   s.returnURL,
			DeviceChannel:               domain.DeviceChannelBrowser,
			AuthenticationTransactionId: req.AuthenticationTransactionId,
		},
		SenderInformation: &cybersource.SenderInfo{
			FirstName:          user.FirstName,
			LastName:           user.LastName,
			Address1:           rem.SenderAddress,
			Locality:           rem.SenderCity,
			AdministrativeArea: s.validatedState(rem.SenderCountry, rem.SenderState),
			PostalCode:         rem.SenderPostalCode,
			CountryCode: func() string {
				alpha2, _ := domain.GetCountryCodes(rem.SenderCountry)
				return alpha2
			}(),
		},
		RecipientInformation: &cybersource.RecipientInfo{
			FirstName: strings.TrimSpace(rem.ReceiverFirstName),
			LastName:  strings.TrimSpace(rem.ReceiverLastName),
			Address1:  strings.TrimSpace(rem.ReceiverAddress),
			Locality:  strings.TrimSpace(rem.ReceiverCity),
			Country: func() string {
				alpha2, _ := domain.GetCountryCodes(rem.ReceiverCountry)
				return alpha2
			}(),
			PostalCode: rem.ReceiverPostalCode, // Default for ETH if missing
		},
	}

	// Token handling: saved card vs new card
	if req.PermanentTokenID != "" {
		// Use stored permanent token (Card-on-File)
		// get card by token id
		card, err := s.db.GetCardByToken(context.Background(), sql.NullString{String: req.PermanentTokenID, Valid: true})
		if err != nil {
			return nil, err
		}
		creq.PaymentInformation = &cybersource.PaymentInfo{
			InstrumentIdentifier: &cybersource.TMSReference{ID: req.PermanentTokenID},
			Card: &cybersource.CardInfo{
				ExpirationMonth: card.ExpirationMonth,
				ExpirationYear:  card.ExpirationYear,
			},
		}
		// Flag as stored credential for Card-on-File compliance - Line 584
		creq.ProcessingInformation.AuthorizationOptions.Initiator = &cybersource.Initiator{
			Type:                 "customer",
			StoredCredentialUsed: "true",
		}
	} else {
		// Use transient token from Flex Microform
		creq.TokenInformation = &cybersource.TokenInfo{
			TransientTokenJWT: req.TransientTokenJWT,
		}
		creq.PaymentInformation = &cybersource.PaymentInfo{}
		// Include ActionTokenTypes only for new cards
		creq.ProcessingInformation.ActionTokenTypes = []string{"paymentInstrument", "instrumentIdentifier"}
	}
	byt, err := json.Marshal(creq)
	fmt.Println("auth request:==>", string(byt))
	// paReq := s.buildPaymentRequest(req, actionList)
	resp, err := s.csRESTClient.AuthorizePayment(creq)
	if err != nil {
		return nil, err
	}

	domainResp, err := s.mapPaymentResponse(resp, req.ID)
	if err != nil {
		return nil, err
	}

	cardMetadata := &domain.CardInfo{}
	if req.TransientTokenJWT != "" {
		cardMetadata, err = domain.DecodeTransientToken(req.TransientTokenJWT)
		if err != nil {
			log.Printf("failed to decode transient token: %v", err)
			return nil, err
		}
	}

	paymentToken := ""
	if resp.TokenInformation != nil {
		if resp.TokenInformation.InstrumentIdentifier != nil {
			paymentToken = resp.TokenInformation.InstrumentIdentifier.ID
		} else if resp.TokenInformation.PaymentInstrument != nil {
			paymentToken = resp.TokenInformation.PaymentInstrument.ID
		} else if resp.TokenInformation.Customer != nil {
			paymentToken = resp.TokenInformation.Customer.ID
		}
		domainResp.PaymentTokenID = paymentToken
	}
	savedCard := database.SenderCard{}
	if domainResp.Status == domain.CSStatusAuthorized && req.TransientTokenJWT != "" {
		savedCard, err = s.db.SaveSenderCard(context.Background(), database.SaveSenderCardParams{
			ID:              uuid.New(),
			UserID:          user.ID.String(),
			TokenID:         sql.NullString{String: paymentToken, Valid: true},
			CardBin:         sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.Number.Bin, Valid: true},
			CardSuffix:      sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.Number.MaskedValue[len(cardMetadata.Content.PaymentInformation.Card.Number.MaskedValue)-4:], Valid: true},
			CardBrand:       sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.Type.Value, Valid: true},
			ExpirationMonth: sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.ExpirationMonth.Value, Valid: true},
			ExpirationYear:  sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.ExpirationYear.Value, Valid: true},
		})
		if err != nil {
			log.Printf("failed to save sender card: %v", err)
			return nil, fmt.Errorf("failed to save sender card: %v", err)
		}
	}

	authID := ""
	transactionID := ""

	newStatus := string(domain.RemittanceFailed)
	switch domainResp.Status {
	case domain.CSStatusAuthorized:
		newStatus = string(domain.RemittanceCollected)
		authID = resp.ConsumerAuthenticationInfo.AuthenticationTransactionId
		transactionID = resp.ID
	case domain.CSStatusAuthorizedPendingReview:
		newStatus = string(domain.RemittanceReviewPending)
	case domain.CSStatusPendingAuth:
		newStatus = string(domain.RemittanceCollectionPending)
	}

	_, err = s.db.UpdateRemittance(context.Background(), database.UpdateRemittanceParams{
		CsTransactionID:               sql.NullString{String: transactionID, Valid: transactionID != ""},
		CsAuthenticationTransactionID: sql.NullString{String: authID, Valid: authID != ""},
		CollectionStatus:              sql.NullString{String: newStatus, Valid: true},
		Status:                        newStatus,
		SenderCardID:                  sql.NullString{String: savedCard.ID.String(), Valid: savedCard.ID.String() != ""},
		ID:                            remID,
	})

	if domainResp.Status == domain.CSStatusAuthorized && s.onCollected != nil {
		s.onCollected(remID)
	}

	return domainResp, nil
}

func (s *collectionService) CheckIfAuthorized(req *domain.ValidateRequest) (*domain.AuthorizeResponse, error) {
	// 1. Fetch remittance first to get amount/currency needed for validation
	remID, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid remittance ID: %v", err)
	}
	t, err := s.db.GetRemittanceByID(context.Background(), remID)
	if err != nil {
		return nil, fmt.Errorf("remittance not found for validation: %w", err)
	}

	user, err := s.db.GetUserByID(context.Background(), database.GetUserByIDParams{ID: uuid.MustParse(t.SenderUserID)})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user for validation: %v", err)
	}

	var cardInfo *database.SenderCard
	paymentTokenID := ""
	if t.SenderCardID.Valid {
		c, err := s.db.GetCardByID(context.Background(), t.SenderCardID.UUID)
		if err == nil {
			cardInfo = &c
			if c.TokenID.Valid {
				paymentTokenID = c.TokenID.String
			}
		}
	}
	transientTokenJWT := ""
	if paymentTokenID == "" {
		tt, err := s.redisClient.Get("transient_token_" + req.ID).Result()
		if err == nil {
			transientTokenJWT = tt
		}
	}

	senderAlpha2 := func() string {
		a2, _ := domain.GetCountryCodes(t.SenderCountry)
		return a2
	}()

	paReq := &cybersource.PaymentRequest{
		ClientReferenceInformation: cybersource.ClientReferenceInfo{
			Code: req.ID,
		},
		ProcessingInformation: cybersource.ProcessingInfo{
			Capture: true,
			ActionList: func() []string {
				list := []string{domain.ActionAuthorize, domain.ActionValidateConsumerAuth}
				if paymentTokenID == "" {
					list = append(list, domain.ActionTokenCreate)
				}
				return list
			}(),
			ActionTokenTypes: func() []string {
				if paymentTokenID == "" {
					return []string{"instrumentIdentifier", "paymentInstrument"}
				}
				return nil
			}(),
			BusinessApplicationId: domain.BusinessAppIDPersonToPerson,
			AuthorizationOptions: &cybersource.AuthOptions{
				AFTIndicator: "true",
				FundingOptions: &cybersource.FundingOptions{
					Initiator: &cybersource.FundingInitiator{Type: domain.FundingInitiatorSender},
				},
				Initiator: func() *cybersource.Initiator {
					if paymentTokenID != "" {
						return &cybersource.Initiator{
							Type:                 "customer",
							StoredCredentialUsed: "true",
						}
					}
					return nil
				}(),
			},
		},
		OrderInformation: cybersource.OrderInfo{
			BillTo: &cybersource.BillTo{
				FirstName:          user.FirstName,
				LastName:           user.LastName,
				Email:              user.Email,
				Address1:           t.SenderAddress,
				Locality:           t.SenderCity,
				AdministrativeArea: t.SenderState,
				PostalCode:         t.SenderPostalCode,
				Country:            senderAlpha2,
			},
			AmountDetails: cybersource.AmountDetails{
				TotalAmount: formatAmount(t.SourceAmount.String()),
				Currency:    t.SourceCurrency,
			},
		},
	}

	if paymentTokenID != "" {
		paReq.PaymentInformation = &cybersource.PaymentInfo{
			InstrumentIdentifier: &cybersource.TMSReference{ID: paymentTokenID},
		}

		if cardInfo != nil {
			paReq.PaymentInformation.Card = &cybersource.CardInfo{
				ExpirationMonth: cardInfo.ExpirationMonth.String,
				ExpirationYear:  cardInfo.ExpirationYear.String,
			}
		}
	} else if transientTokenJWT != "" {
		paReq.TokenInformation = &cybersource.TokenInfo{
			TransientTokenJWT: transientTokenJWT,
		}
	}

	paReq.ConsumerAuthenticationInfo = &cybersource.ConsumerAuthInfo{
		AuthenticationTransactionId: req.AuthenticationTransactionId,
	}

	paReq.SenderInformation = &cybersource.SenderInfo{
		FirstName:          user.FirstName,
		LastName:           user.LastName,
		Address1:           t.SenderAddress,
		Locality:           t.SenderCity,
		AdministrativeArea: t.SenderState,
		CountryCode:        senderAlpha2,
		PostalCode:         t.SenderPostalCode,
	}
	paReq.RecipientInformation = &cybersource.RecipientInfo{
		FirstName:  t.ReceiverFirstName,
		LastName:   t.ReceiverLastName,
		Address1:   t.ReceiverAddress,
		Country:    "ET",
		PostalCode: "1000",
	}

	jsonData, _ := json.Marshal(paReq)
	log.Printf("DEBUG: CheckIfAuthorized - Request payload: %s", string(jsonData))
	resp, err := s.csRESTClient.AuthorizePayment(paReq)
	if err != nil {
		return nil, err
	}

	domainResp, err := s.mapPaymentResponse(resp, req.ID)
	if err != nil {
		return nil, err
	}

	paymentToken := paymentTokenID
	if resp.TokenInformation != nil {
		if resp.TokenInformation.InstrumentIdentifier != nil {
			paymentToken = resp.TokenInformation.InstrumentIdentifier.ID
		} else if resp.TokenInformation.PaymentInstrument != nil {
			paymentToken = resp.TokenInformation.PaymentInstrument.ID
		} else if resp.TokenInformation.Customer != nil {
			paymentToken = resp.TokenInformation.Customer.ID
		}
		domainResp.PaymentTokenID = paymentToken
	}

	// AUTO-SAVE CARD (Post-3DS): Update the card record if we found a permanent token.
	if user.Email != "" && paymentToken != "" && cardInfo == nil {
		transientTokenJWT, err := s.redisClient.Get("transient_token_" + req.ID).Result()
		var bin, suffix, expM, expY, brand sql.NullString
		if err == nil {
			if cardMetadata, err := domain.DecodeTransientToken(transientTokenJWT); err == nil && cardMetadata != nil && cardMetadata.Content.PaymentInformation.Card.Number.Bin != "" {
				bin = sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.Number.Bin, Valid: true}
				suffix = sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.Number.MaskedValue[len(cardMetadata.Content.PaymentInformation.Card.Number.MaskedValue)-4:], Valid: true}
				expM = sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.ExpirationMonth.Value, Valid: true}
				expY = sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.ExpirationYear.Value, Valid: true}
				brand = sql.NullString{String: cardMetadata.Content.PaymentInformation.Card.Type.Value, Valid: true}
			}
		}

		_, _ = s.db.SaveSenderCard(context.Background(), database.SaveSenderCardParams{
			ID:              uuid.New(),
			UserID:          user.ID.String(),
			TokenID:         sql.NullString{String: paymentToken, Valid: true},
			CardBin:         bin,
			CardSuffix:      suffix,
			CardBrand:       brand,
			ExpirationMonth: expM,
			ExpirationYear:  expY,
		})
	}

	authID := req.AuthenticationTransactionId
	if resp.ConsumerAuthenticationInfo != nil && resp.ConsumerAuthenticationInfo.AuthenticationTransactionId != "" {
		authID = resp.ConsumerAuthenticationInfo.AuthenticationTransactionId
	}

	newStatus := string(domain.RemittanceFailed)
	switch domainResp.Status {
	case domain.CSStatusAuthorized:
		newStatus = string(domain.RemittanceCollected)
	case domain.CSStatusAuthorizedPendingReview:
		newStatus = string(domain.RemittanceReviewPending)
	case domain.CSStatusPendingAuth:
		newStatus = string(domain.RemittanceCollectionPending)
	}

	_, _ = s.db.UpdateRemittance(context.Background(), database.UpdateRemittanceParams{
		CsTransactionID:               sql.NullString{String: resp.ID, Valid: resp.ID != ""},
		CsAuthenticationTransactionID: sql.NullString{String: authID, Valid: authID != ""},
		CollectionStatus:              sql.NullString{String: newStatus, Valid: true},
		Status:                        newStatus,
		ID:                            remID,
	})

	if domainResp.Status == domain.CSStatusAuthorized && s.onCollected != nil {
		s.onCollected(remID)
	}

	return domainResp, nil
}

// buildPaymentRequest constructs the CyberSource AFT payment request from domain data.
// All dynamic values (country, address, state) come from the request — nothing is hardcoded.
func (s *collectionService) ReviewPayment(id string, approve bool) error {
	log.Printf("INFO: Manual review update for remittance %s - Approved: %v", id, approve)

	remID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid id: %v", err)
	}

	t, err := s.db.GetRemittanceByID(context.Background(), remID)
	if err != nil && !approve {
		_, _ = s.db.UpdateRemittance(context.Background(), database.UpdateRemittanceParams{
			CollectionStatus: sql.NullString{String: "REVIEW_REJECTED", Valid: true},
			Status:           string(domain.RemittanceFailed),
			ID:               remID,
		})
		return err
	}

	if !approve {
		if err == nil && t.CsTransactionID.Valid && t.CsTransactionID.String != "" {
			_ = s.csRESTClient.ReverseAuthorization(t.CsTransactionID.String)
		}
		_, err = s.db.UpdateRemittance(context.Background(), database.UpdateRemittanceParams{
			CollectionStatus: sql.NullString{String: "REVIEW_REJECTED", Valid: true},
			Status:           string(domain.RemittanceFailed),
			ID:               remID,
		})
		return err
	}

	_, err = s.db.UpdateRemittance(context.Background(), database.UpdateRemittanceParams{
		CollectionStatus: sql.NullString{String: "REVIEW_APPROVED", Valid: true},
		Status:           string(domain.RemittanceCollected),
		ID:               remID,
	})
	if err != nil {
		return err
	}

	if s.onCollected != nil {
		go s.onCollected(remID)
	}

	return nil
}

func (s *collectionService) GetSenderCards(email string) ([]*domain.SenderCard, error) {

	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	// get user by em
	user, err := s.db.GetUserByEmail(context.Background(), email)
	if err != nil {
		return nil, err
	}
	cards, err := s.db.GetCardsByUserID(context.Background(), user.ID.String())
	if err != nil {
		return nil, err
	}
	var senderCards []*domain.SenderCard
	for _, card := range cards {
		senderCards = append(senderCards, &domain.SenderCard{
			ID:              card.ID.String(),
			UserID:          card.UserID,
			TokenID:         card.TokenID.String,
			CardBIN:         card.CardBin.String,
			CardSuffix:      card.CardSuffix.String,
			CardBrand:       card.CardBrand.String,
			ExpirationMonth: card.ExpirationMonth,
			ExpirationYear:  card.ExpirationYear,
		})
	}
	return senderCards, nil
}
func (s *collectionService) UpdateCollectionResult(id, csTransactionID, csAuthTransactionID, collectionStatus, status, paymentTokenID, transientTokenJWT string) error {
	_, err := s.db.UpdateRemittance(context.Background(), database.UpdateRemittanceParams{
		CsTransactionID:               sql.NullString{String: csTransactionID, Valid: csTransactionID != ""},
		CsAuthenticationTransactionID: sql.NullString{String: csAuthTransactionID, Valid: csAuthTransactionID != ""},
		CollectionStatus:              sql.NullString{String: collectionStatus, Valid: collectionStatus != ""},
		Status:                        status,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *collectionService) UpdatePayoutResult(id, boaRef, payoutStatus, status string) error {
	_, err := s.db.UpdatePayoutResult(context.Background(), database.UpdatePayoutResultParams{
		IDOrRef:      id,
		BoaReference: sql.NullString{String: boaRef, Valid: boaRef != ""},
		PayoutStatus: sql.NullString{String: payoutStatus, Valid: payoutStatus != ""},
		Status:       status,
	})
	return err
}

func (s *collectionService) GetRemittanceByID(id string) (*domain.Remittance, error) {
	t, err := s.db.GetRemittanceByID(context.Background(), uuid.MustParse(id))
	if err != nil {
		return nil, err
	}
	// Simplified to satisfy interface minimally (if used)
	return &domain.Remittance{
		ID:              t.ID.String(),
		Status:          domain.RemittanceStatus(t.Status),
		CsTransactionID: t.CsTransactionID.String,
	}, nil
}

func (s *collectionService) GetRemittanceByCSAuthenticationID(authID string) (*domain.Remittance, error) {
	t, err := s.db.GetRemittanceByCSAuthenticationID(context.Background(), sql.NullString{String: authID, Valid: true})
	if err != nil {
		return nil, err
	}
	return &domain.Remittance{
		ID:              t.ID.String(),
		Status:          domain.RemittanceStatus(t.Status),
		CsTransactionID: t.CsTransactionID.String,
	}, nil
}

func (s *collectionService) GetRemittancesBySender(email string, status string) ([]*domain.Remittance, error) {
	return nil, nil // not fully mapped but satisfies interface compilation
}

func (s *collectionService) GetRemittancesByReceiver(phone string, status string) ([]*domain.Remittance, error) {
	return nil, nil // not fully mapped
}

func (s *collectionService) ProcessWebhook(n *domain.CyberSourceNotification) error {
	log.Printf("INFO: Received CyberSource Webhook - Ref: %s, Decision: %s", n.MerchantReferenceCode, n.Decision)

	if n.MerchantReferenceCode == "" {
		return fmt.Errorf("missing merchant_reference_code in webhook")
	}

	approve := strings.ToUpper(n.Decision) == "ACCEPT"
	return s.ReviewPayment(n.MerchantReferenceCode, approve)
}

func (s *collectionService) ProcessCaseManagementWebhook(payload *domain.CaseManagementWebhookPayload) error {
	log.Printf("INFO: Received Case Management Webhook - Ref: %s, Event: %s", payload.Payload.MerchantReferenceCode, payload.EventType)

	if payload.Payload.MerchantReferenceCode == "" {
		return fmt.Errorf("missing merchantReferenceCode in case management webhook")
	}

	approve := payload.EventType == "risk.casemanagement.decision.accept"
	return s.ReviewPayment(payload.Payload.MerchantReferenceCode, approve)
}

func (s *collectionService) ProcessTSUWebhook(payload *domain.TSUWebhookPayload) error {
	tokenID := payload.PaymentInstrument.ID
	if tokenID == "" {
		tokenID = payload.Token.ID
	}
	if tokenID == "" {
		tokenID = payload.InstrumentIdentifier.ID
	}

	if tokenID == "" {
		log.Printf("WARN: TSU webhook received without a valid token or instrument ID")
		return nil
	}

	state := payload.PaymentInstrument.State
	if state == "" {
		state = payload.Token.State
	}
	if state == "" {
		state = payload.InstrumentIdentifier.State
	}
	state = strings.ToUpper(state)

	expMonth := payload.PaymentInstrument.Card.ExpirationMonth
	if expMonth == "" {
		expMonth = payload.Token.Card.ExpirationMonth
	}

	expYear := payload.PaymentInstrument.Card.ExpirationYear
	if expYear == "" {
		expYear = payload.Token.Card.ExpirationYear
	}

	log.Printf("INFO: Processing TSU Webhook for token: %s, state: %s", tokenID, state)

	if state == "CLOSED" || state == "SUSPENDED" || state == "DELETED" {
		_, err := s.db.DeleteSenderCard(context.Background(), sql.NullString{String: tokenID, Valid: true})
		if err != nil {
			log.Printf("ERROR: Failed to delete sender card: %v", err)
		}
	}

	if expMonth != "" && expYear != "" {
		_, err := s.db.UpdateSenderCardExpiration(context.Background(), database.UpdateSenderCardExpirationParams{
			TokenID:         sql.NullString{String: tokenID, Valid: true},
			ExpirationMonth: sql.NullString{String: expMonth, Valid: true},
			ExpirationYear:  sql.NullString{String: expYear, Valid: true},
		})
		if err != nil {
			log.Printf("ERROR: Failed to update sender card expiration: %v", err)
		}
	}

	return nil
}

func (s *collectionService) validatedState(country, state string) string {
	alpha2, _ := domain.GetCountryCodes(country)
	if alpha2 != "US" && alpha2 != "CA" {
		return ""
	}
	return domain.NormalizeState(state)
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

func (s *collectionService) mapPaymentResponse(resp *cybersource.PaymentResponse, id string) (*domain.AuthorizeResponse, error) {
	domainResp := &domain.AuthorizeResponse{
		Status: resp.Status,
		ID:     id,
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

// extractCardDetailsFromJWT parses the Flex Microform transient token to extract card metadata
func extractCardDetailsFromJWT(jwtToken string) (suffix, month, year string) {
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return "", "", ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", ""
	}
	var data struct {
		Data struct {
			Number          string `json:"number"`
			ExpirationMonth string `json:"expirationMonth"`
			ExpirationYear  string `json:"expirationYear"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", "", ""
	}

	suffix = data.Data.Number
	if len(suffix) > 4 {
		suffix = suffix[len(suffix)-4:]
	}
	return suffix, data.Data.ExpirationMonth, data.Data.ExpirationYear
}
