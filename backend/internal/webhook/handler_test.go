package webhook

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"ukt-payment-system/internal/institusi"
	"ukt-payment-system/internal/tagihan"
)

func TestHandler_ReceivePaymentWebhook_RawBodyAndHTTPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	instID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	secretA := "secret_http_test_kampus_a"

	instRepo := &fakeInstRepo{
		byCode: map[string]institusi.Institution{
			"UNIV-NUSANTARA": {
				ID:               instID,
				Code:             "UNIV-NUSANTARA",
				Name:             "Universitas Nusantara Raya",
				HMACSecretEnvKey: "HMAC_SECRET_INST_1",
			},
		},
	}

	webhookRepo := newFakeWebhookRepo()
	webhookRepo.invoices[webhookRepo.invKey(instID, "UKT/2026-1/001")] = &fakeInvoiceState{
		InstitusiID:  instID,
		NomorTagihan: "UKT/2026-1/001",
		Nominal:      5500000,
		Status:       tagihan.StatusUnpaid,
	}

	svc := NewService(webhookRepo, instRepo, func(_ string) string {
		return secretA
	})
	handler := NewHandler(svc)

	router := gin.New()
	router.POST("/api/webhooks/payment", handler.ReceivePaymentWebhook)

	// Perhatikan spasi ekstra pada raw JSON di bawah ini:
	// Jika handler mem-parse ke struct lalu json.Marshal ulang, HMAC pasti gagal!
	// Karena handler membaca Raw Body Bytes asli, verifikasi HMAC tetap sukses.
	rawJSONWithSpaces := []byte(`{  "event_id": "evt_http_01", "event_type": "payment.success", "institution_code": "UNIV-NUSANTARA", "nomor_tagihan": "UKT/2026-1/001", "amount": 5500000 }`)

	// 1. Tanpa Header X-Signature -> 401 Unauthorized
	reqNoSig := httptest.NewRequest(http.MethodPost, "/api/webhooks/payment", bytes.NewReader(rawJSONWithSpaces))
	reqNoSig.Header.Set("Content-Type", "application/json")
	wNoSig := httptest.NewRecorder()
	router.ServeHTTP(wNoSig, reqNoSig)
	if wNoSig.Code != http.StatusUnauthorized {
		t.Fatalf("diharapkan 401 saat tanpa X-Signature, didapat %d", wNoSig.Code)
	}

	// 2. Dengan Header X-Signature yang dihitung dari Raw Body Bytes -> 200 OK
	validSig := ComputeHMACSHA256(secretA, rawJSONWithSpaces)
	reqValid := httptest.NewRequest(http.MethodPost, "/api/webhooks/payment", bytes.NewReader(rawJSONWithSpaces))
	reqValid.Header.Set("Content-Type", "application/json")
	reqValid.Header.Set("X-Signature", validSig)
	wValid := httptest.NewRecorder()
	router.ServeHTTP(wValid, reqValid)
	if wValid.Code != http.StatusOK {
		t.Fatalf("diharapkan 200 OK untuk signature raw body valid, didapat %d: %s", wValid.Code, wValid.Body.String())
	}
}
