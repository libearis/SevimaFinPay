# AGENTS.md — Sistem Manajemen Tagihan UKT Multi-Institusi & Integrasi Webhook (SEVIMA Project M2)

Dokumen ini merupakan panduan utama konteks, aturan bisnis, konvensi kode, dan spesifikasi arsitektur untuk AI Coding Assistant (Antigravity) serta pengembang pada proyek **Integrasi Pembayaran UKT Multi-Institusi via Webhook**.

---

## 1. Deskripsi Singkat

Aplikasi web *full-stack* multi-institusi (multi-kampus) untuk mengelola tagihan Uang Kuliah Tunggal (UKT) mahasiswa per semester dan memproses notifikasi pembayaran secara *real-time* dari *payment gateway* pihak ketiga melalui **Webhook** berkeamanan **HMAC-SHA256**.

Sistem ini dirancang khusus untuk memenuhi standar *engineering* kelas produksi pada ekosistem perguruan tinggi:
- **Isolasi data antar-kampus yang ketat (`institusi_id`)** dalam satu database PostgreSQL.
- **Akurasi status pembayaran penentu KRS** (`UNPAID`, `PAID`, `EXPIRED`).
- **Ketahanan terhadap perilaku jaringan/gateway di dunia nyata** (*idempotency*, *out-of-order events*, *late settlement*, *race condition* via `SELECT ... FOR UPDATE`).
- **Jejak audit (*audit trail*) menyeluruh** pada tabel `webhook_logs` untuk investigasi insiden.

---

## 2. Stack dan Cara Menjalankan

