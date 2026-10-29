# Catatan Keputusan Teknis & Arsitektur (DECISIONS.md)

Dokumen ini mencatat 5 keputusan teknis utama dalam perancangan dan implementasi sistem **Integrasi Pembayaran UKT Multi-Institusi via Webhook (SEVIMA Project M2)**, termasuk penanganan perilaku sistem pembayaran di dunia nyata yang tidak selalu ideal.

---

## 1. Strategi Isolasi Multi-Institusi (Multi-Tenancy) pada Satu Database PostgreSQL

Konteks :
Sistem harus melayani banyak kampus (institusi) di dalam satu database PostgreSQL yang sama. Setiap data tagihan, mahasiswa, riwayat transaksi, dan konfigurasi pembayaran dimiliki oleh satu institusi (`institusi_id`). Kesalahan isolasi yang menyebabkan Admin atau Mahasiswa Kampus A dapat melihat atau mengubah tagihan Kampus B merupakan celah keamanan fatal (*cross-tenant data leakage*). Selain itu, nomor tagihan dapat memiliki format yang sama di dua kampus berbeda, namun harus unik di dalam satu kampus.

Opsi :
1. **Database per Institusi / Schema per Institusi**: Setiap kampus mendapatkan skema PostgreSQL terpisah (`kampus_a.invoices`, `kampus_b.invoices`). Isolasi tinggi namun menyulitkan migrasi skema ketika jumlah kampus bertambah hingga ratusan dan memperumit manajemen *connection pool*.
2. **Shared Schema dengan Discriminator Column (`institusi_id`) + Context-Bound Repository**: Semua kampus berbagi tabel yang sama dengan kolom wajib `institusi_id`, *composite unique constraint* `UNIQUE (institusi_id, nomor_tagihan)`, serta penegakan `institusi_id` yang diekstrak langsung dari klaim JWT pada setiap query di lapisan *repository*.
3. **PostgreSQL Row-Level Security (RLS) dengan Session Variable**: Mengandalkan kebijakan RLS di level database menggunakan `SET LOCAL app.current_institusi_id`. Aman, namun membuat *debugging* query dan pengelolaan koneksi di *connection pool* (`pgxpool`) lebih kompleks.

Keputusan :
Memilih **Opsi 2 (Shared Schema dengan Discriminator Column `institusi_id` + Context-Bound Repository)** yang diperkuat dengan *composite constraint* di level database dan *unit test* isolasi lintas institusi.
- Pada endpoint pengguna (Admin & Mahasiswa), `institusi_id` selalu diambil dari klaim token JWT yang telah diverifikasi oleh middleware autentikasi, tidak pernah dari input body/query parameter.
- Seluruh fungsi pada `tagihan.Repository` mewajibkan parameter `institusiID` secara eksplisit di tanda tangan fungsi (*function signature*) sehingga pengembang maupun AI tidak mungkin memanggil query tanpa menyertakan identitas institusi.

Konsekuensi :
- **Positif**: Skema mudah dimigrasi, performa indeks komposit `(institusi_id, status)` dan `(institusi_id, nomor_tagihan)` sangat cepat, serta mudah diuji secara deterministik lewat *automated unit tests*.
- **Negatif / Mitigasi**: Disiplin penulisan query di *repository* harus dijaga ketat. Risiko kelalaian dimitigasi dengan mewajibkan `institusiID` pada kontrak *interface* Go dan menyediakan pengujian otomatis khusus skenario akses silang antar-kampus.

---

## 2. Penanganan Perilaku Webhook Dunia Nyata: Idempotensi, Pengiriman Ulang (Retry), dan Out-of-Order Events

