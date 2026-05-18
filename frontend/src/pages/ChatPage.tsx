import { useState, useEffect, useRef } from 'react'
import { PanelLeftClose, PanelLeftOpen, Music4, ImageIcon, ArrowRight } from 'lucide-react'
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
  const [creationPickerOpen, setCreationPickerOpen] = useState(true)
  const [creationMode, setCreationMode] = useState<'image' | 'song'>('image')
  const [creationModeVersion, setCreationModeVersion] = useState(0)
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
    setCreationPickerOpen(true)
  }

  const handleInsufficientCredits = () => {
    setNoCreditsAt(new Date().toLocaleTimeString(resolvedLanguage === 'ru' ? 'ru-RU' : 'en-US'))
  }

  const applyCreationMode = (mode: 'image' | 'song') => {
    setCreationMode(mode)
    setCreationModeVersion(value => value + 1)
    setCreationPickerOpen(false)
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
          key={creationModeVersion}
          sessionId={activeSessionId}
          parentId={parentId}
          onSent={handleSent}
          onInsufficientCredits={handleInsufficientCredits}
          disabled={hasPending}
          initialMode={creationMode}
          initialModeVersion={creationModeVersion}
        />
      </main>

      {creationPickerOpen && (
        <div className="creation-picker-backdrop">
          <div className="creation-picker-modal">
            <div className="creation-picker-modal__title">{t('createModalTitle')}</div>
            <div className="creation-picker-modal__subtitle">{t('createModalSubtitle')}</div>

            <div className="creation-picker-grid">
              <button
                type="button"
                className={`creation-picker-card creation-picker-card--song${creationMode === 'song' ? ' creation-picker-card--active' : ''}`}
                onClick={() => applyCreationMode('song')}
              >
                <div className="creation-picker-card__icon">
                  <Music4 size={30} />
                </div>
                <div className="creation-picker-card__title">{t('createModeSongTitle')}</div>
                <div className="creation-picker-card__art creation-picker-card__art--song" aria-hidden="true">
                  <span />
                  <span />
                  <span />
                  <span />
                  <span />
                  <span />
                  <span />
                  <span />
                  <span />
                  <span />
                  <span />
                </div>
                <div className="creation-picker-card__desc">{t('createModeSongDesc')}</div>
              </button>

              <button
                type="button"
                className={`creation-picker-card creation-picker-card--image${creationMode === 'image' ? ' creation-picker-card--active' : ''}`}
                onClick={() => applyCreationMode('image')}
              >
                <div className="creation-picker-card__icon">
                  <ImageIcon size={30} />
                </div>
                <div className="creation-picker-card__title">{t('createModeImageTitle')}</div>
                <div className="creation-picker-card__art creation-picker-card__art--image" aria-hidden="true">
                  <span className="creation-picker-card__image-frame" />
                  <span className="creation-picker-card__image-glow" />
                  <span className="creation-picker-card__image-badge" />
                  <span className="creation-picker-card__image-landscape" />
                  <span className="creation-picker-card__image-line creation-picker-card__image-line--one" />
                  <span className="creation-picker-card__image-line creation-picker-card__image-line--two" />
                </div>
                <div className="creation-picker-card__desc">{t('createModeImageDesc')}</div>
              </button>
            </div>

            <div className="creation-picker-modal__footer">
              <div className="creation-picker-modal__hint">{t('createModeHint')}</div>
              <Button onClick={() => applyCreationMode(creationMode)} size="sm" style={{ gap: 8, minWidth: 154 }}>
                {t('continue')}
                <ArrowRight size={15} />
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
