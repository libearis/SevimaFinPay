# Sistem Pembayaran UKT Multi-Institusi via Webhook (SEVIMA Project M2)

Aplikasi web *full-stack* multi-institusi (multi-kampus) untuk mengelola tagihan Uang Kuliah Tunggal (UKT) mahasiswa per semester serta menerima notifikasi pembayaran secara *real-time* dari *payment gateway* pihak ketiga melalui **Webhook** berkeamanan **HMAC-SHA256**.

Dibangun menggunakan **Go (Gin)**, **Vue 3 (Pinia + TypeScript)**, dan **PostgreSQL 16** (dengan arsitektur *instance* database terpisah sebagai *best practice*, maupun opsi *full Docker Compose*).

---

## 📚 Daftar Dokumen Artefak Proyek

| File Dokumen | Deskripsi |
|---|---|
| [AGENTS.md](file:///c:/Project/UKT%20Test/AGENTS.md) | Konfigurasi konteks AI, aturan bisnis multi-institusi, konvensi kode, batasan AI, dan referensi API (sesuai template SEVIMA). |
| [DECISIONS.md](file:///c:/Project/UKT%20Test/DECISIONS.md) | 5 Keputusan teknis penting, alternatif, alasan, dan penanganan kasus pembayaran dunia nyata (sesuai template SEVIMA). |
| [AI_NOTES.md](file:///c:/Project/UKT%20Test/AI_NOTES.md) | Refleksi kolaborasi *pair programming* bersama AI (sesuai template SEVIMA). |
| [ARD.md](file:///c:/Project/UKT%20Test/ARD.md) | Architecture & Requirements Document (ERD, State Machine Tagihan, Sequence Diagram Webhook). |
| [WEBHOOK_DOCS.md](file:///c:/Project/UKT%20Test/WEBHOOK_DOCS.md) | Dokumentasi lengkap endpoint Webhook (`POST /api/webhooks/payment`) & panduan **Mock Payment Gateway**. |

---

## 👥 Akun Demo (3 Institusi & Setiap Role)

Saat pertama kali dijalankan, sistem secara otomatis melakukan *migrasi* dan *seeding* data untuk **3 institusi kampus**, masing-masing dilengkapi akun **Admin Keuangan** dan **Mahasiswa** beserta contoh tagihan UKT:

> **Password untuk seluruh akun demo**: `KampusMerdeka2026!`

### 1. Universitas Nusantara Raya (`UNIV-NUSANTARA`)
| Role | Email Login | NIM / NIP | Nama |
|---|---|---|---|
| **Admin Keuangan** | `admin@nusantara.ac.id` | `NIP-19850101` | Budi Santoso |
| **Mahasiswa 1** | `andi@mhs.nusantara.ac.id` | `202610001` | Andi Pratama |
| **Mahasiswa 2** | `siti@mhs.nusantara.ac.id` | `202610002` | Siti Aminah |

### 2. Institut Teknologi Maju Indonesia (`INST-TEKNOLOGI-MAJU`)
| Role | Email Login | NIM / NIP | Nama |
|---|---|---|---|
| **Admin Keuangan** | `admin@itmaju.ac.id` | `NIP-19870202` | Dewi Lestari |
| **Mahasiswa 1** | `rizky@mhs.itmaju.ac.id` | `202620001` | Rizky Ramadhan |
| **Mahasiswa 2** | `nadia@mhs.itmaju.ac.id` | `202620002` | Nadia Putri |

### 3. Universitas Merdeka Sejahtera (`UNIV-MERDEKA`)
| Role | Email Login | NIM / NIP | Nama |
|---|---|---|---|
| **Admin Keuangan** | `admin@merdeka.ac.id` | `NIP-19900303` | Hendra Wijaya |
| **Mahasiswa 1** | `fajar@mhs.merdeka.ac.id` | `202630001` | Fajar Nugroho |

---

## 🚀 Cara Setup & Menjalankan Aplikasi

### Tahap 1: Konfigurasi Environment (`.env`)

Salin file `.env.example` menjadi `.env`:

```bash
cp .env.example .env
```

Isi variabel `HMAC_SECRET_INST_1`, `HMAC_SECRET_INST_2`, dan `HMAC_SECRET_INST_3` pada file `.env` sesuai dengan nilai *secret* HMAC yang dikirimkan terpisah melalui email (atau gunakan nilai pengembangan lokal Anda).

---

### Opsi A — Menjalankan Aplikasi Lokal dengan Instance PostgreSQL Terpisah

Sebagai *best practice*, database PostgreSQL dijalankan pada instance terpisah (mis. PostgreSQL 16/17 lokal di `localhost:5432`, database `ukt`). Saat pertama kali dijalankan, backend Go akan **otomatis membuat database `ukt` jika belum ada**, menjalankan migrasi SQL, dan melakukan *seeding* 3 institusi:

1. **Jalankan Backend (Go + Gin)**:
   ```bash
   cd backend
   go mod download
   go run ./cmd/api
   ```
   *Server API berjalan di `http://localhost:8080`.*
2. **Jalankan Frontend (Vue 3 + Pinia + Vite)**:
   ```bash
   cd frontend
   npm install
   npm run dev
   ```
   *Buka aplikasi web di `http://localhost:5173`.*

---

### Opsi B — Menjalankan Stateless Application Containers (`docker compose`) yang Terhubung ke PostgreSQL Host

`docker-compose.yml` mengemas layanan *stateless* (`ukt_backend` dan `ukt_frontend`) yang langsung terhubung ke instance PostgreSQL di mesin host melalui `host.docker.internal:5432`:

```bash
cp .env.example .env
docker compose up --build
```

- **Aplikasi Web (Frontend)**: [http://localhost:3000](http://localhost:3000)
- **Endpoint API & Webhook**: [http://localhost:3000/api](http://localhost:3000/api)

---

## 🧪 Menjalankan Test Otomatis

Untuk menjalankan seluruh unit test logika bisnis inti, verifikasi signature HMAC per institusi, idempotensi webhook, dan pengujian **isolasi data antar-institusi**:

```bash
cd backend
go test ./... -race -cover -v
```

---

## 📌 Daftar Asumsi Teknis & Bisnis

Untuk hal-hal yang tidak dirinci secara eksplisit di dalam soal, proyek ini menggunakan asumsi yang terdokumentasi berikut:

1. **Mata Uang & Satuan Nominal**:
   - Seluruh tagihan UKT menggunakan mata uang **Rupiah (IDR)** dan disimpan sebagai bilangan bulat `BIGINT` (`int64`) tanpa nilai desimal untuk menghindari ketidakakuratan *floating-point*.
2. **Aturan Kelayakan KRS**:
   - Mahasiswa dinyatakan **Dapat Mengisi KRS (Aktif)** untuk suatu semester apabila tagihan UKT pada semester tersebut telah berstatus `PAID` (Lunas).
3. **Satu Tagihan per Mahasiswa per Semester**:
   - Di dalam satu institusi, seorang mahasiswa hanya memiliki maksimal satu tagihan UKT aktif untuk kode semester yang sama (`UNIQUE (institusi_id, mahasiswa_id, semester)`).
4. **Pembayaran Penuh (Full Payment)**:
   - Pembayaran UKT melalui payment gateway diasumsikan bersifat pembayaran penuh (*single full payment*). Apabila payload `payment.success` memiliki nilai `amount` yang berbeda dari nominal tagihan, sistem menolak pembaruan status (`422 AMOUNT_MISMATCH`) dan mencatatnya di log audit.
5. **Penanganan Late Settlement (`EXPIRED` → `PAID`)**:
   - Jika tagihan sempat berstatus `EXPIRED` namun *payment gateway* mengirimkan event `payment.success` dengan signature HMAC valid dan nominal yang sesuai (uang mahasiswa benar-benar berhasil dikliring oleh gateway), sistem memperbarui status menjadi `PAID` agar mahasiswa tidak dirugikan saat pengisian KRS, serta menandainya sebagai `PROCESSED_LATE_SETTLEMENT` pada log audit.
6. **Penanganan Out-of-Order Webhook (`PAID` menerima `payment.expired`)**:
   - Status `PAID` bersifat final (*terminal state*). Notifikasi `payment.expired` yang datang terlambat untuk tagihan yang sudah `PAID` tidak akan membatalkan status lunas tagihan tersebut, namun tetap dicatat di `webhook_logs` sebagai `IGNORED_ALREADY_PAID` dan merespons `200 OK`.
