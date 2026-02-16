const API_URL = import.meta.env.VITE_API_URL || ''

let authToken: string | null = localStorage.getItem('finopsmind_token')

export function setAuthToken(token: string | null) {
  authToken = token
  if (token) {
    localStorage.setItem('finopsmind_token', token)
  } else {
    localStorage.removeItem('finopsmind_token')
  }
}

export function getAuthToken(): string | null {
  return authToken
}

async function request<T>(method: string, path: string, data?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }

  const res = await fetch(`${API_URL}/api/v1${path}`, {
    method,
    headers,
    body: data ? JSON.stringify(data) : undefined,
  })

  if (res.status === 401) {
    setAuthToken(null)
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }

  if (!res.ok) {
    const errBody = await res.json().catch(() => ({ message: `API error: ${res.status}` }))
    throw new Error(errBody.message || `API error: ${res.status}`)
  }

  return res.json()
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, data: unknown) => request<T>('POST', path, data),
  put: <T>(path: string, data: unknown) => request<T>('PUT', path, data),
  patch: <T>(path: string, data: unknown) => request<T>('PATCH', path, data),
  delete: <T>(path: string) => request<T>('DELETE', path),
}