Konteks :
Di dunia nyata, komunikasi dari *payment gateway* tidak selalu ideal:
1. **Webhook Retry / Duplikasi**: Ketika server kampus sedang lambat atau terjadi gangguan jaringan saat mengirim respons `200 OK`, *payment gateway* akan mengirim ulang (*retry*) payload webhook yang sama beberapa kali.
2. **Out-of-Order Delivery**: Karena antrean terdistribusi di sisi gateway, notifikasi `payment.expired` dapat tiba beberapa detik **setelah** notifikasi `payment.success` diterima dan diproses.
3. **Late Settlement (Pembayaran Terlambat namun Dana Masuk)**: Mahasiswa membayar mendekati detik kedaluwarsa; sistem sempat menerima status kedaluwarsa, tetapi bank akhirnya berhasil menyelesaikan kliring (*settlement*) dan mengirimkan `payment.success`.
4. **Concurrent Webhook Race Condition**: Dua worker gateway mengirimkan event untuk tagihan yang sama pada milidetik yang bersamaan.

Opsi :
1. **Pembaruan Langsung Tanpa Penguncian Baris (Naive Update)**: Langsung menjalankan `UPDATE invoices SET status = $1` sesuai event terakhir yang masuk. Sangat berbahaya karena `payment.expired` yang terlambat dapat menimpa status `PAID` milik mahasiswa sehingga mahasiswa gagal mengisi KRS padahal uang sudah terpotong.
2. **State Machine Ketat + Pessimistic Row Locking (`SELECT ... FOR UPDATE`) + Kunci Idempotensi**: Mengunci baris tagihan di dalam transaksi PostgreSQL, memvalidasi matriks transisi status (*state machine*), dan memeriksa keunikan `event_id` / `external_reference` sebelum membuat catatan transaksi.

Keputusan :
Memilih **Opsi 2 (State Machine Ketat + `SELECT ... FOR UPDATE` + Idempotensi Transaksi)**:
- **Idempotensi**: Jika webhook dengan `event_id` yang sama (atau `payment.success` pada tagihan yang sudah `PAID` dengan referensi yang sama) diterima kembali, sistem tidak menambahkan riwayat transaksi baru dan mengembalikan `200 OK` (status *idempotent replay*) agar *payment gateway* berhenti melakukan *retry*.
- **Terminal State Protection (`PAID`)**: Status `PAID` adalah status final. Jika tagihan sudah `PAID` lalu datang event `payment.expired` (*out-of-order*), status tagihan **tetap `PAID`**, event dicatat di `webhook_logs` sebagai `IGNORED_ALREADY_PAID`, dan server mengembalikan `200 OK`.
- **Late Settlement Handling (`EXPIRED` → `PAID`)**: Jika tagihan berstatus `EXPIRED` namun *payment gateway* mengirimkan `payment.success` dengan signature HMAC valid dan nominal yang sesuai (uang mahasiswa benar-benar telah diterima gateway), sistem mengizinkan transisi `EXPIRED` → `PAID` agar hak pengisian KRS mahasiswa tidak dirugikan, sembari menandai transaksi sebagai *late settlement* di log audit.
- **Validasi Nominal (`Amount Verification`)**: Jika nominal pada payload `payment.success` tidak sama dengan nominal tagihan UKT (`amount != invoice.nominal`), transaksi ditolak (`422 Unprocessable Entity`) dan dicatat pada audit log untuk mencegah manipulasi nominal pembayaran.

Konsekuensi :
- **Positif**: Menjamin akurasi status pembayaran untuk pengisian KRS, bebas dari duplikasi pencatatan riwayat transaksi, dan tahan terhadap kondisi balapan (*race condition*) maupun urutan paket jaringan yang terbalik.
- **Negatif**: Adanya sedikit *overhead* penguncian baris (`FOR UPDATE`) pada saat webhook masuk untuk satu tagihan spesifik, namun dampaknya sangat minimal karena penguncian hanya terjadi per baris tagihan (`row-level lock`) dalam waktu beberapa milidetik.

---

## 3. Arsitektur Pencatatan Audit Log Webhook (`webhook_logs`) yang Terpisah dari Transaksi Bisnis

