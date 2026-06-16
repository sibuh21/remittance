package cybersource

import (
	"time"
)

// PaymentRequest represents the combined REST API payment request
// with AFT (Account Funding Transaction) fields for remittance.
type PaymentRequest struct {
	ClientReferenceInformation ClientReferenceInfo      `json:"clientReferenceInformation"`
	ProcessingInformation      ProcessingInfo           `json:"processingInformation"`
	OrderInformation           OrderInfo                `json:"orderInformation"`
	TokenInformation           *TokenInfo               `json:"tokenInformation,omitempty"`
	PaymentInformation         *PaymentInfo             `json:"paymentInformation,omitempty"`
	DeviceInformation          *DeviceInfo              `json:"deviceInformation,omitempty"`
	ConsumerAuthenticationInfo *ConsumerAuthInfo         `json:"consumerAuthenticationInformation,omitempty"`
	AcquirerInformation        *AcquirerInfo            `json:"acquirerInformation,omitempty"`
	RecipientInformation       *RecipientInfo           `json:"recipientInformation,omitempty"`
	SenderInformation          *SenderInfo              `json:"senderInformation,omitempty"`
	MerchantInformation        *MerchantInfo            `json:"merchantInformation,omitempty"`
	MerchantDefinedInfo        []MerchantDefinedField   `json:"merchantDefinedInformation,omitempty"`
}

type ClientReferenceInfo struct {
	Code string `json:"code"`
}

type ProcessingInfo struct {
	Capture              bool          `json:"capture"`
	CommerceIndicator    string        `json:"commerceIndicator,omitempty"`
	ActionList           []string      `json:"actionList"`
	BusinessApplicationId string        `json:"businessApplicationId,omitempty"` // "PP" for Person-to-Person
	ActionTokenTypes      []string      `json:"actionTokenTypes,omitempty"` // ["paymentInstrument", "instrumentIdentifier"]
	AuthorizationOptions  *AuthOptions  `json:"authorizationOptions,omitempty"`
}

type AuthOptions struct {
	Initiator      *Initiator      `json:"initiator,omitempty"`
	AFTIndicator   string          `json:"aftIndicator,omitempty"` // "true" for remittance
	FundingOptions *FundingOptions `json:"fundingOptions,omitempty"`
}

type Initiator struct {
	Type                 string `json:"type"`                 // "customer"
	StoredCredentialUsed string `json:"storedCredentialUsed"` // "true"
}

type FundingOptions struct {
	Initiator *FundingInitiator `json:"initiator,omitempty"`
}

type FundingInitiator struct {
	Type string `json:"type"` // "S" for sender, "P" for recipient
}

type OrderInfo struct {
	BillTo        BillTo        `json:"billTo"`
	AmountDetails AmountDetails `json:"amountDetails"`
}

type BillTo struct {
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	Email        string `json:"email"`
	Address1     string `json:"address1"`
	Locality     string `json:"locality"`
	AdministrativeArea string `json:"administrativeArea"`
	Country            string `json:"country"`
	PostalCode   string `json:"postalCode"`
	PhoneNumber  string `json:"phoneNumber,omitempty"`
	District     string `json:"district,omitempty"`
	BuildingName string `json:"buildingNumber,omitempty"`
}

type AmountDetails struct {
	TotalAmount string `json:"totalAmount"`
	Currency    string `json:"currency"`
}

type TokenInfo struct {
	TransientTokenJWT string `json:"transientTokenJwt,omitempty"`
}

type PaymentInfo struct {
	Customer             *CustomerRef  `json:"customer,omitempty"`
	Card                 *CardInfo     `json:"card,omitempty"`
	PaymentInstrument    *TMSReference `json:"paymentInstrument,omitempty"`
	InstrumentIdentifier *TMSReference `json:"instrumentIdentifier,omitempty"`
}

type TMSReference struct {
	ID string `json:"id"`
}

type CardInfo struct {
	ExpirationMonth string `json:"expirationMonth,omitempty"`
	ExpirationYear  string `json:"expirationYear,omitempty"`
	Type            string `json:"type,omitempty"`
	Bin             string `json:"bin,omitempty"`
	Suffix          string `json:"suffix,omitempty"`
}

type CustomerRef struct {
	ID string `json:"id"`
}

type DeviceInfo struct {
	IPAddress                    string `json:"ipAddress,omitempty"`
	FingerprintSessionId         string `json:"fingerprintSessionId,omitempty"`
	HttpAcceptBrowserValue       string `json:"httpAcceptBrowserValue,omitempty"`
	HttpAcceptContent            string `json:"httpAcceptContent,omitempty"`
	HttpBrowserLanguage          string `json:"httpBrowserLanguage,omitempty"`
	HttpBrowserJavaEnabled       bool   `json:"httpBrowserJavaEnabled,omitempty"`
	HttpBrowserJavaScriptEnabled bool   `json:"httpBrowserJavaScriptEnabled,omitempty"`
	HttpBrowserColorDepth        string `json:"httpBrowserColorDepth,omitempty"`
	HttpBrowserScreenHeight      string `json:"httpBrowserScreenHeight,omitempty"`
	HttpBrowserScreenWidth       string `json:"httpBrowserScreenWidth,omitempty"`
	HttpBrowserTimeDifference    string `json:"httpBrowserTimeDifference,omitempty"`
	UserAgentBrowserValue        string `json:"userAgentBrowserValue,omitempty"`
}

type ConsumerAuthInfo struct {
	ReferenceId                 string `json:"referenceId,omitempty"`
	ReturnUrl                   string `json:"returnUrl,omitempty"`
	DeviceChannel               string `json:"deviceChannel,omitempty"`
	AuthenticationTransactionId string `json:"authenticationTransactionId,omitempty"`
}

