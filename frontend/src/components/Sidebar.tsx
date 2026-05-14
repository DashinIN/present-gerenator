import { useState, useRef, useEffect, type MouseEvent as ReactMouseEvent, type CSSProperties } from 'react'
import { LogOut, Plus, MessageSquare, Receipt, Pencil, Check, X, Languages, Settings, ChevronDown, Moon, Sun } from 'lucide-react'
import { useSessions, useRenameSession } from '@/hooks/useSessions'
import { useCurrentUser, useBalance, useLogout } from '@/hooks/useAuth'
import { Button } from '@/components/ui/Button'
import { AppLogo } from '@/components/AppLogo'
import { formatDate, formatCredits } from '@/lib/utils'
import { TransactionsPanel } from '@/components/TransactionsPanel'
import type { GenerationSession } from '@/lib/types'
import { LANGUAGES, useI18n } from '@/lib/i18n'
import { ACCENT_PRESETS, useTheme } from '@/lib/theme'

const COLLAPSED_W = 56
const EXPANDED_W = 260

interface SidebarProps {
  open: boolean
  activeSessionId: string | null
  onSelectSession: (id: string) => void
  onNewSession: () => void
}

export function Sidebar({ open, activeSessionId, onSelectSession, onNewSession }: SidebarProps) {
  const { data: user } = useCurrentUser()
  const { data: balance } = useBalance()
  const { data: sessions } = useSessions()
  const logout = useLogout()
  const { language, setLanguage, t } = useI18n()
  const { theme, accent, setTheme, setAccent } = useTheme()
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [txPanelOpen, setTxPanelOpen] = useState(false)
  const popupRef = useRef<HTMLDivElement>(null)
  const settingsButtonRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!settingsOpen) return

    const handler = (event: globalThis.MouseEvent) => {
      if (
        popupRef.current && !popupRef.current.contains(event.target as Node) &&
        settingsButtonRef.current && !settingsButtonRef.current.contains(event.target as Node)
      ) {
        setSettingsOpen(false)
      }
    }

    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [settingsOpen])

  const width = open ? EXPANDED_W : COLLAPSED_W

  return (
    <>
      <aside style={{
        width,
        flexShrink: 0,
        background: 'var(--surface)',
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        transition: 'width 0.25s ease',
        overflow: 'hidden',
        position: 'relative',
        boxShadow: '4px 0 24px rgba(0,0,0,0.18), 1px 0 0 rgba(var(--primary-rgb),0.08)',
      }}>
        <div style={{
          height: 60,
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
          padding: '0 11px',
          gap: 10,
        }}>
          <div style={{
            width: 34,
            height: 34,
            borderRadius: 10,
            flexShrink: 0,
            background: 'var(--primary-subtle)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}>
            <AppLogo size={22} />
          </div>

          {open && (
            <>
              <span style={{ fontWeight: 700, fontSize: 15, whiteSpace: 'nowrap', flex: 1 }}>FunGreet</span>

              <div style={{ position: 'relative' }}>
                <button
                  ref={settingsButtonRef}
                  onClick={() => setSettingsOpen(value => !value)}
                  title={t('settings')}
                  aria-label={t('settings')}
                  style={{
                    width: 32,
                    height: 32,
                    borderRadius: 8,
                    border: 'none',
                    background: settingsOpen ? 'var(--primary-subtle)' : 'transparent',
                    color: settingsOpen ? 'var(--primary)' : 'var(--text-muted)',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    transition: 'background 0.15s, color 0.15s',
                  }}
                >
                  <Settings size={16} />
                </button>

                {settingsOpen && (
                  <div
                    ref={popupRef}
                    style={{
                      position: 'absolute',
                      top: 'calc(100% + 8px)',
                      right: 0,
                      background:
                        'linear-gradient(180deg, rgba(var(--primary-rgb),0.08), transparent 28%), var(--surface)',
                      border: '1px solid var(--border)',
                      borderRadius: 16,
                      padding: '14px 14px 12px',
                      boxShadow: '0 18px 48px rgba(0,0,0,0.32)',
                      zIndex: 100,
                      minWidth: 248,
                    }}
                  >
                    <div style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 10,
                      marginBottom: 12,
                    }}>
                      <div style={{
                        width: 34,
                        height: 34,
                        borderRadius: 10,
                        background: 'rgba(var(--primary-rgb),0.12)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        color: 'var(--primary)',
                      }}>
                        <Settings size={16} />
                      </div>
                      <div>
                        <div style={{ fontSize: 13, fontWeight: 700, color: 'var(--text)' }}>{t('settings')}</div>
                        <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>{t('languageHint')}</div>
                      </div>
                    </div>

                    <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 8 }}>
                      {t('theme')}
                    </div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 12 }}>
                      <button
                        type="button"
                        onClick={() => setTheme('dark')}
                        style={settingsChip(theme === 'dark')}
                      >
                        <Moon size={14} />
                        {t('themeDark')}
                      </button>
                      <button
                        type="button"
                        onClick={() => setTheme('light')}
                        style={settingsChip(theme === 'light')}
                      >
                        <Sun size={14} />
                        {t('themeLight')}
                      </button>
                    </div>

                    <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 8 }}>
                      {t('accent')}
                    </div>
                    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
                      {ACCENT_PRESETS.map(preset => (
                        <button
                          key={preset.value}
                          type="button"
                          title={preset.name}
                          onClick={() => setAccent(preset.value)}
                          style={{
                            width: 24,
                            height: 24,
                            borderRadius: '999px',
                            border: accent === preset.value ? '2px solid var(--text)' : '2px solid transparent',
                            outline: `2px solid ${preset.value}`,
                            outlineOffset: 1,
                            background: preset.value,
                            cursor: 'pointer',
                          }}
                        />
                      ))}
                    </div>

                    <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 8 }}>
                      {t('language')}
                    </div>

                    <div style={{ position: 'relative' }}>
                      <Languages
                        size={14}
                        style={{
                          position: 'absolute',
                          left: 12,
                          top: '50%',
                          transform: 'translateY(-50%)',
                          color: 'var(--text-muted)',
                          pointerEvents: 'none',
                        }}
                      />
                      <ChevronDown
                        size={14}
                        style={{
                          position: 'absolute',
                          right: 12,
                          top: '50%',
                          transform: 'translateY(-50%)',
                          color: 'var(--text-muted)',
                          pointerEvents: 'none',
                        }}
                      />
                      <select
                        value={language}
                        onChange={event => setLanguage(event.target.value as typeof language)}
                        className="settings-select"
                        aria-label={t('language')}
                      >
                        {LANGUAGES.map(option => (
                          <option key={option.code} value={option.code}>
                            {option.code === 'auto' ? t('auto') : option.nativeName}
                          </option>
                        ))}
                      </select>
                    </div>
                  </div>
                )}
              </div>
            </>
          )}
        </div>

        {open ? (
          <>
            <div style={{ padding: '6px 10px 4px', flexShrink: 0 }}>
              <Button
                variant="ghost"
                size="sm"
                onClick={onNewSession}
                style={{ width: '100%', justifyContent: 'flex-start', gap: 8, whiteSpace: 'nowrap' }}
              >
                <Plus size={16} />
                {t('newGreeting')}
              </Button>
            </div>

            <div style={{ flex: 1, overflowY: 'auto', padding: '0 8px' }}>
              {sessions && sessions.length > 0 ? (
                <>
                  <div style={{ padding: '4px 8px 6px', fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
                    {t('sessions')}
                  </div>
                  {sessions.map(session => (
                    <SessionItem
                      key={session.id}
                      session={session}
                      active={session.id === activeSessionId}
                      onClick={() => onSelectSession(session.id)}
                    />
                  ))}
                </>
              ) : (
                <div style={{ padding: '32px 8px', textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
                  <MessageSquare size={24} style={{ margin: '0 auto 8px', opacity: 0.35 }} />
                  {t('noSessions')}
                </div>
              )}
            </div>
          </>
        ) : (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', paddingTop: 8 }}>
            <button
              onClick={onNewSession}
              title={t('newGreeting')}
              aria-label={t('newGreeting')}
              style={{
                width: 34,
                height: 34,
                borderRadius: 10,
                border: 'none',
                background: 'transparent',
                color: 'var(--text-muted)',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                transition: 'background 0.15s, color 0.15s',
              }}
              onMouseEnter={event => {
                event.currentTarget.style.background = 'var(--primary-subtle)'
                event.currentTarget.style.color = 'var(--primary)'
              }}
              onMouseLeave={event => {
                event.currentTarget.style.background = 'transparent'
                event.currentTarget.style.color = 'var(--text-muted)'
              }}
            >
              <Plus size={17} />
            </button>
          </div>
        )}

        <div style={{
          flexShrink: 0,
          padding: open ? '12px 12px 16px' : '12px 0 16px',
          display: 'flex',
          flexDirection: 'column',
          alignItems: open ? 'stretch' : 'center',
          gap: 8,
        }}>
          {open ? (
            <>
              <button
                onClick={() => setTxPanelOpen(value => !value)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  padding: '8px 10px',
                  borderRadius: 10,
                  background: 'var(--primary-subtle)',
                  whiteSpace: 'nowrap',
                  border: 'none',
                  cursor: 'pointer',
                  width: '100%',
                  textAlign: 'left',
                }}
                title={t('transactionsTitle')}
              >
                <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>{t('balance')}</span>
                <Receipt size={13} style={{ color: 'var(--text-muted)', marginLeft: 2 }} />
                <span style={{ marginLeft: 'auto', fontWeight: 600, fontSize: 14 }}>
                  {balance !== undefined ? formatCredits(balance) : '...'}
                </span>
              </button>

              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div style={{
                  width: 34,
                  height: 34,
                  borderRadius: '50%',
                  flexShrink: 0,
                  background: 'var(--primary)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontWeight: 700,
                  fontSize: 13,
                  color: '#fff',
                }}>
                  {user?.display_name?.[0]?.toUpperCase() ?? '?'}
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontWeight: 500, fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {user?.display_name ?? '...'}
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {user?.email}
                  </div>
                </div>
                <Button variant="ghost" size="icon" onClick={() => logout.mutate()} title={t('logout')} style={{ flexShrink: 0 }}>
                  <LogOut size={15} />
                </Button>
              </div>
            </>
          ) : (
            <>
              <div style={{
                fontSize: 11,
                fontWeight: 700,
                color: 'var(--primary)',
                textAlign: 'center',
                whiteSpace: 'nowrap',
              }}>
                {balance !== undefined ? String(balance) : '...'}
              </div>

              <div style={{
                width: 34,
                height: 34,
                borderRadius: '50%',
                background: 'var(--primary)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontWeight: 700,
                fontSize: 13,
                color: '#fff',
                cursor: 'default',
              }}>
                {user?.display_name?.[0]?.toUpperCase() ?? '?'}
              </div>

              <button
                onClick={() => logout.mutate()}
                title={t('logout')}
                aria-label={t('logout')}
                style={{
                  width: 34,
                  height: 34,
                  borderRadius: 8,
                  border: 'none',
                  background: 'transparent',
                  color: 'var(--text-muted)',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  transition: 'background 0.15s, color 0.15s',
                }}
                onMouseEnter={event => {
                  event.currentTarget.style.background = 'var(--primary-subtle)'
                  event.currentTarget.style.color = 'var(--error)'
                }}
                onMouseLeave={event => {
                  event.currentTarget.style.background = 'transparent'
                  event.currentTarget.style.color = 'var(--text-muted)'
                }}
              >
                <LogOut size={15} />
              </button>
            </>
          )}
        </div>
      </aside>
      {txPanelOpen && <TransactionsPanel onClose={() => setTxPanelOpen(false)} />}
    </>
  )
}

function settingsChip(active: boolean): CSSProperties {
  return {
    height: 36,
    borderRadius: 10,
    border: `1px solid ${active ? 'rgba(var(--primary-rgb),0.4)' : 'var(--border)'}`,
    background: active ? 'rgba(var(--primary-rgb),0.12)' : 'var(--surface2)',
    color: active ? 'var(--text)' : 'var(--text-muted)',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    cursor: 'pointer',
    fontSize: 12,
    fontWeight: 600,
  }
}

function SessionItem({ session, active, onClick }: {
  session: GenerationSession
  active: boolean
  onClick: () => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const rename = useRenameSession()
  const { t } = useI18n()

  const startEdit = (event: ReactMouseEvent) => {
    event.stopPropagation()
    setDraft(session.title || '')
    setEditing(true)
  }

  const commit = () => {
    if (draft.trim()) rename.mutate({ id: session.id.toString(), title: draft.trim() })
    setEditing(false)
  }

  if (editing) {
    return (
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 4,
        padding: '4px 10px',
        marginBottom: 2,
      }}>
        <input
          autoFocus
          value={draft}
          onChange={event => setDraft(event.target.value)}
          onKeyDown={event => {
            if (event.key === 'Enter') {
              event.preventDefault()
              commit()
            }
            if (event.key === 'Escape') setEditing(false)
          }}
          style={{
            flex: 1,
            fontSize: 13,
            padding: '4px 6px',
            borderRadius: 6,
            border: '1px solid var(--primary)',
            background: 'var(--surface2)',
            color: 'var(--text)',
            outline: 'none',
          }}
        />
        <button onClick={commit} style={{ border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--success)', display: 'flex' }}><Check size={13} /></button>
        <button onClick={() => setEditing(false)} style={{ border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--text-muted)', display: 'flex' }}><X size={13} /></button>
      </div>
    )
  }

  return (
    <div style={{ position: 'relative', marginBottom: 2 }} className="session-item">
      <button
        onClick={onClick}
        style={{
          display: 'block',
          width: '100%',
          textAlign: 'left',
          padding: '8px 32px 8px 10px',
          borderRadius: 8,
          border: 'none',
          background: active ? 'var(--primary-subtle)' : 'transparent',
          color: active ? 'var(--text)' : 'var(--text-muted)',
          borderLeft: active ? '2px solid var(--primary)' : '2px solid transparent',
          cursor: 'pointer',
          transition: 'background 0.15s, color 0.15s',
        }}
        onMouseEnter={event => {
          if (!active) event.currentTarget.style.background = 'var(--primary-subtle)'
        }}
        onMouseLeave={event => {
          if (!active) event.currentTarget.style.background = 'transparent'
        }}
      >
        <div style={{ fontSize: 13, fontWeight: active ? 500 : 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {session.title || t('untitled')}
        </div>
        <div style={{ fontSize: 11, marginTop: 2, opacity: 0.6 }}>
          {formatDate(session.updated_at)}
        </div>
      </button>
      <button
        onClick={startEdit}
        title={t('rename')}
        style={{
          position: 'absolute',
          right: 6,
          top: '50%',
          transform: 'translateY(-50%)',
          border: 'none',
          background: 'transparent',
          cursor: 'pointer',
          color: 'var(--text-muted)',
          display: 'flex',
          padding: 4,
          borderRadius: 4,
          opacity: 0,
          transition: 'opacity 0.15s',
        }}
        className="rename-btn"
      >
        <Pencil size={12} />
      </button>
    </div>
  )
}
