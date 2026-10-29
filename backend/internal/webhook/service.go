package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ukt-payment-system/internal/institusi"
)

// SecretResolver resolves an institution's HMAC secret key from its configured environment variable name.
type SecretResolver func(envKey string) string

// WebhookError represents a structured HTTP error returned by the webhook processor.
type WebhookError struct {
	HTTPStatus int
	Code       string
	Message    string
}

func (e *WebhookError) Error() string {
	return fmt.Sprintf("[%d %s] %s", e.HTTPStatus, e.Code, e.Message)
}

// Service coordinates per-institution HMAC-SHA256 verification, invoice state transitions, and mandatory audit logging.
type Service struct {
	repo          Repository
	instRepo      institusi.Repository
	resolveSecret SecretResolver
}

// NewService constructs a new Webhook Service.
func NewService(repo Repository, instRepo institusi.Repository, resolveSecret SecretResolver) *Service {
	return &Service{
		repo:          repo,
		instRepo:      instRepo,
		resolveSecret: resolveSecret,
	}
}

// ProcessIncomingWebhook validates the raw body and X-Signature header, updates invoice state, and ALWAYS records an audit log.
func (s *Service) ProcessIncomingWebhook(
	ctx context.Context,
	rawBody []byte,
	signatureHeader string,
) (*WebhookSuccessResponse, *WebhookError) {
	auditEntry := WebhookAuditLog{
		SignatureHeader: strings.TrimSpace(signatureHeader),
		RawPayload:      string(rawBody),
	}

	// Ensure every single incoming webhook notification is persisted to webhook_logs for audit & incident analysis.
	defer func() {
		_ = s.repo.InsertAuditLog(ctx, auditEntry)
	}()

	// 1. Parse JSON payload to extract metadata (institution_code, nomor_tagihan, event_id, event_type)
	// Note: HMAC verification below will STILL use the original `rawBody` byte slice, never re-marshaled JSON!
	var payload PaymentWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		auditEntry.HTTPStatus = http.StatusBadRequest
		auditEntry.ProcessingResult = ResultRejected
		auditEntry.ErrorMessage = "INVALID_PAYLOAD: format JSON tidak valid"
		return nil, &WebhookError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "INVALID_PAYLOAD",
			Message:    "Payload JSON tidak valid",
		}
	}

	payload.EventID = strings.TrimSpace(payload.EventID)
	payload.EventType = strings.TrimSpace(payload.EventType)
	payload.InstitutionCode = strings.TrimSpace(payload.InstitutionCode)
	payload.NomorTagihan = strings.TrimSpace(payload.NomorTagihan)

	auditEntry.InstitutionCode = payload.InstitutionCode
	auditEntry.NomorTagihan = payload.NomorTagihan
	auditEntry.EventID = payload.EventID
	auditEntry.EventType = payload.EventType

	if payload.EventID == "" || payload.InstitutionCode == "" || payload.NomorTagihan == "" {
		auditEntry.HTTPStatus = http.StatusBadRequest
		auditEntry.ProcessingResult = ResultRejected
		auditEntry.ErrorMessage = "INVALID_PAYLOAD: event_id, institution_code, dan nomor_tagihan wajib diisi"
		return nil, &WebhookError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "INVALID_PAYLOAD",
			Message:    "Field event_id, institution_code, dan nomor_tagihan wajib diisi",
		}
	}

	if payload.EventType != EventPaymentSuccess && payload.EventType != EventPaymentExpired {
		auditEntry.HTTPStatus = http.StatusBadRequest
		auditEntry.ProcessingResult = ResultRejected
		auditEntry.ErrorMessage = "INVALID_PAYLOAD: event_type harus payment.success atau payment.expired"
		return nil, &WebhookError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "INVALID_PAYLOAD",
			Message:    "event_type tidak dikenali (hanya mendukung payment.success dan payment.expired)",
		}
	}

	// 2. Lookup institution by institution_code
	inst, err := s.instRepo.GetByCode(ctx, payload.InstitutionCode)
	if err != nil {
		auditEntry.HTTPStatus = http.StatusNotFound
		auditEntry.ProcessingResult = ResultRejected
		auditEntry.ErrorMessage = fmt.Sprintf("INSTITUTION_NOT_FOUND: kode institusi %s tidak terdaftar", payload.InstitutionCode)
		return nil, &WebhookError{
			HTTPStatus: http.StatusNotFound,
			Code:       "INSTITUTION_NOT_FOUND",
			Message:    fmt.Sprintf("Kode institusi '%s' tidak terdaftar", payload.InstitutionCode),
		}
	}
	auditEntry.InstitusiID = &inst.ID

	// 3. Verify X-Signature header using the institution's specific HMAC secret over rawBody
	if strings.TrimSpace(signatureHeader) == "" {
		auditEntry.HTTPStatus = http.StatusUnauthorized
		auditEntry.ProcessingResult = ResultRejected
		auditEntry.ErrorMessage = "MISSING_SIGNATURE: header X-Signature tidak disertakan"
		return nil, &WebhookError{
			HTTPStatus: http.StatusUnauthorized,
			Code:       "MISSING_SIGNATURE",
			Message:    "Header X-Signature wajib disertakan",
		}
	}

	secret := s.resolveSecret(inst.HMACSecretEnvKey)
	if !VerifyHMACSHA256(secret, rawBody, signatureHeader) {
		auditEntry.SignatureValid = false
		auditEntry.HTTPStatus = http.StatusUnauthorized
		auditEntry.ProcessingResult = ResultRejected
		auditEntry.ErrorMessage = fmt.Sprintf("INVALID_SIGNATURE: signature HMAC-SHA256 tidak sesuai dengan secret institusi %s", inst.Code)
		return nil, &WebhookError{
			HTTPStatus: http.StatusUnauthorized,
			Code:       "INVALID_SIGNATURE",
			Message:    fmt.Sprintf("Signature HMAC-SHA256 tidak sesuai dengan secret institusi %s", inst.Code),
		}
	}
	auditEntry.SignatureValid = true

	// 4. Execute atomic invoice state transition & transaction history recording
	outcome, err := s.repo.ProcessPaymentEventAtomic(ctx, inst.ID, payload, rawBody)
	if err != nil {
		if errors.Is(err, ErrInvoiceNotFoundInInstitution) {
			auditEntry.HTTPStatus = http.StatusNotFound
			auditEntry.ProcessingResult = ResultRejected
			auditEntry.ErrorMessage = fmt.Sprintf("INVOICE_NOT_FOUND: nomor tagihan %s tidak ditemukan di institusi %s", payload.NomorTagihan, inst.Code)
			return nil, &WebhookError{
				HTTPStatus: http.StatusNotFound,
				Code:       "INVOICE_NOT_FOUND",
				Message:    fmt.Sprintf("Nomor tagihan '%s' tidak ditemukan pada institusi %s", payload.NomorTagihan, inst.Code),
			}
		}
		if errors.Is(err, ErrAmountMismatch) {
			auditEntry.HTTPStatus = http.StatusUnprocessableEntity
			auditEntry.ProcessingResult = ResultRejected
			auditEntry.ErrorMessage = fmt.Sprintf("AMOUNT_MISMATCH: nominal pada webhook (%d) tidak sesuai dengan nominal tagihan UKT", payload.Amount)
			return nil, &WebhookError{
				HTTPStatus: http.StatusUnprocessableEntity,
				Code:       "AMOUNT_MISMATCH",
				Message:    "Nominal pembayaran pada webhook tidak sesuai dengan nominal tagihan UKT",
			}
		}

		auditEntry.HTTPStatus = http.StatusInternalServerError
		auditEntry.ProcessingResult = ResultRejected
		auditEntry.ErrorMessage = "INTERNAL_ERROR: " + err.Error()
		return nil, &WebhookError{
			HTTPStatus: http.StatusInternalServerError,
			Code:       "INTERNAL_ERROR",
			Message:    "Terjadi kesalahan internal saat memproses transaksi pembayaran",
		}
	}

	auditEntry.HTTPStatus = http.StatusOK
	auditEntry.ProcessingResult = outcome.ProcessingResult

	return &WebhookSuccessResponse{
		EventID:          payload.EventID,
		InstitutionCode:  inst.Code,
		NomorTagihan:     payload.NomorTagihan,
		MahasiswaNIM:     outcome.MahasiswaNIM,
		MahasiswaNama:    outcome.MahasiswaNama,
		Semester:         outcome.Semester,
		InvoiceStatus:    outcome.InvoiceStatus,
		ProcessingResult: outcome.ProcessingResult,
		Message:          outcome.Message,
	}, nil
}

