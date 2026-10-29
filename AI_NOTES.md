# Catatan Kolaborasi & Refleksi Bersama AI (AI_NOTES.md)

Dokumen ini berisi refleksi penggunaan asisten coding AI selama pengerjaan **Project M2: Integrasi Pembayaran UKT Multi-Institusi via Webhook**, dengan posisi saya sebagai *driver* dan pengambil keputusan arsitektur/produk, serta AI sebagai *navigator* dan pengeksekusi kode terarah.

---

## Bagian yang dikerjakan bersama AI

1. **Penyusunan Boilerplate & Modular Layered Architecture Go (Gin) + Vue 3 (Pinia)**:
   - Meminta AI menyiapkan kerangka kode per domain (`auth`, `institusi`, `tagihan`, dan `webhook`) di dalam `backend/internal/` dengan pemisahan `handler` → `service` → `repository`.
   - Mempercepat pembuatan *store* Pinia (`auth` dan `billing`), konfigurasi *route guard* Vue Router, serta penerapan *color palette* identitas visual SEVIMA (Navy Blue `#0A2E6E` & Crimson Red `#E11D2E`).
2. **Penulisan Skema Migrasi SQL, Pembuatan Database Otomatis (`EnsureDatabaseExists`), & Seeder 3 Institusi**:
   - Menyusun DDL PostgreSQL untuk tabel `institutions`, `users`, `invoices`, `transactions`, dan `webhook_logs` beserta *composite unique constraint* `UNIQUE (institusi_id, nomor_tagihan)` dan `UNIQUE (institusi_id, event_id)`.
   - Menyiapkan *seeder* otomatis untuk 3 perguruan tinggi (`UNIV-NUSANTARA`, `INST-TEKNOLOGI-MAJU`, `UNIV-MERDEKA`) lengkap dengan akun demo Admin Keuangan dan Mahasiswa.
3. **Implementasi Unit Test Lintas Layer (`middleware`, `service`, dan `handler`)**:
   - Menggunakan AI untuk menyusun kerangka *table-driven unit tests* dan *HTTP handler tests* (`httptest`), yang kemudian saya arahkan untuk menguji isolasi akses lintas-kampus, verifikasi HMAC *raw body bytes*, *idempotent retry*, dan *out-of-order webhook*.
4. **Penyusunan Dokumentasi Teknis (`AGENTS.md`, `ARD.md`, `DECISIONS.md`, `WEBHOOK_DOCS.md`)**:
   - Merapikan diagram alur sekuens, *state machine*, ERD Mermaid, dan panduan pengujian webhook.

---

## Contoh saran AI yang saya tolak/koreksi, dan alasannya

Selama proses *iterative development*, terdapat beberapa hasil generasi awal AI yang saya tolak dan koreksi secara langsung:

1. **Koreksi pada Cakupan Unit Test (Menolak Pengujian yang Hanya di Service Layer)**:
   - **Saran Awal AI**: Pada iterasi pertama, AI hanya membuat satu file unit test di level `service_test.go` saja tanpa menguji lapisan *HTTP Handler (Controller)* dan *Middleware JWT/RBAC*.
   - **Alasan Saya Menolak & Koreksi**: Dalam sistem multi-institusi dan penerima webhook, celah keamanan sering terjadi di batas lapisan HTTP (misalnya ekstraksi klaim `institusi_id` dari header Bearer JWT di middleware, atau pembacaan *raw request body bytes* sebelum verifikasi `X-Signature` di handler). Saya menginstruksikan AI untuk menambahkan `auth_test.go` (pada `internal/middleware`) dan `handler_test.go` (pada `internal/tagihan` dan `internal/webhook`) menggunakan `net/http/httptest`.

2. **Koreksi pada Desain Alur Peran (Role) & Layout UI (Menolak Fitur "Simulasi Bayar" di Akun Admin Keuangan & Layout Form Sempit)**:
   - **Saran Awal AI**: AI awalnya menempatkan form "Terbitkan Tagihan UKT Baru" berdempetan dalam satu halaman dengan tabel "Daftar Tagihan", serta menaruh tombol pintasan "Simulasi Bayar" dan menu "Simulator Webhook" di dalam tampilan Admin Keuangan. Selain itu, saat mencoba menyederhanakan simulator, AI sempat menambahkan dropdown pemilihan mahasiswa lintas-tagihan yang justru membuat UI penuh dan membingungkan.
   - **Alasan Saya Menolak & Koreksi**:
     - Secara logika bisnis perguruan tinggi, **Admin Keuangan** bertugas menerbitkan dan memantau tagihan, bukan melakukan pembayaran tagihan mahasiswa. Menyediakan tombol bayar pada tabel Admin merusak pemisahan peran (*separation of duties*).
     - Menyatukan form pembuatan tagihan di samping tabel membuat tabel daftar tagihan terhimpit dan tidak *user-friendly*.
     - Saya menginstruksikan agar form penerbitan tagihan dipisahkan menjadi halaman mandiri (`/admin/invoices/create`), menghapus seluruh tombol/menu simulasi bayar dari sisi Admin Keuangan, dan membatasi akses **Simulator Webhook (`/mock-gateway`)** hanya pada alur **Mahasiswa** (ketika mahasiswa mengklik *Bayar via Simulator Payment Gateway* pada tagihan miliknya).

