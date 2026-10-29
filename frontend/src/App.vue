<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()

// Bersihkan nama jika di localStorage masih tersimpan "(Admin Keuangan ...)" dari sesi sebelumnya
const cleanUserName = computed(() => {
  const raw = auth.user?.name || ''
  return raw.replace(/\s*\(.*?\)\s*/g, '').trim()
})

const userInitials = computed(() => {
  const parts = cleanUserName.value.split(' ').filter(Boolean)
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase()
  }
  return cleanUserName.value.slice(0, 2).toUpperCase() || 'U'
})

const roleLabel = computed(() => {
  if (!auth.user) return ''
  return auth.user.role === 'ADMIN_KEUANGAN' ? 'Admin Keuangan' : 'Mahasiswa'
})

function handleLogout() {
  auth.logout()
  router.push('/login')
}
</script>

<template>
  <div class="app-shell">
    <!-- SEVIMA Signature Red-Blue Top Accent Bar -->
    <div class="sevima-accent-bar"></div>

    <header class="navbar">
      <div class="navbar-container">
        <!-- Brand Identity -->
        <div class="brand-group" @click="router.push('/')" style="cursor: pointer">
          <div class="brand-mark">
            <span class="brand-s">S</span>
            <span class="brand-dot"></span>
          </div>
          <div class="brand-text">
            <div class="brand-name-row">
              <span class="brand-sevima">SEVIMA</span>
              <span class="brand-product">FinPay</span>
            </div>
            <span class="brand-tagline">Sistem Tagihan UKT Multi-Institusi</span>
          </div>
        </div>

        <!-- Navigation Links -->
        <nav v-if="auth.isAuthenticated" class="nav-menu">
          <RouterLink
            v-if="auth.isAdmin"
            to="/admin/invoices"
            class="nav-item"
            exact-active-class="router-link-active"
          >
            Daftar Tagihan
          </RouterLink>

          <RouterLink
            v-if="auth.isAdmin"
            to="/admin/invoices/create"
            class="nav-item"
            exact-active-class="router-link-active"
          >
            Terbitkan Tagihan
          </RouterLink>

          <RouterLink
            v-if="auth.isMahasiswa"
            to="/student/invoices"
            class="nav-item"
            exact-active-class="router-link-active"
          >
            Tagihan &amp; KRS Saya
          </RouterLink>

          <RouterLink
            v-if="auth.isMahasiswa"
            to="/mock-gateway"
            class="nav-item nav-item-simulator"
            exact-active-class="router-link-active"
          >
            Simulator Webhook
          </RouterLink>
        </nav>

        <!-- Clean & Minimalist User Profile Pill -->
        <div v-if="auth.isAuthenticated && auth.user" class="session-box">
          <div class="user-avatar">{{ userInitials }}</div>
          <div class="session-info">
            <div class="session-user-name">{{ cleanUserName }}</div>
            <div class="session-user-sub">
              <span>{{ roleLabel }}</span>
              <span class="sep">&bull;</span>
              <span class="inst-code">{{ auth.user.kode_institusi }}</span>
            </div>
          </div>
          <button type="button" class="btn-logout" @click="handleLogout" title="Keluar Sesi">
            Keluar
          </button>
        </div>
      </div>
    </header>

    <main class="page-wrapper">
      <RouterView />
    </main>
  </div>
</template>

