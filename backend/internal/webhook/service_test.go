package webhook

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	"ukt-payment-system/internal/institusi"
	"ukt-payment-system/internal/tagihan"
)

type fakeInstRepo struct {
	byCode map[string]institusi.Institution
}

func (f *fakeInstRepo) GetByCode(_ context.Context, code string) (*institusi.Institution, error) {
	inst, ok := f.byCode[code]
	if !ok {
		return nil, institusi.ErrInstitutionNotFound
	}
	return &inst, nil
}

func (f *fakeInstRepo) GetByID(_ context.Context, id uuid.UUID) (*institusi.Institution, error) {
	for _, inst := range f.byCode {
		if inst.ID == id {
			return &inst, nil
		}
	}
	return nil, institusi.ErrInstitutionNotFound
}

func (f *fakeInstRepo) ListAll(_ context.Context) ([]institusi.Institution, error) {
	var res []institusi.Institution
	for _, inst := range f.byCode {
		res = append(res, inst)
	}
	return res, nil
}

type fakeInvoiceState struct {
	InstitusiID  uuid.UUID
	NomorTagihan string
	Nominal      int64
	Status       string
}

type fakeWebhookRepo struct {
	mu              sync.Mutex
	invoices        map[string]*fakeInvoiceState // key: institusiID.String() + "|" + nomorTagihan
	processedEvents map[string]bool              // key: institusiID.String() + "|" + eventID
	txCount         map[string]int               // key: institusiID.String() + "|" + nomorTagihan -> count of transactions
	auditLogs       []WebhookAuditLog
}

func newFakeWebhookRepo() *fakeWebhookRepo {
	return &fakeWebhookRepo{
		invoices:        make(map[string]*fakeInvoiceState),
		processedEvents: make(map[string]bool),
		txCount:         make(map[string]int),
		auditLogs:       []WebhookAuditLog{},
	}
}

func (r *fakeWebhookRepo) invKey(instID uuid.UUID, nomor string) string {
	return instID.String() + "|" + nomor
}

func (r *fakeWebhookRepo) evtKey(instID uuid.UUID, eventID string) string {
	return instID.String() + "|" + eventID
}

func (r *fakeWebhookRepo) ProcessPaymentEventAtomic(
	_ context.Context,
	institusiID uuid.UUID,
	payload PaymentWebhookPayload,
	_ []byte,
) (*AtomicProcessOutcome, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	inv, ok := r.invoices[r.invKey(institusiID, payload.NomorTagihan)]
	if !ok {
		return nil, ErrInvoiceNotFoundInInstitution
	}

	if r.processedEvents[r.evtKey(institusiID, payload.EventID)] {
		return &AtomicProcessOutcome{
			InvoiceStatus:    inv.Status,
			ProcessingResult: ResultIdempotentReplay,
			Message:          "Webhook dengan event_id ini sudah pernah diproses sebelumnya",
		}, nil
	}

	switch payload.EventType {
	case EventPaymentSuccess:
		if inv.Status == tagihan.StatusPaid {
			return &AtomicProcessOutcome{
				InvoiceStatus:    tagihan.StatusPaid,
				ProcessingResult: ResultIdempotentReplay,
				Message:          "Tagihan sudah berstatus PAID",
			}, nil
		}
		if payload.Amount != inv.Nominal {
			return nil, ErrAmountMismatch
		}

		prevStatus := inv.Status
		inv.Status = tagihan.StatusPaid
		r.processedEvents[r.evtKey(institusiID, payload.EventID)] = true
		r.txCount[r.invKey(institusiID, payload.NomorTagihan)]++

		if prevStatus == tagihan.StatusExpired {
			return &AtomicProcessOutcome{
				InvoiceStatus:    tagihan.StatusPaid,
				ProcessingResult: ResultProcessedLateSettlement,
				Message:          "Late settlement berhasil diproses",
			}, nil
		}
		return &AtomicProcessOutcome{
			InvoiceStatus:    tagihan.StatusPaid,
			ProcessingResult: ResultProcessed,
			Message:          "Status tagihan berhasil diperbarui menjadi PAID",
		}, nil

	case EventPaymentExpired:
		if inv.Status == tagihan.StatusPaid {
			return &AtomicProcessOutcome{
				InvoiceStatus:    tagihan.StatusPaid,
				ProcessingResult: ResultIgnoredAlreadyPaid,
				Message:          "Diabaikan karena sudah PAID",
			}, nil
		}
		if inv.Status == tagihan.StatusExpired {
			return &AtomicProcessOutcome{
				InvoiceStatus:    tagihan.StatusExpired,
				ProcessingResult: ResultIdempotentReplay,
				Message:          "Sudah EXPIRED",
			}, nil
		}

		inv.Status = tagihan.StatusExpired
		r.processedEvents[r.evtKey(institusiID, payload.EventID)] = true
		r.txCount[r.invKey(institusiID, payload.NomorTagihan)]++
		return &AtomicProcessOutcome{
			InvoiceStatus:    tagihan.StatusExpired,
			ProcessingResult: ResultProcessed,
			Message:          "Status tagihan diperbarui menjadi EXPIRED",
		}, nil
	}

	return nil, ErrInvoiceNotFoundInInstitution
}