// ListSimulatorInvoices returns invoices along with their student owner for the Mock Payment Gateway selector.
func (s *Service) ListSimulatorInvoices(ctx context.Context, institutionCode string) ([]SimulatorInvoiceOption, error) {
	return s.repo.ListSimulatorInvoices(ctx, institutionCode)
}

// ListAuditLogs returns recent webhook audit logs.
func (s *Service) ListAuditLogs(ctx context.Context, institutionCode string, limit int) ([]WebhookAuditLog, error) {
	return s.repo.ListRecentAuditLogs(ctx, institutionCode, limit)
}

// BuildSimulatedWebhook constructs a raw JSON payload and computes the appropriate HMAC signature for the Mock Payment Gateway.
func (s *Service) BuildSimulatedWebhook(ctx context.Context, req MockTriggerRequest) (rawPayload []byte, signature string, err error) {
	eventID := strings.TrimSpace(req.EventID)
	if eventID == "" {
		eventID = fmt.Sprintf("evt_sim_%d", timeNowUnixMilli())
	}
	channel := strings.TrimSpace(req.PaymentChannel)
	if channel == "" {
		channel = "VA_MANDIRI"
	}

	payload := PaymentWebhookPayload{
		EventID:         eventID,
		EventType:       strings.TrimSpace(req.EventType),
		InstitutionCode: strings.TrimSpace(req.InstitutionCode),
		NomorTagihan:    strings.TrimSpace(req.NomorTagihan),
		Amount:          req.Amount,
		PaymentChannel:  channel,
		Timestamp:       timeNowRFC3339(),
	}

	rawBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	switch strings.ToUpper(strings.TrimSpace(req.SimulationMode)) {
	case "INVALID_SIGNATURE":
		// Intentionally tampered signature to demonstrate 401 INVALID_SIGNATURE and audit log persistence
		return rawBytes, ComputeHMACSHA256("wrong_tampered_secret_key", rawBytes), nil
	case "WRONG_INSTITUTION_SECRET":
		// Use another institution's secret key to demonstrate cross-institution HMAC rejection
		otherEnvKey := "HMAC_SECRET_INST_2"
		if payload.InstitutionCode == "INST-TEKNOLOGI-MAJU" {
			otherEnvKey = "HMAC_SECRET_INST_1"
		}
		return rawBytes, ComputeHMACSHA256(s.resolveSecret(otherEnvKey), rawBytes), nil
	default:
		inst, err := s.instRepo.GetByCode(ctx, payload.InstitutionCode)
		if err != nil {
			// If testing an unknown institution_code, sign with a dummy secret so the request still fires
			return rawBytes, ComputeHMACSHA256("unknown_inst_secret", rawBytes), nil
		}
		secret := s.resolveSecret(inst.HMACSecretEnvKey)
		return rawBytes, ComputeHMACSHA256(secret, rawBytes), nil
	}
}
