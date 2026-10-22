package service

// import (
// 	"context"
// 	"encoding/base64"
// 	"encoding/json"
// 	"fmt"
// 	"testing"

// 	"paymentgateway/internal/constant/model/dto"
// 	"paymentgateway/internal/storage"
// 	"paymentgateway/platform"

// 	"dev.azure.com/2f-capital/go-packages/logit.git"
// 	"github.com/google/uuid"
// 	redis "github.com/redis/go-redis/v9"
// 	"github.com/shopspring/decimal"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// 	"go.uber.org/zap"
// )

// // ═══════════════════════════════════════════════════════════════════════════════
// // Mock implementations
// // ═══════════════════════════════════════════════════════════════════════════════

// // --- Mock RemittanceStorage --------------------------------------------------

// type mockRemittanceStorage struct {
// 	storage.RemittanceStorage

// 	remittances    map[string]*dto.Remittance
// 	cards          map[string]*dto.SenderCard // keyed by tokenID or cardID
// 	savedCards     []*dto.SenderCard
// 	collectionArgs []collectionResultArgs
// 	cardIDUpdates  []cardIDUpdateArgs
// }

// type collectionResultArgs struct {
// 	ID, CsTxnID, CsAuthID, CollectionStatus, Status string
// }

// type cardIDUpdateArgs struct {
// 	ID     uuid.UUID
// 	CardID string
// }

// func newMockStorage() *mockRemittanceStorage {
// 	return &mockRemittanceStorage{
// 		remittances: make(map[string]*dto.Remittance),
// 		cards:       make(map[string]*dto.SenderCard),
// 	}
// }

// func (m *mockRemittanceStorage) GetRemittanceByID(_ context.Context, id string) (*dto.Remittance, error) {
// 	r, ok := m.remittances[id]
// 	if !ok {
// 		return nil, fmt.Errorf("remittance %s not found", id)
// 	}
// 	return r, nil
// }

// func (m *mockRemittanceStorage) GetRemittanceByCSAuthenticationID(_ context.Context, authID string) (*dto.Remittance, error) {
// 	for _, r := range m.remittances {
// 		if r.CsAuthenticationTransactionID == authID {
// 			return r, nil
// 		}
// 	}
// 	return nil, fmt.Errorf("remittance with authID %s not found", authID)
// }

// func (m *mockRemittanceStorage) GetCardByToken(_ context.Context, tokenID string) (*dto.SenderCard, error) {
// 	c, ok := m.cards[tokenID]
// 	if !ok {
// 		return nil, fmt.Errorf("card with token %s not found", tokenID)
// 	}
// 	return c, nil
// }

// func (m *mockRemittanceStorage) GetCardByID(_ context.Context, id string) (*dto.SenderCard, error) {
// 	for _, c := range m.cards {
// 		if c.ID == id {
// 			return c, nil
// 		}
// 	}
// 	return nil, fmt.Errorf("card with id %s not found", id)
// }

// func (m *mockRemittanceStorage) SaveSenderCard(_ context.Context, card *dto.SenderCard) error {
// 	m.savedCards = append(m.savedCards, card)
// 	m.cards[card.TokenID] = card
// 	return nil
// }

// func (m *mockRemittanceStorage) UpdateCollectionResult(
// 	_ context.Context, id, csTxnID, csAuthID, collectionStatus, status string) error {
// 	m.collectionArgs = append(m.collectionArgs, collectionResultArgs{
// 		ID: id, CsTxnID: csTxnID, CsAuthID: csAuthID, CollectionStatus: collectionStatus, Status: status,
// 	})
// 	return nil
// }

// func (m *mockRemittanceStorage) UpdateRemittanceSenderCardID(_ context.Context, id uuid.UUID, cardID string) error {
// 	m.cardIDUpdates = append(m.cardIDUpdates, cardIDUpdateArgs{ID: id, CardID: cardID})
// 	return nil
// }

// func (m *mockRemittanceStorage) CreateRemittance(_ context.Context, txn *dto.Remittance) error {
// 	m.remittances[txn.ID.String()] = txn
// 	return nil
// }

// // --- Mock SSO ----------------------------------------------------------------

// type mockSSO struct {
// 	platform.SSO
// 	users map[uuid.UUID]*dto.UserInfo
// }

// func newMockSSO() *mockSSO {
// 	return &mockSSO{users: make(map[uuid.UUID]*dto.UserInfo)}
// }

// func (m *mockSSO) GetUserByID(_ context.Context, id uuid.UUID) (*dto.UserInfo, error) {
// 	u, ok := m.users[id]
// 	if !ok {
// 		return nil, fmt.Errorf("user %s not found", id)
// 	}
// 	return u, nil
// }

// func (m *mockSSO) GetUserByPhone(_ context.Context, phone string) (*dto.UserInfo, error) {
// 	for _, u := range m.users {
// 		if u.Phone == phone {
// 			return u, nil
// 		}
// 	}
// 	return nil, fmt.Errorf("user with phone %s not found", phone)
// }

// // --- Mock CyberSource --------------------------------------------------------

// type mockCybersource struct {
// 	platform.Cybersource

// 	captureContextResp string
// 	paSetupResp        *dto.PASetupResponse
// 	paymentResp        *dto.PaymentResponse
// 	paSetupCalls       []*dto.PASetupRequest
// 	authorizeCalls     []*dto.AFTRequest
// 	err                error
// }

