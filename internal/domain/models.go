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
	SenderName     string `json:"sender_name"`
	SenderEmail    string `json:"sender_email"`
	SenderCountry  string `json:"sender_country"`
	ReceiverName   string `json:"receiver_name"`
	ReceiverPhone  string `json:"receiver_phone"`
	PayoutType     string `json:"payout_type"`      // WITHIN_BOA, OTHER_BANK, TELEBIRR, MPESA
	AccountNumber  string `json:"account_number"`    // for bank transfers
	BankID         string `json:"bank_id"`           // for other bank transfers
	TargetCurrency string `json:"target_currency"`   // e.g. "ETB"
}

func (r CheckoutRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Amount, validation.Required.Error("amount is required")),
		validation.Field(&r.Currency,
			validation.Required.Error("currency is required"),
			validation.In("USD", "EUR", "GBP", "CAD", "AUD").Error("unsupported source currency"),
		),
		validation.Field(&r.SenderName, validation.Required.Error("sender name is required")),
		validation.Field(&r.ReceiverName, validation.Required.Error("receiver name is required")),
		validation.Field(&r.PayoutType,
			validation.Required.Error("payout_type is required"),
			validation.In("WITHIN_BOA", "OTHER_BANK", "TELEBIRR", "MPESA").Error("invalid payout type"),
		),
	)
}

// SignedFieldsResponse is returned to the frontend with all signed fields + signature
// needed to POST the form to CyberSource Hosted Checkout.
type SignedFieldsResponse struct {
	AccessKey          string `json:"access_key"`
	Amount             string `json:"amount"`
	Currency           string `json:"currency"`
	Locale             string `json:"locale"`
	PaymentMethod      string `json:"payment_method"`
	ProfileID          string `json:"profile_id"`
	ReferenceNumber    string `json:"reference_number"`
	SignedDateTime     string `json:"signed_date_time"`
	SignedFieldNames   string `json:"signed_field_names"`
	TransactionType    string `json:"transaction_type"`
	TransactionUUID    string `json:"transaction_uuid"`
	UnsignedFieldNames string `json:"unsigned_field_names"`
	Signature          string `json:"signature"`
	CheckoutURL        string `json:"checkout_url"`
}

// CyberSourceWebhookData represents the data received from CyberSource
// via return URL POST or silent-order POST (webhook).
type CyberSourceWebhookData struct {
	Decision         string `form:"decision" json:"decision"`
	ReasonCode       string `form:"reason_code" json:"reason_code"`
	TransactionID    string `form:"transaction_id" json:"transaction_id"`
	ReqReferenceNum  string `form:"req_reference_number" json:"req_reference_number"`
	ReqAmount        string `form:"req_amount" json:"req_amount"`
	ReqCurrency      string `form:"req_currency" json:"req_currency"`
	AuthAmount       string `form:"auth_amount" json:"auth_amount"`
	AuthCode         string `form:"auth_code" json:"auth_code"`
	AuthTime         string `form:"auth_time" json:"auth_time"`
	Message          string `form:"message" json:"message"`
	SignedFieldNames string `form:"signed_field_names" json:"signed_field_names"`
	Signature        string `form:"signature" json:"signature"`
}

// PaymentResult is the response after processing a CyberSource webhook.
type PaymentResult struct {
	ID              string        `json:"id"`
	Status          PaymentStatus `json:"status"`
	Amount          string        `json:"amount"`
	Currency        string        `json:"currency"`
	AuthCode        string        `json:"auth_code,omitempty"`
	ReferenceNumber string        `json:"reference_number,omitempty"`
	RemittanceID    string        `json:"remittance_id,omitempty"`
	Message         string        `json:"message,omitempty"`
	ProcessedAt     time.Time     `json:"processed_at"`
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
	Header BoAResponseHeader `json:"Header"`
	Body   map[string]any    `json:"Body"`
}

// BoAResponseHeader holds the header part of BoA responses.
type BoAResponseHeader struct {
	Status    string `json:"status"`
	Reference string `json:"reference,omitempty"`
}