<style>
:root {
  --sevima-blue-dark: #0a2e6e;
  --sevima-blue: #0f4c9c;
  --sevima-blue-bright: #1d5bbf;
  --sevima-blue-soft: #eef4ff;
  --sevima-red: #e11d2e;
  --sevima-red-hover: #be1223;
  --sevima-red-soft: #fff1f2;

  --bg-page: #f5f7fb;
  --bg-surface: #ffffff;
  --bg-subtle: #f1f5f9;
  --border-light: #e2e8f0;
  --border-strong: #cbd5e1;
  --text-primary: #0f172a;
  --text-secondary: #334155;
  --text-muted: #64748b;

  --radius-lg: 14px;
  --radius-md: 10px;
  --radius-sm: 8px;
  --shadow-xs: 0 1px 2px rgba(10, 46, 110, 0.05);
  --shadow-sm: 0 4px 14px -2px rgba(10, 46, 110, 0.06), 0 2px 6px -1px rgba(15, 23, 42, 0.03);
  --shadow-md: 0 12px 28px -6px rgba(10, 46, 110, 0.09), 0 8px 12px -6px rgba(15, 23, 42, 0.04);
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  font-family:
    'Plus Jakarta Sans',
    -apple-system,
    BlinkMacSystemFont,
    'Segoe UI',
    Roboto,
    sans-serif;
  background-color: var(--bg-page);
  color: var(--text-primary);
  line-height: 1.55;
  -webkit-font-smoothing: antialiased;
}

code,
.mono {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 0.85em;
}

.mono-bold {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-weight: 600;
}

.sevima-accent-bar {
  height: 4px;
  background: linear-gradient(
    90deg,
    var(--sevima-blue-dark) 0%,
    var(--sevima-blue-bright) 68%,
    var(--sevima-red) 68%,
    var(--sevima-red) 100%
  );
}

.navbar {
  background: rgba(255, 255, 255, 0.97);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid var(--border-light);
  position: sticky;
  top: 0;
  z-index: 100;
}

.navbar-container {
  max-width: 1280px;
  margin: 0 auto;
  padding: 0.7rem 1.5rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1.25rem;
  flex-wrap: wrap;
}

.brand-group {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.brand-mark {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  background: linear-gradient(135deg, var(--sevima-blue-dark) 0%, var(--sevima-blue-bright) 100%);
  color: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}

.brand-s {
  font-weight: 800;
  font-size: 1.15rem;
}

.brand-dot {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--sevima-red);
  box-shadow: 0 0 0 2px #ffffff;
}

.brand-text {
  display: flex;
  flex-direction: column;
}

.brand-name-row {
  display: flex;
  align-items: baseline;
  gap: 0.3rem;
  line-height: 1.15;
}

.brand-sevima {
  font-weight: 800;
  font-size: 1rem;
  color: var(--sevima-blue-dark);
  letter-spacing: 0.02em;
}

.brand-product {
  font-weight: 800;
  font-size: 0.98rem;
  color: var(--sevima-red);
}

.brand-tagline {
  font-size: 0.72rem;
  color: var(--text-muted);
}

.nav-menu {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  background: var(--bg-subtle);
  padding: 0.28rem;
  border-radius: 10px;
  border: 1px solid var(--border-light);
}

.nav-item {
  text-decoration: none;
  color: var(--text-secondary);
  font-size: 0.84rem;
  font-weight: 600;
  padding: 0.44rem 0.95rem;
  border-radius: 7px;
  transition: all 0.15s ease;
}

.nav-item:hover {
  color: var(--sevima-blue-dark);
}

.nav-item.router-link-active {
  background: #ffffff;
  color: var(--sevima-blue);
  box-shadow: var(--shadow-xs);
}

.nav-item-simulator.router-link-active {
  background: #ffffff;
  color: var(--sevima-red);
  box-shadow: var(--shadow-xs);
}

/* Minimalist Profile Section */
.session-box {
  display: flex;
  align-items: center;
  gap: 0.7rem;
}

.user-avatar {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: var(--sevima-blue-soft);
  color: var(--sevima-blue-dark);
  border: 1px solid #bfdbfe;
  font-size: 0.76rem;
  font-weight: 800;
  display: flex;
  align-items: center;
  justify-content: center;
}

.session-info {
  display: flex;
  flex-direction: column;
  line-height: 1.25;
}

.session-user-name {
  font-size: 0.84rem;
  font-weight: 700;
  color: var(--text-primary);
}

.session-user-sub {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.72rem;
  color: var(--text-muted);
  font-weight: 500;
}

