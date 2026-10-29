package webhook

import (
	"time"

	"github.com/google/uuid"
)

const (
	EventPaymentSuccess = "payment.success"
	EventPaymentExpired = "payment.expired"

	ResultProcessed               = "PROCESSED"
	ResultProcessedLateSettlement = "PROCESSED_LATE_SETTLEMENT"
	ResultIdempotentReplay        = "IDEMPOTENT_REPLAY"
	ResultIgnoredAlreadyPaid      = "IGNORED_ALREADY_PAID"
	ResultRejected                = "REJECTED"
)

// PaymentWebhookPayload represents the incoming JSON body on POST /api/webhooks/payment.
type PaymentWebhookPayload struct {
	EventID         string `json:"event_id"`
	EventType       string `json:"event_type"`
	InstitutionCode string `json:"institution_code"`
	NomorTagihan    string `json:"nomor_tagihan"`
	Amount          int64  `json:"amount"`
	PaymentChannel  string `json:"payment_channel"`
	Timestamp       string `json:"timestamp"`
}

// WebhookSuccessResponse represents the data payload returned on 200 OK.
type WebhookSuccessResponse struct {
	EventID          string `json:"event_id"`
	InstitutionCode  string `json:"institution_code"`
	NomorTagihan     string `json:"nomor_tagihan"`
	MahasiswaNIM     string `json:"mahasiswa_nim,omitempty"`
	MahasiswaNama    string `json:"mahasiswa_nama,omitempty"`
	Semester         string `json:"semester,omitempty"`
	InvoiceStatus    string `json:"invoice_status"`
	ProcessingResult string `json:"processing_result"`
	Message          string `json:"message"`
}

// SimulatorInvoiceOption represents an invoice lookup item for the Mock Payment Gateway UI.
type SimulatorInvoiceOption struct {
	InstitutionCode string `json:"institution_code"`
	InstitutionName string `json:"institution_name"`
	NomorTagihan    string `json:"nomor_tagihan"`
	Semester        string `json:"semester"`
	MahasiswaNIM    string `json:"mahasiswa_nim"`
	MahasiswaNama   string `json:"mahasiswa_nama"`
	Nominal         int64  `json:"nominal"`
	Status          string `json:"status"`
}

// WebhookAuditLog represents a permanent record in the webhook_logs table.
type WebhookAuditLog struct {
	ID               uuid.UUID  `json:"id"`
	InstitusiID      *uuid.UUID `json:"institusi_id"`
	InstitutionCode  string     `json:"institution_code"`
	NomorTagihan     string     `json:"nomor_tagihan"`
	EventID          string     `json:"event_id"`
	EventType        string     `json:"event_type"`
	SignatureHeader  string     `json:"signature_header"`
	SignatureValid   bool       `json:"signature_valid"`
	HTTPStatus       int        `json:"http_status"`
	ProcessingResult string     `json:"processing_result"`
	ErrorMessage     string     `json:"error_message"`
	RawPayload       string     `json:"raw_payload"`
	ReceivedAt       time.Time  `json:"received_at"`
}

// MockTriggerRequest represents the simulator payload for POST /api/mock-gateway/trigger.
type MockTriggerRequest struct {
	InstitutionCode string `json:"institution_code" binding:"required"`
	NomorTagihan    string `json:"nomor_tagihan" binding:"required"`
	EventType       string `json:"event_type" binding:"required"`
	Amount          int64  `json:"amount"`
	PaymentChannel  string `json:"payment_channel"`
	EventID         string `json:"event_id"`
	SimulationMode  string `json:"simulation_mode"` // "VALID" | "INVALID_SIGNATURE" | "WRONG_INSTITUTION_SECRET"
}

// MockTriggerResponse returns full transparency of the simulated webhook request & response.
type MockTriggerResponse struct {
	ComputedSignature string      `json:"computed_signature"`
	SentPayload       string      `json:"sent_payload"`
	HTTPStatus        int         `json:"http_status"`
	WebhookResponse   interface{} `json:"webhook_response"`
}
