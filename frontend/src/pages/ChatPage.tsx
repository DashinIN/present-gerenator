import { useState, useEffect, useRef } from 'react'
import { PanelLeftClose, PanelLeftOpen } from 'lucide-react'
import { Sidebar } from '@/components/Sidebar'
import { ChatThread } from '@/components/ChatThread'
import { ChatInput } from '@/components/ChatInput'
import { useSession } from '@/hooks/useSessions'
import { useBalance } from '@/hooks/useAuth'
import { useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/Button'
import { useI18n } from '@/lib/i18n'

export function ChatPage() {
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null)
  const [noCreditsAt, setNoCreditsAt] = useState<string | null>(null)
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const qc = useQueryClient()
  const { t, resolvedLanguage } = useI18n()

  const { data: balance } = useBalance()
  const prevBalanceRef = useRef<number | undefined>(undefined)

  useEffect(() => {
    if (balance === undefined) return
    const prev = prevBalanceRef.current
    if (balance <= 0 && (prev === undefined || prev > 0)) {
      setNoCreditsAt(new Date().toLocaleTimeString(resolvedLanguage === 'ru' ? 'ru-RU' : 'en-US'))
    }
    prevBalanceRef.current = balance
  }, [balance, resolvedLanguage])

  const { data: thread } = useSession(activeSessionId)

  const generations = thread?.generations ?? []
  const lastGen = generations[generations.length - 1]
  const parentId = lastGen?.id ?? null
  const hasPending = generations.some(g =>
    g.status === 'pending' || g.status === 'processing_images' || g.status === 'processing_audio'
  )

  const handleSent = (_genId: string, sessionId: string) => {
    setNoCreditsAt(null)
    setActiveSessionId(sessionId)
    qc.invalidateQueries({ queryKey: ['session', sessionId] })
  }

  const handleNewSession = () => {
    setActiveSessionId(null)
    setNoCreditsAt(null)
  }

  const handleInsufficientCredits = () => {
    setNoCreditsAt(new Date().toLocaleTimeString(resolvedLanguage === 'ru' ? 'ru-RU' : 'en-US'))
  }

  return (
    <div style={{ display: 'flex', height: '100%', width: '100%', background: 'var(--bg)' }}>
      <Sidebar
        open={sidebarOpen}
        activeSessionId={activeSessionId}
        onSelectSession={setActiveSessionId}
        onNewSession={handleNewSession}
      />

      <main style={{ flex: 1, display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
        <div style={{
          height: 60, flexShrink: 0,
          padding: '0 20px',
          background: 'var(--surface)',
          display: 'flex', alignItems: 'center', gap: 12,
        }}>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setSidebarOpen(v => !v)}
            title={sidebarOpen ? t('hideSidebar') : t('showSidebar')}
            style={{ flexShrink: 0 }}
          >
            {sidebarOpen ? <PanelLeftClose size={18} /> : <PanelLeftOpen size={18} />}
          </Button>

          {thread ? (
            <div>
              <div style={{ fontWeight: 600, fontSize: 15 }}>{thread.session.title}</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>
                {generations.length} {generations.length === 1 ? t('greetingCount_one') : t('greetingCount_many')}
                {hasPending && ` · ${t('generating')}`}
              </div>
            </div>
          ) : (
            <div>
              <div style={{ fontWeight: 600, fontSize: 15 }}>{t('newGreeting')}</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>{t('describeGreeting')}</div>
            </div>
          )}
        </div>

        <ChatThread generations={generations} noCreditsAt={noCreditsAt} />

        <ChatInput
          sessionId={activeSessionId}
          parentId={parentId}
          onSent={handleSent}
          onInsufficientCredits={handleInsufficientCredits}
          disabled={hasPending}
        />
      </main>
    </div>
  )
}
