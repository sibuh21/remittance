package domain

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/labstack/echo/v4"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Configuration
// ═══════════════════════════════════════════════════════════════════════════════

// Config holds the entire application configuration.
type Config struct {
	Server struct {
		Port           string        `mapstructure:"port"`
		ReadTimeout    time.Duration `mapstructure:"read_timeout"`
		WriteTimeout   time.Duration `mapstructure:"write_timeout"`
		IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
		RequestTimeout time.Duration `mapstructure:"request_timeout"`
	} `mapstructure:"server"`

	CyberSource struct {
		MerchantID    string   `mapstructure:"merchant_id"`
		KeyID         string   `mapstructure:"key_id"`
		SharedSecret  string   `mapstructure:"shared_secret"`
		BaseURL       string   `mapstructure:"base_url"`
		TargetOrigins []string `mapstructure:"target_origins"`
		ReturnURL     string   `mapstructure:"return_url"`

		// Legacy (for backward compatibility if needed)
		AccessKey   string `mapstructure:"access_key"`
		ProfileID   string `mapstructure:"profile_id"`
		SecretKey   string `mapstructure:"secret_key"`
		CheckoutURL string `mapstructure:"checkout_url"`
	} `mapstructure:"cybersource"`

	Database struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		DBName   string `mapstructure:"dbname"`
		SSLMode  string `mapstructure:"sslmode"`
	} `mapstructure:"database"`

	BoA struct {
		BaseURL      string `mapstructure:"base_url"`
		TokenURL     string `mapstructure:"token_url"`
		ClientID     string `mapstructure:"client_id"`
		ClientSecret string `mapstructure:"client_secret"`
		RefreshToken string `mapstructure:"refresh_token"`
		APIKey       string `mapstructure:"api_key"`
		MockMode     bool   `mapstructure:"mock_mode"`
	} `mapstructure:"boa"`

	CORS struct {
		AllowedOrigins []string `mapstructure:"allowed_origins"`
	} `mapstructure:"cors"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// CyberSource (Inbound Collection) Models
// ═══════════════════════════════════════════════════════════════════════════════

// CheckoutRequest is the request body from the frontend to initiate collection.
type CheckoutRequest struct {
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	Locale        string `json:"locale"`
	PaymentMethod string `json:"payment_method"`

	// Remittance enrichment
	SenderName      string `json:"sender_name"`
	SenderEmail     string `json:"sender_email"`
	SenderAddress   string `json:"sender_address"`
	SenderCity      string `json:"sender_city"`
	SenderState     string `json:"sender_state"`
	SenderPostal    string `json:"sender_postal"`
	SenderCountry   string `json:"sender_country"`
	ReceiverName    string `json:"receiver_name"`
	ReceiverAddress string `json:"receiver_address"`
	ReceiverCity    string `json:"receiver_city"`
	ReceiverCountry string `json:"receiver_country"`
	ReceiverPhone   string `json:"receiver_phone"`
	PayoutType      string `json:"payout_type"`     // WITHIN_BOA, OTHER_BANK, TELEBIRR, MPESA
	AccountNumber   string `json:"account_number"`  // for bank transfers
	BankID          string `json:"bank_id"`         // for other bank transfers
	TargetCurrency  string `json:"target_currency"` // e.g. "ETB"
}

func (r CheckoutRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Amount, validation.Required.Error("amount is required")),
		validation.Field(&r.Currency,
			validation.Required.Error("currency is required"),
			validation.In("USD", "EUR", "GBP", "CAD", "AUD").Error("unsupported source currency"),
		),
		validation.Field(&r.SenderName, validation.Required.Error("sender name is required")),
		validation.Field(&r.SenderAddress, validation.Required.Error("sender address is required")),
		validation.Field(&r.SenderCity, validation.Required.Error("sender city is required")),
		validation.Field(&r.SenderState, validation.Required.Error("sender state/province is required")),
		validation.Field(&r.SenderPostal, validation.Required.Error("sender postal code is required")),
		validation.Field(&r.SenderCountry, validation.Required.Error("sender country is required")),
		validation.Field(&r.ReceiverName, validation.Required.Error("receiver name is required")),
		validation.Field(&r.ReceiverAddress, validation.Required.Error("receiver address is required")),
		validation.Field(&r.ReceiverCity, validation.Required.Error("receiver city is required")),
		validation.Field(&r.ReceiverCountry, validation.Required.Error("receiver country is required")),
		validation.Field(&r.PayoutType,
			validation.Required.Error("payout_type is required"),
			validation.In("WITHIN_BOA", "OTHER_BANK", "TELEBIRR", "MPESA").Error("invalid payout type"),
		),
	)
}

// PaymentResult is the response after processing a CyberSource webhook.
type PaymentResult struct {
	CsTransactionID string        `json:"cs_transaction_id"`
	Status          PaymentStatus `json:"status"`
	Amount          string        `json:"amount"`
	Currency        string        `json:"currency"`
	AuthCode        string        `json:"auth_code,omitempty"`
	ReferenceNumber string        `json:"reference_number,omitempty"`
	ID              string        `json:"id,omitempty"`
	Message         string        `json:"message,omitempty"`
	ProcessedAt     time.Time     `json:"processed_at"`
}

// CyberSourceNotification represents the automated callback from CyberSource Decision Manager.
type CyberSourceNotification struct {
	MerchantReferenceCode string `json:"merchant_reference_code" form:"merchant_reference_code"`
	Decision              string `json:"decision" form:"decision"`
	RequestID             string `json:"request_id" form:"request_id"`
	ReasonCode            string `json:"reason_code" form:"reason_code"`
}

// TSUWebhookPayload represents a Token Status Update (TSU) notification from CyberSource.
type TSUWebhookPayload struct {
	EventObject       string `json:"eventObject,omitempty"` // For validation if needed
	PaymentInstrument struct {
		ID    string `json:"id"`
		State string `json:"state"`
		Card  struct {
			ExpirationMonth string `json:"expirationMonth"`
			ExpirationYear  string `json:"expirationYear"`
		} `json:"card"`
	} `json:"paymentInstrument"`
	Token struct {
		ID    string `json:"id"`
		State string `json:"state"`
		Card  struct {
			ExpirationMonth string `json:"expirationMonth"`
			ExpirationYear  string `json:"expirationYear"`
		} `json:"card"`
	} `json:"token"`
	InstrumentIdentifier struct {
		ID    string `json:"id"`
		State string `json:"state"`
	} `json:"instrumentIdentifier"`
}

// CaseManagementWebhookPayload represents a webhook from CyberSource Decision Manager for a manual review decision.
type CaseManagementWebhookPayload struct {
	EventID   string `json:"eventId"`
	EventType string `json:"eventType"`
	EventDate string `json:"eventDate"`
	Payload   struct {
		MerchantReferenceCode string `json:"merchantReferenceCode"`
		CaseID                string `json:"caseId"`
	} `json:"payload"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// CyberSource Constants
