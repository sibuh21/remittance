package domain

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type CardInfo struct {
	Content struct {
		PaymentInformation struct {
			Card struct {
				ExpirationYear struct {
					Value string `json:"value"`
				} `json:"expirationYear"`
				Number struct {
					MaskedValue string `json:"maskedValue"`
					Bin         string `json:"bin"`
				} `json:"number"`
				ExpirationMonth struct {
					Value string `json:"value"`
				} `json:"expirationMonth"`
				Type struct {
					Value string `json:"value"`
				} `json:"type"`
			} `json:"card"`
		} `json:"paymentInformation"`
	} `json:"content"`
}

// DecodeTransientToken parses the base64 payload segments of a CyberSource Flex token offline
func DecodeTransientToken(tokenStr string) (*CardInfo, error) {
	// JWT components are separated by periods: Header.Payload.Signature
	segments := strings.Split(tokenStr, ".")
	if len(segments) < 2 {
		return nil, fmt.Errorf("invalid transient token format string")
	}

	// Extract the middle segment (the payload body)
	payloadSegment := segments[1]

	// Handle missing base64 padding constraints safely
	if rem := len(payloadSegment) % 4; rem > 0 {
		payloadSegment += strings.Repeat("=", 4-rem)
	}

	// Decode base64URL representation to raw bytes
	decodedBytes, err := base64.URLEncoding.DecodeString(payloadSegment)
	if err != nil {
		return nil, fmt.Errorf("failed base64 token payload decode: %w", err)
	}
	var payload CardInfo
	if err := json.Unmarshal(decodedBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload JSON: %w", err)
	}

	return &payload, nil
}