// BoAAccountInfo is returned by getAccount APIs.
type BoAAccountInfo struct {
	AccountHolderName string `json:"accountHolderName"`
	CurrencyCode      string `json:"currencyCode"`
	EnquiryStatus     int    `json:"enquiryStatus"` // 0 = failure, 1 = success
	ErrorCode         string `json:"errorCode,omitempty"`
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
	ClientID      string `json:"client_id"`
	Amount        string `json:"amount"`
	Reference     string `json:"reference"`
	BankID        string `json:"bankId"`
	AccountNumber string `json:"accountNumber"`
	ReceiverName  string `json:"receiverName"`
}

// BoAWalletTransferRequest is the request body for Telebirr/Mpesa transfers.
type BoAWalletTransferRequest struct {
	ClientID         string `json:"client_id"`
	Amount           string `json:"amount"`
	PhoneNumber      string `json:"phoneNumber"`
	MMProvider       string `json:"mmProvider"` // TELEBIRR, MPESA
	Reference        string `json:"reference"`
	ReceiverName     string `json:"receiverName"`
	OrderingCustomer string `json:"orderingCustomer"`
	SenderPhone      string `json:"senderPhone"`
}

// BoABankInfo holds bank information from the getBankId API.
type BoABankInfo struct {
	BankID   string `json:"bankId"`
	BankName string `json:"bankName"`
}

// BoACurrencyRate holds exchange rate information.
type BoACurrencyRate struct {
	BaseCurrency   string  `json:"baseCurrency"`
	TargetCurrency string  `json:"targetCurrency"`
	Rate           float64 `json:"rate"`
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
	SenderCountry string `json:"sender_country"`
	SenderPhone   string `json:"sender_phone"`

	// Amount
	SendAmount     string `json:"send_amount" validate:"required"`
	SendCurrency   string `json:"send_currency" validate:"required"`
	TargetCurrency string `json:"target_currency"` // defaults to ETB

	// Receiver
	ReceiverName  string `json:"receiver_name" validate:"required"`
	ReceiverPhone string `json:"receiver_phone"`

	// Payout
	PayoutType    PayoutType `json:"payout_type" validate:"required"` // WITHIN_BOA, OTHER_BANK, TELEBIRR, MPESA
	AccountNumber string     `json:"account_number"`                   // for bank transfers
	BankID        string     `json:"bank_id"`                          // for other bank transfers
}