// ═══════════════════════════════════════════════════════════════════════════════

const (
	// CSStatus values returned by CyberSource REST API
	CSStatusAuthorized              = "AUTHORIZED"
	CSStatusAuthorizedPendingReview = "AUTHORIZED_PENDING_REVIEW"
	CSStatusPendingAuth             = "PENDING_AUTHENTICATION"
	CSStatusDeclined                = "DECLINED"
	CSStatusRejected                = "REJECTED"

	// Processing Action Lists
	ActionAuthorize            = "AUTHORIZATION"
	ActionConsumerAuth         = "CONSUMER_AUTHENTICATION"
	ActionValidateConsumerAuth = "VALIDATE_CONSUMER_AUTHENTICATION"
	ActionTokenCreate          = "TOKEN_CREATE"

	// AFT (Account Funding Transaction) Constants
	BusinessAppIDPersonToPerson = "PP" // Person-to-Person transfer
	CommerceIndicatorInternet   = "internet"
	DeviceChannelBrowser        = "Browser"
	FundingInitiatorSender      = "S" // Sender-initiated
)

// ═══════════════════════════════════════════════════════════════════════════════
// Flex Microform (REST API) Orchestration Models
// ═══════════════════════════════════════════════════════════════════════════════

// CaptureContextRequest
type CaptureContextRequest struct {
	TargetOrigins []string `json:"target_origins"`
}

// PASetupRequest
type PASetupRequest struct {
	ID                string `json:"id"`
	TransientTokenJti string `json:"transient_token_jti"`
	TransientTokenJWT string `json:"transient_token_jwt,omitempty"`
	ExpirationMonth   string `json:"expiration_month,omitempty"`
	ExpirationYear    string `json:"expiration_year,omitempty"`
	PermanentTokenID  string `json:"permanent_token_id,omitempty"`
}

// PASetupResponse
type PASetupResponse struct {
	ID                      string `json:"id"`
	AccessToken             string `json:"access_token"`
	DeviceDataCollectionUrl string `json:"device_data_collection_url"`
	ReferenceId             string `json:"reference_id"`
}

