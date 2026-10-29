<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()

const email = ref('admin@nusantara.ac.id')
const password = ref('KampusMerdeka2026!')
const loginError = ref<string | null>(null)

interface InstitutionGroup {
  code: string
  name: string
  accounts: {
    role: 'ADMIN_KEUANGAN' | 'MAHASISWA'
    roleLabel: string
    name: string
    identifier: string
    email: string
  }[]
}

const institutionGroups: InstitutionGroup[] = [
  {
    code: 'UNIV-NUSANTARA',
    name: 'Universitas Nusantara Raya',
    accounts: [
      {
        role: 'ADMIN_KEUANGAN',
        roleLabel: 'Admin Keuangan',
        name: 'Budi Santoso',
        identifier: 'NIP-19850101',
        email: 'admin@nusantara.ac.id'
      },
      {
        role: 'MAHASISWA',
        roleLabel: 'Mahasiswa',
        name: 'Andi Pratama',
        identifier: 'NIM 202610001',
        email: 'andi@mhs.nusantara.ac.id'
      }
    ]
  },
  {
    code: 'INST-TEKNOLOGI-MAJU',
    name: 'Institut Teknologi Maju Indonesia',
    accounts: [
      {
        role: 'ADMIN_KEUANGAN',
        roleLabel: 'Admin Keuangan',
        name: 'Dewi Lestari',
        identifier: 'NIP-19870202',
        email: 'admin@itmaju.ac.id'
      },
      {
        role: 'MAHASISWA',
        roleLabel: 'Mahasiswa',
        name: 'Rizky Ramadhan',
        identifier: 'NIM 202620001',
        email: 'rizky@mhs.itmaju.ac.id'
      }
    ]
  },
  {
    code: 'UNIV-MERDEKA',
    name: 'Universitas Merdeka Sejahtera',
    accounts: [
      {
        role: 'ADMIN_KEUANGAN',
        roleLabel: 'Admin Keuangan',
        name: 'Hendra Wijaya',
        identifier: 'NIP-19900303',
        email: 'admin@merdeka.ac.id'
      },
      {
        role: 'MAHASISWA',
        roleLabel: 'Mahasiswa',
        name: 'Fajar Nugroho',
        identifier: 'NIM 202630001',
        email: 'fajar@mhs.merdeka.ac.id'
      }
    ]
  }
]

function selectAccount(targetEmail: string) {
  email.value = targetEmail
  password.value = 'KampusMerdeka2026!'
  loginError.value = null
}

async function handleLogin() {
  loginError.value = null
  try {
    const profile = await auth.login(email.value, password.value)
    if (profile.role === 'ADMIN_KEUANGAN') {
      router.push('/admin/invoices')
    } else {
      router.push('/student/invoices')
    }
  } catch (err: any) {
    loginError.value = err.message
  }
}
</script>

<template>
  <div class="auth-wrapper">
    <div class="auth-shell">
      <!-- Left Column: Login Form -->
      <div class="auth-form-pane">
        <div class="eyebrow">
          <span class="eyebrow-dot"></span>
          SEVIMA EDUTECH ECOSYSTEM
        </div>
        <h1>Portal Manajemen Tagihan UKT</h1>
        <p class="lead">
          Setiap sesi pengguna diisolasi secara ketat berdasarkan klaim
          <code>institusi_id</code> pada token JWT.
        </p>

        <div v-if="loginError" class="alert alert-error">
          {{ loginError }}
        </div>

        <form @submit.prevent="handleLogin" class="auth-form">
          <div class="form-group">
            <label for="email">Email Institusi</label>
            <input
              id="email"
              v-model="email"
              type="email"
              required
              placeholder="nama@kampus.ac.id"
            />
          </div>

          <div class="form-group">
            <label for="password">
              <span>Password</span>
              <span class="field-tag">KampusMerdeka2026!</span>
            </label>
            <input
              id="password"
              v-model="password"
              type="password"
              required
              placeholder="Masukkan password"
            />
          </div>

          <button type="submit" class="btn btn-crimson btn-block btn-lg" :disabled="auth.loading">
            {{ auth.loading ? 'Memverifikasi Sesi...' : 'Masuk ke Portal' }}
          </button>
        </form>

        <div class="security-footnotes">
          <div class="sec-item">
            <span class="dot dot-blue"></span>
            <span>Isolasi Data Multi-Institusi (3 Perguruan Tinggi)</span>
          </div>
          <div class="sec-item">
            <span class="dot dot-red"></span>
            <span>Verifikasi Webhook Pembayaran Real-Time (HMAC-SHA256)</span>
          </div>
        </div>
      </div>

      <!-- Right Column: Organized Demo Account Selector per University -->
      <div class="auth-demo-pane">
        <div class="demo-pane-header">
          <div>
            <h2>Pilih Cepat Akun Demo (3 Institusi)</h2>
            <p>Klik salah satu akun di bawah untuk mengisi kredensial login otomatis.</p>
          </div>
        </div>

        <div class="inst-list">
          <div v-for="inst in institutionGroups" :key="inst.code" class="inst-group-card">
            <div class="inst-group-header">
              <span class="inst-code-pill">{{ inst.code }}</span>
              <span class="inst-title">{{ inst.name }}</span>
            </div>

            <div class="inst-accounts-row">
              <button
                v-for="acc in inst.accounts"
                :key="acc.email"
                type="button"
                class="acc-card"
                :class="{ selected: email === acc.email }"
                @click="selectAccount(acc.email)"
              >
                <div class="acc-top">
                  <span
                    class="acc-role-badge"
                    :class="acc.role === 'ADMIN_KEUANGAN' ? 'badge-admin' : 'badge-student'"
                  >
                    {{ acc.roleLabel }}
                  </span>
                  <span class="acc-id">{{ acc.identifier }}</span>
                </div>
                <div class="acc-name">{{ acc.name }}</div>
                <div class="acc-email">{{ acc.email }}</div>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth-wrapper {
  display: flex;
  justify-content: center;
  padding-top: 0.25rem;
}

