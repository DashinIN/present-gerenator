import type {
  User, Tariff, CreditTransaction,
  GenerationRequest, GenerationSession, SessionThread,
} from './types'

const API_BASE = '/api/v1'
const AUTH_BASE = '/api/auth'

interface ApiErrorBody {
  error?: {
    code?: string
    message?: string
  }
}

async function readJson(res: Response) {
  if (res.status === 204) return undefined
  return res.json()
}

async function apiRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE + path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })

  if (res.status === 401) {
    const refreshed = await refreshAccessToken()
    if (refreshed.ok) {
      const retry = await fetch(API_BASE + path, {
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', ...init?.headers },
        ...init,
      })
      if (!retry.ok) throw new ApiError(retry.status, await readJson(retry))
      return readJson(retry) as Promise<T>
    }
    throw new ApiError(401, { error: { code: 'unauthorized' } })
  }

  if (!res.ok) {
    throw new ApiError(res.status, await readJson(res))
  }

  return readJson(res) as Promise<T>
}

async function authRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(AUTH_BASE + path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) throw new ApiError(res.status, await readJson(res))
  return readJson(res) as Promise<T>
}

function refreshAccessToken() {
  return fetch(`${AUTH_BASE}/refresh`, {
    method: 'POST',
    credentials: 'include',
  })
}

export class ApiError extends Error {
  status: number
  body: unknown

  constructor(status: number, body: unknown) {
    const apiBody = body as ApiErrorBody
    super(apiBody?.error?.message ?? `HTTP ${status}`)
    this.status = status
    this.body = body
  }

  get code() {
    return (this.body as ApiErrorBody)?.error?.code
  }
}

// Auth
export const api = {
  auth: {
    me: () => apiRequest<User>('/user/me'),
    devLogin: () => authRequest<{ user_id: number }>('/dev/login'),
    logout: () => authRequest<void>('/logout', { method: 'POST' }),
  },

  billing: {
    balance: () => apiRequest<{ balance: number }>('/billing/balance'),
    tariff: () => apiRequest<Tariff>('/billing/tariff'),
    estimate: (images: number, songs: number) =>
      apiRequest<{ cost: number; price_per_image: number; price_per_song: number }>(
        `/billing/estimate?images=${images}&songs=${songs}`
      ),
    transactions: (limit = 20, offset = 0) =>
      apiRequest<{ transactions: CreditTransaction[] }>(
        `/billing/transactions?limit=${limit}&offset=${offset}`
      ),
  },

  sessions: {
    list: (limit = 30) =>
      apiRequest<{ sessions: GenerationSession[] }>(`/sessions?limit=${limit}`),
    get: (id: string) => apiRequest<SessionThread>(`/sessions/${id}`),
    rename: (id: string, title: string) =>
      apiRequest<void>(`/sessions/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({ title }),
      }),
  },

  generations: {
    create: (form: FormData) =>
      fetch(`${API_BASE}/generations`, { method: 'POST', credentials: 'include', body: form })
        .then(async res => {
          if (!res.ok) throw new ApiError(res.status, await readJson(res))
          return res.json() as Promise<{ id: string; session_id: string; status: string }>
        }),
    lyrics: (prompt: string) =>
      apiRequest<{ text: string; title: string }>('/generations/lyrics', {
        method: 'POST',
        body: JSON.stringify({ prompt }),
      }),
    status: (id: string) =>
      apiRequest<{
        id: string
        status: string
        error_message?: string
        result_images: string[]
        result_audios: string[]
        completed_at?: string
      }>(`/generations/${id}/status`),
    get: (id: string) => apiRequest<GenerationRequest>(`/generations/${id}`),
  },
}