// AuthorizeRequest (Step 5)
type AuthorizeRequest struct {
	ID                          string          `json:"id"`
	TransientTokenJti           string          `json:"transient_token_jti"`
	TransientTokenJWT           string          `json:"transient_token_jwt"`
	IPAddress                   string          `json:"ip_address,omitempty"`
	PAReferenceId               string          `json:"pa_reference_id"`
	ExpirationMonth             string          `json:"expiration_month"`
	ExpirationYear              string          `json:"expiration_year"`
	PermanentTokenID            string          `json:"permanent_token_id,omitempty"`
	ShouldTokenize              bool            `json:"should_tokenize,omitempty"`
	FingerprintID               string          `json:"fingerprint_id,omitempty"` // For Section 6
	Sender                      RemittanceParty `json:"sender"`
	Recipient                   RemittanceParty `json:"recipient"`
	Amount                      string          `json:"amount"`
	Currency                    string          `json:"currency"`
	AuthenticationTransactionId string          `json:"authentication_transaction_id,omitempty"`
}

// ValidateRequest (Step 7)
type ValidateRequest struct {
	ID                          string `json:"id"`
	AuthenticationTransactionId string `json:"authentication_transaction_id"`
}

type ConsumerAuthResponse struct {
	AccessToken                 string `json:"accessToken,omitempty"`
	StepUpUrl                   string `json:"stepUpUrl,omitempty"`
	Pareq                       string `json:"pareq,omitempty"`
	AuthenticationTransactionId string `json:"authenticationTransactionId,omitempty"`
	AcsUrl                      string `json:"acsUrl,omitempty"`
	VeresEnrolled               string `json:"veresEnrolled,omitempty"`
	ParesStatus                 string `json:"paresStatus,omitempty"`
	Eci                         string `json:"eci,omitempty"`
	Cavv                        string `json:"cavv,omitempty"`
	SpecificationVersion        string `json:"specificationVersion,omitempty"`
	ChallengeRequired           string `json:"challengeRequired,omitempty"`
	AuthenticationResult        string `json:"authenticationResult,omitempty"`
	AuthenticationStatusMsg     string `json:"authenticationStatusMsg,omitempty"`
	Indicator                   string `json:"indicator,omitempty"`
	DeviceChannel               string `json:"deviceChannel,omitempty"` // For DDC URL
}

// RemittanceParty helper for AuthorizeRequest
type RemittanceParty struct {
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	Email              string `json:"email,omitempty"`
	Address            string `json:"address"`
	City               string `json:"city"`
	AdministrativeArea string `json:"administrative_area,omitempty"` // State/Province (e.g. "NY")
	Country            string `json:"country"`                       // Full name or code, will be converted in backend
	PostalCode         string `json:"postal_code"`
	Phone              string `json:"phone,omitempty"`
	IDNumber           string `json:"id_number,omitempty"`
	AccountNumber      string `json:"account_number,omitempty"`
}

// SenderCard represents a payment token saved for a specific sender.
type SenderCard struct {
	ID              string    `json:"id"`
	SenderEmail     string    `json:"sender_email"`
	TokenID         string    `json:"token_id"`
	CardBIN         string    `json:"card_bin"`
	CardSuffix      string    `json:"card_suffix"`
	CardBrand       string    `json:"card_brand"`
	ExpirationMonth string    `json:"expiration_month"`
	ExpirationYear  string    `json:"expiration_year"`
	CreatedAt       time.Time `json:"created_at"`
}