.auth-shell {
  width: 100%;
  background: #ffffff;
  border: 1px solid var(--border-light);
  border-top: 4px solid var(--sevima-blue);
  border-radius: 16px;
  box-shadow: var(--shadow-md);
  display: grid;
  grid-template-columns: 440px 1fr;
  overflow: hidden;
}

@media (max-width: 980px) {
  .auth-shell {
    grid-template-columns: 1fr;
  }
}

.auth-form-pane {
  padding: 2.5rem 2.25rem;
  display: flex;
  flex-direction: column;
  justify-content: center;
  border-right: 1px solid var(--border-light);
}

.eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  align-self: flex-start;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.07em;
  color: var(--sevima-blue-dark);
  background: var(--sevima-blue-soft);
  border: 1px solid #bfdbfe;
  padding: 0.25rem 0.7rem;
  border-radius: 999px;
  margin-bottom: 0.9rem;
}

.eyebrow-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--sevima-red);
}

h1 {
  font-size: 1.55rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0 0 0.5rem;
  color: var(--sevima-blue-dark);
}

.lead {
  font-size: 0.88rem;
  color: #475569;
  margin: 0 0 1.65rem;
}

.auth-form {
  display: flex;
  flex-direction: column;
  gap: 1.15rem;
}

.btn-lg {
  padding: 0.75rem 1.2rem;
  font-size: 0.92rem;
  margin-top: 0.25rem;
}

.security-footnotes {
  margin-top: 2rem;
  padding-top: 1.25rem;
  border-top: 1px solid var(--border-light);
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.sec-item {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  font-size: 0.78rem;
  color: #64748b;
  font-weight: 500;
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.dot-blue {
  background: var(--sevima-blue);
}

.dot-red {
  background: var(--sevima-red);
}

/* Right Pane */
.auth-demo-pane {
  background: #f8fafc;
  padding: 2.25rem;
}

.demo-pane-header h2 {
  font-size: 1.12rem;
  font-weight: 800;
  margin: 0 0 0.25rem;
  color: var(--sevima-blue-dark);
}

.demo-pane-header p {
  font-size: 0.84rem;
  color: #64748b;
  margin: 0 0 1.25rem;
}

.inst-list {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.inst-group-card {
  background: #ffffff;
  border: 1px solid var(--border-light);
  border-radius: 12px;
  padding: 0.95rem 1.1rem;
}

.inst-group-header {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 0.75rem;
}

.inst-code-pill {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--sevima-blue-dark);
  background: var(--sevima-blue-soft);
  border: 1px solid #bfdbfe;
  padding: 0.15rem 0.5rem;
  border-radius: 5px;
}

.inst-title {
  font-size: 0.85rem;
  font-weight: 700;
  color: #1e293b;
}

.inst-accounts-row {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 0.75rem;
}

@media (max-width: 640px) {
  .inst-accounts-row {
    grid-template-columns: 1fr;
  }
}

.acc-card {
  text-align: left;
  background: #f8fafc;
  border: 1.5px solid #e2e8f0;
  border-radius: 10px;
  padding: 0.75rem 0.9rem;
  cursor: pointer;
  transition: all 0.15s ease;
}

.acc-card:hover {
  border-color: #93c5fd;
  background: var(--sevima-blue-soft);
}

.acc-card.selected {
  border-color: var(--sevima-blue);
  background: var(--sevima-blue-soft);
  box-shadow: 0 0 0 3px rgba(15, 76, 156, 0.12);
}

.acc-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.35rem;
}

.acc-role-badge {
  font-size: 0.65rem;
  font-weight: 700;
  padding: 0.12rem 0.45rem;
  border-radius: 4px;
  text-transform: uppercase;
}

.badge-admin {
  background: var(--sevima-red-soft);
  color: var(--sevima-red);
  border: 1px solid #fecdd3;
}

.badge-student {
  background: #e0f2fe;
  color: #0369a1;
  border: 1px solid #bae6fd;
}

.acc-id {
  font-size: 0.7rem;
  color: #64748b;
  font-weight: 500;
}

.acc-name {
  font-size: 0.88rem;
  font-weight: 700;
  color: #0f172a;
}

.acc-email {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.74rem;
  color: var(--sevima-blue);
  margin-top: 0.15rem;
}
</style>
