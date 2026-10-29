# Dokumentasi Endpoint Webhook & Mock Payment Gateway (WEBHOOK_DOCS.md)

Dokumen ini menjelaskan spesifikasi teknis endpoint penerima *webhook* pembayaran UKT serta panduan penggunaan **Mock Payment Gateway** untuk memicu notifikasi `payment.success` dan `payment.expired` pada ketiga institusi yang tersedia di data *seed*.

---

## 1. Spesifikasi Endpoint Webhook

- **URL Endpoint**: `POST /api/webhooks/payment`
- **Content-Type**: `application/json`
- **Autentikasi**: Verifikasi signature `HMAC-SHA256` melalui header HTTP `X-Signature`

### Header Wajib

| Nama Header | Wajib | Contoh Nilai | Deskripsi |
|---|---|---|---|
| `Content-Type` | Ya | `application/json` | Format body request harus berupa JSON. |
| `X-Signature` | Ya | `8f4e2b9a1c3d...` (64 karakter hex) | Nilai hash `HMAC-SHA256` dalam format *lowercase hexadecimal* yang dihitung dari **raw request body** menggunakan *secret key* milik institusi terkait. |

### Rumus Perhitungan Signature (`X-Signature`)

```text
X-Signature = Hex( HMAC-SHA256( key = INSTITUTION_HMAC_SECRET, message = RAW_JSON_BODY ) )
```

> **Catatan Penting**: Signature dihitung langsung terhadap *raw byte* dari body HTTP yang dikirimkan. Jangan mengubah spasi atau urutan field JSON setelah menghitung signature.

---

## 2. Jenis Event & Format Payload Request

Sistem mendukung dua jenis event sesuai spesifikasi:
1. `payment.success` — Pembayaran UKT berhasil diselesaikan oleh mahasiswa melalui kanal pembayaran.
2. `payment.expired` — Masa berlaku pembayaran/Virtual Account telah habis tanpa pembayaran.

### Skema Field Payload

| Field | Tipe | Wajib | Deskripsi |
|---|---|---|---|
| `event_id` | `string` | Ya | ID unik notifikasi dari payment gateway (digunakan sebagai kunci idempotensi). |
| `event_type` | `string` | Ya | Jenis event: `payment.success` atau `payment.expired`. |
| `institution_code` | `string` | Ya | Kode unik institusi kampus (`UNIV-NUSANTARA`, `INST-TEKNOLOGI-MAJU`, atau `UNIV-MERDEKA`). |
| `nomor_tagihan` | `string` | Ya | Nomor tagihan UKT pada institusi tersebut (misal: `UKT/2026-1/001`). |
| `amount` | `integer` (`int64`) | Ya | Nominal transaksi dalam Rupiah bulat (wajib sama dengan nominal tagihan untuk `payment.success`). |
| `payment_channel` | `string` | Tidak | Kanal pembayaran (misal: `VA_MANDIRI`, `VA_BRI`, `QRIS`). |
| `timestamp` | `string` (ISO-8601) | Ya | Waktu terjadinya event pada payment gateway (format RFC3339, misal: `2026-10-28T10:15:30Z`). |

### Contoh Payload 1: `payment.success`

```json
{
  "event_id": "evt_20261028_0001",
  "event_type": "payment.success",
  "institution_code": "UNIV-NUSANTARA",
  "nomor_tagihan": "UKT/2026-1/001",
  "amount": 5500000,
  "payment_channel": "VA_MANDIRI",
  "timestamp": "2026-10-28T10:15:30Z"
}
```

### Contoh Payload 2: `payment.expired`

```json
{
  "event_id": "evt_20261028_0002",
  "event_type": "payment.expired",
  "institution_code": "INST-TEKNOLOGI-MAJU",
  "nomor_tagihan": "UKT/2026-1/001",
  "amount": 7000000,
  "payment_channel": "VA_BNI",
  "timestamp": "2026-10-28T23:59:59Z"
}
```

---

## 3. Format Respons & Kode Status HTTP

### Respons Sukses (`200 OK`)

Dikembalikan apabila webhook berhasil diverifikasi dan diproses (termasuk apabila merupakan *retry* idempotent atau event *expired* yang diabaikan karena tagihan sudah berstatus `PAID`):

```json
{
  "status": "ok",
  "data": {
    "event_id": "evt_20261028_0001",
    "institution_code": "UNIV-NUSANTARA",
    "nomor_tagihan": "UKT/2026-1/001",
    "invoice_status": "PAID",
    "processing_result": "PROCESSED",
    "message": "Status tagihan berhasil diperbarui menjadi PAID"
  }
}
```

Nilai `processing_result` yang mungkin pada `200 OK`:
- `PROCESSED`: Status tagihan berhasil diperbarui dan riwayat transaksi baru telah disimpan.
- `PROCESSED_LATE_SETTLEMENT`: Tagihan yang sebelumnya `EXPIRED` diperbarui menjadi `PAID` karena dana pembayaran berhasil diverifikasi masuk.
- `IDEMPOTENT_REPLAY`: Webhook dengan `event_id` ini (atau pembayaran pada tagihan yang sudah lunas) pernah diproses sebelumnya; tidak ada duplikasi transaksi.
- `IGNORED_ALREADY_PAID`: Event `payment.expired` diterima untuk tagihan yang sudah `PAID`; diabaikan demi menjaga keakuratan status lunas mahasiswa.

