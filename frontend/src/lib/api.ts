import type {
  User, Tariff, CreditTransaction,
  GenerationRequest, GenerationSession, SessionThread,
} from './types'

const BASE = '/api/v1'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })

  if (res.status === 401) {
    // Пробуем refresh
    const refreshed = await fetch('/api/auth/refresh', {
      method: 'POST',
      credentials: 'include',
    })
    if (refreshed.ok) {
      // Повторяем оригинальный запрос
      const retry = await fetch(BASE + path, {
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', ...init?.headers },
        ...init,
      })
      if (!retry.ok) throw new ApiError(retry.status, await retry.json())
      return retry.json()
    }
    throw new ApiError(401, { error: { code: 'unauthorized' } })
  }

  if (!res.ok) {
    throw new ApiError(res.status, await res.json())
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

export class ApiError extends Error {
  constructor(public status: number, public body: any) {
    super(body?.error?.message ?? `HTTP ${status}`)
  }
  get code() { return this.body?.error?.code }
}

// Auth
export const api = {
  auth: {
    me: () => request<User>(`${BASE}/user/me`),
    devLogin: () => request<{ user_id: number }>('/api/auth/dev/login'),
    logout: () => request<void>('/api/auth/logout', { method: 'POST' }),
  },

  billing: {
    balance: () => request<{ balance: number }>(`${BASE}/billing/balance`),
    tariff: () => request<Tariff>(`${BASE}/billing/tariff`),
    estimate: (images: number, songs: number) =>
      request<{ cost: number; price_per_image: number; price_per_song: number }>(
        `${BASE}/billing/estimate?images=${images}&songs=${songs}`
      ),
    transactions: (limit = 20, offset = 0) =>
      request<{ transactions: CreditTransaction[] }>(
        `${BASE}/billing/transactions?limit=${limit}&offset=${offset}`
      ),
  },

  sessions: {
    list: (limit = 30) =>
      request<{ sessions: GenerationSession[] }>(`${BASE}/sessions?limit=${limit}`),
    get: (id: string) => request<SessionThread>(`${BASE}/sessions/${id}`),
    rename: (id: string, title: string) =>
      request<void>(`${BASE}/sessions/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({ title }),
      }),
  },

  generations: {
    create: (form: FormData) =>
      fetch(`${BASE}/generations`, { method: 'POST', credentials: 'include', body: form })
        .then(async res => {
          if (!res.ok) throw new ApiError(res.status, await res.json())
          return res.json() as Promise<{ id: string; session_id: string; status: string }>
        }),
    status: (id: string) =>
      request<{
        id: string
        status: string
        error_message?: string
        result_images: string[]
        result_audios: string[]
        completed_at?: string
      }>(`${BASE}/generations/${id}/status`),
    get: (id: string) => request<GenerationRequest>(`${BASE}/generations/${id}`),
  },
}