type AcquirerInfo struct {
	MerchantId string `json:"merchantId"`
}

type RecipientInfo struct {
	AccountNumber   string `json:"accountId,omitempty"`
	AccountType     string `json:"accountType,omitempty"` // "99"
	FirstName       string `json:"firstName,omitempty"`
	LastName        string `json:"lastName,omitempty"`
	MiddleName      string `json:"middleName,omitempty"`
	Address1        string `json:"address1,omitempty"`
	Locality        string `json:"locality,omitempty"`
	Country         string `json:"country,omitempty"`
	PostalCode      string `json:"postalCode,omitempty"`
	BeneficiaryId   string `json:"beneficiaryId,omitempty"`
	BeneficiaryName string `json:"beneficiaryName,omitempty"`
}

type SenderInfo struct {
	Account              *SenderAccount `json:"account,omitempty"`
	FirstName            string         `json:"firstName,omitempty"`
	MiddleName           string         `json:"middleName,omitempty"`
	LastName             string         `json:"lastName,omitempty"`
	Address1             string         `json:"address1,omitempty"`
	Locality             string         `json:"locality,omitempty"`
	AdministrativeArea   string         `json:"administrativeArea"`
	CountryCode          string         `json:"countryCode,omitempty"`
	PostalCode           string         `json:"postalCode,omitempty"`
	PhoneNumber          string         `json:"phoneNumber,omitempty"`
	IdentificationNumber string         `json:"identificationNumber,omitempty"`
	PersonalIdType       string         `json:"personalIdType,omitempty"` // "TXIN"
	Type                 string         `json:"type,omitempty"`           // "B" = business
	Name                 string         `json:"name,omitempty"`
	ReferenceNumber      string         `json:"referenceNumber,omitempty"`
}

type SenderAccount struct {
	Number      string `json:"number"`
	FundsSource string `json:"fundsSource"` // "02"
}

type MerchantInfo struct {
	VatRegistrationNumber string              `json:"vatRegistrationNumber,omitempty"`
	MerchantDescriptor    *MerchantDescriptor `json:"merchantDescriptor,omitempty"`
}

type MerchantDescriptor struct {
	Name     string `json:"name"`
	Locality string `json:"locality"`
	Country  string `json:"country,omitempty"`
}

type MerchantDefinedField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Response Models

type PaymentResponse struct {
	ID                         string                `json:"id"`
	Status                     string                `json:"status"`
	ClientReferenceInformation *ClientReferenceInfo  `json:"clientReferenceInformation,omitempty"`
	ConsumerAuthenticationInfo *ConsumerAuthResponse `json:"consumerAuthenticationInformation,omitempty"`
	ProcessorInformation       *ProcessorInfo        `json:"processorInformation,omitempty"`
	OrderInformation           *OrderInfoResponse    `json:"orderInformation,omitempty"`
	TokenInformation           *TokenInfoResponse    `json:"tokenInformation,omitempty"`
	PaymentInformation         *PaymentInfo          `json:"paymentInformation,omitempty"`
	ErrorInformation           *ErrorInfo            `json:"errorInformation,omitempty"`
	SubmitTimeUtc              time.Time             `json:"submitTimeUtc"`
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
	Indicator                   string `json:"indicator,omitempty"`
	DeviceChannel               string `json:"deviceChannel,omitempty"`
	DeviceDataCollectionUrl     string `json:"deviceDataCollectionUrl,omitempty"`
	ReferenceId                 string `json:"referenceId,omitempty"`
}

type ProcessorInfo struct {
	ApprovalCode          string `json:"approvalCode,omitempty"`
	ResponseCode          string `json:"responseCode,omitempty"`
	NetworkTransactionId  string `json:"networkTransactionId,omitempty"`
	TransactionId         string `json:"transactionId,omitempty"`
}

type OrderInfoResponse struct {
	AmountDetails AmountDetailsResponse `json:"amountDetails"`
}

type AmountDetailsResponse struct {
	AuthorizedAmount string `json:"authorizedAmount"`
	Currency         string `json:"currency"`
}

type TokenInfoResponse struct {
	Customer             *TMSTokenRef `json:"customer,omitempty"`
	PaymentInstrument    *TMSTokenRef `json:"paymentInstrument,omitempty"`
	InstrumentIdentifier *TMSTokenRef `json:"instrumentIdentifier,omitempty"`
}

type TMSTokenRef struct {
	ID string `json:"id"`
}

type ErrorInfo struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// Capture Context Models

type CaptureContextRequest struct {
	TargetOrigins []string `json:"targetOrigins"`
}

// PASetup Models

type PASetupRequest struct {
	ClientReferenceInformation ClientReferenceInfo `json:"clientReferenceInformation"`
	TokenInformation           *PASetupTokenInfo   `json:"tokenInformation,omitempty"`
	PaymentInformation         *PASetupPaymentInfo `json:"paymentInformation,omitempty"`
}

type PASetupTokenInfo struct {
	TransientToken    string `json:"transientToken,omitempty"`
	TransientTokenJWT string `json:"transientTokenJwt,omitempty"`
}

type PASetupPaymentInfo struct {
	Customer             *CustomerRef  `json:"customer,omitempty"`
	InstrumentIdentifier *TMSReference `json:"instrumentIdentifier,omitempty"`
	Card                 *CardInfo     `json:"card,omitempty"`
}

type PASetupResponse struct {
	ID                         string                `json:"id"`
	Status                     string                `json:"status"`
	ConsumerAuthenticationInfo ConsumerAuthResponse `json:"consumerAuthenticationInformation"`
}