| Lapisan | Teknologi |
|---|---|
| **Backend** | Go 1.22+, [Gin](https://github.com/gin-gonic/gin), `golang-jwt/jwt/v5`, `crypto/hmac` + `crypto/sha256`, `bcrypt` |
| **Database** | PostgreSQL 16 (via `jackc/pgx/v5`), *connection pooling*, migrasi SQL otomatis & *seeder* 3 institusi |
| **Frontend** | Vue 3 (Composition API, `<script setup lang="ts">`), TypeScript, Vite, Vue Router, **Pinia**, Axios |
| **Mock Gateway** | Simulator *payment gateway* terintegrasi (UI Simulator `/mock-gateway` & CLI `cmd/mockgateway`) |
| **Container** | Docker, Docker Compose (`docker-compose.db.yml` untuk DB terpisah & `docker-compose.yml` untuk full stack) |
| **Testing** | Go `testing` + `net/http/httptest` (*unit tests* di level `middleware`, `service`, dan `handler`) |

### Cara Menjalankan (Best Practice — Instance PostgreSQL Terpisah + Aplikasi Lokal)

1. Salin konfigurasi environment:
   ```bash
   cp .env.example .env
   ```
   *(Jangan pernah melakukan commit file `.env` atau menuliskan secret produksi di dalam repositori. Secret HMAC disuntikkan melalui variabel `HMAC_SECRET_INST_1`, `HMAC_SECRET_INST_2`, `HMAC_SECRET_INST_3`).*
2. Pastikan instance PostgreSQL berjalan (atau nyalakan container DB terpisah dengan `docker compose -f docker-compose.db.yml up -d`).
3. Jalankan backend Go (otomatis menjalankan migrasi SQL & *seed* 3 institusi beserta akun demo):
   ```bash
   cd backend
   go mod download
   go run ./cmd/api
   ```
4. Jalankan frontend Vue 3 + Pinia:
   ```bash
   cd frontend
   npm install
   npm run dev
   ```
5. Buka `http://localhost:5173` (atau gunakan `docker compose up --build` untuk menjalankan seluruh stack di `http://localhost:3000`).

### Cara Menjalankan Unit Tests

```bash
cd backend
go test ./... -cover -v
```

---

## 3. Arsitektur Singkat & Struktur Proyek

Backend menggunakan **Modular Layered Architecture** (`handler` → `service` → `repository`).

```text
.
├── backend/
│   ├── cmd/
│   │   ├── api/
│   │   │   └── main.go               # Entry point server API, DB pool, migrasi, seeder, graceful shutdown
│   │   └── mockgateway/
│   │       └── main.go               # CLI pemicu webhook mock payment gateway
│   ├── internal/
│   │   ├── config/                   # Konfigurasi & validasi environment variables
│   │   ├── database/
│   │   │   ├── postgres.go           # Koneksi pgxpool ke instance PostgreSQL
│   │   │   ├── migrations/           # File migrasi SQL (institutions, users, invoices, transactions, webhook_logs)
│   │   │   └── seeder.go             # Seed 3 institusi, akun demo admin & mahasiswa, contoh tagihan
│   │   ├── middleware/
│   │   │   ├── auth.go               # Verifikasi JWT, injeksi user_id, role, dan institusi_id ke Context
│   │   │   ├── auth_test.go          # Unit test middleware JWT & RBAC
│   │   │   ├── role.go               # Otorisasi berbasis role (ADMIN_KEUANGAN vs MAHASISWA)
│   │   │   └── cors.go
│   │   ├── auth/                     # Modul autentikasi login & manajemen sesi JWT
│   │   ├── institusi/                # Modul data institusi & resolusi secret HMAC per institusi
│   │   ├── tagihan/                  # Modul pembuatan & pembacaan tagihan UKT + riwayat transaksi
│   │   │   ├── handler.go            # HTTP Controller untuk Admin & Mahasiswa
│   │   │   ├── handler_test.go       # Unit test HTTP Handler & isolasi JWT antar-institusi
│   │   │   ├── service.go            # Logika bisnis pembuatan & pembacaan tagihan
│   │   │   ├── service_test.go       # Unit test logika bisnis & isolasi multi-institusi
│   │   │   ├── repository.go         # Query PostgreSQL terparametrisasi wajib dengan institusi_id
│   │   │   ├── model.go
│   │   │   └── dto.go
│   │   └── webhook/                  # Modul pemrosesan webhook pembayaran & audit logging
│   │       ├── handler.go            # Baca raw body bytes, ekstrak X-Signature, panggil service
│   │       ├── handler_test.go       # Unit test HTTP Handler webhook (200, 400, 401, 422)
│   │       ├── service.go            # Verifikasi HMAC, state machine tagihan, idempotensi, audit log
│   │       ├── service_test.go       # Unit test webhook, HMAC per institusi, idempotensi, state transisi
│   │       ├── repository.go         # Transaksi DB atomik (SELECT FOR UPDATE + insert transaksi + log)
│   │       └── signature.go          # Utilitas HMAC-SHA256 constant-time verification (hmac.Equal)
│   ├── pkg/
│   │   └── response/                 # Envelope JSON standar untuk respons sukses & error
│   ├── Dockerfile
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── api/client.ts                    # Instance Axios dengan interceptor Bearer JWT
│   │   ├── stores/auth.ts                   # Pinia store: sesi user, role, institusi_id
│   │   ├── stores/billing.ts                # Pinia store: state daftar tagihan, filter status, riwayat transaksi
│   │   ├── router/index.ts                  # Route guard berdasarkan autentikasi & role
│   │   ├── views/LoginView.vue              # Halaman login dengan pemilih cepat akun demo 3 institusi
│   │   ├── views/AdminBillingView.vue       # Halaman Daftar Tagihan Admin Keuangan (full-width table & filter)
│   │   ├── views/AdminCreateInvoiceView.vue # Halaman khusus Terbitkan Tagihan UKT Baru (Admin Keuangan)
│   │   ├── views/StudentBillingView.vue     # Dashboard Mahasiswa: status tagihan UKT, status KRS, riwayat transaksi
│   │   └── views/MockGatewayView.vue        # Simulator Payment Gateway & inspektur webhook_logs (diakses dari alur Mahasiswa)
├── docker-compose.yml                       # Menjalankan stateless backend + frontend yang terhubung ke PostgreSQL terpisah
├── .env.example                             # Contoh variabel lingkungan tanpa membocorkan secret asli
├── AGENTS.md                                # Konfigurasi konteks & aturan AI (dokumen ini)
├── DECISIONS.md                             # 5 Keputusan arsitektur & penanganan kasus dunia nyata
├── AI_NOTES.md                              # Refleksi kolaborasi bersama AI sesuai template wajib SEVIMA
├── ARD.md                                   # Architecture & Requirements Document (ERD, State Machine, Sequence)
├── WEBHOOK_DOCS.md                          # Dokumentasi spesifikasi endpoint Webhook & Mock Payment Gateway
└── README.md                                # Panduan setup, akun demo, dan asumsi proyek
```

---

## 4. Aturan Bisnis Penting (Termasuk Multi-Institusi)

1. **Isolasi Data Antar-Institusi Mutlak (`institusi_id`)**:
   - Setiap pengguna (Admin Keuangan maupun Mahasiswa) hanya terikat pada satu `institusi_id`.
   - Pengguna dari Institusi A dilarang keras melihat, membuat, atau memodifikasi data milik Institusi B.
   - `institusi_id` pada operasi pengguna **wajib diambil dari klaim JWT** yang sudah diverifikasi oleh middleware, **tidak pernah** dari body atau query parameter request klien.
   - Seed data bawaan mencakup **3 institusi** berbeda: `UNIV-NUSANTARA`, `INST-TEKNOLOGI-MAJU`, dan `UNIV-MERDEKA`.
2. **Keunikan Nomor Tagihan & Tagihan Semester**:
   - Nomor tagihan unik di dalam satu institusi (`UNIQUE (institusi_id, nomor_tagihan)`), tetapi **boleh sama** di institusi yang berbeda.
   - Satu mahasiswa hanya memiliki satu tagihan UKT per semester pada institusi yang sama (`UNIQUE (institusi_id, mahasiswa_id, semester)`).
3. **Verifikasi Signature HMAC-SHA256 Per Institusi**:
   - Setiap institusi memiliki *secret key* HMAC masing-masing (`HMAC_SECRET_INST_1`, `HMAC_SECRET_INST_2`, `HMAC_SECRET_INST_3`).
   - Webhook membawa `institution_code` dan header `X-Signature`. Signature dihitung langsung dari **raw request body bytes** menggunakan *secret* institusi tersebut dan dibandingkan secara *constant-time* (`hmac.Equal`).
4. **Kebenaran Status Tagihan (State Machine & Idempotensi)**:
   - Status awal tagihan adalah `UNPAID` (Belum Bayar).
   - `UNPAID` → `PAID` saat menerima `payment.success` dengan nominal yang sama persis (`amount == nominal`).
   - `UNPAID` → `EXPIRED` saat menerima `payment.expired`.
   - `EXPIRED` → `PAID` saat terjadi *late settlement* (`payment.success` sah dengan nominal yang sesuai).
   - `PAID` bersifat **final (*terminal state*)**: notifikasi `payment.expired` yang tiba terlambat (*out-of-order*) tidak boleh pernah mengubah status `PAID` kembali menjadi `EXPIRED`.
   - Pengiriman ulang (*retry*) webhook yang sama bersifat *idempotent* (`200 OK` tanpa menduplikasi baris `transactions`).
5. **Pencatatan Audit Log Menyeluruh (`webhook_logs`)**:
   - Seluruh notifikasi webhook yang diterima (baik sukses `200`, gagal verifikasi signature `401`, tidak ditemukan `404`, maupun gagal validasi nominal `422`) wajib tercatat di tabel `webhook_logs` di luar transaksi rollback bisnis.

---

## 5. Konvensi Kode

- **Go (Backend)**:
  - Gunakan penamaan standar Go (`camelCase` untuk internal, `PascalCase` untuk simbol terekspos) dan format dengan `gofmt`.
  - Seluruh nilai uang/nominal UKT wajib menggunakan tipe bilangan bulat `int64` (`BIGINT` di PostgreSQL), dilarang menggunakan `float32`/`float64`.
  - Unit test diletakkan di *package* yang sama (`*_test.go`) mencakup pengujian di level *middleware*, *service*, dan *HTTP handler* (`httptest`).
  - Respons JSON menggunakan struktur envelope konsisten dari `pkg/response`.
- **Vue 3 & TypeScript (Frontend)**:
  - Gunakan `<script setup lang="ts">` dan Composition API.
  - Seluruh komunikasi HTTP melalui instance Axios terpusat (`src/api/client.ts`) dan manajemen state melalui Pinia store (`src/stores/`).
- **Git & Konfigurasi**:
  - Pesan *commit* harus granular dan deskriptif (format Conventional Commits: `feat:`, `fix:`, `test:`, `docs:`).

---

## 6. Hal yang Tidak Boleh Dilakukan AI

1. **DILARANG melakukan query database pada tabel bisnis tanpa menyertakan filter `institusi_id`** (kecuali proses resolusi awal institusi pada webhook yang langsung memvalidasi HMAC institusi tersebut).
2. **DILARANG mengambil atau mempercayai `institusi_id` dari body/query parameter request pada endpoint Admin/Mahasiswa**; selalu gunakan `institusi_id` dari konteks JWT.
3. **DILARANG menuliskan (hardcode) secret HMAC asli atau kredensial sensitif di dalam kode sumber, file migrasi/seeder, atau dokumen yang di-commit ke repositori**; selalu baca dari variabel lingkungan (`.env`).
4. **DILARANG mem-parsing JSON lalu men-serialisasi ulang (`json.Marshal`) untuk memverifikasi HMAC**; verifikasi HMAC-SHA256 wajib menggunakan *raw request body bytes* asli.
5. **DILARANG menggunakan tipe data floating-point (`float`, `double`, `REAL`) untuk perhitungan atau penyimpanan nominal tagihan UKT**.
6. **DILARANG menghapus atau melewatkan pencatatan `webhook_logs` ketika verifikasi signature atau validasi bisnis gagal**; kegagalan justru harus tercatat untuk kebutuhan audit dan analisis insiden.

---

## 7. Referensi API

| Method | Endpoint | Role / Auth | Deskripsi |
|---|---|---|---|
| `GET` | `/api/healthz` | Publik | Pemeriksaan kesehatan layanan & koneksi database |
| `GET` | `/api/institutions` | Publik | Daftar institusi kampus terdaftar (tanpa mengekspos secret HMAC) |
| `POST` | `/api/auth/login` | Publik | Login pengguna; mengembalikan JWT berisi `user_id`, `role`, `institusi_id`, `kode_institusi` |
| `GET` | `/api/me` | JWT (Semua Role) | Profil pengguna aktif beserta informasi institusinya |
| `GET` | `/api/admin/students` | JWT (`ADMIN_KEUANGAN`) | Daftar mahasiswa di institusi admin untuk pilihan pembuatan tagihan |
| `GET` | `/api/admin/invoices` | JWT (`ADMIN_KEUANGAN`) | Daftar tagihan UKT di institusi admin (`?status=UNPAID\|PAID\|EXPIRED&semester=...`) |
| `POST` | `/api/admin/invoices` | JWT (`ADMIN_KEUANGAN`) | Membuat tagihan UKT baru untuk mahasiswa di institusi admin |
| `GET` | `/api/student/invoices` | JWT (`MAHASISWA`) | Daftar status tagihan UKT beserta riwayat transaksi milik mahasiswa yang sedang login |
| `GET` | `/api/student/invoices/:id` | JWT (`MAHASISWA`) | Detail satu tagihan UKT & riwayat transaksinya (tervalidasi milik mahasiswa & institusi tersebut) |
| `POST` | `/api/webhooks/payment` | `X-Signature` (HMAC-SHA256) | Endpoint penerima notifikasi pembayaran dari Payment Gateway |
| `POST` | `/api/mock-gateway/trigger` | Simulator | Endpoint bantu Mock Payment Gateway untuk men-trigger webhook ke `/api/webhooks/payment` |
| `GET` | `/api/mock-gateway/logs` | Simulator / Audit | Mengambil daftar riwayat `webhook_logs` terbaru untuk investigasi insiden & demo |
