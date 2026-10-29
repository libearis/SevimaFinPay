<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useBillingStore, type Invoice } from '@/stores/billing'

const auth = useAuthStore()
const billing = useBillingStore()
const router = useRouter()

const selectedInvoice = ref<Invoice | null>(null)

const summaryStats = computed(() => {
  const list = billing.invoices
  return {
    total: list.length,
    unpaid: list.filter((i) => i.status === 'UNPAID').length,
    paid: list.filter((i) => i.status === 'PAID').length,
    expired: list.filter((i) => i.status === 'EXPIRED').length
  }
})

function formatRupiah(amount: number): string {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 0
  }).format(amount)
}

function formatDate(iso: string | null): string {
  if (!iso) return '-'
  return new Date(iso).toLocaleDateString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric'
  })
}

async function applyFilter() {
  await billing.fetchAdminInvoices()
}

async function resetFilter() {
  billing.statusFilter = ''
  billing.semesterFilter = ''
  await billing.fetchAdminInvoices()
}

function goToMockGateway(inv: Invoice) {
  router.push({
    path: '/mock-gateway',
    query: {
      institution: auth.user?.kode_institusi || '',
      invoice: inv.nomor_tagihan,
      amount: String(inv.nominal)
    }
  })
}

onMounted(async () => {
  await billing.fetchAdminInvoices()
})
</script>

