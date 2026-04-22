import { Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { api } from '@/lib/api'
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'

export function LoginPage() {
  const qc = useQueryClient()
  const [loading, setLoading] = useState(false)

  const devLogin = async () => {
    setLoading(true)
    try {
      await api.auth.devLogin()
      qc.invalidateQueries({ queryKey: ['me'] })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center',
      background: 'var(--bg)',
    }}>
      <div style={{
        width: 360, background: 'var(--surface)',
        border: '1px solid var(--border)', borderRadius: 20,
        padding: 40, textAlign: 'center',
      }}>
        <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 20 }}>
          <div style={{
            width: 56, height: 56, borderRadius: 16,
            background: 'var(--primary)', display: 'flex',
            alignItems: 'center', justifyContent: 'center',
          }}>
            <Sparkles size={26} style={{ color: '#fff' }} />
          </div>
        </div>
        <h1 style={{ fontSize: 22, fontWeight: 700, marginBottom: 8 }}>FunGreet</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: 14, marginBottom: 32 }}>
          Персональные поздравления с ИИ — картинки и музыка за минуты
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          {/* Реальные OAuth кнопки будут здесь */}
          <Button
            onClick={() => window.location.href = '/api/auth/google/login'}
            variant="ghost"
            style={{ width: '100%', justifyContent: 'center', border: '1px solid var(--border)' }}
          >
            <img src="https://www.google.com/favicon.ico" width={16} height={16} alt="" />
            Войти через Google
          </Button>
          <Button
            onClick={() => window.location.href = '/api/auth/vk/login'}
            variant="ghost"
            style={{ width: '100%', justifyContent: 'center', border: '1px solid var(--border)' }}
          >
            Войти через VK
          </Button>
        </div>

        {/* Dev-кнопка — только в dev режиме */}
        {import.meta.env.DEV && (
          <div style={{ marginTop: 24, paddingTop: 24, borderTop: '1px solid var(--border)' }}>
            <p style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 12 }}>DEV режим</p>
            <Button onClick={devLogin} loading={loading} size="sm" style={{ width: '100%' }}>
              Войти как тестовый пользователь
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