// func (m *mockCybersource) CreateCaptureContext(_ context.Context, _ *dto.CaptureContextRequest) (string, error) {
// 	return m.captureContextResp, m.err
// }

// func (m *mockCybersource) PASetup(_ context.Context, req *dto.PASetupRequest) (*dto.PASetupResponse, error) {
// 	m.paSetupCalls = append(m.paSetupCalls, req)
// 	return m.paSetupResp, m.err
// }

// func (m *mockCybersource) AuthorizePayment(_ context.Context, req *dto.AFTRequest) (*dto.PaymentResponse, error) {
// 	m.authorizeCalls = append(m.authorizeCalls, req)
// 	return m.paymentResp, m.err
// }

// // ═══════════════════════════════════════════════════════════════════════════════
// // Test helper: build a fake JWT-like transient token
// // ═══════════════════════════════════════════════════════════════════════════════

// func buildFakeTransientToken(bin, maskedValue, expMonth, expYear, cardType string) string {
// 	payload := map[string]interface{}{
// 		"content": map[string]interface{}{
// 			"paymentInformation": map[string]interface{}{
// 				"card": map[string]interface{}{
// 					"number": map[string]interface{}{
// 						"maskedValue": maskedValue,
// 						"bin":         bin,
// 					},
// 					"expirationMonth": map[string]interface{}{
// 						"value": expMonth,
// 					},
// 					"expirationYear": map[string]interface{}{
// 						"value": expYear,
// 					},
// 					"type": map[string]interface{}{
// 						"value": cardType,
// 					},
// 				},
// 			},
// 		},
// 	}

// 	bytes, _ := json.Marshal(payload)
// 	encoded := base64.URLEncoding.EncodeToString(bytes)
// 	// JWT-like: header.payload.signature
// 	return "eyJhbGciOiJIUzI1NiJ9." + encoded + ".fake-signature"
// }

// // ═══════════════════════════════════════════════════════════════════════════════
// // Test helper: create the service under test
// // ═══════════════════════════════════════════════════════════════════════════════

// func newTestService(
// 	db *mockRemittanceStorage,
// 	sso *mockSSO,
// 	cs *mockCybersource,
// 	redisClient *redis.Client,
// ) *remittance {
// 	logger := logit.New(zap.NewNop(), logit.Options{})
// 	return &remittance{
// 		db:                db,
// 		sso:               sso,
// 		log:               logger,
// 		targetOrigins:     []string{"https://example.com"},
// 		redirectURL:       "https://example.com/callback",
// 		cybersourceClient: cs,
// 		redisClient:       redisClient,
// 	}
// }

// // ═══════════════════════════════════════════════════════════════════════════════
// // Test helper: seed a remittance in DB
// // ═══════════════════════════════════════════════════════════════════════════════

// func seedRemittance(db *mockRemittanceStorage, id uuid.UUID, status dto.RemittanceStatus) *dto.Remittance {
// 	txn := &dto.Remittance{
// 		ID:                 id,
// 		Status:             status,
// 		SenderUserID:       uuid.NewString(),
// 		SenderAddress:      "123 Main St",
// 		SenderCity:         "New York",
// 		SenderState:        "NY",
// 		SenderPostalCode:   "10001",
// 		SenderCountry:      "US",
// 		SourceAmount:       decimal.NewFromFloat(100.00),
// 		SourceCurrency:     "USD",
// 		ReceiverFirstName:  "Jane",
// 		ReceiverLastName:   "Doe",
// 		ReceiverPhone:      "+251911223344",
// 		ReceiverEmail:      "jane@example.com",
// 		ReceiverAddress:    "Bole, AA",
// 		ReceiverCity:       "Addis Ababa",
// 		ReceiverState:      "",
// 		ReceiverPostalCode: "1000",
// 		ReceiverCountry:    "Ethiopia",
// 		PayoutType:         dto.PayoutWithinBoA,
// 		AccountNumber:      "1234567890",
// 	}
// 	db.remittances[id.String()] = txn
// 	return txn
// }

// func seedSender(sso *mockSSO, userID string) *dto.UserInfo {
// 	id, _ := uuid.Parse(userID)
// 	info := &dto.UserInfo{
// 		Sub:       userID,
// 		FirstName: "John",
// 		LastName:  "Sender",
// 		Email:     "john@example.com",
// 		Phone:     "+1234567890",
// 	}
// 	sso.users[id] = info
// 	return info
// }

// // ═══════════════════════════════════════════════════════════════════════════════
// // PASetup Tests
// // ═══════════════════════════════════════════════════════════════════════════════

// func TestPASetup_WithTransientToken(t *testing.T) {
// 	db := newMockStorage()
// 	cs := &mockCybersource{
// 		paSetupResp: &dto.PASetupResponse{
// 			ID:     "pa-setup-id-123",
// 			Status: "COMPLETED",
// 			ConsumerAuthenticationInfo: dto.ConsumerAuthResponse{
// 				AccessToken:             "test-access-token",
// 				DeviceDataCollectionUrl: "https://cybersource.com/ddc",
// 				ReferenceId:             "pa-ref-id-456",
// 			},
// 		},
// 	}
// 	svc := newTestService(db, newMockSSO(), cs, nil)

