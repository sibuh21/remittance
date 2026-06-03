package cybersource

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GenerateHTTPSignature produces all required auth headers for CyberSource REST API.
func (c *RESTClient) GenerateHTTPSignature(method, path string, body []byte) (map[string]string, error) {
	now := time.Now().UTC()
	dateStr := now.Format(http.TimeFormat) // RFC 1123

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base url: %w", err)
	}
	host := u.Host

	requestTarget := strings.ToLower(method) + " " + path

	var headerList string
	var signatureString string

	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		digest := "SHA-256=" + base64.StdEncoding.EncodeToString(sha256Sum(body))
		headerList = "host date request-target digest v-c-merchant-id"
		signatureString = fmt.Sprintf(
			"host: %s\ndate: %s\nrequest-target: %s\ndigest: %s\nv-c-merchant-id: %s",
			host, dateStr, requestTarget, digest, c.merchantID,
		)
	} else {
		headerList = "host date request-target v-c-merchant-id"
		signatureString = fmt.Sprintf(
			"host: %s\ndate: %s\nrequest-target: %s\nv-c-merchant-id: %s",
			host, dateStr, requestTarget, c.merchantID,
		)
	}

	// HMAC-SHA256
	mac := hmac.New(sha256.New, c.sharedSecret)
	mac.Write([]byte(signatureString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	headers := map[string]string{
		"Host":            host,
		"Date":            dateStr,
		"v-c-merchant-id": c.merchantID,
		"Signature": fmt.Sprintf(
			`keyid="%s", algorithm="HmacSHA256", headers="%s", signature="%s"`,
			c.keyID, headerList, signature,
		),
	}

	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		digest := "SHA-256=" + base64.StdEncoding.EncodeToString(sha256Sum(body))
		headers["Digest"] = digest
		headers["Content-Type"] = "application/json"
	}

	return headers, nil
}

func sha256Sum(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}