.session-user-sub .inst-code {
  color: var(--sevima-blue);
  font-weight: 700;
}

.btn-logout {
  margin-left: 0.35rem;
  padding: 0.4rem 0.75rem;
  font-size: 0.78rem;
  font-weight: 600;
  color: #475569;
  background: #ffffff;
  border: 1px solid var(--border-strong);
  border-radius: 7px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-logout:hover {
  color: var(--sevima-red);
  border-color: #fecdd3;
  background: var(--sevima-red-soft);
}

.page-wrapper {
  max-width: 1280px;
  margin: 0 auto;
  padding: 1.75rem 1.5rem 3rem;
}

/* Shared Surface & Form Primitives */
.card {
  background: var(--bg-surface);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  padding: 1.5rem;
  box-shadow: var(--shadow-sm);
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.38rem;
}

.form-group label {
  font-size: 0.82rem;
  font-weight: 600;
  color: #334155;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.field-tag {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.72rem;
  font-weight: 500;
  color: #64748b;
  background: #f1f5f9;
  padding: 0.1rem 0.4rem;
  border-radius: 4px;
}

input,
select {
  width: 100%;
  padding: 0.62rem 0.85rem;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  font-family: inherit;
  font-size: 0.88rem;
  color: var(--text-primary);
  background: #ffffff;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

input:hover,
select:hover {
  border-color: #94a3b8;
}

input:focus,
select:focus {
  outline: none;
  border-color: var(--sevima-blue);
  box-shadow: 0 0 0 3px rgba(15, 76, 156, 0.14);
}

.hint {
  font-size: 0.75rem;
  color: var(--text-muted);
  line-height: 1.4;
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  padding: 0.6rem 1.1rem;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: 600;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
  text-decoration: none;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--sevima-blue);
  color: #ffffff;
}

.btn-primary:hover:not(:disabled) {
  background: var(--sevima-blue-dark);
}

.btn-crimson {
  background: var(--sevima-red);
  color: #ffffff;
}

.btn-crimson:hover:not(:disabled) {
  background: var(--sevima-red-hover);
}

.btn-outline {
  background: #ffffff;
  border-color: var(--border-strong);
  color: #334155;
}

.btn-outline:hover:not(:disabled) {
  background: var(--bg-subtle);
  border-color: #94a3b8;
}

.btn-block {
  width: 100%;
}

.btn-xs {
  padding: 0.36rem 0.75rem;
  font-size: 0.78rem;
  border-radius: 6px;
}

.alert {
  padding: 0.85rem 1rem;
  border-radius: var(--radius-sm);
  font-size: 0.85rem;
  font-weight: 500;
  margin-bottom: 1.1rem;
}

.alert-error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #991b1b;
}

.alert-success {
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  color: #166534;
}

/* Tables */
.table-responsive {
  overflow-x: auto;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.86rem;
  background: #ffffff;
}

.data-table th,
.data-table td {
  padding: 0.9rem 1.1rem;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
  vertical-align: middle;
}

.data-table tbody tr:last-child td {
  border-bottom: none;
}

.data-table tbody tr:hover {
  background: #f8fafc;
}

.data-table th {
  background: #f8fafc;
  font-weight: 700;
  color: #475569;
  font-size: 0.74rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  white-space: nowrap;
}

.empty-cell {
  text-align: center !important;
  color: var(--text-muted);
  padding: 2.5rem 1rem !important;
}

/* Status Badges */
.status-pill {
  display: inline-flex;
  align-items: center;
  font-size: 0.72rem;
  font-weight: 700;
  padding: 0.2rem 0.65rem;
  border-radius: 999px;
}

.status-unpaid {
  background: #fef3c7;
  color: #92400e;
  border: 1px solid #fde68a;
}

.status-paid {
  background: #dcfce7;
  color: #166534;
  border: 1px solid #bbf7d0;
}

.status-expired {
  background: #fee2e2;
  color: #991b1b;
  border: 1px solid #fecaca;
}
</style>