// 	resp, err := svc.PASetup(context.Background(), &dto.RemittancePASetupRequest{
// 		ID:                "rem-id-123",
// 		TransientTokenJti: "transient-jti-value",
// 		TransientTokenJWT: "eyJhbGciOiJIUzI1NiJ9.eyJ0ZXN0IjoiZGF0YSJ9.sig",
// 	})

// 	require.NoError(t, err)
// 	assert.Equal(t, "test-access-token", resp.AccessToken)
// 	assert.Equal(t, "https://cybersource.com/ddc", resp.DeviceDataCollectionUrl)
// 	assert.Equal(t, "pa-ref-id-456", resp.ReferenceId)

// 	// Verify the CyberSource request used token info (not payment info)
// 	require.Len(t, cs.paSetupCalls, 1)
// 	paReq := cs.paSetupCalls[0]
// 	assert.Equal(t, "rem-id-123", paReq.ClientReferenceInformation.Code)
// 	assert.NotNil(t, paReq.TokenInformation)
// 	assert.Nil(t, paReq.PaymentInformation)
// 	assert.Equal(t, "transient-jti-value", paReq.TokenInformation.TransientToken)
// 	assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.eyJ0ZXN0IjoiZGF0YSJ9.sig", paReq.TokenInformation.TransientTokenJWT)
// }

// func TestPASetup_WithPermanentToken(t *testing.T) {
// 	db := newMockStorage()

// 	// Seed a saved card in DB
// 	card := &dto.SenderCard{
// 		ID:              uuid.NewString(),
// 		UserID:          uuid.NewString(),
// 		TokenID:         "permanent-token-abc",
// 		ExpirationMonth: "12",
// 		ExpirationYear:  "2027",
// 	}
// 	db.cards["permanent-token-abc"] = card

// 	cs := &mockCybersource{
// 		paSetupResp: &dto.PASetupResponse{
// 			ID:     "pa-setup-id-789",
// 			Status: "COMPLETED",
// 			ConsumerAuthenticationInfo: dto.ConsumerAuthResponse{
// 				AccessToken:             "perm-access-token",
// 				DeviceDataCollectionUrl: "https://cybersource.com/ddc",
// 				ReferenceId:             "",
// 			},
// 		},
// 	}
// 	svc := newTestService(db, newMockSSO(), cs, nil)

// 	resp, err := svc.PASetup(context.Background(), &dto.RemittancePASetupRequest{
// 		ID:               "rem-id-456",
// 		PermanentTokenID: "permanent-token-abc",
// 	})

// 	require.NoError(t, err)
// 	assert.Equal(t, "perm-access-token", resp.AccessToken)
// 	// When ReferenceId is empty, should fallback to response ID
// 	assert.Equal(t, "pa-setup-id-789", resp.ReferenceId)

// 	// Verify the CyberSource request used payment info (not token info)
// 	require.Len(t, cs.paSetupCalls, 1)
// 	paReq := cs.paSetupCalls[0]
// 	assert.Nil(t, paReq.TokenInformation)
// 	assert.NotNil(t, paReq.PaymentInformation)
// 	assert.Equal(t, "permanent-token-abc", paReq.PaymentInformation.InstrumentIdentifier.ID)
// 	assert.Equal(t, "12", paReq.PaymentInformation.Card.ExpirationMonth)
// 	assert.Equal(t, "2027", paReq.PaymentInformation.Card.ExpirationYear)
// }

// func TestPASetup_PermanentTokenNotFound(t *testing.T) {
// 	db := newMockStorage()
// 	cs := &mockCybersource{}
// 	svc := newTestService(db, newMockSSO(), cs, nil)

// 	resp, err := svc.PASetup(context.Background(), &dto.RemittancePASetupRequest{
// 		ID:               "rem-id-789",
// 		PermanentTokenID: "non-existent-token",
// 	})

// 	assert.Error(t, err)
// 	assert.Nil(t, resp)
// 	// Should not call CyberSource if card lookup fails
// 	assert.Empty(t, cs.paSetupCalls)
// }

// func TestPASetup_CybersourceError(t *testing.T) {
// 	db := newMockStorage()
// 	cs := &mockCybersource{
// 		err: fmt.Errorf("cybersource unavailable"),
// 	}
// 	svc := newTestService(db, newMockSSO(), cs, nil)

// 	resp, err := svc.PASetup(context.Background(), &dto.RemittancePASetupRequest{
// 		ID:                "rem-id-err",
// 		TransientTokenJti: "jti",
// 		TransientTokenJWT: "jwt",
// 	})

// 	assert.Error(t, err)
// 	assert.Nil(t, resp)
// 	assert.Contains(t, err.Error(), "cybersource unavailable")
// }

// // ═══════════════════════════════════════════════════════════════════════════════
// // AuthorizePayment Tests
// // ═══════════════════════════════════════════════════════════════════════════════

// func TestAuthorizePayment_WithTransientToken_Authorized(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	_ = seedSender(sso, txn.SenderUserID)

// 	transientToken := buildFakeTransientToken("411111", "XXXXXXXXXXXX1111", "12", "2029", "001")

