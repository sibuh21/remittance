package domain

// ─── Payment Status ─────────────────────────────────────────────────────────────

// PaymentStatus represents the state of a payment transaction.
type PaymentStatus string

const (
	StatusPending  PaymentStatus = "PENDING"
	StatusAccepted PaymentStatus = "ACCEPT"
	StatusDeclined PaymentStatus = "DECLINE"
	StatusReview   PaymentStatus = "REVIEW"
	StatusError    PaymentStatus = "ERROR"
	StatusCancel   PaymentStatus = "CANCEL"
)

// ─── Payout Type ────────────────────────────────────────────────────────────────

// PayoutType represents the destination type for outbound disbursement.
type PayoutType string

const (
	PayoutWithinBoA PayoutType = "WITHIN_BOA"
	PayoutOtherBank PayoutType = "OTHER_BANK"
	PayoutTelebirr  PayoutType = "TELEBIRR"
	PayoutMpesa     PayoutType = "MPESA"
)

// ─── Remittance Status ──────────────────────────────────────────────────────────

// RemittanceStatus tracks the end-to-end lifecycle of a remittance.
type RemittanceStatus string

const (
	RemittanceInitiated          RemittanceStatus = "INITIATED"
	RemittanceCollectionPending  RemittanceStatus = "COLLECTION_PENDING"
	Remittance3DSPending         RemittanceStatus = "3DS_PENDING"
	RemittanceCollected          RemittanceStatus = "COLLECTED"
	RemittancePayoutPending      RemittanceStatus = "PAYOUT_PENDING"
	RemittancePayoutProcessing   RemittanceStatus = "PAYOUT_PROCESSING"
	RemittanceCompleted          RemittanceStatus = "COMPLETED"
	RemittanceFailed             RemittanceStatus = "FAILED"
	RemittanceCancelled          RemittanceStatus = "CANCELLED"
)

// ─── BoA Error Codes ────────────────────────────────────────────────────────────

// BoAErrorCode maps BoA error codes to human-readable descriptions.
var BoAErrorCodes = map[string]string{
	"WSH914": "INVALID ACCOUNT",
	"WSH928": "ACCOUNT RESTRICTED",
	"WSH968": "WRONG AMOUNT",
	"WSH903": "INVALID AMOUNT",
	"WSH920": "AMOUNT NOT ALLOWED",
	"WSH962": "TRANSACTION NOT ALLOWED",
	"WSH963": "TRANSACTION FAILED",
	"WSH825": "CURRENCY MISSING",
	"WSH828": "INSTITUTION NOT FOUND",
	"WSH801": "TIME OUT",
	"WSH802": "SERVICE UNAVAILABLE",
	"WSH841": "INVALID CURRENCY",
	"WSH876": "CAN NOT PROCESS AMOUNT",
}
