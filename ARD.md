# Architecture & Requirements Document (ARD.md)
**Project M2: Integrasi Pembayaran UKT Multi-Institusi via Webhook**

Dokumen ini menjabarkan arsitektur sistem, pemodelan data (ERD), diagram alur sekuens (*sequence diagram*), mesin status (*state machine*) tagihan UKT, serta rancangan isolasi keamanan multi-institusi.

---

## 1. Gambaran Umum Arsitektur (System Architecture)

Sistem dibangun menggunakan pola **Decoupled Full-Stack Architecture**:
- **Frontend SPA**: Vue 3 + TypeScript + Pinia + Vue Router.
- **Backend REST API & Webhook Receiver**: Go (Gin) dengan *Modular Layered Architecture*.
- **Database**: **Instance PostgreSQL 16 Terpisah** (*Dedicated/Isolated Stateful Instance*), mengikuti *best practice* pemisahan lapisan komputasi aplikasi (*stateless*) dan penyimpanan data (*stateful*).
- **Mock Payment Gateway**: Layanan/modul simulator pihak ketiga yang memicu *webhook* `payment.success` dan `payment.expired` lengkap dengan signature `HMAC-SHA256` per institusi.

```mermaid
flowchart LR
    subgraph Client["Klien (Browser)"]
        AdminUI["Halaman Admin Keuangan\n(Vue 3 + Pinia)"]
        StudentUI["Halaman Mahasiswa\n(Vue 3 + Pinia)"]
        MockUI["Mock Payment Gateway UI/CLI\n(Webhook Trigger)"]
    end

    subgraph AppServer["Application Layer (Stateless)"]
        API["Go Backend API (Gin)\n- Auth & Tenant Middleware\n- Modul Tagihan UKT\n- Modul Webhook HMAC-SHA256"]
    end

    subgraph DatabaseLayer["Database Layer (Separate Instance - Best Practice)"]
        PG[("PostgreSQL 16 Instance\n- institutions\n- users\n- invoices\n- transactions\n- webhook_logs")]
    end

    AdminUI -->|"JWT (ADMIN_KEUANGAN)\nGET/POST /api/admin/invoices"| API
    StudentUI -->|"JWT (MAHASISWA)\nGET /api/student/invoices"| API
    MockUI -->|"POST /api/webhooks/payment\nHeader: X-Signature (HMAC-SHA256)"| API
    API -->|"TCP / pgxpool\n(Tenant-Scoped Queries & ACID Tx)"| PG
```

---

## 2. Desain Skema Database (Entity-Relationship Diagram)

Seluruh tabel bisnis memiliki relasi langsung ke `institutions` (`institusi_id`) untuk menjamin isolasi data antar-kampus di dalam satu database.

