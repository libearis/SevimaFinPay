<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useBillingStore } from '@/stores/billing'

const auth = useAuthStore()
const billing = useBillingStore()
const router = useRouter()

const mahasiswaId = ref('')
const nomorTagihan = ref('UKT/2026-1/003')
const semester = ref('2026/2027-GANJIL')
const nominal = ref<number>(5500000)
const tanggalKedaluwarsa = ref('2026-12-31')

const formSubmitting = ref(false)
const formFeedback = ref<{ type: 'success' | 'error'; message: string } | null>(null)

function formatRupiah(amount: number): string {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 0
  }).format(amount)
}

async function handleCreateInvoice() {
  formFeedback.value = null
  if (!mahasiswaId.value) {
    formFeedback.value = { type: 'error', message: 'Silakan pilih mahasiswa terlebih dahulu.' }
    return
  }

  formSubmitting.value = true
  try {
    await billing.createAdminInvoice({
      mahasiswa_id: mahasiswaId.value,
      nomor_tagihan: nomorTagihan.value,
      semester: semester.value,
      nominal: Number(nominal.value),
      tanggal_kedaluwarsa: tanggalKedaluwarsa.value
    })
    router.push('/admin/invoices')
  } catch (err: any) {
    formFeedback.value = { type: 'error', message: err.message }
  } finally {
    formSubmitting.value = false
  }
}

onMounted(async () => {
  await Promise.all([billing.fetchInstitutionStudents(), billing.fetchAdminInvoices()])
  if (billing.students.length > 0) {
    mahasiswaId.value = billing.students[0].id
  }
  nomorTagihan.value = `UKT/2026-1/00${billing.invoices.length + 1}`
})
</script>

<template>
  <div class="create-page-wrapper">
    <div class="create-card card">
      <div class="create-header">
        <div>
          <span class="inst-badge">{{ auth.user?.kode_institusi }} &bull; {{ auth.user?.nama_institusi }}</span>
          <h1>Terbitkan Tagihan UKT Baru</h1>
          <p>
            Tagihan yang diterbitkan otomatis terikat pada institusi
            <strong>{{ auth.user?.nama_institusi }}</strong> dengan status awal <code>UNPAID</code>.
          </p>
        </div>
        <RouterLink to="/admin/invoices" class="btn btn-outline">
          &larr; Kembali ke Daftar
        </RouterLink>
      </div>

      <div
        v-if="formFeedback"
        class="alert"
        :class="formFeedback.type === 'success' ? 'alert-success' : 'alert-error'"
      >
        {{ formFeedback.message }}
      </div>

      <form @submit.prevent="handleCreateInvoice" class="create-form">
        <div class="form-group">
          <label for="mhs">
            <span>Pilih Mahasiswa</span>
            <span class="field-tag">Khusus Mahasiswa {{ auth.user?.kode_institusi }}</span>
          </label>
          <select id="mhs" v-model="mahasiswaId" required>
            <option value="" disabled>-- Pilih Mahasiswa --</option>
            <option v-for="m in billing.students" :key="m.id" :value="m.id">
              {{ m.nim }} — {{ m.name }} ({{ m.email }})
            </option>
          </select>
        </div>

        <div class="form-row-2">
          <div class="form-group">
            <label for="nomor">
              <span>Nomor Tagihan UKT</span>
              <span class="field-tag">Unik per Institusi</span>
            </label>
            <input
              id="nomor"
              v-model="nomorTagihan"
              type="text"
              required
              placeholder="Contoh: UKT/2026-1/003"
            />
          </div>

          <div class="form-group">
            <label for="semester">
              <span>Semester Akademik</span>
              <span class="field-tag">1 Tagihan / Semester</span>
            </label>
            <input
              id="semester"
              v-model="semester"
              type="text"
              required
              placeholder="Contoh: 2026/2027-GANJIL"
            />
          </div>
        </div>

        <div class="form-row-2">
          <div class="form-group">
            <label for="nominal">
              <span>Nominal UKT (IDR)</span>
              <span class="field-tag">{{ formatRupiah(nominal || 0) }}</span>
            </label>
            <input
              id="nominal"
              v-model.number="nominal"
              type="number"
              min="1"
              step="1"
              required
            />
          </div>

          <div class="form-group">
            <label for="due">Batas Waktu Pembayaran</label>
            <input id="due" v-model="tanggalKedaluwarsa" type="date" required />
          </div>
        </div>

        <div class="form-footer">
          <RouterLink to="/admin/invoices" class="btn btn-outline">Batal</RouterLink>
          <button type="submit" class="btn btn-crimson" :disabled="formSubmitting">
            {{ formSubmitting ? 'Menyimpan Tagihan...' : 'Terbitkan Tagihan UKT' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.create-page-wrapper {
  max-width: 760px;
  margin: 0.5rem auto 0;
}

.create-card {
  border-top: 4px solid var(--sevima-blue);
  padding: 2rem 2.25rem;
}

.create-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  flex-wrap: wrap;
  padding-bottom: 1.25rem;
  margin-bottom: 1.5rem;
  border-bottom: 1px solid var(--border-light);
}

.inst-badge {
  display: inline-block;
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--sevima-blue);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 0.3rem;
}

.create-header h1 {
  margin: 0 0 0.35rem;
  font-size: 1.4rem;
  font-weight: 800;
  color: var(--sevima-blue-dark);
}

.create-header p {
  margin: 0;
  font-size: 0.86rem;
  color: #64748b;
  max-width: 500px;
}

.create-form {
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
}

.form-row-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
}

@media (max-width: 640px) {
  .form-row-2 {
    grid-template-columns: 1fr;
  }
}

.form-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding-top: 1rem;
  margin-top: 0.5rem;
  border-top: 1px solid var(--border-light);
}
</style>