// 	cs := &mockCybersource{
// 		paymentResp: &dto.PaymentResponse{
// 			ID:     "cs-txn-id-001",
// 			Status: dto.CSStatusAuthorized,
// 			ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{
// 				AuthenticationTransactionId: "auth-txn-123",
// 			},
// 			TokenInformation: &dto.TokenInfoResponse{
// 				InstrumentIdentifier: &dto.TMSTokenRef{ID: "new-permanent-token-xyz"},
// 			},
// 		},
// 	}
// 	svc := newTestService(db, sso, cs, nil)

// 	user := &dto.UserDetail{
// 		ID: uuid.New(),
// 		UserInfo: dto.UserInfo{
// 			Sub:       uuid.NewString(),
// 			FirstName: "John",
// 			LastName:  "Sender",
// 			Email:     "john@example.com",
// 			Phone:     "+1234567890",
// 		},
// 	}

// 	resp, err := svc.AuthorizePayment(context.Background(), user, &dto.AuthorizeRequest{
// 		ID:                remID.String(),
// 		TransientTokenJti: "jti-value",
// 		TransientTokenJWT: transientToken,
// 		IPAddress:         "1.2.3.4",
// 		PAReferenceId:     "pa-ref-id",
// 		FingerprintID:     "fp-session-id",
// 	})

// 	require.NoError(t, err)
// 	assert.Equal(t, dto.CSStatusAuthorized, resp.Status)
// 	assert.Equal(t, remID.String(), resp.RemittanceID)
// 	assert.Equal(t, "new-permanent-token-xyz", resp.PaymentTokenID)

// 	// Verify AFT request was built correctly:
// 	require.Len(t, cs.authorizeCalls, 1)
// 	aftReq := cs.authorizeCalls[0]
// 	assert.Equal(t, remID.String(), aftReq.ClientReferenceInformation.Code)
// 	assert.True(t, aftReq.ProcessingInformation.Capture)
// 	assert.Contains(t, aftReq.ProcessingInformation.ActionList, dto.ActionAuthorize)
// 	assert.Contains(t, aftReq.ProcessingInformation.ActionList, dto.ActionConsumerAuth)
// 	assert.Contains(t, aftReq.ProcessingInformation.ActionList, dto.ActionTokenCreate)
// 	assert.Equal(t, []string{"paymentInstrument", "instrumentIdentifier"}, aftReq.ProcessingInformation.ActionTokenTypes)
// 	assert.NotNil(t, aftReq.TokenInformation)
// 	assert.Nil(t, aftReq.PaymentInformation)

// 	// Card should be saved for future use
// 	require.Len(t, db.savedCards, 1)
// 	savedCard := db.savedCards[0]
// 	assert.Equal(t, "new-permanent-token-xyz", savedCard.TokenID)
// 	assert.Equal(t, "411111", savedCard.CardBIN)
// 	assert.Equal(t, "1111", savedCard.CardSuffix)
// 	assert.Equal(t, "12", savedCard.ExpirationMonth)
// 	assert.Equal(t, "2029", savedCard.ExpirationYear)

// 	// UpdateCollectionResult should be called with AUTHORIZED status
// 	require.Len(t, db.collectionArgs, 1)
// 	assert.Equal(t, remID.String(), db.collectionArgs[0].ID)
// 	assert.Equal(t, dto.CSStatusAuthorized, db.collectionArgs[0].CollectionStatus)
// 	assert.Equal(t, string(dto.RemittanceCollected), db.collectionArgs[0].Status)
// }

// func TestAuthorizePayment_WithPermanentToken_Authorized(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	_ = seedSender(sso, txn.SenderUserID)

// 	// Seed an existing card
// 	card := &dto.SenderCard{
// 		ID:              uuid.NewString(),
// 		UserID:          txn.SenderUserID,
// 		TokenID:         "existing-perm-token",
// 		ExpirationMonth: "06",
// 		ExpirationYear:  "2028",
// 	}
// 	db.cards["existing-perm-token"] = card

// 	cs := &mockCybersource{
// 		paymentResp: &dto.PaymentResponse{
// 			ID:     "cs-txn-id-002",
// 			Status: dto.CSStatusAuthorized,
// 			ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{
// 				AuthenticationTransactionId: "auth-txn-456",
// 			},
// 		},
// 	}
// 	svc := newTestService(db, sso, cs, nil)

// 	user := &dto.UserDetail{
// 		ID: uuid.New(),
// 		UserInfo: dto.UserInfo{
// 			Sub:       txn.SenderUserID,
// 			FirstName: "John",
// 			LastName:  "Sender",
// 			Email:     "john@example.com",
// 		},
// 	}

// 	resp, err := svc.AuthorizePayment(context.Background(), user, &dto.AuthorizeRequest{
// 		ID:               remID.String(),
// 		PermanentTokenID: "existing-perm-token",
// 		PAReferenceId:    "pa-ref-id",
// 	})

// 	require.NoError(t, err)
// 	assert.Equal(t, dto.CSStatusAuthorized, resp.Status)

// 	// Verify AFT request used payment info (not token info)
// 	require.Len(t, cs.authorizeCalls, 1)
// 	aftReq := cs.authorizeCalls[0]
// 	assert.NotNil(t, aftReq.PaymentInformation)
// 	assert.Nil(t, aftReq.TokenInformation)
// 	assert.Equal(t, "existing-perm-token", aftReq.PaymentInformation.InstrumentIdentifier.ID)
// 	assert.Equal(t, "06", aftReq.PaymentInformation.Card.ExpirationMonth)
// 	assert.Equal(t, "2028", aftReq.PaymentInformation.Card.ExpirationYear)