```mermaid
erDiagram
    INSTITUTIONS ||--o{ USERS : "memiliki"
    INSTITUTIONS ||--o{ INVOICES : "memiliki"
    INSTITUTIONS ||--o{ TRANSACTIONS : "memiliki"
    INSTITUTIONS ||--o{ WEBHOOK_LOGS : "menerima"
    USERS ||--o{ INVOICES : "ditagih ke (mahasiswa)"
    INVOICES ||--o{ TRANSACTIONS : "memiliki riwayat"

    INSTITUTIONS {
        uuid id PK
        varchar code UK "Kode unik (mis. UNIV-NUSANTARA)"
        varchar name "Nama Perguruan Tinggi"
        varchar hmac_secret_env_key "Nama variabel env untuk HMAC secret"
        timestamptz created_at
    }

    USERS {
        uuid id PK
        uuid institusi_id FK "Wajib: isolasi kampus"
        varchar nim_nip "NIM mahasiswa atau NIP admin"
        varchar name "Nama lengkap"
        varchar email "Email login (unik per institusi)"
        varchar password_hash "Bcrypt hash"
        varchar role "ADMIN_KEUANGAN | MAHASISWA"
        timestamptz created_at
    }

    INVOICES {
        uuid id PK
        uuid institusi_id FK "Wajib: isolasi kampus"
        uuid mahasiswa_id FK "Referensi ke users.id"
        varchar nomor_tagihan "Unik di dalam institusi_id yang sama"
        varchar semester "Contoh: 2026/2027-GANJIL"
        bigint nominal "Nominal UKT dalam Rupiah (BIGINT > 0)"
        varchar status "UNPAID | PAID | EXPIRED"
        timestamptz tanggal_kedaluwarsa "Batas waktu pembayaran"
        timestamptz paid_at "Waktu lunas (nullable)"
        timestamptz created_at
        timestamptz updated_at
    }

    TRANSACTIONS {
        uuid id PK
        uuid institusi_id FK "Wajib: isolasi kampus"
        uuid invoice_id FK "Referensi ke invoices.id"
        varchar event_id UK "ID unik event dari gateway (Idempotency)"
        varchar event_type "payment.success | payment.expired"
        varchar payment_channel "VA_MANDIRI, QRIS, dll."
        bigint amount "Nominal yang dibayarkan"
        varchar status "SUCCESS | EXPIRED"
        jsonb raw_payload "Salinan payload transaksi"
        timestamptz processed_at
    }

    WEBHOOK_LOGS {
        uuid id PK
        uuid institusi_id FK "Nullable jika kode institusi tidak dikenal"
        varchar institution_code "Kode institusi dari payload"
        varchar nomor_tagihan "Nomor tagihan dari payload"
        varchar event_id "ID event dari payload"
        varchar event_type "payment.success | payment.expired"
        varchar signature_header "Nilai header X-Signature"
        boolean signature_valid "Hasil verifikasi HMAC-SHA256"
        integer http_status "Kode HTTP yang dikembalikan (200, 401, 404, 422)"
        varchar processing_result "PROCESSED | IDEMPOTENT_REPLAY | IGNORED_ALREADY_PAID | REJECTED"
        text error_message "Pesan error jika ditolak (untuk analisis insiden)"
        jsonb raw_payload "Seluruh body JSON yang diterima"
        timestamptz received_at
    }
```

### Constraint Database Penting
1. **`UNIQUE (institusi_id, nomor_tagihan)` pada tabel `invoices`**:
   Memastikan nomor tagihan unik di dalam satu institusi, tetapi **diizinkan sama** pada institusi berbeda.
2. **`UNIQUE (institusi_id, mahasiswa_id, semester)` pada tabel `invoices`**:
   Mencegah pembuatan tagihan ganda untuk mahasiswa yang sama pada semester yang sama di satu institusi.
3. **`CHECK (nominal > 0)` pada tabel `invoices`**:
   Menjamin nominal tagihan selalu bilangan bulat positif.
4. **`UNIQUE (institusi_id, event_id)` pada tabel `transactions`**:
   Menjamin idempotensi di level database agar satu event dari *payment gateway* tidak pernah menghasilkan catatan pembayaran ganda.

---

## 3. State Machine Status Tagihan UKT

Status tagihan mempengaruhi langsung hak akademik mahasiswa dalam mengisi KRS (`status == 'PAID'`). Oleh karena itu, transisi status diatur secara ketat:

```mermaid
stateDiagram-v2
    [*] --> UNPAID : Admin Keuangan membuat tagihan UKT

    UNPAID --> PAID : Webhook payment.success\n(HMAC valid & nominal sesuai)
    UNPAID --> EXPIRED : Webhook payment.expired\n(HMAC valid)

    EXPIRED --> PAID : Webhook payment.success\n(Late Settlement: dana sah masuk di gateway)
    EXPIRED --> EXPIRED : Webhook payment.expired\n(Idempotent replay - 200 OK)

    PAID --> PAID : Webhook payment.success duplikat\n(Idempotent replay - 200 OK, tanpa duplikasi transaksi)
    PAID --> PAID : Webhook payment.expired terlambat (Out-of-Order)\n(Diabaikan agar tidak membatalkan KRS - 200 OK, dicatat di log)
```