### Respons Error (`4xx`)

Seluruh respons error menggunakan struktur konsisten dan **tetap tercatat** di tabel `webhook_logs` untuk keperluan audit:

```json
{
  "error": {
    "code": "INVALID_SIGNATURE",
    "message": "Signature HMAC-SHA256 tidak sesuai dengan secret institusi UNIV-NUSANTARA"
  }
}
```

| Kode HTTP | Kode Error | Penyebab |
|---|---|---|
| `400 Bad Request` | `INVALID_PAYLOAD` | JSON tidak valid, field wajib kosong, atau `event_type` tidak dikenali. |
| `401 Unauthorized` | `MISSING_SIGNATURE` | Header `X-Signature` tidak disertakan pada request. |
| `401 Unauthorized` | `INVALID_SIGNATURE` | Hash `X-Signature` tidak cocok dengan hasil perhitungan HMAC-SHA256 secret institusi terkait. |
| `404 Not Found` | `INSTITUTION_NOT_FOUND` | `institution_code` tidak terdaftar di dalam sistem. |
| `404 Not Found` | `INVOICE_NOT_FOUND` | `nomor_tagihan` tidak ditemukan pada institusi yang bersangkutan. |
| `422 Unprocessable Entity` | `AMOUNT_MISMATCH` | Nilai `amount` pada event `payment.success` tidak sesuai dengan nominal tagihan UKT. |

---

## 4. Data Seed Tiga Institusi

Sesuai ketentuan teknis, sistem menyertakan *seed data* awal untuk **3 institusi kampus** berbeda:

| No | Kode Institusi (`institution_code`) | Nama Kampus | Variabel Environment Secret HMAC |
|---|---|---|---|
| 1 | `UNIV-NUSANTARA` | Universitas Nusantara Raya | `HMAC_SECRET_INST_1` |
| 2 | `INST-TEKNOLOGI-MAJU` | Institut Teknologi Maju Indonesia | `HMAC_SECRET_INST_2` |
| 3 | `UNIV-MERDEKA` | Universitas Merdeka Sejahtera | `HMAC_SECRET_INST_3` |

> **Keamanan Secret**: Nilai asli dari `HMAC_SECRET_INST_1`, `HMAC_SECRET_INST_2`, dan `HMAC_SECRET_INST_3` tidak disimpan di dalam repositori Git, melainkan dikonfigurasi melalui `.env` dan dikirimkan terpisah melalui email pengumpulan.

---

## 5. Cara Memicu Webhook dari Mock Payment Gateway

Terdapat **3 cara** untuk memicu webhook pada sistem ini:

### Cara 1: Melalui Halaman UI Mock Payment Gateway Simulator (Paling Mudah)
1. Buka aplikasi frontend (`/mock-gateway`).
2. Pilih **Institusi**, **Nomor Tagihan**, dan **Jenis Event** (`payment.success` atau `payment.expired`).
3. Pilih mode pengiriman:
   - **Valid Signature (Otomatis)**: Simulator menghitung `X-Signature` yang sah sesuai secret institusi terpilih.
   - **Simulasi Signature Salah (Uji Keamanan/Audit Log)**: Mengirimkan signature yang sengaja disalahkan untuk menguji penolakan `401 INVALID_SIGNATURE` dan pencatatan di `webhook_logs`.
   - **Simulasi Pengiriman Ulang / Retry (Uji Idempotensi)**: Mengirimkan `event_id` yang sama dua kali berturut-turut.
4. Klik **Kirim Webhook Sekarang** dan amati respons HTTP serta perubahan status tagihan secara langsung.

### Cara 2: Melalui CLI Mock Gateway (Go Command)
Jalankan perintah CLI bawaan di folder `backend`:

```bash
cd backend
go run ./cmd/mockgateway \
  -institution="UNIV-NUSANTARA" \
  -invoice="UKT/2026-1/001" \
  -event="payment.success" \
  -amount=5500000
```

### Cara 3: Menggunakan `curl` + `openssl` di Terminal (Bash / PowerShell)

**Contoh di Bash / Git Bash / WSL:**

```bash
SECRET="isi_sesuai_env_HMAC_SECRET_INST_1"
PAYLOAD='{"event_id":"evt_demo_001","event_type":"payment.success","institution_code":"UNIV-NUSANTARA","nomor_tagihan":"UKT/2026-1/001","amount":5500000,"payment_channel":"VA_MANDIRI","timestamp":"2026-10-28T10:15:30Z"}'

SIGNATURE=$(printf '%s' "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | awk '{print $2}')

curl -i -X POST http://localhost:8080/api/webhooks/payment \
  -H "Content-Type: application/json" \
  -H "X-Signature: $SIGNATURE" \
  -d "$PAYLOAD"
```