// 	// Should NOT include TOKEN_CREATE action
// 	assert.NotContains(t, aftReq.ProcessingInformation.ActionList, dto.ActionTokenCreate)

// 	// Should have initiator info for stored credential
// 	assert.NotNil(t, aftReq.ProcessingInformation.AuthorizationOptions.Initiator)
// 	assert.Equal(t, "customer", aftReq.ProcessingInformation.AuthorizationOptions.Initiator.Type)
// 	assert.Equal(t, "true", aftReq.ProcessingInformation.AuthorizationOptions.Initiator.StoredCredentialUsed)

// 	// Card should NOT be saved again (already exists)
// 	assert.Empty(t, db.savedCards)

// 	// Sender card ID should be updated on the remittance
// 	require.Len(t, db.cardIDUpdates, 1)
// 	assert.Equal(t, remID, db.cardIDUpdates[0].ID)
// 	assert.Equal(t, card.ID, db.cardIDUpdates[0].CardID)
// }

// func TestAuthorizePayment_PendingAuthentication(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	_ = seedSender(sso, txn.SenderUserID)

// 	transientToken := buildFakeTransientToken("411111", "XXXXXXXXXXXX1111", "12", "2029", "001")

// 	cs := &mockCybersource{
// 		paymentResp: &dto.PaymentResponse{
// 			ID:     "cs-txn-id-003",
// 			Status: dto.CSStatusPendingAuth,
// 			ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{
// 				StepUpUrl:                   "https://cybersource.com/3ds-stepup",
// 				AccessToken:                 "3ds-access-token",
// 				AuthenticationTransactionId: "3ds-auth-txn-id",
// 			},
// 		},
// 	}
// 	svc := newTestService(db, sso, cs, nil)

// 	user := &dto.UserDetail{
// 		ID: uuid.New(),
// 		UserInfo: dto.UserInfo{
// 			FirstName: "John",
// 			LastName:  "Sender",
// 			Email:     "john@example.com",
// 		},
// 	}

// 	resp, err := svc.AuthorizePayment(context.Background(), user, &dto.AuthorizeRequest{
// 		ID:                remID.String(),
// 		TransientTokenJti: "jti",
// 		TransientTokenJWT: transientToken,
// 		PAReferenceId:     "pa-ref",
// 	})

// 	require.NoError(t, err)
// 	assert.Equal(t, dto.CSStatusPendingAuth, resp.Status)
// 	assert.Equal(t, "https://cybersource.com/3ds-stepup", resp.StepUpUrl)
// 	assert.Equal(t, "3ds-access-token", resp.AccessToken)
// 	assert.Equal(t, "3ds-auth-txn-id", resp.AuthenticationTransactionId)

// 	// DB should be updated with COLLECTION_PENDING status
// 	require.Len(t, db.collectionArgs, 1)
// 	assert.Equal(t, string(dto.RemittanceCollectionPending), db.collectionArgs[0].Status)
// }

// func TestAuthorizePayment_Declined(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	_ = seedSender(sso, txn.SenderUserID)

// 	transientToken := buildFakeTransientToken("411111", "XXXXXXXXXXXX1111", "12", "2029", "001")

// 	cs := &mockCybersource{
// 		paymentResp: &dto.PaymentResponse{
// 			ID:                         "cs-txn-id-004",
// 			Status:                     dto.CSStatusDeclined,
// 			ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{},
// 			ErrorInformation: &dto.ErrorInfo{
// 				Reason:  "DECLINED",
// 				Message: "Insufficient funds",
// 			},
// 		},
// 	}
// 	svc := newTestService(db, sso, cs, nil)

// 	user := &dto.UserDetail{
// 		ID: uuid.New(),
// 		UserInfo: dto.UserInfo{
// 			FirstName: "John",
// 			LastName:  "Sender",
// 			Email:     "john@example.com",
// 		},
// 	}

// 	resp, err := svc.AuthorizePayment(context.Background(), user, &dto.AuthorizeRequest{
// 		ID:                remID.String(),
// 		TransientTokenJti: "jti",
// 		TransientTokenJWT: transientToken,
// 		PAReferenceId:     "pa-ref",
// 	})

// 	require.NoError(t, err) // method returns response, not error for declined
// 	assert.Equal(t, dto.CSStatusDeclined, resp.Status)
// 	assert.Equal(t, "Insufficient funds", resp.Message)

// 	// DB should be updated with FAILED status
// 	require.Len(t, db.collectionArgs, 1)
// 	assert.Equal(t, string(dto.RemittanceFailed), db.collectionArgs[0].Status)

// 	// No card should be saved for a declined transaction
// 	assert.Empty(t, db.savedCards)
// }

// func TestAuthorizePayment_ReviewPending(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	_ = seedSender(sso, txn.SenderUserID)

// 	transientToken := buildFakeTransientToken("411111", "XXXXXXXXXXXX1111", "12", "2029", "001")