3. **Koreksi pada Arsitektur Docker & Database (Menolak Container DB Ganda di `docker-compose.yml`)**:
   - **Saran Awal AI**: AI awalnya membuat container PostgreSQL baru di dalam `docker-compose.yml` yang berjalan berdampingan dengan instance PostgreSQL 17 lokal saya.
   - **Alasan Saya Menolak & Koreksi**: Untuk menjaga konsistensi *best practice* bahwa *stateful database* berjalan di instance terpisah (*dedicated instance* pada `localhost:5432`, database `ukt`), saya menolak pembuatan container database duplikat di Docker Compose dan mengarahkan agar `docker-compose.yml` hanya menjalankan lapisan aplikasi *stateless* (`backend` dan `frontend`) yang terhubung langsung ke instance PostgreSQL host melalui `host.docker.internal:5432`.

4. **Koreksi Teknis Pemrosesan Webhook (Menolak Re-marshaling JSON untuk HMAC & Menolak Audit Log di Dalam Transaksi Rollback)**:
   - **Alasan Saya Menolak & Koreksi**: Verifikasi `HMAC-SHA256` wajib dihitung di atas *raw request body bytes* asli (`io.ReadAll`), bukan hasil `json.Marshal` ulang dari struct Go. Selain itu, penyimpanan `webhook_logs` wajib dieksekusi di luar transaksi rollback bisnis agar setiap kegagalan (`401 INVALID_SIGNATURE`, `404`, `422 AMOUNT_MISMATCH`) tetap tercatat permanen untuk investigasi insiden.

---

## Bagian yang saya kerjakan tanpa AI, dan alasannya

1. **Penentuan Batas Peran Bisnis (Admin Keuangan vs Mahasiswa) & Validasi UX**:
   - Menentukan alur produk yang masuk akal bagi pengguna kampus: bagaimana Admin Keuangan bekerja, bagaimana Mahasiswa membayar tagihannya, informasi apa saja yang redundan pada profil header, serta bagaimana konsistensi navigasi harus disajikan.
   - **Alasan**: AI cenderung mencampur fitur teknis/debugging ke dalam tampilan semua role pengguna. Keputusan *user experience* dan ketepatan konteks bisnis harus dikendalikan langsung oleh *engineer*.
2. **Keputusan Infrastruktur Database Lokal & Manajemen Kredensial (`.env`)**:
   - Menyiapkan instance PostgreSQL 17 lokal (`localhost:5432`, database `ukt`), memastikan pemisahan kredensial lokal pada `.env` yang di-*gitignore*, serta memverifikasi langsung perilaku *build* container Docker di lingkungan Windows.
   - **Alasan**: Keamanan kredensial lokal dan topologi infrastruktur di mesin eksekusi merupakan tanggung jawab langsung pengembang.
3. **Verifikasi End-to-End & Pengujian Skenario Insiden**:
   - Menguji langsung alur login lintas-institusi, penerbitan tagihan, pemicuan webhook pembayaran mahasiswa, serta pengecekan perubahan status tagihan dan KRS di database.

---

## Kalau mengulang dari awal, apa yang akan saya ubah

1. **Menetapkan Aturan Desain UI & Pemisahan Halaman di `AGENTS.md` Sejak Awal**:
   - Menuliskan spesifikasi tata letak halaman (seperti pemisahan halaman tabel dan halaman form pembuatan, standar palet warna SEVIMA, serta batasan menu per role) langsung di `AGENTS.md` sebelum AI meng-generate kode frontend agar tidak perlu melakukan beberapa kali iterasi perombakan UI.
2. **Menerapkan PostgreSQL Row-Level Security (RLS) sebagai Pertahanan Lapis Kedua**:
   - Menambahkan kebijakan *Row-Level Security (RLS)* di level PostgreSQL sebagai *defense-in-depth* pendamping filter eksplisit `WHERE institusi_id = $1` pada *repository*.
3. **Menyediakan Automated Integration Test terhadap Test Database**:
   - Selain unit test di level `middleware`, `service`, dan `handler`, menambahkan *integration test* otomatis yang menguji query SQL `SELECT ... FOR UPDATE` dan constraint PostgreSQL secara langsung pada database uji khusus.
