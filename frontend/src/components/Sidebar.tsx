import { useState, useRef, useEffect, type MouseEvent } from 'react'
import { LogOut, Plus, Sparkles, MessageSquare, Sun, Moon, Palette, Receipt, Pencil, Check, X } from 'lucide-react'
import { useSessions, useRenameSession } from '@/hooks/useSessions'
import { useCurrentUser, useBalance, useLogout } from '@/hooks/useAuth'
import { useTheme, ACCENT_PRESETS } from '@/lib/theme'
import { Button } from '@/components/ui/Button'
import { formatDate, formatCredits } from '@/lib/utils'
import { TransactionsPanel } from '@/components/TransactionsPanel'
import type { GenerationSession } from '@/lib/types'

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
  const { theme, accent, setTheme, setAccent } = useTheme()
  const [themePopupOpen, setThemePopupOpen] = useState(false)
  const [txPanelOpen, setTxPanelOpen] = useState(false)
  const popupRef = useRef<HTMLDivElement>(null)
  const paletteRef = useRef<HTMLButtonElement>(null)

  // Close popup on outside click
  useEffect(() => {
    if (!themePopupOpen) return
    const handler = (e: MouseEvent) => {
      if (
        popupRef.current && !popupRef.current.contains(e.target as Node) &&
        paletteRef.current && !paletteRef.current.contains(e.target as Node)
      ) setThemePopupOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [themePopupOpen])

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

      {/* ── HEADER ── */}
      <div style={{
        height: 60, flexShrink: 0,
        display: 'flex', alignItems: 'center',
        padding: '0 11px',
        gap: 10,
      }}>
        <div style={{
          width: 34, height: 34, borderRadius: 10, flexShrink: 0,
          background: 'var(--primary-subtle)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <Sparkles size={17} style={{ color: 'var(--primary)' }} />
        </div>

        {open && (
          <>
            <span style={{ fontWeight: 700, fontSize: 15, whiteSpace: 'nowrap', flex: 1 }}>FunGreet</span>

            {/* Кнопка открытия попапа настроек темы */}
            <div style={{ position: 'relative' }}>
              <button
                ref={paletteRef}
                onClick={() => setThemePopupOpen(v => !v)}
                title="Тема и цвет"
                style={{
                  width: 32, height: 32, borderRadius: 8, border: 'none',
                  background: themePopupOpen ? 'var(--primary-subtle)' : 'transparent',
                  color: themePopupOpen ? 'var(--primary)' : 'var(--text-muted)',
                  cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center',
                  transition: 'background 0.15s, color 0.15s',
                }}
              >
                <Palette size={16} />
              </button>

              {/* Попап */}
              {themePopupOpen && (
                <div ref={popupRef} style={{
                  position: 'absolute', top: 'calc(100% + 8px)', right: 0,
                  background: 'var(--surface)',
                  border: '1px solid var(--border)',
                  borderRadius: 12,
                  padding: '14px 16px',
                  boxShadow: '0 8px 32px rgba(0,0,0,0.25)',
                  zIndex: 100, minWidth: 200,
                }}>
                  <div style={{ fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: 10 }}>
                    Тема
                  </div>
                  <div style={{ display: 'flex', gap: 6, marginBottom: 14 }}>
                    {(['dark', 'light'] as const).map(t => (
                      <button
                        key={t}
                        onClick={() => setTheme(t)}
                        style={{
                          flex: 1, padding: '7px 0', borderRadius: 8, border: 'none', cursor: 'pointer',
                          background: theme === t ? 'var(--primary)' : 'var(--surface2)',
                          color: theme === t ? '#fff' : 'var(--text-muted)',
                          fontSize: 12, fontWeight: 500, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 5,
                          transition: 'background 0.15s, color 0.15s',
                        }}
                      >
                        {t === 'dark' ? <Moon size={13} /> : <Sun size={13} />}
                        {t === 'dark' ? 'Тёмная' : 'Светлая'}
                      </button>
                    ))}
                  </div>

                  <div style={{ fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: 10 }}>
                    Цвет акцента
                  </div>
                  <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    {ACCENT_PRESETS.map(p => (
                      <button
                        key={p.value}
                        title={p.name}
                        onClick={() => setAccent(p.value)}
                        style={{
                          width: 28, height: 28, borderRadius: '50%',
                          background: p.value, border: 'none', cursor: 'pointer',
                          outline: accent === p.value ? `2px solid ${p.value}` : '2px solid transparent',
                          outlineOffset: 3,
                          transform: accent === p.value ? 'scale(1.15)' : 'scale(1)',
                          transition: 'transform 0.15s, outline 0.15s',
                        }}
                      />
                    ))}
                  </div>
                </div>
              )}
            </div>
          </>
        )}
      </div>

      {/* ── MIDDLE ── */}
      {open ? (
        <>
          {/* Новая сессия */}
          <div style={{ padding: '6px 10px 4px', flexShrink: 0 }}>
            <Button
              variant="ghost"
              size="sm"
              onClick={onNewSession}
              style={{ width: '100%', justifyContent: 'flex-start', gap: 8, whiteSpace: 'nowrap' }}
            >
              <Plus size={16} />
              Новое поздравление
            </Button>
          </div>

          {/* История сессий */}
          <div style={{ flex: 1, overflowY: 'auto', padding: '0 8px' }}>
            {sessions && sessions.length > 0 ? (
              <>
                <div style={{ padding: '4px 8px 6px', fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
                  История
                </div>
                {sessions.map(s => (
                  <SessionItem
                    key={s.id}
                    session={s}
                    active={s.id === activeSessionId}
                    onClick={() => onSelectSession(s.id)}
                  />
                ))}
              </>
            ) : (
              <div style={{ padding: '32px 8px', textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
                <MessageSquare size={24} style={{ margin: '0 auto 8px', opacity: 0.35 }} />
                Пока нет поздравлений
              </div>
            )}
          </div>
        </>
      ) : (
        /* Collapsed: центровая кнопка "новая сессия" */
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', paddingTop: 8 }}>
          <button
            onClick={onNewSession}
            title="Новое поздравление"
            style={{
              width: 34, height: 34, borderRadius: 10, border: 'none',
              background: 'transparent', color: 'var(--text-muted)',
              cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center',
              transition: 'background 0.15s, color 0.15s',
            }}
            onMouseEnter={e => { e.currentTarget.style.background = 'var(--primary-subtle)'; e.currentTarget.style.color = 'var(--primary)' }}
            onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-muted)' }}
          >
            <Plus size={17} />
          </button>
        </div>
      )}

      {/* ── FOOTER ── */}
      <div style={{
        flexShrink: 0,
        padding: open ? '12px 12px 16px' : '12px 0 16px',
        display: 'flex', flexDirection: 'column',
        alignItems: open ? 'stretch' : 'center', gap: 8,
      }}>
        {open ? (
          <>
            {/* Баланс + транзакции */}
            <button
              onClick={() => setTxPanelOpen(v => !v)}
              style={{
                display: 'flex', alignItems: 'center', gap: 8,
                padding: '8px 10px', borderRadius: 10,
                background: 'var(--primary-subtle)',
                whiteSpace: 'nowrap', border: 'none', cursor: 'pointer',
                width: '100%', textAlign: 'left',
              }}
              title="История транзакций"
            >
              <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>Баланс</span>
              <Receipt size={13} style={{ color: 'var(--text-muted)', marginLeft: 2 }} />
              <span style={{ marginLeft: 'auto', fontWeight: 600, fontSize: 14 }}>
                {balance !== undefined ? formatCredits(balance) : '—'}
              </span>
            </button>

            {/* Пользователь */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <div style={{
                width: 34, height: 34, borderRadius: '50%', flexShrink: 0,
                background: 'var(--primary)', display: 'flex', alignItems: 'center',
                justifyContent: 'center', fontWeight: 700, fontSize: 13, color: '#fff',
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
              <Button variant="ghost" size="icon" onClick={() => logout.mutate()} title="Выйти" style={{ flexShrink: 0 }}>
                <LogOut size={15} />
              </Button>
            </div>
          </>
        ) : (
          /* Collapsed footer: баланс-число → аватар → выход */
          <>
            <div style={{
              fontSize: 11, fontWeight: 700, color: 'var(--primary)',
              textAlign: 'center', whiteSpace: 'nowrap',
            }}>
              {balance !== undefined ? String(balance) : '—'}
            </div>

            <div style={{
              width: 34, height: 34, borderRadius: '50%',
              background: 'var(--primary)', display: 'flex', alignItems: 'center',
              justifyContent: 'center', fontWeight: 700, fontSize: 13, color: '#fff',
              cursor: 'default',
            }}>
              {user?.display_name?.[0]?.toUpperCase() ?? '?'}
            </div>

            <button
              onClick={() => logout.mutate()}
              title="Выйти"
              style={{
                width: 34, height: 34, borderRadius: 8, border: 'none',
                background: 'transparent', color: 'var(--text-muted)',
                cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center',
                transition: 'background 0.15s, color 0.15s',
              }}
              onMouseEnter={e => { e.currentTarget.style.background = 'var(--primary-subtle)'; e.currentTarget.style.color = 'var(--error)' }}
              onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-muted)' }}
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

function SessionItem({ session, active, onClick }: {
  session: GenerationSession
  active: boolean
  onClick: () => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const rename = useRenameSession()

  const startEdit = (e: MouseEvent) => {
    e.stopPropagation()
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
        display: 'flex', alignItems: 'center', gap: 4,
        padding: '4px 10px', marginBottom: 2,
      }}>
        <input
          autoFocus
          value={draft}
          onChange={e => setDraft(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); commit() } if (e.key === 'Escape') setEditing(false) }}
          style={{
            flex: 1, fontSize: 13, padding: '4px 6px', borderRadius: 6,
            border: '1px solid var(--primary)', background: 'var(--surface2)',
            color: 'var(--text)', outline: 'none',
          }}
        />
        <button onClick={commit} style={{ border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--success)', display: 'flex' }}><Check size={13} /></button>
        <button onClick={() => setEditing(false)} style={{ border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--text-muted)', display: 'flex' }}><X size={13} /></button>
      </div>
    )
  }

  return (
    <div
      style={{ position: 'relative', marginBottom: 2 }}
      className="session-item"
    >
      <button
        onClick={onClick}
        style={{
          display: 'block', width: '100%', textAlign: 'left',
          padding: '8px 32px 8px 10px', borderRadius: 8, border: 'none',
          background: active ? 'var(--primary-subtle)' : 'transparent',
          color: active ? 'var(--text)' : 'var(--text-muted)',
          borderLeft: active ? '2px solid var(--primary)' : '2px solid transparent',
          cursor: 'pointer',
          transition: 'background 0.15s, color 0.15s',
        }}
        onMouseEnter={e => { if (!active) e.currentTarget.style.background = 'var(--primary-subtle)' }}
        onMouseLeave={e => { if (!active) e.currentTarget.style.background = 'transparent' }}
      >
        <div style={{ fontSize: 13, fontWeight: active ? 500 : 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {session.title || 'Без названия'}
        </div>
        <div style={{ fontSize: 11, marginTop: 2, opacity: 0.6 }}>
          {formatDate(session.updated_at)}
        </div>
      </button>
      <button
        onClick={startEdit}
        title="Переименовать"
        style={{
          position: 'absolute', right: 6, top: '50%', transform: 'translateY(-50%)',
          border: 'none', background: 'transparent', cursor: 'pointer',
          color: 'var(--text-muted)', display: 'flex', padding: 4, borderRadius: 4,
          opacity: 0, transition: 'opacity 0.15s',
        }}
        className="rename-btn"
      >
        <Pencil size={12} />
      </button>
    </div>
  )
}
