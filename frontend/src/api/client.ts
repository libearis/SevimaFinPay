import axios from 'axios'

const apiClient = axios.create({
  baseURL: '/api',
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// Interceptor: otomatis menyertakan Bearer JWT dari localStorage pada setiap request
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('ukt_auth_token')
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

export default apiClient
