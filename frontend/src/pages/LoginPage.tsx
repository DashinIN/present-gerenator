import { Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/Button'

export function LoginPage() {
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
          Персональные поздравления с ИИ: картинки и музыка за минуты
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Button
            onClick={() => window.location.href = '/api/auth/google/login'}
            variant="ghost"
            style={{ width: '100%', justifyContent: 'center', border: '1px solid var(--border)' }}
          >
            <img src="https://www.google.com/favicon.ico" width={16} height={16} alt="" />
            Войти через Google
          </Button>
        </div>
      </div>
    </div>
  )
}