Konteks :
Aturan bisnis mewajibkan **seluruh notifikasi yang diterima dicatat untuk keperluan audit**, dan pada Tahap 2 wawancara teknis terdapat sesi **analisis insiden dari log**. Apabila penyimpanan `webhook_logs` digabungkan di dalam satu transaksi database (`BEGIN ... COMMIT`) yang sama dengan pembaruan tagihan, maka ketika terjadi kegagalan bisnis (misalnya: signature HMAC tidak cocok, tagihan tidak ditemukan, atau nominal tidak sesuai) dan transaksi di-*rollback*, catatan audit log tersebut akan ikut hilang.

Opsi :
1. **Mencatat ke File Log Teks (Stdout / File)**: Mudah dilakukan, namun sulit di-query secara terstruktur berdasarkan institusi, nomor tagihan, atau status verifikasi saat investigasi insiden.
2. **Mencatat ke Tabel `webhook_logs` di Dalam Transaksi yang Sama**: Log akan hilang setiap kali transaksi utama melakukan `ROLLBACK`.
3. **Pencatatan `webhook_logs` Mandiri (Out-of-Transaction / Independent Commit) + Structured Logging**: Setiap request webhook langsung dicatat ke tabel `webhook_logs` pada eksekusi independen di luar blok rollback transaksi tagihan, dilengkapi dengan *raw payload*, nilai header `X-Signature`, hasil verifikasi signature, kode HTTP respons, dan alasan penolakan (`error_reason`).

Keputusan :
Memilih **Opsi 3 (Pencatatan `webhook_logs` Mandiri di PostgreSQL + Structured JSON Logging di stdout)**.
Setiap request ke `POST /api/webhooks/payment` selalu menghasilkan satu baris rekaman permanen di tabel `webhook_logs` baik ketika sukses (`200 OK`) maupun ketika ditolak (`400`, `401`, `404`, `422`).

Konsekuensi :
- **Positif**: Memberikan visibilitas 100% saat menginvestigasi insiden pembayaran (misalnya: mendeteksi apakah kegagalan disebabkan oleh *secret* HMAC yang salah dikonfigurasi oleh salah satu kampus, nomor tagihan salah ketik dari gateway, atau percobaan pemalsuan *webhook*).
- **Negatif**: Tabel `webhook_logs` akan terus bertambah besar seiring waktu; pada skala produksi dapat dikelola dengan partisi tabel bulanan atau kebijakan *archiving*.

---

## 4. Verifikasi HMAC-SHA256 Menggunakan Raw Request Body & Penyimpanan Secret di Environment

Konteks :
Setiap institusi memiliki *secret* HMAC masing-masing. Notifikasi gateway membawa kode institusi dan signature `HMAC-SHA256` pada header `X-Signature`. Kesalahan umum pada implementasi webhook di Go/Node.js adalah mem-parsing body ke *struct*/objek terlebih dahulu, lalu melakukan `json.Marshal` ulang sebelum menghitung HMAC. Perbedaan spasi, urutan *key* JSON, atau karakter *escape* akan membuat hash HMAC berubah sehingga verifikasi gagal meskipun pengirimnya sah. Selain itu, aturan pengumpulan melarang *secret* tertulis di dalam repositori.

Opsi :
1. **Menghitung HMAC dari Hasil Serialisasi Ulang Struct Go**: Rentan gagal jika format spasi atau urutan field JSON dari gateway berbeda dengan urutan field struct di Go.
2. **Membaca `io.ReadAll(c.Request.Body)` sebagai Raw Bytes + Constant-Time Comparison (`hmac.Equal`) + Resolusi Secret dari Environment Variables**: Menghitung HMAC-SHA256 langsung di atas *byte* mentah yang diterima lewat jaringan dan mengambil *secret* institusi dari variabel lingkungan yang dipetakan saat *startup* atau *seeding*.

