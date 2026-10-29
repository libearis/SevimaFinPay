package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"ukt-payment-system/internal/config"
	"ukt-payment-system/internal/webhook"
)

func main() {
	targetURL := flag.String("url", "http://localhost:8080/api/webhooks/payment", "URL endpoint webhook pembayaran")
	institution := flag.String("institution", "UNIV-NUSANTARA", "Kode institusi (UNIV-NUSANTARA | INST-TEKNOLOGI-MAJU | UNIV-MERDEKA)")
	invoice := flag.String("invoice", "UKT/2026-1/001", "Nomor tagihan UKT")
	eventType := flag.String("event", "payment.success", "Jenis event (payment.success | payment.expired)")
	amount := flag.Int64("amount", 5500000, "Nominal pembayaran dalam Rupiah bulat")
	channel := flag.String("channel", "VA_MANDIRI", "Kanal pembayaran")
	eventID := flag.String("event-id", "", "ID event opsional (untuk menguji idempotensi)")
	secretOverride := flag.String("secret", "", "Override secret HMAC secara manual (opsional)")
	tamperSig := flag.Bool("invalid-sig", false, "Simulasikan signature tidak valid (untuk menguji 401 & webhook_logs)")
	flag.Parse()

	cfg, _ := config.Load()

	evtID := *eventID
	if evtID == "" {
		evtID = fmt.Sprintf("evt_cli_%d", time.Now().UTC().UnixMilli())
	}

	payload := webhook.PaymentWebhookPayload{
		EventID:         evtID,
		EventType:       *eventType,
		InstitutionCode: *institution,
		NomorTagihan:    *invoice,
		Amount:          *amount,
		PaymentChannel:  *channel,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	rawBody, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Gagal membuat JSON payload: %v", err)
	}

	secret := *secretOverride
	if secret == "" && cfg != nil {
		switch *institution {
		case "UNIV-NUSANTARA":
			secret = cfg.HMACSecretInst1
		case "INST-TEKNOLOGI-MAJU":
			secret = cfg.HMACSecretInst2
		case "UNIV-MERDEKA":
			secret = cfg.HMACSecretInst3
		}
	}
	if *tamperSig {
		secret = "tampered_invalid_secret_key"
	}

	signature := webhook.ComputeHMACSHA256(secret, rawBody)

	fmt.Println("==================================================================")
	fmt.Println(" MOCK PAYMENT GATEWAY CLI — WEBHOOK SENDER")
	fmt.Println("==================================================================")
	fmt.Printf("Target URL   : %s\n", *targetURL)
	fmt.Printf("Institution  : %s\n", *institution)
	fmt.Printf("X-Signature  : %s\n", signature)
	fmt.Printf("Raw Payload  : %s\n", string(rawBody))
	fmt.Println("------------------------------------------------------------------")

	req, err := http.NewRequest(http.MethodPost, *targetURL, bytes.NewReader(rawBody))
	if err != nil {
		log.Fatalf("Gagal menyiapkan HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal menghubungi server API: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("HTTP Status  : %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	fmt.Printf("Response Body: %s\n", string(respBytes))
	fmt.Println("==================================================================")
}