// AuthorizeResponse
type AuthorizeResponse struct {
	Status                      string `json:"status"` // AUTHORIZED, PENDING_AUTHENTICATION, DECLINED, ERROR
	ID                          string `json:"id"`
	StepUpUrl                   string `json:"step_up_url,omitempty"`
	AccessToken                 string `json:"access_token,omitempty"`
	Pareq                       string `json:"pareq,omitempty"`
	AuthenticationTransactionId string `json:"authentication_transaction_id,omitempty"`
	PaymentTokenID              string `json:"payment_token_id,omitempty"`
	Message                     string `json:"message,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Bank of Abyssinia (Outbound Payout) Models
// ═══════════════════════════════════════════════════════════════════════════════

// BoATokenResponse holds the OAuth token response from BoA.
type BoATokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// BoAAPIResponse is the generic wrapper for BoA API responses.
type BoAAPIResponse struct {
	Header BoAResponseHeader `json:"header"`
	Body   map[string]any    `json:"body"`
}

// BoAResponseHeader holds the header part of BoA responses.
type BoAResponseHeader struct {
	Status    string `json:"status"`
	Reference string `json:"reference,omitempty"`
}

// BoAAccountInfo is returned by getAccount APIs.
type BoAAccountInfo struct {
	CustomerName    string `json:"customerName"`
	AccountCurrency string `json:"accountCurrency"`
}

// BoATransferWithinRequest is the request body for within-BoA transfers.
type BoATransferWithinRequest struct {
	ClientID      string `json:"client_id"`
	Amount        string `json:"amount"`
	AccountNumber string `json:"accountNumber"`
	Reference     string `json:"reference"`
}

// BoAOtherBankTransferRequest is the request body for other-bank transfers via EthSwitch.
type BoAOtherBankTransferRequest struct {
	ClientID       string `json:"client_id"`
	Amount         string `json:"amount"`
	Reference      string `json:"reference"`
	BankCode       string `json:"bankCode"`
	AccountNumber  string `json:"accountNumber"`
	ReceiverName   string `json:"receiverName"`
	CreditCurrency string `json:"creditCurrency"`
	DebitCurrency  string `json:"debitCurrency"`
}

// BoAWalletTransferRequest is the request body for Telebirr/Mpesa or MoneySend transfers.
type BoAWalletTransferRequest struct {
	ClientID            string `json:"client_id"`
	Amount              string `json:"amount"`
	RemitterName        string `json:"remitterName"`
	RemitterPhonenumber string `json:"remitterPhonenumber"`
	ReceiverName        string `json:"receiverName"`
	ReceiverAddress     string `json:"receiverAddress"`
	BeneficiaryTel      string `json:"beneficiaryTel"`
	Reference           string `json:"reference"`
	DebitCurrency       string `json:"debitCurrency"`
	CreditCurrency      string `json:"creditCurrency"`
	MMProvider          string `json:"mmProvider,omitempty"` // Added for internal logic
}

// BoABankInfo holds bank information from the getBankId API.
type BoABankInfo struct {
	BankID   string `json:"id"`
	BankName string `json:"institutionName"`
}

// BoACurrencyRate holds exchange rate information.
type BoACurrencyRate struct {
	CurrencyCode string `json:"currencyCode"`
	CurrencyName string `json:"currencyName"`
	BuyRate      string `json:"buyRate"`
	SellRate     string `json:"sellRate"`
}

// BoABalanceResponse holds account balance response.
type BoABalanceResponse struct {
	Currency string  `json:"currency"`
	Balance  float64 `json:"balance"`
}

// BoANameCheckResponse holds name fetch result for wallet providers.
type BoANameCheckResponse struct {
	Status       string `json:"status"`
	CustomerName string `json:"customerName"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Remittance (End-to-End) Models
// ═══════════════════════════════════════════════════════════════════════════════

// RemittanceRequest is the full end-to-end remittance initiation request.
type RemittanceRequest struct {
	// Sender
	SenderName    string `json:"sender_name" validate:"required"`
	SenderEmail   string `json:"sender_email"`
	SenderAddress string `json:"sender_address"`
	SenderCity    string `json:"sender_city"`
	SenderState   string `json:"sender_state"`
	SenderPostal  string `json:"sender_postal"`
	SenderCountry string `json:"sender_country"`
	SenderPhone   string `json:"sender_phone"`

	// Amount
	SendAmount     string `json:"send_amount" validate:"required"`
	SendCurrency   string `json:"send_currency" validate:"required"`
	TargetCurrency string `json:"target_currency"` // defaults to ETB

	// Receiver
	ReceiverName    string `json:"receiver_name" validate:"required"`
	ReceiverPhone   string `json:"receiver_phone"`
	ReceiverAddress string `json:"receiver_address"`
	ReceiverCity    string `json:"receiver_city"`
	ReceiverCountry string `json:"receiver_country"`

	// Payout
	PayoutType    PayoutType `json:"payout_type" validate:"required"` // WITHIN_BOA, OTHER_BANK, TELEBIRR, MPESA
	AccountNumber string     `json:"account_number"`                  // for bank transfers
	BankID        string     `json:"bank_id"`                         // for other bank transfers
}

// ManualPayoutRequest is used for manual payout triggers.
type ManualPayoutRequest struct {
	ID    string `json:"id"`
	Phone string `json:"phone,omitempty"` // used for receiver verification
}

func (r RemittanceRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.SenderName, validation.Required.Error("sender name is required")),
		validation.Field(&r.SendAmount, validation.Required.Error("send amount is required")),
		validation.Field(&r.SendCurrency,
			validation.Required.Error("send currency is required"),
			validation.In("USD", "EUR", "GBP", "CAD", "AUD").Error("unsupported currency"),
		),
		validation.Field(&r.ReceiverName, validation.Required.Error("receiver name is required")),
		validation.Field(&r.PayoutType,
			validation.Required.Error("payout type is required"),
			validation.In(PayoutWithinBoA, PayoutOtherBank, PayoutTelebirr, PayoutMpesa).Error("invalid payout type"),
		),
	)
}