<template>
  <div class="admin-dashboard">
    <!-- Clean Page Header with Primary CTA to Separate Create Page -->
    <div class="page-header">
      <div>
        <div class="inst-eyebrow">{{ auth.user?.kode_institusi }} &bull; {{ auth.user?.nama_institusi }}</div>
        <h1>Manajemen Tagihan UKT Mahasiswa</h1>
      </div>

      <div class="header-actions">
        <button type="button" class="btn btn-outline" @click="billing.fetchAdminInvoices()">
          Muat Ulang Data
        </button>
      </div>
    </div>

    <!-- Summary Metric Cards -->
    <div class="stats-grid">
      <div class="stat-card stat-total">
        <div class="stat-label">Total Tagihan</div>
        <div class="stat-value">{{ summaryStats.total }}</div>
      </div>
      <div class="stat-card stat-unpaid">
        <div class="stat-label">Belum Bayar (UNPAID)</div>
        <div class="stat-value">{{ summaryStats.unpaid }}</div>
      </div>
      <div class="stat-card stat-paid">
        <div class="stat-label">Lunas (PAID / KRS Aktif)</div>
        <div class="stat-value">{{ summaryStats.paid }}</div>
      </div>
      <div class="stat-card stat-expired">
        <div class="stat-label">Kedaluwarsa (EXPIRED)</div>
        <div class="stat-value">{{ summaryStats.expired }}</div>
      </div>
    </div>

    <!-- Full-Width Spacious Invoices Table Card -->
    <section class="card">
      <div class="table-toolbar">
        <div>
          <h3>Daftar Tagihan UKT</h3>
          <p class="sub-desc">
            Data terisolasi khusus untuk mahasiswa <strong>{{ auth.user?.nama_institusi }}</strong>.
          </p>
        </div>

        <div class="filter-bar">
          <select v-model="billing.statusFilter" @change="applyFilter" class="filter-select">
            <option value="">Semua Status</option>
            <option value="UNPAID">UNPAID (Belum Bayar)</option>
            <option value="PAID">PAID (Lunas)</option>
            <option value="EXPIRED">EXPIRED (Kedaluwarsa)</option>
          </select>

          <input
            v-model="billing.semesterFilter"
            type="text"
            class="filter-input"
            placeholder="Filter semester (mis. 2026)..."
            @keyup.enter="applyFilter"
          />

          <button type="button" class="btn btn-primary" @click="applyFilter">Filter</button>
          <button
            v-if="billing.statusFilter || billing.semesterFilter"
            type="button"
            class="btn btn-outline"
            @click="resetFilter"
          >
            Reset
          </button>
        </div>
      </div>

      <div v-if="billing.error" class="alert alert-error">{{ billing.error }}</div>

      <div class="table-responsive">
        <table class="data-table">
          <thead>
            <tr>
              <th>Nomor Tagihan</th>
              <th>Mahasiswa</th>
              <th>Semester</th>
              <th>Nominal UKT</th>
              <th>Status Tagihan</th>
              <th>Status KRS</th>
              <th>Jatuh Tempo</th>
              <th style="text-align: right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="billing.invoices.length === 0">
              <td colspan="8" class="empty-cell">
                Tidak ada data tagihan UKT yang sesuai dengan filter.
              </td>
            </tr>
            <tr v-for="inv in billing.invoices" :key="inv.id">
              <td class="mono-bold nowrap">{{ inv.nomor_tagihan }}</td>
              <td>
                <div class="mhs-name">{{ inv.mahasiswa_nama }}</div>
                <div class="mhs-nim">NIM {{ inv.mahasiswa_nim }}</div>
              </td>
              <td class="nowrap">{{ inv.semester }}</td>
              <td class="mono-bold nowrap">{{ formatRupiah(inv.nominal) }}</td>
              <td class="nowrap">
                <span class="status-pill" :class="`status-${inv.status.toLowerCase()}`">
                  {{ inv.status }}
                </span>
              </td>
              <td class="nowrap">
                <span class="krs-tag" :class="inv.eligible_for_krs ? 'krs-ok' : 'krs-hold'">
                  {{ inv.eligible_for_krs ? 'KRS Aktif' : 'KRS Tertahan' }}
                </span>
              </td>
              <td class="nowrap">
                <div>{{ formatDate(inv.tanggal_kedaluwarsa) }}</div>
                <div v-if="inv.paid_at" class="paid-at-note">
                  Lunas {{ formatDate(inv.paid_at) }}
                </div>
              </td>
              <td class="nowrap" style="text-align: right">
                <button
                  type="button"
                  class="btn btn-xs btn-outline"
                  @click="selectedInvoice = inv"
                >
                  Riwayat Transaksi ({{ inv.transactions?.length || 0 }})
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Transaction History Drawer -->
      <div v-if="selectedInvoice" class="tx-drawer">
        <div class="tx-drawer-header">
          <h4>
            Riwayat Transaksi Tagihan <code>{{ selectedInvoice.nomor_tagihan }}</code> ({{
              selectedInvoice.mahasiswa_nama
            }})
          </h4>
          <button type="button" class="btn btn-xs btn-outline" @click="selectedInvoice = null">
            Tutup Riwayat
          </button>
        </div>

        <div v-if="!selectedInvoice.transactions?.length" class="empty-tx">
          Belum ada notifikasi transaksi dari Payment Gateway untuk tagihan ini.
        </div>
        <div v-else class="table-responsive">
          <table class="data-table">
            <thead>
              <tr>
                <th>Event ID (Idempotency Key)</th>
                <th>Event Type</th>
                <th>Kanal Pembayaran</th>
                <th>Nominal</th>
                <th>Status</th>
                <th>Waktu Diproses</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="tx in selectedInvoice.transactions" :key="tx.id">
                <td class="mono">{{ tx.event_id }}</td>
                <td><code>{{ tx.event_type }}</code></td>
                <td>{{ tx.payment_channel }}</td>
                <td class="mono-bold">{{ formatRupiah(tx.amount) }}</td>
                <td>
                  <span
                    class="status-pill"
                    :class="tx.status === 'SUCCESS' ? 'status-paid' : 'status-expired'"
                  >
                    {{ tx.status }}
                  </span>
                </td>
                <td>{{ formatDate(tx.processed_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 1rem;
  flex-wrap: wrap;
  margin-bottom: 1.5rem;
}

.inst-eyebrow {
  font-size: 0.76rem;
  font-weight: 700;
  color: var(--sevima-blue);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 0.25rem;
}

.page-header h1 {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 800;
  color: var(--sevima-blue-dark);
  letter-spacing: -0.02em;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
  margin-bottom: 1.5rem;
}

@media (max-width: 860px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.stat-card {
  background: #ffffff;
  border: 1px solid var(--border-light);
  border-radius: 12px;
  padding: 1.15rem 1.35rem;
  box-shadow: var(--shadow-xs);
}

.stat-label {
  font-size: 0.78rem;
  color: #64748b;
  font-weight: 600;
}

.stat-value {
  font-size: 1.75rem;
  font-weight: 800;
  color: var(--sevima-blue-dark);
  margin-top: 0.2rem;
}

.stat-total {
  border-top: 3px solid var(--sevima-blue);
}

.stat-unpaid {
  border-top: 3px solid #f59e0b;
}

.stat-paid {
  border-top: 3px solid #10b981;
}

.stat-expired {
  border-top: 3px solid var(--sevima-red);
}

.table-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 1rem;
  margin-bottom: 1.25rem;
}

.table-toolbar h3 {
  margin: 0 0 0.2rem;
  font-size: 1.1rem;
  font-weight: 800;
  color: var(--sevima-blue-dark);
}

.sub-desc {
  margin: 0;
  font-size: 0.83rem;
  color: #64748b;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.filter-select {
  width: 200px;
}

.filter-input {
  width: 210px;
}

.nowrap {
  white-space: nowrap;
}

.mhs-name {
  font-weight: 700;
  color: #0f172a;
}

.mhs-nim {
  font-size: 0.75rem;
  color: #64748b;
}

.krs-tag {
  font-size: 0.75rem;
  font-weight: 700;
}

.krs-ok {
  color: #15803d;
}

.krs-hold {
  color: #b45309;
}

.paid-at-note {
  font-size: 0.72rem;
  color: #15803d;
  margin-top: 0.15rem;
}

.action-buttons {
  display: inline-flex;
  gap: 0.45rem;
}

.tx-drawer {
  margin-top: 1.5rem;
  padding-top: 1.25rem;
  border-top: 1px solid var(--border-light);
}

.tx-drawer-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.85rem;
}

.tx-drawer-header h4 {
  margin: 0;
  font-size: 0.96rem;
  color: var(--sevima-blue-dark);
}

.empty-tx {
  padding: 1rem;
  background: #f8fafc;
  border-radius: 8px;
  font-size: 0.84rem;
  color: #64748b;
}
</style>
