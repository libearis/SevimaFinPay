package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// DefaultDemoPassword is the password used ONLY for demo user accounts in local/seed data.
// Note: HMAC secrets are NEVER stored in the database or hardcoded here; only the env variable key name is stored.
const DefaultDemoPassword = "KampusMerdeka2026!"

// SeedInitialData seeds the 3 institutions, demo users (Admin Keuangan & Mahasiswa), and sample UKT invoices idempotently.
func SeedInitialData(ctx context.Context, pool *pgxpool.Pool) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM institutions`).Scan(&count); err != nil {
		return fmt.Errorf("gagal memeriksa tabel institutions: %w", err)
	}
	if count > 0 {
		// Pastikan nama admin bersih tanpa duplikasi jabatan di dalam kurung
		_, _ = pool.Exec(ctx, `UPDATE users SET name = 'Budi Santoso' WHERE email = 'admin@nusantara.ac.id'`)
		_, _ = pool.Exec(ctx, `UPDATE users SET name = 'Dewi Lestari' WHERE email = 'admin@itmaju.ac.id'`)
		_, _ = pool.Exec(ctx, `UPDATE users SET name = 'Hendra Wijaya' WHERE email = 'admin@merdeka.ac.id'`)
		log.Println("[SEEDER] Data institusi sudah tersedia, melewati proses seeding awal.")
		return nil
	}

	log.Println("[SEEDER] Memulai seeding 3 institusi, akun demo, dan sampel tagihan UKT...")

	passwordHashBytes, err := bcrypt.GenerateFromPassword([]byte(DefaultDemoPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("gagal membuat hash password demo: %w", err)
	}
	passwordHash := string(passwordHashBytes)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Seed 3 Institusi Berbeda
	inst1ID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	inst2ID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	inst3ID := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	institutions := []struct {
		ID         uuid.UUID
		Code       string
		Name       string
		HMACEnvKey string
	}{
		{inst1ID, "UNIV-NUSANTARA", "Universitas Nusantara Raya", "HMAC_SECRET_INST_1"},
		{inst2ID, "INST-TEKNOLOGI-MAJU", "Institut Teknologi Maju Indonesia", "HMAC_SECRET_INST_2"},
		{inst3ID, "UNIV-MERDEKA", "Universitas Merdeka Sejahtera", "HMAC_SECRET_INST_3"},
	}

	for _, inst := range institutions {
		_, err := tx.Exec(ctx, `
			INSERT INTO institutions (id, code, name, hmac_secret_env_key)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (code) DO NOTHING
		`, inst.ID, inst.Code, inst.Name, inst.HMACEnvKey)
		if err != nil {
			return fmt.Errorf("gagal seed institusi %s: %w", inst.Code, err)
		}
	}

	// 2. Seed Users (Admin Keuangan & Mahasiswa) untuk masing-masing institusi
	mhs1Inst1 := uuid.MustParse("11111111-aaaa-4111-8111-000000000001")
	mhs2Inst1 := uuid.MustParse("11111111-aaaa-4111-8111-000000000002")
	mhs1Inst2 := uuid.MustParse("22222222-aaaa-4222-8222-000000000001")
	mhs2Inst2 := uuid.MustParse("22222222-aaaa-4222-8222-000000000002")
	mhs1Inst3 := uuid.MustParse("33333333-aaaa-4333-8333-000000000001")

	users := []struct {
		ID          uuid.UUID
		InstitusiID uuid.UUID
		NimNip      string
		Name        string
		Email       string
		Role        string
	}{
		// Institusi 1: UNIV-NUSANTARA
		{uuid.MustParse("11111111-0000-4111-8111-000000000001"), inst1ID, "NIP-19850101", "Budi Santoso", "admin@nusantara.ac.id", "ADMIN_KEUANGAN"},
		{mhs1Inst1, inst1ID, "202610001", "Andi Pratama", "andi@mhs.nusantara.ac.id", "MAHASISWA"},
		{mhs2Inst1, inst1ID, "202610002", "Siti Aminah", "siti@mhs.nusantara.ac.id", "MAHASISWA"},

		// Institusi 2: INST-TEKNOLOGI-MAJU
		{uuid.MustParse("22222222-0000-4222-8222-000000000001"), inst2ID, "NIP-19870202", "Dewi Lestari", "admin@itmaju.ac.id", "ADMIN_KEUANGAN"},
		{mhs1Inst2, inst2ID, "202620001", "Rizky Ramadhan", "rizky@mhs.itmaju.ac.id", "MAHASISWA"},
		{mhs2Inst2, inst2ID, "202620002", "Nadia Putri", "nadia@mhs.itmaju.ac.id", "MAHASISWA"},

		// Institusi 3: UNIV-MERDEKA
		{uuid.MustParse("33333333-0000-4333-8333-000000000001"), inst3ID, "NIP-19900303", "Hendra Wijaya", "admin@merdeka.ac.id", "ADMIN_KEUANGAN"},
		{mhs1Inst3, inst3ID, "202630001", "Fajar Nugroho", "fajar@mhs.merdeka.ac.id", "MAHASISWA"},
	}

	for _, u := range users {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, institusi_id, nim_nip, name, email, password_hash, role)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (institusi_id, email) DO NOTHING
		`, u.ID, u.InstitusiID, u.NimNip, u.Name, u.Email, passwordHash, u.Role)
		if err != nil {
			return fmt.Errorf("gagal seed user %s: %w", u.Email, err)
		}
	}

	// 3. Seed Tagihan UKT
	// Perhatikan bahwa nomor tagihan "UKT/2026-1/001" sengaja dibuat sama di ke-3 institusi
	// untuk mendemonstrasikan aturan bisnis bahwa nomor tagihan unik per institusi namun boleh sama antar-institusi!
	now := time.Now().UTC()
	dueFuture := now.Add(30 * 24 * time.Hour)
	duePast := now.Add(-5 * 24 * time.Hour)
	paidTime := now.Add(-2 * 24 * time.Hour)

	invPaidInst1 := uuid.MustParse("aaaa1111-1111-4111-8111-000000000002")

	invoices := []struct {
		ID                 uuid.UUID
		InstitusiID        uuid.UUID
		MahasiswaID        uuid.UUID
		NomorTagihan       string
		Semester           string
		Nominal            int64
		Status             string
		TanggalKedaluwarsa time.Time
		PaidAt             *time.Time
	}{
		// UNIV-NUSANTARA
		{uuid.MustParse("aaaa1111-1111-4111-8111-000000000001"), inst1ID, mhs1Inst1, "UKT/2026-1/001", "2026/2027-GANJIL", 5500000, "UNPAID", dueFuture, nil},
		{invPaidInst1, inst1ID, mhs1Inst1, "UKT/2025-2/001", "2025/2026-GENAP", 5500000, "PAID", duePast, &paidTime},
		{uuid.MustParse("aaaa1111-1111-4111-8111-000000000003"), inst1ID, mhs2Inst1, "UKT/2026-1/002", "2026/2027-GANJIL", 4750000, "UNPAID", dueFuture, nil},

		// INST-TEKNOLOGI-MAJU (Memiliki nomor tagihan UKT/2026-1/001 juga — membuktikan isolasi multi-institusi)
		{uuid.MustParse("aaaa2222-2222-4222-8222-000000000001"), inst2ID, mhs1Inst2, "UKT/2026-1/001", "2026/2027-GANJIL", 7000000, "UNPAID", dueFuture, nil},
		{uuid.MustParse("aaaa2222-2222-4222-8222-000000000002"), inst2ID, mhs2Inst2, "UKT/2026-1/002", "2026/2027-GANJIL", 6500000, "EXPIRED", duePast, nil},

		// UNIV-MERDEKA (Memiliki nomor tagihan UKT/2026-1/001 juga)
		{uuid.MustParse("aaaa3333-3333-4333-8333-000000000001"), inst3ID, mhs1Inst3, "UKT/2026-1/001", "2026/2027-GANJIL", 4200000, "UNPAID", dueFuture, nil},
	}

	for _, inv := range invoices {
		_, err := tx.Exec(ctx, `
			INSERT INTO invoices (id, institusi_id, mahasiswa_id, nomor_tagihan, semester, nominal, status, tanggal_kedaluwarsa, paid_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (institusi_id, nomor_tagihan) DO NOTHING
		`, inv.ID, inv.InstitusiID, inv.MahasiswaID, inv.NomorTagihan, inv.Semester, inv.Nominal, inv.Status, inv.TanggalKedaluwarsa, inv.PaidAt)
		if err != nil {
			return fmt.Errorf("gagal seed tagihan %s: %w", inv.NomorTagihan, err)
		}
	}

	// Tambahkan 1 riwayat transaksi untuk tagihan semester lalu yang sudah PAID pada UNIV-NUSANTARA
	_, err = tx.Exec(ctx, `
		INSERT INTO transactions (institusi_id, invoice_id, event_id, event_type, payment_channel, amount, status, raw_payload, processed_at)
		VALUES ($1, $2, $3, 'payment.success', 'VA_MANDIRI', 5500000, 'SUCCESS', $4::jsonb, $5)
		ON CONFLICT (institusi_id, event_id) DO NOTHING
	`, inst1ID, invPaidInst1, "evt_seed_initial_001",
		`{"event_id":"evt_seed_initial_001","event_type":"payment.success","institution_code":"UNIV-NUSANTARA","nomor_tagihan":"UKT/2025-2/001","amount":5500000,"payment_channel":"VA_MANDIRI","timestamp":"2026-02-10T09:00:00Z"}`,
		paidTime,
	)
	if err != nil {
		return fmt.Errorf("gagal seed transaksi awal: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	log.Println("[SEEDER] Seeding 3 institusi, akun demo, dan tagihan UKT berhasil diselesaikan.")
	return nil
}
