package domain

import "fmt"

// ─── Application Error ──────────────────────────────────────────────────────────

// AppError is a structured error with HTTP status code for API responses.
type AppError struct {
	Code    int    `json:"-"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Detail)
	}
	return e.Message
}

// NewAppError creates a new AppError.
func NewAppError(code int, message, detail string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Detail:  detail,
	}
}

// ─── BoA Error ──────────────────────────────────────────────────────────────────

// BoAError wraps errors returned by the Bank of Abyssinia API.
type BoAError struct {
	StatusCode int    `json:"-"`
	ErrorCode  string `json:"error_code,omitempty"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
}

func (e *BoAError) Error() string {
	msg := e.Message
	if e.ErrorCode != "" {
		desc, ok := BoAErrorCodes[e.ErrorCode]
		if ok {
			msg = fmt.Sprintf("%s (%s): %s", e.ErrorCode, desc, e.Message)
		} else {
			msg = fmt.Sprintf("%s: %s", e.ErrorCode, e.Message)
		}
	}
	if e.Detail != "" {
		return fmt.Sprintf("BoA error: %s - Detail: %s", msg, e.Detail)
	}
	return fmt.Sprintf("BoA error: %s", msg)
}

// ─── Signature Verification Error ───────────────────────────────────────────────

// SignatureError represents an HMAC signature mismatch.
type SignatureError struct {
	Source string
}

func (e *SignatureError) Error() string {
	return fmt.Sprintf("invalid %s signature", e.Source)
}
