package webhook

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"ukt-payment-system/pkg/response"
)

const maxWebhookBodyBytes = 1 << 20 // 1 MB safety limit

// Handler handles incoming payment gateway webhooks and the integrated mock gateway simulator endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new Webhook HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ReceivePaymentWebhook handles POST /api/webhooks/payment.
// It reads the raw request body bytes once and verifies X-Signature before updating invoice state.
func (h *Handler) ReceivePaymentWebhook(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBodyBytes)
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAYLOAD", "Gagal membaca body request webhook")
		return
	}

	signatureHeader := c.GetHeader("X-Signature")
	res, webhookErr := h.service.ProcessIncomingWebhook(c.Request.Context(), rawBody, signatureHeader)
	if webhookErr != nil {
		response.Error(c, webhookErr.HTTPStatus, webhookErr.Code, webhookErr.Message)
		return
	}

	response.OK(c, res)
}

// TriggerMockWebhook handles POST /api/mock-gateway/trigger.
// It builds the raw JSON payload, calculates the HMAC-SHA256 signature, and dispatches it through ProcessIncomingWebhook.
func (h *Handler) TriggerMockWebhook(c *gin.Context) {
	var req MockTriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAYLOAD", "institution_code, nomor_tagihan, dan event_type wajib diisi")
		return
	}

	rawPayload, signature, err := h.service.BuildSimulatedWebhook(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SIMULATION_ERROR", "Gagal menyiapkan payload simulasi webhook")
		return
	}

	res, webhookErr := h.service.ProcessIncomingWebhook(c.Request.Context(), rawPayload, signature)
	if webhookErr != nil {
		response.OK(c, MockTriggerResponse{
			ComputedSignature: signature,
			SentPayload:       string(rawPayload),
			HTTPStatus:        webhookErr.HTTPStatus,
			WebhookResponse: response.ErrorEnvelope{
				Error: response.ErrorDetail{
					Code:    webhookErr.Code,
					Message: webhookErr.Message,
				},
			},
		})
		return
	}

	response.OK(c, MockTriggerResponse{
		ComputedSignature: signature,
		SentPayload:       string(rawPayload),
		HTTPStatus:        http.StatusOK,
		WebhookResponse: response.SuccessEnvelope{
			Status: "ok",
			Data:   res,
		},
	})
}

// ListWebhookLogs handles GET /api/mock-gateway/logs (and GET /api/admin/webhook-logs).
func (h *Handler) ListWebhookLogs(c *gin.Context) {
	institutionCode := c.Query("institution_code")
	logs, err := h.service.ListAuditLogs(c.Request.Context(), institutionCode, 30)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil riwayat audit log webhook")
		return
	}
	response.OK(c, logs)
}

// ListSimulatorInvoices handles GET /api/mock-gateway/invoices.
func (h *Handler) ListSimulatorInvoices(c *gin.Context) {
	institutionCode := c.Query("institution_code")
	items, err := h.service.ListSimulatorInvoices(c.Request.Context(), institutionCode)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil daftar tagihan untuk simulator")
		return
	}
	response.OK(c, items)
}

func timeNowUnixMilli() int64 {
	return time.Now().UTC().UnixMilli()
}

func timeNowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