// 	cs := &mockCybersource{
// 		paymentResp: &dto.PaymentResponse{
// 			ID:     "cs-txn-id-005",
// 			Status: dto.CSStatusAuthorizedPendingReview,
// 			ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{
// 				AuthenticationTransactionId: "review-auth-txn",
// 			},
// 			TokenInformation: &dto.TokenInfoResponse{
// 				InstrumentIdentifier: &dto.TMSTokenRef{ID: "review-perm-token"},
// 			},
// 		},
// 	}
// 	svc := newTestService(db, sso, cs, nil)

// 	user := &dto.UserDetail{
// 		ID: uuid.New(),
// 		UserInfo: dto.UserInfo{
// 			Sub:       uuid.NewString(),
// 			FirstName: "John",
// 			LastName:  "Sender",
// 			Email:     "john@example.com",
// 		},
// 	}

// 	resp, err := svc.AuthorizePayment(context.Background(), user, &dto.AuthorizeRequest{
// 		ID:                remID.String(),
// 		TransientTokenJti: "jti",
// 		TransientTokenJWT: transientToken,
// 		PAReferenceId:     "pa-ref",
// 	})

// 	require.NoError(t, err)
// 	assert.Equal(t, dto.CSStatusAuthorizedPendingReview, resp.Status)

// 	// DB should be updated with REVIEW_PENDING status
// 	require.Len(t, db.collectionArgs, 1)
// 	assert.Equal(t, string(dto.RemittanceReviewPending), db.collectionArgs[0].Status)
// }

// func TestAuthorizePayment_RemittanceNotFound(t *testing.T) {
// 	db := newMockStorage()
// 	cs := &mockCybersource{}
// 	svc := newTestService(db, newMockSSO(), cs, nil)

// 	user := &dto.UserDetail{
// 		ID: uuid.New(),
// 		UserInfo: dto.UserInfo{
// 			FirstName: "John",
// 			LastName:  "Sender",
// 		},
// 	}

// 	resp, err := svc.AuthorizePayment(context.Background(), user, &dto.AuthorizeRequest{
// 		ID:                uuid.NewString(),
// 		TransientTokenJti: "jti",
// 		TransientTokenJWT: "some.jwt.value",
// 		PAReferenceId:     "pa-ref",
// 	})

// 	assert.Error(t, err)
// 	assert.Nil(t, resp)
// 	assert.Empty(t, cs.authorizeCalls)
// }

// func TestAuthorizePayment_InvalidTransientToken(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	_ = seedSender(sso, txn.SenderUserID)

// 	cs := &mockCybersource{}
// 	svc := newTestService(db, sso, cs, nil)

// 	user := &dto.UserDetail{
// 		ID: uuid.New(),
// 		UserInfo: dto.UserInfo{
// 			FirstName: "John",
// 			LastName:  "Sender",
// 		},
// 	}

// 	resp, err := svc.AuthorizePayment(context.Background(), user, &dto.AuthorizeRequest{
// 		ID:                remID.String(),
// 		TransientTokenJti: "jti",
// 		TransientTokenJWT: "invalid-not-a-jwt",
// 		PAReferenceId:     "pa-ref",
// 	})

// 	assert.Error(t, err)
// 	assert.Nil(t, resp)
// 	// Should not reach CyberSource
// 	assert.Empty(t, cs.authorizeCalls)
// }

// func TestAuthorizePayment_PermanentTokenNotFound(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	_ = seedSender(sso, txn.SenderUserID)

// 	cs := &mockCybersource{}
// 	svc := newTestService(db, sso, cs, nil)

// 	user := &dto.UserDetail{
// 		ID: uuid.New(),
// 		UserInfo: dto.UserInfo{
// 			FirstName: "John",
// 			LastName:  "Sender",
// 		},
// 	}

// 	resp, err := svc.AuthorizePayment(context.Background(), user, &dto.AuthorizeRequest{
// 		ID:               remID.String(),
// 		PermanentTokenID: "non-existent-token",
// 		PAReferenceId:    "pa-ref",
// 	})

// 	assert.Error(t, err)
// 	assert.Nil(t, resp)
// 	assert.Empty(t, cs.authorizeCalls)
// }

// func TestAuthorizePayment_BusinessApplicationID(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	_ = seedSender(sso, txn.SenderUserID)
// 	transientToken := buildFakeTransientToken("411111", "XXXXXXXXXXXX1111", "12", "2029", "001")

// 	cs := &mockCybersource{
// 		paymentResp: &dto.PaymentResponse{
// 			ID:     "cs-txn-id",
// 			Status: dto.CSStatusAuthorized,
// 			ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{
// 				AuthenticationTransactionId: "auth-txn",
// 			},
// 			TokenInformation: &dto.TokenInfoResponse{
// 				InstrumentIdentifier: &dto.TMSTokenRef{ID: "new-token"},
// 			},
// 		},
// 	}
// 	svc := newTestService(db, sso, cs, nil)

// 	user := &dto.UserDetail{
// 		ID: uuid.New(),
// 		UserInfo: dto.UserInfo{
// 			Sub:       uuid.NewString(),
// 			FirstName: "John",
// 			LastName:  "Sender",
// 			Email:     "john@example.com",
// 		},
// 	}

// 	_, err := svc.AuthorizePayment(context.Background(), user, &dto.AuthorizeRequest{
// 		ID:                remID.String(),
// 		TransientTokenJti: "jti",
// 		TransientTokenJWT: transientToken,
// 		PAReferenceId:     "pa-ref",
// 	})