Keputusan :
Memilih **Opsi 2 (Verifikasi di atas Raw Bytes + `hmac.Equal` + Injeksi Secret dari Environment Variables)**.
- Handler membaca *raw body bytes* sekali di awal (dengan batas ukuran payload `MaxBytesReader` 1 MB untuk mencegah serangan *memory exhaustion*).
- *Raw bytes* tersebut digunakan untuk menghitung `HMAC-SHA256(secret_institusi, raw_body)` dan dibandingkan menggunakan `crypto/hmac.Equal` (waktu konstan untuk mencegah *timing side-channel attack*).
- *Secret* HMAC untuk ketiga institusi diambil dari variabel lingkungan (`HMAC_SECRET_INST_1`, `HMAC_SECRET_INST_2`, `HMAC_SECRET_INST_3`) sehingga tidak ada *secret* produksi yang tertulis di kode sumber maupun riwayat Git.

Konsekuensi :
- **Positif**: Verifikasi signature akurat terhadap segala variasi pemformatan JSON dari gateway, tahan terhadap *timing attack*, dan memenuhi standar keamanan repositori tanpa *hardcoded secret*.
- **Negatif**: Penguji perlu menyalin `.env.example` ke `.env` (atau memasukkan nilai *secret* yang dikirim via email) sebelum menjalankan aplikasi.

---

## 5. Pemisahan Instance PostgreSQL (Database Isolation) dan Penyediaan Mock Payment Gateway Terintegrasi

Konteks :
Sesuai praktik terbaik (*best practice*) rekayasa perangkat lunak, *stateful database* (PostgreSQL) sebaiknya berjalan pada *instance* yang terpisah dari *runtime* aplikasi agar siklus hidup data, *backup*, dan alokasi memori database tidak terganggu oleh *restart* atau *scaling* layanan aplikasi. Di sisi lain, penilai juga membutuhkan cara yang cepat dan terdokumentasi untuk menjalankan proyek serta memicu *webhook* dari *mock payment gateway*.

Opsi :
1. **Hanya Menyediakan Script cURL Manual**: Menyulitkan demonstrasi interaktif saat wawancara Tahap 2 karena harus menghitung hash HMAC-SHA256 secara manual di terminal setiap kali mengubah payload.
2. **Menyediakan Konfigurasi PostgreSQL Terpisah (`docker-compose.db.yml` / Koneksi Eksternal) + Full Docker Compose (`docker-compose.yml`) + Mock Payment Gateway Ganda (UI Simulator & CLI/API)**: Mendukung koneksi ke instance PostgreSQL mandiri melalui variabel `.env`, menyediakan Docker Compose untuk eksekusi satu perintah, serta menyertakan simulator Mock Payment Gateway yang dapat menghitung signature HMAC secara otomatis maupun mensimulasikan skenario *error* (signature salah, nominal salah, event *expired*, *retry* duplikat).

Keputusan :
Memilih **Opsi 2**.
- Aplikasi membaca konfigurasi database sepenuhnya dari variabel lingkungan (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`) sehingga dapat langsung diarahkan ke instance PostgreSQL terpisah (recommended *best practice*), container DB khusus (`docker-compose.db.yml`), maupun stack penuh (`docker-compose.yml`).
- Menyediakan **Mock Payment Gateway** yang dapat dijalankan baik melalui antarmuka web simulator maupun melalui perintah CLI/cURL sebagaimana didokumentasikan pada `WEBHOOK_DOCS.md`.

Konsekuensi :
- **Positif**: Memenuhi *best practice* infrastruktur database terpisah sekaligus memudahkan pengujian dan demonstrasi *live coding* pada wawancara Tahap 2.
- **Negatif**: Menambah sedikit kode untuk modul simulator *mock gateway*, namun diisolasi dengan rapi sehingga tidak mencemari logika domain utama tagihan UKT.
