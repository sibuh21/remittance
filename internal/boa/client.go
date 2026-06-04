package boa

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"remittance-service/internal/domain"
)

// Client encapsulates all Bank of Abyssinia API interactions.
type Client struct {
	baseURL      string
	tokenURL     string
	clientID     string
	clientSecret string
	apiKey       string
	httpClient   *http.Client

	// Token management with thread-safe refresh
	mu             sync.RWMutex
	accessToken    string
	refreshToken   string
	tokenExpiry    time.Time
	onTokenRefresh func(string) // Callback for saving rotated tokens
}

// NewClient creates a new Bank of Abyssinia API client.
func NewClient(baseURL, tokenURL, clientID, clientSecret, refreshToken, apiKey string, onTokenRefresh func(string)) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	tokenURL = strings.TrimRight(strings.TrimSpace(tokenURL), "/")

	if baseURL == "" || tokenURL == "" || clientID == "" || clientSecret == "" || refreshToken == "" || apiKey == "" {
		return nil, fmt.Errorf("boa: all configuration fields are required (baseURL, tokenURL, clientID, clientSecret, refreshToken, apiKey)")
	}

	// Ensure HTTPS
	if !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + strings.TrimPrefix(baseURL, "http://")
	}
	if !strings.HasPrefix(tokenURL, "https://") {
		tokenURL = "https://" + strings.TrimPrefix(tokenURL, "http://")
	}

	client := &Client{
		baseURL:        baseURL,
		tokenURL:       tokenURL,
		clientID:       clientID,
		clientSecret:   clientSecret,
		apiKey:         apiKey,
		refreshToken:   refreshToken,
		onTokenRefresh: onTokenRefresh,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	log.Printf("INFO: BoA client initialized (BaseURL: %s)", baseURL)
	return client, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Authentication
// ═══════════════════════════════════════════════════════════════════════════════

// Authenticate obtains or refreshes the OAuth 2.0 access token.
func (c *Client) Authenticate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if the current token is still valid (with 60 second buffer)
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-60*time.Second)) {
		return nil
	}

	log.Printf("INFO: Requesting new BoA access token...")

	authReq := map[string]string{
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
		"refresh_token": c.refreshToken,
		"grant_type":    "refresh_token",
	}

	jsonData, err := json.Marshal(authReq)
	if err != nil {
		return fmt.Errorf("boa auth: failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(c.tokenURL, "/")+"/token", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("boa auth: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	log.Printf("DEBUG: BoA auth response: %v, %v", resp, err)
	if err != nil {
		return fmt.Errorf("boa auth: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("DEBUG: BoA auth response: %d", resp.StatusCode)
	log.Printf("DEBUG: BoA auth response body: %s", string(body))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("boa auth: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp domain.BoATokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("boa auth: failed to parse response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	c.refreshToken = tokenResp.RefreshToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	log.Printf("INFO: BoA access token obtained (expires in %ds)", tokenResp.ExpiresIn)

	// Persist the rotated refresh token if a handler is provided
	if c.onTokenRefresh != nil {
		go c.onTokenRefresh(c.refreshToken)
	}

	return nil
}

// getAccessToken returns the current access token, refreshing if needed.
func (c *Client) getAccessToken() (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-60*time.Second)) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	if err := c.Authenticate(); err != nil {
		return "", err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Name Fetch (Beneficiary Validation)
// ═══════════════════════════════════════════════════════════════════════════════

// FetchAccountName validates an account within BoA and returns account holder info.
func (c *Client) FetchAccountName(accountID string) (*domain.BoAAccountInfo, error) {
	path := fmt.Sprintf("/getAccount/%s", accountID)
	body, err := c.doGet(path)
	if err != nil {
		return nil, fmt.Errorf("boa fetch name: %w", err)
	}

	var resp struct {
		Header domain.BoAResponseHeader `json:"header"`
		Body   []domain.BoAAccountInfo  `json:"body"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa fetch name: parse error: %w", err)
	}

	if strings.ToLower(resp.Header.Status) != "success" || len(resp.Body) == 0 {
		return nil, fmt.Errorf("boa fetch name: account not found or error status: %s", resp.Header.Status)
	}

	return &resp.Body[0], nil
}

// FetchAccountNameOtherBank validates an account at another bank.
func (c *Client) FetchAccountNameOtherBank(bankID, accountID string) (*domain.BoAAccountInfo, error) {
	path := fmt.Sprintf("/otherBank/getAccount/%s/%s", bankID, accountID)
	body, err := c.doGet(path)
	if err != nil {
		return nil, fmt.Errorf("boa fetch name other bank: %w", err)
	}

	var resp struct {
		Header domain.BoAResponseHeader `json:"header"`
		Body   []domain.BoAAccountInfo  `json:"body"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa fetch name other bank: parse error: %w", err)
	}

	if strings.ToLower(resp.Header.Status) != "success" || len(resp.Body) == 0 {
		return nil, &domain.BoAError{
			Message: "account validation failed or error status",
		}
	}

	return &resp.Body[0], nil
}

// FetchNameTelebirr validates a Telebirr wallet.
func (c *Client) FetchNameTelebirr(phoneNumber string) (*domain.BoANameCheckResponse, error) {
	path := fmt.Sprintf("/getName/telebirr/%s", phoneNumber)
	body, err := c.doGet(path)
	if err != nil {
		return nil, fmt.Errorf("boa fetch telebirr name: %w", err)
	}

	var resp domain.BoANameCheckResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa fetch telebirr name: parse error: %w", err)
	}

	return &resp, nil
}

// FetchNameMpesa validates an Mpesa wallet.
func (c *Client) FetchNameMpesa(phoneNumber string) (*domain.BoANameCheckResponse, error) {
	path := fmt.Sprintf("/getName/mpesa/%s", phoneNumber)
	body, err := c.doGet(path)
	if err != nil {
		return nil, fmt.Errorf("boa fetch mpesa name: %w", err)
	}

	var resp domain.BoANameCheckResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa fetch mpesa name: parse error: %w", err)
	}

	return &resp, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Transfers
// ═══════════════════════════════════════════════════════════════════════════════

// TransferWithin initiates a transfer within Bank of Abyssinia.
func (c *Client) TransferWithin(req *domain.BoATransferWithinRequest) (*domain.BoAAPIResponse, error) {
	req.ClientID = c.clientID

	body, err := c.doPost("/transferWithin", req)
	if err != nil {
		return nil, fmt.Errorf("boa transfer within: %w", err)
	}

	var resp domain.BoAAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa transfer within: parse error: %w", err)
	}

	log.Printf("INFO: BoA within-bank transfer completed - Ref: %s, Status: %s",
		req.Reference, resp.Header.Status)

	return &resp, nil
}

// TransferOtherBank initiates a transfer to another bank via EthSwitch.
func (c *Client) TransferOtherBank(req *domain.BoAOtherBankTransferRequest) (*domain.BoAAPIResponse, error) {
	req.ClientID = c.clientID

	body, err := c.doPost("/otherBank/transferEthswitch", req)
	if err != nil {
		return nil, fmt.Errorf("boa other bank transfer: %w", err)
	}

	var resp domain.BoAAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa other bank transfer: parse error: %w", err)
	}

	log.Printf("INFO: BoA other-bank transfer completed - Ref: %s, Status: %s",
		req.Reference, resp.Header.Status)

	return &resp, nil
}

// TransferWallet initiates a transfer to Telebirr or Mpesa wallet.
func (c *Client) TransferWallet(req *domain.BoAWalletTransferRequest) (*domain.BoAAPIResponse, error) {
	req.ClientID = c.clientID

	// Using the /moneySend endpoint as requested
	body, err := c.doPost("/moneySend", req)
	if err != nil {
		return nil, fmt.Errorf("boa wallet transfer: %w", err)
	}

	var resp domain.BoAAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa wallet transfer: parse error: %w", err)
	}

	log.Printf("INFO: BoA wallet transfer completed - Provider: %s, Ref: %s, Status: %s",
		req.MMProvider, req.Reference, resp.Header.Status)

	return &resp, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Utilities
// ═══════════════════════════════════════════════════════════════════════════════

// GetBankIDs retrieves the list of available banks for other-bank transfers.
func (c *Client) GetBankIDs() ([]domain.BoABankInfo, error) {
	body, err := c.doGet("/otherBank/bankId")
	if err != nil {
		return nil, fmt.Errorf("boa get bank ids: %w", err)
	}

	var resp struct {
		Header domain.BoAResponseHeader `json:"header"`
		Body   []domain.BoABankInfo     `json:"body"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa get bank ids: parse error: %w", err)
	}

	return resp.Body, nil
}

// GetTransactionStatus checks the status of a previously initiated transaction.
func (c *Client) GetTransactionStatus(transactionID string) (*domain.BoAAPIResponse, error) {
	path := fmt.Sprintf("/transactionStatus/%s", transactionID)

	body, err := c.doGet(path)
	if err != nil {
		return nil, fmt.Errorf("boa transaction status: %w", err)
	}

	var resp domain.BoAAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa transaction status: parse error: %w", err)
	}

	return &resp, nil
}

// GetExchangeRate retrieves the exchange rate for a given base currency.
func (c *Client) GetExchangeRate(baseCurrency string) (*domain.BoAAPIResponse, error) {
	path := fmt.Sprintf("/rate/%s", baseCurrency)

	body, err := c.doGet(path)
	if err != nil {
		return nil, fmt.Errorf("boa exchange rate: %w", err)
	}

	var resp struct {
		Header domain.BoAResponseHeader `json:"Header"`
		Body   []domain.BoACurrencyRate `json:"Body"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa exchange rate: parse error: %w", err)
	}

	if len(resp.Body) == 0 {
		return nil, fmt.Errorf("boa exchange rate: no rate returned")
	}

	// Map old API response format to new internal representation for compatibility
	rateStr := resp.Body[0].BuyRate
	rate, _ := strconv.ParseFloat(rateStr, 64)

	return &domain.BoAAPIResponse{
		Header: resp.Header,
		Body: map[string]any{
			"rate":         rate,
			"buyRate":      resp.Body[0].BuyRate,
			"sellRate":     resp.Body[0].SellRate,
			"currencyCode": resp.Body[0].CurrencyCode,
		},
	}, nil
}

// GetBalance retrieves the settlement account balance.
func (c *Client) GetBalance() (*domain.BoAAPIResponse, error) {
	body, err := c.doGet("/getBalance")
	if err != nil {
		return nil, fmt.Errorf("boa get balance: %w", err)
	}

	var resp domain.BoAAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("boa get balance: parse error: %w", err)
	}

	return &resp, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HTTP Helpers
// ═══════════════════════════════════════════════════════════════════════════════

// doGet performs an authenticated GET request to the BoA API.
func (c *Client) doGet(path string) ([]byte, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	reqURL := c.baseURL + path
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	log.Printf("DEBUG: BoA GET %s", path)
	log.Printf("URL: %s", reqURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &domain.BoAError{
			StatusCode: resp.StatusCode,
			Message:    "authentication error",
			Detail:     string(body),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &domain.BoAError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status %d", resp.StatusCode),
			Detail:     string(body),
		}
	}

	return body, nil
}

// doPost performs an authenticated POST request to the BoA API.
func (c *Client) doPost(path string, payload any) ([]byte, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	reqURL := c.baseURL + path

	var bodyReader io.Reader
	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequest(http.MethodPost, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	log.Printf("DEBUG: BoA POST %s", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &domain.BoAError{
			StatusCode: resp.StatusCode,
			Message:    "authentication error",
			Detail:     string(body),
		}
	}

	if resp.StatusCode == http.StatusGatewayTimeout || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, &domain.BoAError{
			StatusCode: resp.StatusCode,
			Message:    "BoA gateway timeout or service unavailable",
			Detail:     string(body),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &domain.BoAError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status %d", resp.StatusCode),
			Detail:     string(body),
		}
	}

	return body, nil
}