// 	require.NoError(t, err)
// 	require.Len(t, cs.authorizeCalls, 1)

// 	// Verify AFT-specific fields
// 	aftReq := cs.authorizeCalls[0]
// 	assert.Equal(t, dto.BusinessAppIDPersonToPerson, aftReq.ProcessingInformation.BusinessApplicationId)
// 	assert.Equal(t, "true", aftReq.ProcessingInformation.AuthorizationOptions.AFTIndicator)
// 	assert.NotNil(t, aftReq.SenderInformation)
// 	assert.NotNil(t, aftReq.RecipientInformation)
// }

// // ═══════════════════════════════════════════════════════════════════════════════
// // CheckIfAuthorized Tests
// // ═══════════════════════════════════════════════════════════════════════════════

// func TestCheckIfAuthorized_WithExistingCard_Authorized(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	sender := seedSender(sso, txn.SenderUserID)

// 	// Assign a card to the remittance
// 	cardID := uuid.NewString()
// 	card := &dto.SenderCard{
// 		ID:              cardID,
// 		UserID:          sender.Sub,
// 		TokenID:         "saved-token-for-validate",
// 		ExpirationMonth: "03",
// 		ExpirationYear:  "2026",
// 	}
// 	db.cards[card.ID] = card // keyed by ID for GetCardByID
// 	txn.SenderCardID = cardID

// 	cs := &mockCybersource{
// 		paymentResp: &dto.PaymentResponse{
// 			ID:     "cs-validate-txn-001",
// 			Status: dto.CSStatusAuthorized,
// 			ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{
// 				AuthenticationTransactionId: "validate-auth-txn",
// 			},
// 		},
// 	}

// 	// Create a mini-redis for this test (we'll use nil since the card flow doesn't need redis)
// 	svc := newTestService(db, sso, cs, nil)

// 	resp, err := svc.CheckIfAuthorized(context.Background(), &dto.ValidateRequest{
// 		ID:                          remID.String(),
// 		AuthenticationTransactionId: "3ds-challenge-auth-txn",
// 	})

// 	require.NoError(t, err)
// 	assert.Equal(t, dto.CSStatusAuthorized, resp.Status)
// 	assert.Equal(t, remID.String(), resp.RemittanceID)

// 	// Verify the AFT request used payment info with stored credential
// 	require.Len(t, cs.authorizeCalls, 1)
// 	aftReq := cs.authorizeCalls[0]
// 	assert.Contains(t, aftReq.ProcessingInformation.ActionList, dto.ActionAuthorize)
// 	assert.Contains(t, aftReq.ProcessingInformation.ActionList, dto.ActionValidateConsumerAuth)
// 	assert.NotNil(t, aftReq.PaymentInformation)
// 	assert.Equal(t, "saved-token-for-validate", aftReq.PaymentInformation.InstrumentIdentifier.ID)
// 	assert.Equal(t, "03", aftReq.PaymentInformation.Card.ExpirationMonth)
// 	assert.Equal(t, "2026", aftReq.PaymentInformation.Card.ExpirationYear)
// 	assert.NotNil(t, aftReq.ProcessingInformation.AuthorizationOptions.Initiator)

// 	// Collection result should be updated
// 	require.Len(t, db.collectionArgs, 1)
// 	assert.Equal(t, string(dto.RemittanceCollected), db.collectionArgs[0].Status)
// }

// func TestCheckIfAuthorized_RemittanceNotFound(t *testing.T) {
// 	db := newMockStorage()
// 	cs := &mockCybersource{}
// 	svc := newTestService(db, newMockSSO(), cs, nil)

// 	resp, err := svc.CheckIfAuthorized(context.Background(), &dto.ValidateRequest{
// 		ID:                          uuid.NewString(),
// 		AuthenticationTransactionId: "some-auth-txn",
// 	})

// 	assert.Error(t, err)
// 	assert.Nil(t, resp)
// }

// func TestCheckIfAuthorized_SenderNotFound(t *testing.T) {
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	// Deliberately NOT seeding the sender in SSO
// 	_ = txn

// 	cs := &mockCybersource{}
// 	svc := newTestService(db, sso, cs, nil)

// 	resp, err := svc.CheckIfAuthorized(context.Background(), &dto.ValidateRequest{
// 		ID:                          remID.String(),
// 		AuthenticationTransactionId: "some-auth-txn",
// 	})

// 	assert.Error(t, err)
// 	assert.Nil(t, resp)
// }

// func TestCheckIfAuthorized_ReceiverFallback(t *testing.T) {
// 	// When ReceiverFirstName is empty, CheckIfAuthorized fetches from SSO
// 	db := newMockStorage()
// 	sso := newMockSSO()
// 	remID := uuid.New()
// 	txn := seedRemittance(db, remID, dto.RemittanceCollectionPending)
// 	txn.ReceiverFirstName = "" // trigger SSO lookup
// 	txn.ReceiverPhone = "+251911000000"
// 	sender := seedSender(sso, txn.SenderUserID)

// 	// Seed receiver in SSO
// 	receiverID := uuid.New()
// 	sso.users[receiverID] = &dto.UserInfo{
// 		Sub:       receiverID.String(),
// 		FirstName: "ReceiverFirst",
// 		LastName:  "ReceiverLast",
// 		Phone:     "+251911000000",
// 	}