// RemittanceResponse is the response after initiating a remittance.
type RemittanceResponse struct {
	ID             string           `json:"id"`
	Status         RemittanceStatus `json:"status"`
	SendAmount     string           `json:"send_amount"`
	SendCurrency   string           `json:"send_currency"`
	ExchangeRate   float64          `json:"exchange_rate,omitempty"`
	ReceiveAmount  string           `json:"receive_amount,omitempty"`
	CaptureContext string           `json:"capture_context,omitempty"` // Flex Microform NEW
	Message        string           `json:"message,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

// PayoutResult is the result of an outbound payout via BoA.
type PayoutResult struct {
	PayoutID     string     `json:"payout_id"`
	Status       string     `json:"status"`
	BoAReference string     `json:"boa_reference,omitempty"`
	Amount       string     `json:"amount"`
	Currency     string     `json:"currency"`
	ReceiverName string     `json:"receiver_name"`
	PayoutType   PayoutType `json:"payout_type"`
	Message      string     `json:"message,omitempty"`
	ProcessedAt  time.Time  `json:"processed_at"`
}

// ExchangeRateResponse is returned for exchange rate queries.
type ExchangeRateResponse struct {
	BaseCurrency   string    `json:"base_currency"`
	TargetCurrency string    `json:"target_currency"`
	Rate           float64   `json:"rate"`
	Timestamp      time.Time `json:"timestamp"`
}

// BeneficiaryCheckResponse is returned after validating a beneficiary.
type BeneficiaryCheckResponse struct {
	Valid        bool   `json:"valid"`
	Name         string `json:"name,omitempty"`
	CurrencyCode string `json:"currency_code,omitempty"`
	Message      string `json:"message,omitempty"`
}

// RemittanceStatusResponse wraps BoA transaction status check results.
type RemittanceStatusResponse struct {
	ID     string         `json:"id"`
	Status string         `json:"status"`
	Detail map[string]any `json:"detail,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Persistence Models
// ═══════════════════════════════════════════════════════════════════════════════

// Remittance represents a full record of a remittance for database storage.
type Remittance struct {
	ID                            string           `json:"id"`
	CsTransactionID               string           `json:"cs_transaction_id"`
	CsAuthenticationTransactionID string           `json:"cs_authentication_transaction_id"`
	Status                        RemittanceStatus `json:"status"`

	// Inbound (Card Collection)
	SenderName       string `json:"sender_name"`
	SenderEmail      string `json:"sender_email"`
	SourceAmount     string `json:"source_amount"`
	SourceCurrency   string `json:"source_currency"`
	CollectionStatus string `json:"collection_status,omitempty"`

	// Conversion
	ExchangeRate   float64 `json:"exchange_rate"`
	TargetAmount   string  `json:"target_amount"`
	TargetCurrency string  `json:"target_currency"`

	// Outbound (Bank Payout)
	ReceiverName  string     `json:"receiver_name"`
	ReceiverPhone string     `json:"receiver_phone,omitempty"`
	PayoutType    PayoutType `json:"payout_type"`
	AccountNumber string     `json:"account_number,omitempty"`
	BankID        string     `json:"bank_id,omitempty"`
	BoAReference  string     `json:"boa_reference,omitempty"`
	PayoutStatus  string     `json:"payout_status,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Service & Handler Interfaces
// ═══════════════════════════════════════════════════════════════════════════════

// CollectionService handles inbound payment collection via CyberSource.
type CollectionService interface {
	// Flex Microform (REST API)
	CreateCaptureContext(origins []string) (string, error)
	SetupPASetup(req *PASetupRequest) (*PASetupResponse, error)
	AuthorizePayment(req *AuthorizeRequest) (*AuthorizeResponse, error)
	ValidateAndAuthorize(req *ValidateRequest) (*AuthorizeResponse, error)
	ReviewPayment(id string, approve bool) error
	ProcessWebhook(notification *CyberSourceNotification) error
	ProcessTSUWebhook(payload *TSUWebhookPayload) error
	ProcessCaseManagementWebhook(payload *CaseManagementWebhookPayload) error
	GetSenderCards(email string) ([]*SenderCard, error)
	UpdateCollectionResult(id, csTransactionID, csAuthTransactionID, collectionStatus, status string) error
	UpdatePayoutResult(id, boaRef, payoutStatus, status string) error
	GetRemittanceByID(id string) (*Remittance, error)
	GetRemittanceByCSAuthenticationID(authID string) (*Remittance, error)
	GetRemittancesBySender(email string, status string) ([]*Remittance, error)
	GetRemittancesByReceiver(phone string, status string) ([]*Remittance, error)
}

// BoAClient defines the interface for interacting with Bank of Abyssinia APIs.
type BoAClient interface {
	FetchAccountName(accountID string) (*BoAAccountInfo, error)
	FetchAccountNameOtherBank(bankID, accountID string) (*BoAAccountInfo, error)
	FetchNameTelebirr(phoneNumber string) (*BoANameCheckResponse, error)
	FetchNameMpesa(phoneNumber string) (*BoANameCheckResponse, error)
	TransferWithin(req *BoATransferWithinRequest) (*BoAAPIResponse, error)
	TransferOtherBank(req *BoAOtherBankTransferRequest) (*BoAAPIResponse, error)
	TransferWallet(req *BoAWalletTransferRequest) (*BoAAPIResponse, error)
	GetBankIDs() ([]BoABankInfo, error)
	GetTransactionStatus(id string) (*BoAAPIResponse, error)
	GetExchangeRate(baseCurrency string) (*BoAAPIResponse, error)
	GetBalance() (*BoAAPIResponse, error)
}

// PayoutService handles outbound disbursement via Bank of Abyssinia.
type PayoutService interface {
	ValidateBeneficiary(payoutType PayoutType, accountOrPhone, bankID string) (*BeneficiaryCheckResponse, error)
	TransferWithinBoA(amount, accountNumber, reference string) (*PayoutResult, error)
	TransferOtherBank(amount, bankID, accountNumber, receiverName, reference string) (*PayoutResult, error)
	TransferWallet(amount, receiverPhoneNumber, provider, receiverName, senderName, senderPhoneNumber, reference string) (*PayoutResult, error)
	CheckRemittanceStatus(id string) (*RemittanceStatusResponse, error)
	GetExchangeRate(baseCurrency string) (*ExchangeRateResponse, error)
	GetBalance() (*BoABalanceResponse, error)
	GetBanks() ([]BoABankInfo, error)
}

// RemittanceService orchestrates the end-to-end remittance flow.
type RemittanceService interface {
	InitiateRemittance(req *RemittanceRequest) (*RemittanceResponse, error)
	ExecutePayout(id string) (*PayoutResult, error)
	TriggerManualPayout(req *ManualPayoutRequest) (*PayoutResult, error)
	GetRemittanceStatus(id string) (*Remittance, error)
	GetSenderRemittances(email string, status RemittanceStatus) ([]*Remittance, error)
	GetReceiverRemittances(phone string, status RemittanceStatus) ([]*Remittance, error)
}

// CollectionHandler handles CyberSource checkout HTTP endpoints.
type CollectionHandler interface {
	// Flex Microform
	CreateCaptureContext(c echo.Context) error
	SetupPayerAuth(c echo.Context) error
	AuthorizePayment(c echo.Context) error
	ValidateAndAuthorize(c echo.Context) error
	ReviewPayment(c echo.Context) error
	HandleWebhook(c echo.Context) error
	GetSenderCards(c echo.Context) error
	Handle3DSReturn(c echo.Context) error
}

// PayoutHandler handles Bank of Abyssinia payout HTTP endpoints.
type PayoutHandler interface {
	ValidateBeneficiary(c echo.Context) error
	GetExchangeRate(c echo.Context) error
	GetBanks(c echo.Context) error
	GetBalance(c echo.Context) error
	CheckRemittanceStatus(c echo.Context) error
}

// RemittanceHandler handles full remittance flow HTTP endpoints.
type RemittanceHandler interface {
	InitiateRemittance(c echo.Context) error
	TriggerPayout(c echo.Context) error
	GetStatus(c echo.Context) error
	ListSenderRemittances(c echo.Context) error
	ListReceiverRemittances(c echo.Context) error
}
