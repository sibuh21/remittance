package cybersource

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// RESTClient encapsulates CyberSource REST API interactions.
type RESTClient struct {
	merchantID   string
	keyID        string
	sharedSecret []byte
	baseURL      string
	httpClient   *http.Client
}

// NewRESTClient creates a new CyberSource REST API client.
func NewRESTClient(merchantID, keyID, sharedSecretBase64, baseURL string) (*RESTClient, error) {
	decodedSecret, err := base64.StdEncoding.DecodeString(sharedSecretBase64)
	if err != nil {
		return nil, fmt.Errorf("cybersource: invalid base64 shared secret: %w", err)
	}

	return &RESTClient{
		merchantID:   merchantID,
		keyID:        keyID,
		sharedSecret: decodedSecret,
		baseURL:      strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// CreateCaptureContext generates a JWT capture context for Flex Microform.
func (c *RESTClient) CreateCaptureContext(req *CaptureContextRequest) (string, error) {
	path := "/microform/v2/sessions"
	respBody, err := c.doPost(path, req)
	if err != nil {
		return "", fmt.Errorf("capture context generation failed: %w", err)
	}

	var resp struct {
		CaptureContext string `json:"captureContext"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		// Fallback: the body itself may be the JWT in some API versions
		if strings.Count(string(respBody), ".") == 2 {
			return string(respBody), nil
		}
		return "", fmt.Errorf("cybersource: failed to parse capture context response: %w", err)
	}

	return resp.CaptureContext, nil
}

// PASetup initiates the Payer Authentication Setup.
func (c *RESTClient) PASetup(req *PASetupRequest) (*PASetupResponse, error) {
	path := "/risk/v1/authentication-setups"
	respBody, err := c.doPost(path, req)
	if err != nil {
		return nil, fmt.Errorf("PA Setup failed: %w", err)
	}

	var resp PASetupResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("cybersource: failed to parse PA Setup response: %w", err)
	}

	return &resp, nil
}

// AuthorizePayment performs a combined payment call (Auth + Capture + PA-Enrollment).
func (c *RESTClient) AuthorizePayment(req *PaymentRequest) (*PaymentResponse, error) {
	path := "/pts/v2/payments"
	respBody, err := c.doPost(path, req)
	if err != nil {
		log.Printf("ERROR: CyberSource authorize failed on %s: %v", path, err)
		return nil, err
	}

	var resp PaymentResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("cybersource: failed to parse payment response: %w", err)
	}

	return &resp, nil
}

// doPost handles REST POST requests with HTTP Signature authentication.
func (c *RESTClient) doPost(path string, payload any) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("cybersource: failed to marshal request: %w", err)
	}

	headers, err := c.GenerateHTTPSignature(http.MethodPost, path, jsonData)
	if err != nil {
		return nil, err
	}

	reqURL := c.baseURL + path
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("cybersource: failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cybersource: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cybersource: failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return respBody, fmt.Errorf("cybersource: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
