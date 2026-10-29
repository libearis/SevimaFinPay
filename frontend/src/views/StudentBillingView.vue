<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useBillingStore, type Invoice } from '@/stores/billing'

const auth = useAuthStore()
const billing = useBillingStore()
const router = useRouter()

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

function goToSimulator(inv: Invoice) {
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
  await billing.fetchStudentInvoices()
})
</script>

<template>
  <div class="student-view">
    <div class="page-header">
      <div>
        <div class="inst-eyebrow">
          {{ auth.user?.kode_institusi }} &bull; {{ auth.user?.nama_institusi }} &bull; NIM
          {{ auth.user?.nim_nip }}
        </div>
        <h1>Tagihan UKT &amp; Status KRS Saya</h1>
      </div>

      <button type="button" class="btn btn-outline" @click="billing.fetchStudentInvoices()">
        Muat Ulang Data
      </button>
    </div>

    <div v-if="billing.error" class="alert alert-error">{{ billing.error }}</div>

    <div v-if="billing.invoices.length === 0 && !billing.loading" class="card empty-state">
      Belum ada tagihan UKT yang diterbitkan untuk akun Anda.
    </div>

    <div class="invoice-cards">
      <article v-for="inv in billing.invoices" :key="inv.id" class="card invoice-card">
        <div class="inv-top">
          <div>
            <span class="semester-tag">Semester {{ inv.semester }}</span>
            <h3>Tagihan <code>{{ inv.nomor_tagihan }}</code></h3>
          </div>
          <div class="inv-badges">
            <span class="status-pill" :class="`status-${inv.status.toLowerCase()}`">
              {{ inv.status }}
            </span>
          </div>
        </div>

        <div class="inv-meta-grid">
          <div class="meta-item">
            <span class="meta-label">Nominal Tagihan UKT</span>
            <span class="meta-val amount">{{ formatRupiah(inv.nominal) }}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Batas Waktu Pembayaran</span>
            <span class="meta-val">{{ formatDate(inv.tanggal_kedaluwarsa) }}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Tanggal Pelunasan</span>
            <span class="meta-val">{{ formatDate(inv.paid_at) }}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Status Pengisian KRS</span>
            <span
              class="meta-val krs-badge"
              :class="inv.eligible_for_krs ? 'krs-eligible' : 'krs-blocked'"
            >
              {{
                inv.eligible_for_krs
                  ? 'KRS Aktif (Dapat Mengisi KRS)'
                  : 'KRS Tertahan (Belum Lunas)'
              }}
            </span>
          </div>
        </div>

        <div v-if="inv.status !== 'PAID'" class="pay-cta-bar">
          <span>
            Lakukan simulasi pembayaran tagihan ini melalui Webhook Payment Gateway:
          </span>
          <button type="button" class="btn btn-crimson btn-xs" @click="goToSimulator(inv)">
            Bayar via Simulator Payment Gateway
          </button>
        </div>

        <!-- Riwayat Transaksi -->
        <div class="tx-section">
          <h4>Riwayat Transaksi ({{ inv.transactions?.length || 0 }})</h4>
          <div v-if="!inv.transactions?.length" class="no-tx">
            Belum ada riwayat transaksi untuk tagihan ini.
          </div>
          <div v-else class="table-responsive">
            <table class="data-table">
              <thead>
                <tr>
                  <th>ID Referensi</th>
                  <th>Event</th>
                  <th>Kanal Pembayaran</th>
                  <th>Nominal</th>
                  <th>Status</th>
                  <th>Tanggal</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="tx in inv.transactions" :key="tx.id">
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
      </article>
    </div>
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

.invoice-cards {
  display: flex;
  flex-direction: column;
  gap: 1.35rem;
}

.inv-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  border-bottom: 1px solid var(--border-light);
  padding-bottom: 0.9rem;
  margin-bottom: 1.1rem;
}

.semester-tag {
  font-size: 0.74rem;
  font-weight: 700;
  color: var(--sevima-red);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.inv-top h3 {
  margin: 0.2rem 0 0;
  font-size: 1.15rem;
  color: var(--sevima-blue-dark);
}

.inv-meta-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
  margin-bottom: 1.2rem;
}

@media (max-width: 900px) {
  .inv-meta-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.meta-item {
  background: #f8fafc;
  padding: 0.9rem 1.05rem;
  border-radius: 10px;
  border: 1px solid var(--border-light);
}

.meta-label {
  display: block;
  font-size: 0.75rem;
  color: #64748b;
  margin-bottom: 0.25rem;
}

.meta-val {
  font-size: 0.92rem;
  font-weight: 700;
  color: #0f172a;
}

.meta-val.amount {
  font-size: 1.2rem;
  color: var(--sevima-blue);
}

.krs-badge {
  display: inline-block;
  font-size: 0.75rem;
  padding: 0.18rem 0.55rem;
  border-radius: 5px;
}

.krs-eligible {
  background: #dcfce7;
  color: #166534;
}

.krs-blocked {
  background: var(--sevima-red-soft);
  color: var(--sevima-red);
}

.pay-cta-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
  background: var(--sevima-blue-soft);
  border: 1px solid #bfdbfe;
  padding: 0.85rem 1.15rem;
  border-radius: 10px;
  margin-bottom: 1.15rem;
  font-size: 0.86rem;
  color: var(--sevima-blue-dark);
}

.tx-section h4 {
  font-size: 0.92rem;
  margin: 0 0 0.75rem;
  color: var(--sevima-blue-dark);
}

.no-tx {
  font-size: 0.84rem;
  color: #64748b;
  background: #f8fafc;
  padding: 0.85rem 1rem;
  border-radius: 8px;
}
</style>
