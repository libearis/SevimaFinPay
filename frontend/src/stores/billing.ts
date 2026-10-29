import { defineStore } from 'pinia'
import { ref } from 'vue'
import apiClient from '@/api/client'

export interface StudentSummary {
  id: string
  institusi_id: string
  nim: string
  name: string
  email: string
}

export interface TransactionRecord {
  id: string
  institusi_id: string
  invoice_id: string
  event_id: string
  event_type: 'payment.success' | 'payment.expired'
  payment_channel: string
  amount: number
  status: 'SUCCESS' | 'EXPIRED'
  processed_at: string
}

export interface Invoice {
  id: string
  institusi_id: string
  mahasiswa_id: string
  mahasiswa_nim: string
  mahasiswa_nama: string
  mahasiswa_email: string
  nomor_tagihan: string
  semester: string
  nominal: number
  status: 'UNPAID' | 'PAID' | 'EXPIRED'
  eligible_for_krs: boolean
  tanggal_kedaluwarsa: string
  paid_at: string | null
  created_at: string
  updated_at: string
  transactions: TransactionRecord[]
}

export interface CreateInvoicePayload {
  mahasiswa_id: string
  nomor_tagihan: string
  semester: string
  nominal: number
  tanggal_kedaluwarsa: string
}

export const useBillingStore = defineStore('billing', () => {
  const invoices = ref<Invoice[]>([])
  const students = ref<StudentSummary[]>([])
  const statusFilter = ref<string>('')
  const semesterFilter = ref<string>('')
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Admin: ambil daftar mahasiswa di institusi admin
  async function fetchInstitutionStudents() {
    const res = await apiClient.get('/admin/students')
    students.value = res.data.data || []
  }

  // Admin: ambil daftar tagihan UKT di institusi admin dengan filter status & semester
  async function fetchAdminInvoices() {
    loading.value = true
    error.value = null
    try {
      const params: Record<string, string> = {}
      if (statusFilter.value) params.status = statusFilter.value
      if (semesterFilter.value) params.semester = semesterFilter.value

      const res = await apiClient.get('/admin/invoices', { params })
      invoices.value = res.data.data || []
    } catch (err: any) {
      error.value =
        err.response?.data?.error?.message || 'Gagal memuat daftar tagihan UKT institusi.'
    } finally {
      loading.value = false
    }
  }

  // Admin: buat tagihan UKT baru untuk mahasiswa di institusinya
  async function createAdminInvoice(payload: CreateInvoicePayload): Promise<Invoice> {
    error.value = null
    try {
      const res = await apiClient.post('/admin/invoices', payload)
      const created: Invoice = res.data.data
      await fetchAdminInvoices()
      return created
    } catch (err: any) {
      const msg =
        err.response?.data?.error?.message || 'Gagal membuat tagihan UKT baru.'
      throw new Error(msg)
    }
  }

  // Mahasiswa: ambil daftar tagihan UKT & riwayat transaksi milik mahasiswa yang sedang login
  async function fetchStudentInvoices() {
    loading.value = true
    error.value = null
    try {
      const res = await apiClient.get('/student/invoices')
      invoices.value = res.data.data || []
    } catch (err: any) {
      error.value =
        err.response?.data?.error?.message || 'Gagal memuat status tagihan UKT mahasiswa.'
    } finally {
      loading.value = false
    }
  }

  return {
    invoices,
    students,
    statusFilter,
    semesterFilter,
    loading,
    error,
    fetchInstitutionStudents,
    fetchAdminInvoices,
    createAdminInvoice,
    fetchStudentInvoices
  }
})
