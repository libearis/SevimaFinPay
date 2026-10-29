<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'

const route = useRoute()
const auth = useAuthStore()

const institutionCode = ref(
  (route.query.institution as string) || auth.user?.kode_institusi || 'UNIV-NUSANTARA'
)
const nomorTagihan = ref((route.query.invoice as string) || 'UKT/2026-1/001')
const eventType = ref('payment.success')
const amount = ref<number>(Number(route.query.amount) || 5500000)
const paymentChannel = ref('VA_MANDIRI')
const eventId = ref('')
const simulationMode = ref<'VALID' | 'INVALID_SIGNATURE' | 'WRONG_INSTITUTION_SECRET'>('VALID')

const sending = ref(false)
const triggerResult = ref<{
  computed_signature: string
  sent_payload: string
  http_status: number
  webhook_response: any
} | null>(null)

interface WebhookLogItem {
  id: string
  institution_code: string
  nomor_tagihan: string
  event_id: string
  event_type: string
  signature_header: string
  signature_valid: boolean
  http_status: number
  processing_result: string
  error_message: string
  raw_payload: string
  received_at: string
}

const auditLogs = ref<WebhookLogItem[]>([])

function generateNewEventId() {
  eventId.value = `evt_${Date.now()}`
}

async function fetchAuditLogs() {
  const params: Record<string, string> = {
    institution_code: institutionCode.value
  }
  const res = await apiClient.get('/mock-gateway/logs', { params })
  auditLogs.value = res.data.data || []
}

async function sendWebhook() {
  if (!eventId.value) {
    generateNewEventId()
  }
  sending.value = true
  try {
    const res = await apiClient.post('/mock-gateway/trigger', {
      institution_code: institutionCode.value,
      nomor_tagihan: nomorTagihan.value,
      event_type: eventType.value,
      amount: Number(amount.value),
      payment_channel: paymentChannel.value,
      event_id: eventId.value,
      simulation_mode: simulationMode.value
    })
    triggerResult.value = res.data.data
    await fetchAuditLogs()
  } finally {
    sending.value = false
  }
}

async function sendWebhookWithFreshEventId() {
  generateNewEventId()
  await sendWebhook()
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('id-ID', {
    dateStyle: 'short',
    timeStyle: 'medium'
  })
}

onMounted(async () => {
  generateNewEventId()
  await fetchAuditLogs()
})
</script>

