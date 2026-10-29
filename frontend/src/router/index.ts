import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LoginView from '@/views/LoginView.vue'
import AdminBillingView from '@/views/AdminBillingView.vue'
import AdminCreateInvoiceView from '@/views/AdminCreateInvoiceView.vue'
import StudentBillingView from '@/views/StudentBillingView.vue'
import MockGatewayView from '@/views/MockGatewayView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/login'
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView
    },
    {
      path: '/admin/invoices',
      name: 'admin-invoices',
      component: AdminBillingView,
      meta: { requiresAuth: true, role: 'ADMIN_KEUANGAN' }
    },
    {
      path: '/admin/invoices/create',
      name: 'admin-create-invoice',
      component: AdminCreateInvoiceView,
      meta: { requiresAuth: true, role: 'ADMIN_KEUANGAN' }
    },
    {
      path: '/student/invoices',
      name: 'student-invoices',
      component: StudentBillingView,
      meta: { requiresAuth: true, role: 'MAHASISWA' }
    },
    {
      path: '/mock-gateway',
      name: 'mock-gateway',
      component: MockGatewayView
    }
  ]
})

router.beforeEach((to) => {
  const auth = useAuthStore()

  if (to.name === 'login' && auth.isAuthenticated) {
    return auth.isAdmin ? { name: 'admin-invoices' } : { name: 'student-invoices' }
  }

  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'login' }
  }

  if (to.meta.role && auth.user?.role !== to.meta.role) {
    return auth.isAdmin ? { name: 'admin-invoices' } : { name: 'student-invoices' }
  }

  return true
})

export default router