func (r *fakeWebhookRepo) InsertAuditLog(_ context.Context, logEntry WebhookAuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auditLogs = append(r.auditLogs, logEntry)
	return nil
}

func (r *fakeWebhookRepo) ListRecentAuditLogs(_ context.Context, _ string, _ int) ([]WebhookAuditLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.auditLogs, nil
}

func (r *fakeWebhookRepo) ListSimulatorInvoices(_ context.Context, _ string) ([]SimulatorInvoiceOption, error) {
	return nil, nil
}

func TestWebhookHMACVerification_MultiInstitutionAndAuditLog(t *testing.T) {
	ctx := context.Background()

	instAID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	instBID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	instRepo := &fakeInstRepo{
		byCode: map[string]institusi.Institution{
			"UNIV-NUSANTARA": {
				ID:               instAID,
				Code:             "UNIV-NUSANTARA",
				Name:             "Universitas Nusantara Raya",
				HMACSecretEnvKey: "HMAC_SECRET_INST_1",
			},
			"INST-TEKNOLOGI-MAJU": {
				ID:               instBID,
				Code:             "INST-TEKNOLOGI-MAJU",
				Name:             "Institut Teknologi Maju Indonesia",
				HMACSecretEnvKey: "HMAC_SECRET_INST_2",
			},
		},
	}

	secrets := map[string]string{
		"HMAC_SECRET_INST_1": "test_secret_kampus_a_12345",
		"HMAC_SECRET_INST_2": "test_secret_kampus_b_67890",
	}

	webhookRepo := newFakeWebhookRepo()
	// Kedua kampus sama-sama memiliki nomor tagihan "UKT/2026-1/001" dengan nominal berbeda
	webhookRepo.invoices[webhookRepo.invKey(instAID, "UKT/2026-1/001")] = &fakeInvoiceState{
		InstitusiID:  instAID,
		NomorTagihan: "UKT/2026-1/001",
		Nominal:      5500000,
		Status:       tagihan.StatusUnpaid,
	}
	webhookRepo.invoices[webhookRepo.invKey(instBID, "UKT/2026-1/001")] = &fakeInvoiceState{
		InstitusiID:  instBID,
		NomorTagihan: "UKT/2026-1/001",
		Nominal:      7000000,
		Status:       tagihan.StatusUnpaid,
	}

	svc := NewService(webhookRepo, instRepo, func(envKey string) string {
		return secrets[envKey]
	})

	payloadKampusA := []byte(`{"event_id":"evt_001","event_type":"payment.success","institution_code":"UNIV-NUSANTARA","nomor_tagihan":"UKT/2026-1/001","amount":5500000,"payment_channel":"VA_MANDIRI","timestamp":"2026-10-28T10:15:30Z"}`)

	// Skenario 1: Menggunakan secret milik Kampus B untuk menandatangani webhook Kampus A -> WAJIB DITOLAK 401 INVALID_SIGNATURE
	wrongInstSignature := ComputeHMACSHA256(secrets["HMAC_SECRET_INST_2"], payloadKampusA)
	_, wErr := svc.ProcessIncomingWebhook(ctx, payloadKampusA, wrongInstSignature)
	if wErr == nil || wErr.HTTPStatus != http.StatusUnauthorized || wErr.Code != "INVALID_SIGNATURE" {
		t.Fatalf("seharusnya ditolak 401 INVALID_SIGNATURE saat memakai secret institusi lain, didapat: %v", wErr)
	}
	// Pastikan tagihan Kampus A masih UNPAID dan log kegagalan tercatat di webhook_logs
	if webhookRepo.invoices[webhookRepo.invKey(instAID, "UKT/2026-1/001")].Status != tagihan.StatusUnpaid {
		t.Fatal("status tagihan Kampus A tidak boleh berubah saat signature tidak valid")
	}
	if len(webhookRepo.auditLogs) != 1 || webhookRepo.auditLogs[0].SignatureValid != false || webhookRepo.auditLogs[0].HTTPStatus != http.StatusUnauthorized {
		t.Fatal("kegagalan verifikasi signature wajib tercatat di webhook_logs")
	}

	// Skenario 2: Menggunakan secret yang benar milik Kampus A -> SUKSES 200 OK & Tagihan Kampus A menjadi PAID
	validSignatureA := ComputeHMACSHA256(secrets["HMAC_SECRET_INST_1"], payloadKampusA)
	res, wErr := svc.ProcessIncomingWebhook(ctx, payloadKampusA, validSignatureA)
	if wErr != nil {
		t.Fatalf("seharusnya webhook valid Kampus A berhasil diproses: %v", wErr)
	}
	if res.InvoiceStatus != tagihan.StatusPaid || res.ProcessingResult != ResultProcessed {
		t.Fatalf("status harus PAID dan PROCESSED, didapat %+v", res)
	}
	// Pastikan tagihan dengan nomor yang sama di Kampus B TIDAK ikut berubah (Tetap UNPAID)
	if webhookRepo.invoices[webhookRepo.invKey(instBID, "UKT/2026-1/001")].Status != tagihan.StatusUnpaid {
		t.Fatal("pelanggaran isolasi data: tagihan Kampus B ikut berubah saat memproses webhook Kampus A!")
	}

	// Skenario 3: Pengiriman ulang (Retry / Duplicate Webhook) dengan payload yang sama -> Idempotent 200 OK tanpa menggandakan transaksi
	resReplay, wErr := svc.ProcessIncomingWebhook(ctx, payloadKampusA, validSignatureA)
	if wErr != nil {
		t.Fatalf("retry idempotent seharusnya mengembalikan 200 OK, bukan error: %v", wErr)
	}
	if resReplay.ProcessingResult != ResultIdempotentReplay {
		t.Errorf("diharapkan processing_result = IDEMPOTENT_REPLAY, didapat %s", resReplay.ProcessingResult)
	}
	if webhookRepo.txCount[webhookRepo.invKey(instAID, "UKT/2026-1/001")] != 1 {
		t.Errorf("jumlah transaksi tercatat tidak boleh bertambah saat retry, didapat %d", webhookRepo.txCount[webhookRepo.invKey(instAID, "UKT/2026-1/001")])
	}

	// Skenario 4: Out-of-Order Event (payment.expired tiba terlambat setelah tagihan berstatus PAID) -> Diabaikan, tetap PAID
	payloadExpiredLate := []byte(`{"event_id":"evt_002_late_expired","event_type":"payment.expired","institution_code":"UNIV-NUSANTARA","nomor_tagihan":"UKT/2026-1/001","amount":5500000,"payment_channel":"VA_MANDIRI","timestamp":"2026-10-28T10:16:00Z"}`)
	sigExpiredLate := ComputeHMACSHA256(secrets["HMAC_SECRET_INST_1"], payloadExpiredLate)
	resOutOfOrder, wErr := svc.ProcessIncomingWebhook(ctx, payloadExpiredLate, sigExpiredLate)
	if wErr != nil {
		t.Fatalf("out-of-order expired pada tagihan PAID seharusnya 200 OK: %v", wErr)
	}
	if resOutOfOrder.InvoiceStatus != tagihan.StatusPaid || resOutOfOrder.ProcessingResult != ResultIgnoredAlreadyPaid {
		t.Fatalf("tagihan PAID tidak boleh berubah menjadi EXPIRED oleh event terlambat, didapat: %+v", resOutOfOrder)
	}

	// Skenario 5: Nominal tidak sesuai (Amount Mismatch) pada Kampus B -> Ditolak 422 AMOUNT_MISMATCH
	payloadMismatchB := []byte(`{"event_id":"evt_b_mismatch","event_type":"payment.success","institution_code":"INST-TEKNOLOGI-MAJU","nomor_tagihan":"UKT/2026-1/001","amount":100000,"payment_channel":"QRIS","timestamp":"2026-10-28T11:00:00Z"}`)
	sigMismatchB := ComputeHMACSHA256(secrets["HMAC_SECRET_INST_2"], payloadMismatchB)
	_, wErr = svc.ProcessIncomingWebhook(ctx, payloadMismatchB, sigMismatchB)
	if wErr == nil || wErr.HTTPStatus != http.StatusUnprocessableEntity || wErr.Code != "AMOUNT_MISMATCH" {
		t.Fatalf("pembayaran dengan nominal tidak sesuai harus ditolak 422 AMOUNT_MISMATCH, didapat: %v", wErr)
	}

	// Skenario 6: Late Settlement (UNPAID -> EXPIRED -> PAID ketika dana kliring sah masuk) pada Kampus B
	payloadExpireB := []byte(`{"event_id":"evt_b_expire","event_type":"payment.expired","institution_code":"INST-TEKNOLOGI-MAJU","nomor_tagihan":"UKT/2026-1/001","amount":7000000,"payment_channel":"VA_BNI","timestamp":"2026-10-28T12:00:00Z"}`)
	sigExpireB := ComputeHMACSHA256(secrets["HMAC_SECRET_INST_2"], payloadExpireB)
	resExpB, wErr := svc.ProcessIncomingWebhook(ctx, payloadExpireB, sigExpireB)
	if wErr != nil || resExpB.InvoiceStatus != tagihan.StatusExpired {
		t.Fatalf("tagihan Kampus B seharusnya berubah menjadi EXPIRED, didapat: %v", wErr)
	}

	payloadLateSettleB := []byte(`{"event_id":"evt_b_latesettle","event_type":"payment.success","institution_code":"INST-TEKNOLOGI-MAJU","nomor_tagihan":"UKT/2026-1/001","amount":7000000,"payment_channel":"VA_BNI","timestamp":"2026-10-28T12:05:00Z"}`)
	sigLateSettleB := ComputeHMACSHA256(secrets["HMAC_SECRET_INST_2"], payloadLateSettleB)
	resLateB, wErr := svc.ProcessIncomingWebhook(ctx, payloadLateSettleB, sigLateSettleB)
	if wErr != nil || resLateB.InvoiceStatus != tagihan.StatusPaid || resLateB.ProcessingResult != ResultProcessedLateSettlement {
		t.Fatalf("late settlement pada tagihan EXPIRED dengan nominal & HMAC valid harus memperbarui status ke PAID, didapat: %+v (%v)", resLateB, wErr)
	}

	// Pastikan seluruh 7 percobaan di atas tercatat di tabel audit log (webhook_logs)
	if len(webhookRepo.auditLogs) != 7 {
		t.Fatalf("seluruh 7 request webhook (sukses maupun gagal) wajib tercatat di webhook_logs, tercatat: %d", len(webhookRepo.auditLogs))
	}
}
