package cybersource

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BaseSignedFieldNames is the list of core fields that MUST always be signed.
var BaseSignedFieldNames = []string{
	"access_key",
	"amount",
	"currency",
	"locale",
	"payment_method",
	"profile_id",
	"reference_number",
	"signed_date_time",
	"signed_field_names",
	"transaction_type",
	"transaction_uuid",
}

// Client encapsulates CyberSource Secure Acceptance Hosted Checkout operations.
type Client struct {
	accessKey   string
	profileID   string
	secretKey   []byte
	checkoutURL string
}

// NewClient creates a new CyberSource Secure Acceptance client.
func NewClient(accessKey, profileID, secretKeyStr, checkoutURL string) (*Client, error) {
	accessKey = strings.TrimSpace(accessKey)
	profileID = strings.TrimSpace(profileID)
	secretKeyStr = strings.TrimSpace(secretKeyStr)

	if accessKey == "" || profileID == "" || secretKeyStr == "" {
		return nil, fmt.Errorf("cybersource: accessKey, profileID, and secretKey are all required")
	}

	secretKey := []byte(secretKeyStr)
	log.Printf("INFO: CyberSource client initialized (Key length: %d bytes)", len(secretKey))

	if checkoutURL == "" {
		checkoutURL = "https://testsecureacceptance.cybersource.com/pay"
	}

	return &Client{
		accessKey:   accessKey,
		profileID:   profileID,
		secretKey:   secretKey,
		checkoutURL: checkoutURL,
	}, nil
}

// CheckoutURL returns the CyberSource hosted checkout URL.
func (c *Client) CheckoutURL() string {
	return c.checkoutURL
}

// SignedFields contains all the fields and signature needed for the hosted checkout form.
type SignedFields struct {
	AccessKey          string
	Amount             string
	Currency           string
	Locale             string
	PaymentMethod      string
	ProfileID          string
	ReferenceNumber    string
	SignedDateTime     string
	SignedFieldNames   string
	TransactionType    string
	TransactionUUID    string
	UnsignedFieldNames string
	Signature          string
}

// GenerateSignedFields creates all the signed fields needed for the checkout form.
// For remittance, we use transaction_type "sale" to immediately capture funds.
func (c *Client) GenerateSignedFields(amountStr, currency, locale, paymentMethod string) *SignedFields {
	txnUUID := uuid.New().String()

	if locale == "" {
		locale = "en"
	}
	if paymentMethod == "" {
		paymentMethod = "card"
	}

	// Force 2 decimal places for amount
	amountFloat, _ := strconv.ParseFloat(amountStr, 64)
	formattedAmount := fmt.Sprintf("%.2f", amountFloat)

	// Start with a copy of core fields
	signedList := make([]string, len(BaseSignedFieldNames))
	copy(signedList, BaseSignedFieldNames)

	// MUST be in alphabetical order for CyberSource
	sort.Strings(signedList)

	fields := &SignedFields{
		AccessKey:          c.accessKey,
		Amount:             formattedAmount,
		Currency:           currency,
		Locale:             locale,
		PaymentMethod:      paymentMethod,
		ProfileID:          c.profileID,
		ReferenceNumber:    txnUUID,
		SignedDateTime:     convertToSignatureDate(time.Now()),
		SignedFieldNames:   strings.Join(signedList, ","),
		TransactionType:    "sale",
		TransactionUUID:    txnUUID,
		UnsignedFieldNames: "",
	}

	dataMap := fields.toMap()
	fields.Signature = c.sign(dataMap)

	return fields
}

// VerifySignature verifies the HMAC signature on a CyberSource response.
func (c *Client) VerifySignature(data map[string]string) bool {
	signedFieldNames, ok := data["signed_field_names"]
	if !ok {
		return false
	}
	receivedSignature, ok := data["signature"]
	if !ok {
		return false
	}

	fieldNames := strings.Split(signedFieldNames, ",")
	pairs := make([]string, 0, len(fieldNames))
	for _, name := range fieldNames {
		name = strings.TrimSpace(name)
		if val, exists := data[name]; exists {
			pairs = append(pairs, fmt.Sprintf("%s=%s", name, val))
		}
	}

	dataToSign := strings.Join(pairs, ",")
	expectedSignature := c.computeSignature(dataToSign)

	return hmac.Equal([]byte(receivedSignature), []byte(expectedSignature))
}

// sign creates the HMAC-SHA256 signature for the fields map.
func (c *Client) sign(fields map[string]string) string {
	signedFieldNames := strings.Split(fields["signed_field_names"], ",")

	pairs := make([]string, 0, len(signedFieldNames))
	for _, name := range signedFieldNames {
		val := strings.TrimSpace(fields[name])
		pairs = append(pairs, fmt.Sprintf("%s=%s", name, val))
	}

	dataToSign := strings.Join(pairs, ",")
	log.Printf("DEBUG: String to sign: %s", dataToSign)

	signature := c.computeSignature(dataToSign)
	log.Printf("DEBUG: Generated signature: %s", signature)

	return signature
}

// computeSignature computes HMAC-SHA256 and returns base64 encoded string.
func (c *Client) computeSignature(data string) string {
	mac := hmac.New(sha256.New, c.secretKey)
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// convertToSignatureDate converts a time to the CyberSource date format.
func convertToSignatureDate(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// toMap converts SignedFields to a map for signing.
func (f *SignedFields) toMap() map[string]string {
	return map[string]string{
		"access_key":           f.AccessKey,
		"amount":               f.Amount,
		"currency":             f.Currency,
		"locale":               f.Locale,
		"payment_method":       f.PaymentMethod,
		"profile_id":           f.ProfileID,
		"reference_number":     f.ReferenceNumber,
		"signed_date_time":     f.SignedDateTime,
		"signed_field_names":   f.SignedFieldNames,
		"transaction_type":     f.TransactionType,
		"transaction_uuid":     f.TransactionUUID,
		"unsigned_field_names": f.UnsignedFieldNames,
	}
}
