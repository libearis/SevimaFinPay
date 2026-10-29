-- ============================================================================
-- Skema Database Multi-Institusi Pembayaran UKT & Webhook Audit Log
-- PostgreSQL 16+
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Tabel Institusi (Perguruan Tinggi / Kampus)
CREATE TABLE IF NOT EXISTS institutions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    hmac_secret_env_key VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Tabel Pengguna (Admin Keuangan & Mahasiswa)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institusi_id UUID NOT NULL REFERENCES institutions(id) ON DELETE RESTRICT,
    nim_nip VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL CHECK (role IN ('ADMIN_KEUANGAN', 'MAHASISWA')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_users_institusi_email UNIQUE (institusi_id, email),
    CONSTRAINT uq_users_institusi_nim_nip UNIQUE (institusi_id, nim_nip)
);

CREATE INDEX IF NOT EXISTS idx_users_institusi_role ON users(institusi_id, role);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- 3. Tabel Tagihan UKT (Invoices)
CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institusi_id UUID NOT NULL REFERENCES institutions(id) ON DELETE RESTRICT,
    mahasiswa_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    nomor_tagihan VARCHAR(128) NOT NULL,
    semester VARCHAR(64) NOT NULL,
    nominal BIGINT NOT NULL CHECK (nominal > 0),
    status VARCHAR(32) NOT NULL DEFAULT 'UNPAID' CHECK (status IN ('UNPAID', 'PAID', 'EXPIRED')),
    tanggal_kedaluwarsa TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Nomor tagihan wajib unik di dalam satu institusi, namun boleh sama di institusi berbeda
    CONSTRAINT uq_invoices_institusi_nomor UNIQUE (institusi_id, nomor_tagihan),
    -- Satu mahasiswa hanya memiliki satu tagihan UKT per semester di institusi yang sama
    CONSTRAINT uq_invoices_institusi_mhs_semester UNIQUE (institusi_id, mahasiswa_id, semester)
);

CREATE INDEX IF NOT EXISTS idx_invoices_institusi_status ON invoices(institusi_id, status);
CREATE INDEX IF NOT EXISTS idx_invoices_institusi_mahasiswa ON invoices(institusi_id, mahasiswa_id);

-- 4. Tabel Riwayat Transaksi Pembayaran (Transactions)
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institusi_id UUID NOT NULL REFERENCES institutions(id) ON DELETE RESTRICT,
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE RESTRICT,
    event_id VARCHAR(128) NOT NULL,
    event_type VARCHAR(64) NOT NULL CHECK (event_type IN ('payment.success', 'payment.expired')),
    payment_channel VARCHAR(64) NOT NULL DEFAULT 'VIRTUAL_ACCOUNT',
    amount BIGINT NOT NULL CHECK (amount >= 0),
    status VARCHAR(32) NOT NULL CHECK (status IN ('SUCCESS', 'EXPIRED')),
    raw_payload JSONB NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Kunci idempotensi per institusi untuk mencegah pencatatan transaksi ganda saat retry
    CONSTRAINT uq_transactions_institusi_event UNIQUE (institusi_id, event_id)
);

CREATE INDEX IF NOT EXISTS idx_transactions_institusi_invoice ON transactions(institusi_id, invoice_id);

-- 5. Tabel Audit Log Webhook (Dicatat di luar transaksi rollback bisnis)
CREATE TABLE IF NOT EXISTS webhook_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institusi_id UUID NULL REFERENCES institutions(id) ON DELETE SET NULL,
    institution_code VARCHAR(64) NOT NULL DEFAULT '',
    nomor_tagihan VARCHAR(128) NOT NULL DEFAULT '',
    event_id VARCHAR(128) NOT NULL DEFAULT '',
    event_type VARCHAR(64) NOT NULL DEFAULT '',
    signature_header VARCHAR(255) NOT NULL DEFAULT '',
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    http_status INTEGER NOT NULL,
    processing_result VARCHAR(64) NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_logs_institusi_received ON webhook_logs(institusi_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_institution_code ON webhook_logs(institution_code, received_at DESC);
