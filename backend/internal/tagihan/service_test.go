package tagihan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeTagihanRepo is an in-memory multi-tenant repository used to verify strict data isolation between institutions.
type fakeTagihanRepo struct {
	students []StudentSummary
	invoices []Invoice
}

func newFakeTagihanRepo() *fakeTagihanRepo {
	return &fakeTagihanRepo{
		students: []StudentSummary{},
		invoices: []Invoice{},
	}
}

func (f *fakeTagihanRepo) GetStudentInInstitution(_ context.Context, institusiID, mahasiswaID uuid.UUID) (*StudentSummary, error) {
	for _, s := range f.students {
		if s.ID == mahasiswaID && s.InstitusiID == institusiID {
			cp := s
			return &cp, nil
		}
	}
	return nil, ErrStudentNotFoundInTenant
}

func (f *fakeTagihanRepo) ListStudentsByInstitution(_ context.Context, institusiID uuid.UUID) ([]StudentSummary, error) {
	var res []StudentSummary
	for _, s := range f.students {
		if s.InstitusiID == institusiID {
			res = append(res, s)
		}
	}
	return res, nil
}

func (f *fakeTagihanRepo) CreateInvoice(_ context.Context, inv *Invoice) (*Invoice, error) {
	for _, existing := range f.invoices {
		if existing.InstitusiID == inv.InstitusiID && existing.NomorTagihan == inv.NomorTagihan {
			return nil, ErrDuplicateInvoiceNumber
		}
		if existing.InstitusiID == inv.InstitusiID && existing.MahasiswaID == inv.MahasiswaID && existing.Semester == inv.Semester {
			return nil, ErrDuplicateStudentSemester
		}
	}

	cp := *inv
	cp.ID = uuid.New()
	cp.CreatedAt = time.Now().UTC()
	cp.UpdatedAt = cp.CreatedAt
	cp.Transactions = []TransactionRecord{}
	f.invoices = append(f.invoices, cp)
	return &cp, nil
}

func (f *fakeTagihanRepo) ListInvoicesByInstitution(_ context.Context, institusiID uuid.UUID, filter InvoiceFilter) ([]Invoice, error) {
	var res []Invoice
	for _, inv := range f.invoices {
		if inv.InstitusiID != institusiID {
			continue
		}
		if filter.Status != "" && inv.Status != filter.Status {
			continue
		}
		if filter.Semester != "" && inv.Semester != filter.Semester {
			continue
		}
		res = append(res, inv)
	}
	return res, nil
}

func (f *fakeTagihanRepo) ListInvoicesByStudent(_ context.Context, institusiID, mahasiswaID uuid.UUID) ([]Invoice, error) {
	var res []Invoice
	for _, inv := range f.invoices {
		if inv.InstitusiID == institusiID && inv.MahasiswaID == mahasiswaID {
			res = append(res, inv)
		}
	}
	return res, nil
}

func (f *fakeTagihanRepo) GetInvoiceByIDForStudent(_ context.Context, institusiID, mahasiswaID, invoiceID uuid.UUID) (*Invoice, error) {
	for _, inv := range f.invoices {
		if inv.ID == invoiceID && inv.InstitusiID == institusiID && inv.MahasiswaID == mahasiswaID {
			cp := inv
			return &cp, nil
		}
	}
	return nil, ErrInvoiceNotFound
}