<template>
  <div class="simulator-page">
    <div class="page-header">
      <div>
        <div class="inst-eyebrow">
          SIMULATOR PAYMENT GATEWAY &bull; POST /api/webhooks/payment
        </div>
        <h1>Simulasi Pembayaran Webhook (HMAC-SHA256)</h1>
      </div>

      <RouterLink v-if="auth.isMahasiswa" to="/student/invoices" class="btn btn-outline">
        &larr; Kembali ke Tagihan Saya
      </RouterLink>
    </div>

    <!-- Top Workspace: Left (Clean Form) + Right (Live HTTP & HMAC Inspector) -->
    <div class="sim-workspace-grid">
      <section class="card sim-form-card">
        <div class="section-heading">
          <h3>Konfigurasi Payload Webhook</h3>
          <p>Uji pengiriman notifikasi pembayaran untuk tagihan Anda.</p>
        </div>

        <form @submit.prevent="sendWebhookWithFreshEventId" class="sim-form">
          <div class="form-row-2">
            <div class="form-group">
              <label>
                <span>Institusi</span>
                <span class="field-tag">institution_code</span>
              </label>
              <input v-model="institutionCode" type="text" readonly class="readonly-input" />
            </div>

            <div class="form-group">
              <label>
                <span>Nomor Tagihan</span>
                <span class="field-tag">nomor_tagihan</span>
              </label>
              <input v-model="nomorTagihan" type="text" required placeholder="UKT/2026-1/001" />
            </div>
          </div>

          <div class="form-row-2">
            <div class="form-group">
              <label>
                <span>Jenis Event</span>
                <span class="field-tag">event_type</span>
              </label>
              <select v-model="eventType">
                <option value="payment.success">payment.success (Lunas)</option>
                <option value="payment.expired">payment.expired (Kedaluwarsa)</option>
              </select>
            </div>

            <div class="form-group">
              <label>
                <span>Kanal Bayar</span>
                <span class="field-tag">payment_channel</span>
              </label>
              <select v-model="paymentChannel">
                <option value="VA_MANDIRI">VA_MANDIRI</option>
                <option value="VA_BNI">VA_BNI</option>
                <option value="VA_BRI">VA_BRI</option>
                <option value="QRIS">QRIS</option>
              </select>
            </div>
          </div>

          <div class="form-row-2">
            <div class="form-group">
              <label>
                <span>Nominal (IDR)</span>
                <span class="field-tag">amount</span>
              </label>
              <input v-model.number="amount" type="number" required />
            </div>

            <div class="form-group">
              <label>
                <span>Event ID</span>
                <span class="field-tag">event_id</span>
              </label>
              <div class="input-with-action">
                <input v-model="eventId" type="text" required />
                <button type="button" class="btn btn-outline btn-xs" @click="generateNewEventId">
                  Acak
                </button>
              </div>
            </div>
          </div>

          <div class="form-group">
            <label>
              <span>Mode Signature HMAC-SHA256</span>
              <span class="field-tag">X-Signature</span>
            </label>
            <select v-model="simulationMode">
              <option value="VALID">
                Signature Valid — Gunakan Secret HMAC Institusi (200 OK)
              </option>
              <option value="INVALID_SIGNATURE">
                Signature Palsu / Tampered — Uji Penolakan (401 Unauthorized)
              </option>
              <option value="WRONG_INSTITUTION_SECRET">
                Secret Kampus Lain — Uji Isolasi Antar-Institusi (401 Unauthorized)
              </option>
            </select>
          </div>

          <div class="sim-actions">
            <button type="submit" class="btn btn-crimson btn-block" :disabled="sending">
              {{ sending ? 'Mengirim Webhook...' : 'Kirim Webhook Pembayaran' }}
            </button>
            <button
              type="button"
              class="btn btn-outline btn-block"
              :disabled="sending"
              @click="sendWebhook"
            >
              Kirim Ulang Event ID Sama (Uji Idempotent Retry)
            </button>
          </div>
        </form>
      </section>

      <!-- Right Column: Live HTTP & Signature Inspector -->
      <section class="card sim-inspector-card">
        <div class="section-heading">
          <h3>Hasil Verifikasi Signature &amp; Respons Server</h3>
          <p>
            Perhitungan header <code>X-Signature</code> di atas <em>raw request body</em> dan
            balasan JSON dari endpoint webhook.
          </p>
        </div>

        <div v-if="!triggerResult" class="inspector-placeholder">
          <div class="placeholder-title">Belum Ada Webhook yang Dikirimkan</div>
          <p>
            Klik tombol <strong>Kirim Webhook Pembayaran</strong> di sebelah kiri untuk memproses
            pembayaran dan melihat respons server secara langsung.
          </p>
        </div>

        <div v-else class="inspector-console">
          <div class="console-statusbar">
            <span>Status Respons Endpoint</span>
            <span
              class="status-pill"
              :class="triggerResult.http_status === 200 ? 'status-paid' : 'status-expired'"
            >
              HTTP {{ triggerResult.http_status }}
            </span>
          </div>

          <div class="console-block">
            <div class="console-label">Header X-Signature (HMAC-SHA256)</div>
            <div class="console-code sig-highlight">{{ triggerResult.computed_signature }}</div>
          </div>

          <div class="console-block">
            <div class="console-label">Raw Request Body</div>
            <pre class="console-pre">{{ triggerResult.sent_payload }}</pre>
          </div>

          <div class="console-block">
            <div class="console-label">Response Body</div>
            <pre class="console-pre">{{ JSON.stringify(triggerResult.webhook_response, null, 2) }}</pre>
          </div>
        </div>
      </section>
    </div>

    <!-- Bottom Full-Width Card: Webhook Audit Logs Table -->
    <section class="card audit-log-card">
      <div class="audit-toolbar">
        <div>
          <h3>Jejak Audit Webhook (<code>webhook_logs</code>)</h3>
          <p>Riwayat notifikasi webhook terbaru untuk institusi {{ institutionCode }}.</p>
        </div>

        <button type="button" class="btn btn-outline" @click="fetchAuditLogs">
          Muat Ulang Log
        </button>
      </div>

      <div class="table-responsive">
        <table class="data-table">
          <thead>
            <tr>
              <th>Waktu</th>
              <th>Institusi</th>
              <th>Nomor Tagihan</th>
              <th>Event &amp; ID</th>
              <th>HMAC</th>
              <th>HTTP</th>
              <th>Hasil Pemrosesan</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="auditLogs.length === 0">
              <td colspan="7" class="empty-cell">
                Belum ada riwayat notifikasi webhook yang tercatat.
              </td>
            </tr>
            <tr v-for="log in auditLogs" :key="log.id">
              <td class="mono">{{ formatDate(log.received_at) }}</td>
              <td>
                <span class="inst-chip">{{ log.institution_code || '-' }}</span>
              </td>
              <td class="mono-bold">{{ log.nomor_tagihan || '-' }}</td>
              <td>
                <div class="mono-bold">{{ log.event_type }}</div>
                <div class="sub-mono">{{ log.event_id }}</div>
              </td>
              <td>
                <span
                  class="status-pill"
                  :class="log.signature_valid ? 'status-paid' : 'status-expired'"
                >
                  {{ log.signature_valid ? 'VALID' : 'INVALID' }}
                </span>
              </td>
              <td>
                <span class="http-code" :class="log.http_status === 200 ? 'http-ok' : 'http-err'">
                  {{ log.http_status }}
                </span>
              </td>
              <td>
                <div class="mono-bold">{{ log.processing_result }}</div>
                <div v-if="log.error_message" class="incident-msg">{{ log.error_message }}</div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.simulator-page {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 1rem;
  flex-wrap: wrap;
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

.sim-workspace-grid {
  display: grid;
  grid-template-columns: 470px 1fr;
  gap: 1.5rem;
  align-items: stretch;
}

@media (max-width: 1080px) {
  .sim-workspace-grid {
    grid-template-columns: 1fr;
  }
}

.section-heading {
  margin-bottom: 1.15rem;
  padding-bottom: 0.85rem;
  border-bottom: 1px solid var(--border-light);
}

.section-heading h3 {
  margin: 0 0 0.2rem;
  font-size: 1.05rem;
  font-weight: 800;
  color: var(--sevima-blue-dark);
}

.section-heading p {
  margin: 0;
  font-size: 0.82rem;
  color: #64748b;
}

.sim-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.readonly-input {
  background: #f1f5f9;
  color: #475569;
  font-weight: 600;
}

.form-row-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.85rem;
}