// 	// The card must exist so we don't need redis
// 	cardID := uuid.NewString()
// 	card := &dto.SenderCard{
// 		ID:              cardID,
// 		UserID:          sender.Sub,
// 		TokenID:         "card-for-receiver-test",
// 		ExpirationMonth: "01",
// 		ExpirationYear:  "2030",
// 	}
// 	db.cards[card.ID] = card
// 	txn.SenderCardID = cardID

// 	cs := &mockCybersource{
// 		paymentResp: &dto.PaymentResponse{
// 			ID:     "cs-recv-test",
// 			Status: dto.CSStatusAuthorized,
// 			ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{
// 				AuthenticationTransactionId: "recv-auth-txn",
// 			},
// 		},
// 	}
// 	svc := newTestService(db, sso, cs, nil)

// 	resp, err := svc.CheckIfAuthorized(context.Background(), &dto.ValidateRequest{
// 		ID:                          remID.String(),
// 		AuthenticationTransactionId: "recv-auth-txn",
// 	})

// 	require.NoError(t, err)
// 	assert.Equal(t, dto.CSStatusAuthorized, resp.Status)

// 	// Verify recipient info uses the SSO-fetched receiver name
// 	require.Len(t, cs.authorizeCalls, 1)
// 	aftReq := cs.authorizeCalls[0]
// 	assert.Equal(t, "ReceiverFirst", aftReq.RecipientInformation.FirstName)
// 	assert.Equal(t, "ReceiverLast", aftReq.RecipientInformation.LastName)
// }

// // ═══════════════════════════════════════════════════════════════════════════════
// // mapPaymentResponse Tests
// // ═══════════════════════════════════════════════════════════════════════════════

// func TestMapPaymentResponse_Authorized(t *testing.T) {
// 	svc := &remittance{}
// 	resp := svc.mapPaymentResponse(&dto.PaymentResponse{
// 		ID:     "resp-id",
// 		Status: dto.CSStatusAuthorized,
// 	}, "rem-123")

// 	assert.Equal(t, dto.CSStatusAuthorized, resp.Status)
// 	assert.Equal(t, "rem-123", resp.RemittanceID)
// 	assert.Empty(t, resp.StepUpUrl)
// }

// func TestMapPaymentResponse_PendingAuth(t *testing.T) {
// 	svc := &remittance{}
// 	resp := svc.mapPaymentResponse(&dto.PaymentResponse{
// 		ID:     "resp-id",
// 		Status: dto.CSStatusPendingAuth,
// 		ConsumerAuthenticationInfo: &dto.ConsumerAuthResponse{
// 			StepUpUrl:                   "https://stepup.example.com",
// 			AccessToken:                 "3ds-token",
// 			AuthenticationTransactionId: "3ds-auth-id",
// 		},
// 	}, "rem-456")

// 	assert.Equal(t, dto.CSStatusPendingAuth, resp.Status)
// 	assert.Equal(t, "https://stepup.example.com", resp.StepUpUrl)
// 	assert.Equal(t, "3ds-token", resp.AccessToken)
// 	assert.Equal(t, "3ds-auth-id", resp.AuthenticationTransactionId)
// }

// func TestMapPaymentResponse_Declined(t *testing.T) {
// 	svc := &remittance{}
// 	resp := svc.mapPaymentResponse(&dto.PaymentResponse{
// 		ID:     "resp-id",
// 		Status: dto.CSStatusDeclined,
// 		ErrorInformation: &dto.ErrorInfo{
// 			Reason:  "INSUFFICIENT_FUNDS",
// 			Message: "Card has insufficient funds",
// 		},
// 	}, "rem-789")

// 	assert.Equal(t, dto.CSStatusDeclined, resp.Status)
// 	assert.Equal(t, "Card has insufficient funds", resp.Message)
// }

// // ═══════════════════════════════════════════════════════════════════════════════
// // DecodeTransientToken Tests
// // ═══════════════════════════════════════════════════════════════════════════════

// func TestDecodeTransientToken_Valid(t *testing.T) {
// 	token := buildFakeTransientToken("411111", "XXXXXXXXXXXX1111", "12", "2029", "001")
// 	cardInfo, err := DecodeTransientToken(token)

// 	require.NoError(t, err)
// 	assert.Equal(t, "411111", cardInfo.Content.PaymentInformation.Card.Number.Bin)
// 	assert.Equal(t, "XXXXXXXXXXXX1111", cardInfo.Content.PaymentInformation.Card.Number.MaskedValue)
// 	assert.Equal(t, "12", cardInfo.Content.PaymentInformation.Card.ExpirationMonth.Value)
// 	assert.Equal(t, "2029", cardInfo.Content.PaymentInformation.Card.ExpirationYear.Value)
// 	assert.Equal(t, "001", cardInfo.Content.PaymentInformation.Card.Type.Value)
// }

// func TestDecodeTransientToken_InvalidFormat(t *testing.T) {
// 	_, err := DecodeTransientToken("not-a-jwt")
// 	assert.Error(t, err)
// 	assert.Contains(t, err.Error(), "invalid transient token format")
// }

// func TestDecodeTransientToken_InvalidBase64(t *testing.T) {
// 	_, err := DecodeTransientToken("header.!!!invalid-base64!!!.sig")
// 	assert.Error(t, err)
// }
