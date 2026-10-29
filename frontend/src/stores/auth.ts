import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import apiClient from '@/api/client'

export interface UserProfile {
  id: string
  institusi_id: string
  kode_institusi: string
  nama_institusi: string
  nim_nip: string
  name: string
  email: string
  role: 'ADMIN_KEUANGAN' | 'MAHASISWA'
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('ukt_auth_token'))
  const user = ref<UserProfile | null>(
    localStorage.getItem('ukt_auth_user')
      ? JSON.parse(localStorage.getItem('ukt_auth_user') as string)
      : null
  )
  const loading = ref(false)
  const error = ref<string | null>(null)

  const isAuthenticated = computed(() => Boolean(token.value && user.value))
  const isAdmin = computed(() => user.value?.role === 'ADMIN_KEUANGAN')
  const isMahasiswa = computed(() => user.value?.role === 'MAHASISWA')

  async function login(email: string, password: string): Promise<UserProfile> {
    loading.value = true
    error.value = null
    try {
      const response = await apiClient.post('/auth/login', { email, password })
      const data = response.data.data
      token.value = data.token
      user.value = data.user

      localStorage.setItem('ukt_auth_token', data.token)
      localStorage.setItem('ukt_auth_user', JSON.stringify(data.user))
      return data.user
    } catch (err: any) {
      const msg =
        err.response?.data?.error?.message ||
        'Gagal login. Periksa kembali email dan password Anda.'
      error.value = msg
      throw new Error(msg)
    } finally {
      loading.value = false
    }
  }

  async function refreshProfile() {
    if (!token.value) return
    try {
      const response = await apiClient.get('/me')
      user.value = response.data.data
      localStorage.setItem('ukt_auth_user', JSON.stringify(user.value))
    } catch {
      logout()
    }
  }

  function logout() {
    token.value = null
    user.value = null
    localStorage.removeItem('ukt_auth_token')
    localStorage.removeItem('ukt_auth_user')
  }

  return {
    token,
    user,
    loading,
    error,
    isAuthenticated,
    isAdmin,
    isMahasiswa,
    login,
    refreshProfile,
    logout
  }
})