// ManualPayoutRequest is used for manual payout triggers.
type ManualPayoutRequest struct {
	RemittanceID string `json:"remittance_id"`
	Phone        string `json:"phone,omitempty"` // used for receiver verification
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
	RemittanceID  string           `json:"remittance_id"`
	Status        RemittanceStatus `json:"status"`
	SendAmount    string           `json:"send_amount"`
	SendCurrency  string           `json:"send_currency"`
	ExchangeRate  float64          `json:"exchange_rate,omitempty"`
	ReceiveAmount string           `json:"receive_amount,omitempty"`
	CheckoutURL   string           `json:"checkout_url,omitempty"` // CyberSource checkout URL
	SignedFields  *SignedFieldsResponse `json:"signed_fields,omitempty"`
	Message       string           `json:"message,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

// PayoutResult is the result of an outbound payout via BoA.
type PayoutResult struct {
	PayoutID       string        `json:"payout_id"`
	Status         string        `json:"status"`
	BoAReference   string        `json:"boa_reference,omitempty"`
	Amount         string        `json:"amount"`
	Currency       string        `json:"currency"`
	ReceiverName   string        `json:"receiver_name"`
	PayoutType     PayoutType    `json:"payout_type"`
	Message        string        `json:"message,omitempty"`
	ProcessedAt    time.Time     `json:"processed_at"`
}

// ExchangeRateResponse is returned for exchange rate queries.
type ExchangeRateResponse struct {
	BaseCurrency   string  `json:"base_currency"`
	TargetCurrency string  `json:"target_currency"`
	Rate           float64 `json:"rate"`
	Timestamp      time.Time `json:"timestamp"`
}

// BeneficiaryCheckResponse is returned after validating a beneficiary.
type BeneficiaryCheckResponse struct {
	Valid        bool   `json:"valid"`
	Name         string `json:"name,omitempty"`
	CurrencyCode string `json:"currency_code,omitempty"`
	Message      string `json:"message,omitempty"`
}

// TransactionStatusResponse wraps BoA transaction status check results.
type TransactionStatusResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Detail        map[string]any `json:"detail,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Persistence Models
// ═══════════════════════════════════════════════════════════════════════════════

// Transaction represents a full record of a remittance for database storage.
type Transaction struct {
	ID                 string           `json:"id"`
	RemittanceID       string           `json:"remittance_id"`
	Status             RemittanceStatus `json:"status"`

	// Inbound (Card Collection)
	SenderName         string           `json:"sender_name"`
	SenderEmail        string           `json:"sender_email"`
	SourceAmount       string           `json:"source_amount"`
	SourceCurrency     string           `json:"source_currency"`
	CybersourceRef     string           `json:"cybersource_ref,omitempty"`
	CollectionStatus   string           `json:"collection_status,omitempty"`

	// Conversion
	ExchangeRate       float64          `json:"exchange_rate"`
	TargetAmount       string           `json:"target_amount"`
	TargetCurrency     string           `json:"target_currency"`

	// Outbound (Bank Payout)
	ReceiverName       string           `json:"receiver_name"`
	ReceiverPhone      string           `json:"receiver_phone,omitempty"`
	PayoutType         PayoutType       `json:"payout_type"`
	AccountNumber      string           `json:"account_number,omitempty"`
	BankID             string           `json:"bank_id,omitempty"`
	BoAReference       string           `json:"boa_reference,omitempty"`
	PayoutStatus       string           `json:"payout_status,omitempty"`

	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Service & Handler Interfaces
// ═══════════════════════════════════════════════════════════════════════════════

// CollectionService handles inbound payment collection via CyberSource.
type CollectionService interface {
	GenerateSignedFields(req *CheckoutRequest) (*SignedFieldsResponse, error)
	HandleWebhook(data map[string]string) (*PaymentResult, error)
}

// PayoutService handles outbound disbursement via Bank of Abyssinia.
type PayoutService interface {
	ValidateBeneficiary(payoutType PayoutType, accountOrPhone, bankID string) (*BeneficiaryCheckResponse, error)
	TransferWithinBoA(amount, accountNumber, reference string) (*PayoutResult, error)
	TransferOtherBank(amount, bankID, accountNumber, receiverName, reference string) (*PayoutResult, error)
	TransferWallet(amount, phoneNumber, provider, receiverName, senderName, senderPhone, reference string) (*PayoutResult, error)
	CheckTransactionStatus(transactionID string) (*TransactionStatusResponse, error)
	GetExchangeRate(baseCurrency string) (*ExchangeRateResponse, error)
	GetBalance() (*BoABalanceResponse, error)
	GetBanks() ([]BoABankInfo, error)
}

// RemittanceService orchestrates the end-to-end remittance flow.
type RemittanceService interface {
	InitiateRemittance(req *RemittanceRequest) (*RemittanceResponse, error)
	// ProcessCollectionResult handles CyberSource result. Returns (result, alreadyProcessed, error).
	// alreadyProcessed is true if the other channel (redirect or webhook) already processed this result.
	ProcessCollectionResult(data map[string]string) (*PaymentResult, bool, error)
	ExecutePayout(remittanceID string) (*PayoutResult, error)
	TriggerManualPayout(req *ManualPayoutRequest) (*PayoutResult, error)
	GetTransactionStatus(id string) (*Transaction, error)
	GetSenderRemittances(email string, status RemittanceStatus) ([]*Transaction, error)
	GetReceiverRemittances(phone string, status RemittanceStatus) ([]*Transaction, error)
}

// CollectionHandler handles CyberSource checkout HTTP endpoints.
type CollectionHandler interface {
	GenerateSignedFields(c echo.Context) error
	HandleResponse(c echo.Context) error
	HandleWebhook(c echo.Context) error
}

// PayoutHandler handles Bank of Abyssinia payout HTTP endpoints.
type PayoutHandler interface {
	ValidateBeneficiary(c echo.Context) error
	GetExchangeRate(c echo.Context) error
	GetBanks(c echo.Context) error
	GetBalance(c echo.Context) error
	CheckTransactionStatus(c echo.Context) error
}

// RemittanceHandler handles full remittance flow HTTP endpoints.
type RemittanceHandler interface {
	InitiateRemittance(c echo.Context) error
	TriggerPayout(c echo.Context) error
	GetStatus(c echo.Context) error
	ListSenderRemittances(c echo.Context) error
	ListReceiverRemittances(c echo.Context) error
}