### Aturan Transisi Status:
| Status Saat Ini | Event Masuk | Kondisi | Status Baru | Aksi Sistem & Kode HTTP |
|---|---|---|---|---|
| `UNPAID` | `payment.success` | Nominal sesuai | `PAID` | Update tagihan ke `PAID`, tambah baris `transactions`, tulis `webhook_logs` (`200 OK`) |
| `UNPAID` | `payment.success` | Nominal tidak sesuai | `UNPAID` | Tolak perubahan, tulis `webhook_logs` dengan alasan `AMOUNT_MISMATCH` (`422 Unprocessable Entity`) |
| `UNPAID` | `payment.expired` | Valid | `EXPIRED` | Update tagihan ke `EXPIRED`, tambah baris `transactions`, tulis `webhook_logs` (`200 OK`) |
| `PAID` | `payment.success` | Duplikat / Retry | `PAID` | Tidak mengubah tagihan & tidak menduplikasi `transactions`, tulis `webhook_logs` sebagai `IDEMPOTENT_REPLAY` (`200 OK`) |
| `PAID` | `payment.expired` | Out-of-order (terlambat) | `PAID` | Pertahankan `PAID`, tulis `webhook_logs` sebagai `IGNORED_ALREADY_PAID` (`200 OK`) |
| `EXPIRED` | `payment.success` | Late settlement (nominal sesuai) | `PAID` | Update tagihan ke `PAID`, tambah baris `transactions`, tulis `webhook_logs` sebagai `PROCESSED_LATE_SETTLEMENT` (`200 OK`) |

---

## 4. Alur Pemrosesan Webhook & Verifikasi HMAC-SHA256

```mermaid
sequenceDiagram
    autonumber
    participant PGW as Payment Gateway (Mock)
    participant API as Go Webhook Handler & Service
    participant DB as PostgreSQL (Separate Instance)

    PGW->>API: POST /api/webhooks/payment\nHeader: X-Signature: <hmac_hex>\nBody: raw JSON bytes
    API->>API: Baca raw body bytes (MaxBytesReader 1MB)\nParse institution_code & nomor_tagihan
    API->>DB: Cari institusi berdasarkan institution_code
    alt Institusi Tidak Ditemukan
        API->>DB: INSERT webhook_logs (signature_valid=false, http_status=404)
        API-->>PGW: 404 Not Found (INSTITUTION_NOT_FOUND)
    else Institusi Ditemukan
        API->>API: Ambil secret HMAC institusi dari Env\nHitung HMAC-SHA256(secret, raw_body)\nBandingkan dengan hmac.Equal
        alt Signature Tidak Valid
            API->>DB: INSERT webhook_logs (signature_valid=false, http_status=401)
            API-->>PGW: 401 Unauthorized (INVALID_SIGNATURE)
        else Signature Valid
            API->>DB: BEGIN TRANSACTION
            API->>DB: SELECT * FROM invoices\nWHERE institusi_id = $1 AND nomor_tagihan = $2\nFOR UPDATE
            alt Tagihan Tidak Ditemukan di Institusi Ini
                API->>DB: ROLLBACK
                API->>DB: INSERT webhook_logs (signature_valid=true, http_status=404)
                API-->>PGW: 404 Not Found (INVOICE_NOT_FOUND)
            else Tagihan Ditemukan
                API->>API: Evaluasi Idempotensi (event_id) & State Machine
                API->>DB: UPDATE invoices & INSERT transactions (jika belum diproses)
                API->>DB: COMMIT TRANSACTION
                API->>DB: INSERT webhook_logs (signature_valid=true, http_status=200)
                API-->>PGW: 200 OK
            end
        end
    end
```

---

## 5. Matriks Isolasi Hak Akses (RBAC & Multi-Tenancy)

| Aktor | Cara Autentikasi | Sumber `institusi_id` | Hak Akses Data |
|---|---|---|---|
| **Admin Keuangan** | Bearer JWT (`role = ADMIN_KEUANGAN`) | Klaim JWT (`claims.InstitusiID`) | Melihat daftar tagihan, memfilter status, dan membuat tagihan **hanya** untuk mahasiswa di dalam `institusi_id` miliknya. |
| **Mahasiswa** | Bearer JWT (`role = MAHASISWA`) | Klaim JWT (`claims.InstitusiID` & `claims.UserID`) | Melihat status tagihan UKT, status hak pengisian KRS, dan riwayat transaksi **hanya** milik dirinya sendiri di `institusi_id` miliknya. |
| **Payment Gateway** | Header `X-Signature` (`HMAC-SHA256`) | `institution_code` pada payload yang terverifikasi oleh *secret* HMAC institusi tersebut | Memperbarui status tagihan yang berada di bawah `institusi_id` yang sesuai dengan pemilik *secret* HMAC. |