func TestMultiInstitutionIsolation_CreateAndListInvoices(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTagihanRepo()
	svc := NewService(repo)

	instKampusA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	instKampusB := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	mhsKampusA := uuid.MustParse("11111111-aaaa-4111-8111-000000000001")
	mhsKampusB := uuid.MustParse("22222222-aaaa-4222-8222-000000000001")

	repo.students = []StudentSummary{
		{ID: mhsKampusA, InstitusiID: instKampusA, NIM: "202610001", Name: "Andi Kampus A", Email: "andi@kampusa.ac.id"},
		{ID: mhsKampusB, InstitusiID: instKampusB, NIM: "202620001", Name: "Rizky Kampus B", Email: "rizky@kampusb.ac.id"},
	}

	// 1. Admin Kampus A berhasil membuat tagihan untuk mahasiswa Kampus A dengan nomor UKT/2026-1/001
	invA, err := svc.CreateInvoice(ctx, instKampusA, CreateInvoiceRequest{
		MahasiswaID:        mhsKampusA,
		NomorTagihan:       "UKT/2026-1/001",
		Semester:           "2026/2027-GANJIL",
		Nominal:            5500000,
		TanggalKedaluwarsa: "2026-12-31",
	})
	if err != nil {
		t.Fatalf("seharusnya Admin Kampus A berhasil membuat tagihan: %v", err)
	}
	if invA.Status != StatusUnpaid {
		t.Errorf("status awal tagihan harus UNPAID, didapat %s", invA.Status)
	}

	// 2. Admin Kampus B juga boleh menggunakan nomor tagihan yang SAMA ("UKT/2026-1/001") di Kampus B
	invB, err := svc.CreateInvoice(ctx, instKampusB, CreateInvoiceRequest{
		MahasiswaID:        mhsKampusB,
		NomorTagihan:       "UKT/2026-1/001",
		Semester:           "2026/2027-GANJIL",
		Nominal:            7000000,
		TanggalKedaluwarsa: "2026-12-31",
	})
	if err != nil {
		t.Fatalf("seharusnya nomor tagihan yang sama diizinkan pada institusi berbeda: %v", err)
	}
	if invB.Nominal != 7000000 {
		t.Errorf("nominal tagihan Kampus B harus 7000000, didapat %d", invB.Nominal)
	}

	// 3. Duplikasi nomor tagihan di dalam institusi yang sama (Kampus A) WAJIB ditolak
	_, err = svc.CreateInvoice(ctx, instKampusA, CreateInvoiceRequest{
		MahasiswaID:        mhsKampusA,
		NomorTagihan:       "UKT/2026-1/001",
		Semester:           "2026/2027-GENAP",
		Nominal:            5500000,
		TanggalKedaluwarsa: "2027-06-30",
	})
	if !errors.Is(err, ErrDuplicateInvoiceNumber) {
		t.Fatalf("seharusnya duplikasi nomor tagihan di institusi yang sama ditolak dengan ErrDuplicateInvoiceNumber, didapat: %v", err)
	}

	// 4. Admin Kampus A mencoba membuat tagihan untuk mahasiswa milik Kampus B -> WAJIB ditolak (Cross-Tenant Protection)
	_, err = svc.CreateInvoice(ctx, instKampusA, CreateInvoiceRequest{
		MahasiswaID:        mhsKampusB,
		NomorTagihan:       "UKT/2026-1/999",
		Semester:           "2026/2027-GANJIL",
		Nominal:            5000000,
		TanggalKedaluwarsa: "2026-12-31",
	})
	if !errors.Is(err, ErrStudentNotFoundInTenant) {
		t.Fatalf("seharusnya Admin Kampus A ditolak saat membuat tagihan untuk mahasiswa Kampus B, didapat: %v", err)
	}

	// 5. Admin Kampus A melihat daftar tagihan -> Hanya melihat tagihan Kampus A, tidak boleh melihat tagihan Kampus B
	listA, err := svc.ListInvoicesForAdmin(ctx, instKampusA, InvoiceFilter{})
	if err != nil {
		t.Fatalf("gagal mengambil daftar tagihan Kampus A: %v", err)
	}
	if len(listA) != 1 || listA[0].InstitusiID != instKampusA {
		t.Fatalf("Admin Kampus A hanya boleh melihat 1 tagihan miliknya, didapat %d", len(listA))
	}

	// 6. Mahasiswa Kampus A mencoba mengakses detail tagihan milik Mahasiswa Kampus B -> WAJIB ditolak
	_, err = svc.GetInvoiceDetailForStudent(ctx, instKampusA, mhsKampusA, invB.ID)
	if !errors.Is(err, ErrInvoiceNotFound) {
		t.Fatalf("Mahasiswa Kampus A tidak boleh bisa mengakses ID tagihan Kampus B, didapat: %v", err)
	}
}

func TestValidationAndStatusFiltering(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTagihanRepo()
	svc := NewService(repo)

	instID := uuid.New()
	mhsID := uuid.New()
	repo.students = append(repo.students, StudentSummary{
		ID:          mhsID,
		InstitusiID: instID,
		NIM:         "2026999",
		Name:        "Mahasiswa Uji",
		Email:       "uji@kampus.ac.id",
	})

	tests := []struct {
		name    string
		req     CreateInvoiceRequest
		wantErr error
	}{
		{
			name: "Nominal negatif atau nol ditolak",
			req: CreateInvoiceRequest{
				MahasiswaID:        mhsID,
				NomorTagihan:       "INV-001",
				Semester:           "2026/2027-GANJIL",
				Nominal:            0,
				TanggalKedaluwarsa: "2026-12-31",
			},
			wantErr: ErrInvalidInvoiceAmount,
		},
		{
			name: "Format tanggal salah ditolak",
			req: CreateInvoiceRequest{
				MahasiswaID:        mhsID,
				NomorTagihan:       "INV-002",
				Semester:           "2026/2027-GANJIL",
				Nominal:            4500000,
				TanggalKedaluwarsa: "31-12-2026",
			},
			wantErr: ErrInvalidDueDate,
		},
		{
			name: "Nomor tagihan kosong ditolak",
			req: CreateInvoiceRequest{
				MahasiswaID:        mhsID,
				NomorTagihan:       "   ",
				Semester:           "2026/2027-GANJIL",
				Nominal:            4500000,
				TanggalKedaluwarsa: "2026-12-31",
			},
			wantErr: ErrEmptyRequiredFields,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateInvoice(ctx, instID, tc.req)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("diharapkan error %v, didapat %v", tc.wantErr, err)
			}
		})
	}

	// Filter status tidak valid harus ditolak
	_, err := svc.ListInvoicesForAdmin(ctx, instID, InvoiceFilter{Status: "INVALID_STATUS"})
	if !errors.Is(err, ErrInvalidStatusFilter) {
		t.Errorf("filter status tidak valid harus menghasilkan ErrInvalidStatusFilter, didapat %v", err)
	}
}