.input-with-action {
  display: flex;
  gap: 0.45rem;
}

.input-with-action input {
  flex: 1;
}

.sim-actions {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  margin-top: 0.35rem;
}

.sim-inspector-card {
  display: flex;
  flex-direction: column;
}

.inspector-placeholder {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 2.5rem 2rem;
  background: #f8fafc;
  border: 1.5px dashed #cbd5e1;
  border-radius: 12px;
}

.placeholder-title {
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--sevima-blue-dark);
  margin-bottom: 0.35rem;
}

.inspector-placeholder p {
  margin: 0;
  font-size: 0.84rem;
  color: #64748b;
  max-width: 420px;
}

.inspector-console {
  background: #0f172a;
  color: #f8fafc;
  border-radius: 12px;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.console-statusbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 0.7rem;
  border-bottom: 1px solid #1e293b;
  font-size: 0.82rem;
  font-weight: 600;
  color: #94a3b8;
}

.console-label {
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: #94a3b8;
  margin-bottom: 0.3rem;
}

.console-code {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.76rem;
  background: #1e293b;
  padding: 0.55rem 0.7rem;
  border-radius: 8px;
  word-break: break-all;
}

.sig-highlight {
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.25);
}

.console-pre {
  margin: 0;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.76rem;
  background: #1e293b;
  color: #e2e8f0;
  padding: 0.7rem;
  border-radius: 8px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

.audit-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
  margin-bottom: 1.15rem;
}

.audit-toolbar h3 {
  margin: 0 0 0.2rem;
  font-size: 1.05rem;
  font-weight: 800;
  color: var(--sevima-blue-dark);
}

.audit-toolbar p {
  margin: 0;
  font-size: 0.82rem;
  color: #64748b;
}

.inst-chip {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.72rem;
  font-weight: 700;
  background: var(--sevima-blue-soft);
  color: var(--sevima-blue-dark);
  padding: 0.2rem 0.5rem;
  border-radius: 5px;
  border: 1px solid #bfdbfe;
}

.sub-mono {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.72rem;
  color: #64748b;
  margin-top: 0.15rem;
}

.http-code {
  font-family: 'JetBrains Mono', monospace;
  font-weight: 700;
  font-size: 0.82rem;
}

.http-ok {
  color: #16a34a;
}

.http-err {
  color: var(--sevima-red);
}

.incident-msg {
  font-size: 0.76rem;
  color: var(--sevima-red);
  margin-top: 0.2rem;
}
</style>
