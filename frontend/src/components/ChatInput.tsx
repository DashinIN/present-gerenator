import { useState, useRef, useCallback } from 'react'
import { Paperclip, Send, X, Music, ImageIcon } from 'lucide-react'
import { useTariff, useBalance } from '@/hooks/useAuth'
import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Textarea'
import { api } from '@/lib/api'
import { useQueryClient } from '@tanstack/react-query'
import type React from 'react'

interface ChatInputProps {
  sessionId: string | null
  parentId: string | null
  onSent: (genId: string, sessionId: string) => void
  onInsufficientCredits?: () => void
  disabled?: boolean
}

interface AttachedFile {
  file: File
  preview?: string
  type: 'image' | 'audio'
}

export function ChatInput({ sessionId, parentId, onSent, onInsufficientCredits, disabled }: ChatInputProps) {
  const { data: tariff } = useTariff()
  const { data: balance } = useBalance()
  const qc = useQueryClient()

  const [prompt, setPrompt] = useState('')
  const [songLyrics, setSongLyrics] = useState('')
  const [songStyle, setSongStyle] = useState('')
  const [imageCount, setImageCount] = useState(1)
  const [songCount, setSongCount] = useState(1)
  const [files, setFiles] = useState<AttachedFile[]>([])
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')

  const fileInputRef = useRef<HTMLInputElement>(null)

  const cost = tariff
    ? tariff.price_per_image * imageCount + tariff.price_per_song * songCount
    : 0

  const canSend = !sending && !disabled && (prompt.trim() || songLyrics.trim()) && (imageCount > 0 || songCount > 0)
  const notEnough = balance !== undefined && cost > balance

  const handleFiles = useCallback((picked: FileList | null) => {
    if (!picked) return
    const added: AttachedFile[] = []
    for (const f of Array.from(picked)) {
      if (f.type.startsWith('image/')) {
        const preview = URL.createObjectURL(f)
        added.push({ file: f, preview, type: 'image' })
      } else if (f.type.startsWith('audio/')) {
        added.push({ file: f, type: 'audio' })
      }
    }
    setFiles(prev => [...prev, ...added].slice(0, 4))
  }, [])

  const removeFile = (idx: number) => {
    setFiles(prev => {
      const next = [...prev]
      if (next[idx].preview) URL.revokeObjectURL(next[idx].preview!)
      next.splice(idx, 1)
      return next
    })
  }

  const send = async () => {
    if (!canSend || notEnough) return
    setSending(true)
    setError('')

    const form = new FormData()
    if (sessionId) form.append('session_id', sessionId)
    if (parentId) form.append('parent_id', parentId)
    form.append('image_prompt', prompt)
    form.append('song_lyrics', songLyrics)
    form.append('song_style', songStyle)
    form.append('image_count', String(imageCount))
    form.append('song_count', String(songCount))

    for (const f of files) {
      if (f.type === 'image') form.append('photos', f.file)
      else form.append('audio', f.file)
    }

    try {
      const result = await api.generations.create(form)
      setPrompt(''); setSongLyrics(''); setSongStyle('')
      setFiles([])
      qc.invalidateQueries({ queryKey: ['sessions'] })
      qc.invalidateQueries({ queryKey: ['balance'] })
      onSent(result.id, result.session_id)
    } catch (e: any) {
      if (e?.code === 'insufficient_credits') {
        onInsufficientCredits?.()
      } else {
        setError(e.message ?? 'Ошибка отправки')
      }
    } finally {
      setSending(false)
    }
  }

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      send()
    }
  }

  const truncateName = (name: string, max = 18) =>
    name.length > max ? name.slice(0, max - 1) + '…' : name

  return (
    <div style={{ padding: '14px 24px 20px', background: 'var(--surface)' }}>
      <div style={{ maxWidth: 720, margin: '0 auto' }}>

        {error && (
          <div style={{ marginBottom: 10, padding: '8px 14px', background: 'rgba(239,68,68,0.08)', borderRadius: 10, fontSize: 13, color: 'var(--error)' }}>
            {error}
          </div>
        )}

        {/* Поля песни */}
        <div className={`song-fields${songCount > 0 ? ' open' : ''}`}>
          <div style={{ display: 'flex', gap: 8 }}>
            <Textarea
              value={songLyrics}
              onChange={e => setSongLyrics(e.target.value)}
              placeholder="Текст песни (необязательно)..."
              rows={2}
              style={{ ...inputStyle, flex: 2, resize: 'vertical' }}
            />
            <input
              value={songStyle}
              onChange={e => setSongStyle(e.target.value)}
              placeholder="Стиль (поп, джаз, рок...)"
              style={{ ...inputStyle, flex: 1 }}
            />
          </div>
        </div>

        {/* Прикреплённые файлы */}
        {files.length > 0 && (
          <div style={{ display: 'flex', gap: 10, marginBottom: 10, flexWrap: 'wrap' }}>
            {files.map((f, i) => (
              <div key={i} style={{ position: 'relative', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4, maxWidth: 72 }}>
                {f.type === 'image' && f.preview ? (
                  <img src={f.preview} alt="" style={{ width: 64, height: 64, borderRadius: 10, objectFit: 'cover' }} />
                ) : (
                  <div style={{ width: 64, height: 64, borderRadius: 10, background: 'var(--primary-subtle)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 4 }}>
                    <Music size={20} style={{ color: 'var(--primary)' }} />
                    <span style={{ fontSize: 9, color: 'var(--text-muted)', fontWeight: 600 }}>MP3</span>
                  </div>
                )}
                <span style={{ fontSize: 10, color: 'var(--text-muted)', textAlign: 'center', maxWidth: 72, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {truncateName(f.file.name)}
                </span>
                <button
                  onClick={() => removeFile(i)}
                  style={{ position: 'absolute', top: -5, right: -5, width: 18, height: 18, borderRadius: '50%', background: 'var(--error)', border: 'none', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                >
                  <X size={10} style={{ color: '#fff' }} />
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Основной инпут */}
        <div style={{
          background: 'var(--bg)',
          borderRadius: 14,
          padding: '12px 14px',
          boxShadow: '0 0 0 1.5px rgba(var(--primary-rgb),0.18), 0 4px 24px rgba(var(--primary-rgb),0.08)',
        }}>
          <Textarea
            value={prompt}
            onChange={e => setPrompt(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder="Опишите поздравление... (Enter — отправить, Shift+Enter — новая строка)"
            rows={2}
            disabled={sending}
          />

          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 10 }}>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*,audio/*"
              multiple
              style={{ display: 'none' }}
              onChange={e => handleFiles(e.target.files)}
            />
            <Button variant="ghost" size="icon" onClick={() => fileInputRef.current?.click()} title="Прикрепить фото или аудио">
              <Paperclip size={16} />
            </Button>

            <CountPicker icon={<ImageIcon size={13} />} label="Картинки" value={imageCount} max={3} onChange={setImageCount} />
            <CountPicker icon={<Music size={13} />} label="Песни" value={songCount} max={3} onChange={setSongCount} />

            <div style={{ flex: 1 }} />

            <span style={{ fontSize: 12, color: notEnough ? 'var(--error)' : 'var(--text-muted)', whiteSpace: 'nowrap' }}>
              {cost > 0 ? `${cost} кр.` : ''}
              {notEnough ? ' — недостаточно' : ''}
            </span>

            <Button
              onClick={send}
              disabled={!canSend || notEnough}
              loading={sending}
              size="sm"
              style={{ gap: 6 }}
            >
              <Send size={14} />
              Отправить
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

function CountPicker({ icon, label, value, max, onChange }: {
  icon: React.ReactNode; label: string; value: number; max: number; onChange: (v: number) => void
}) {
  return (
    <div title={label} style={{ display: 'flex', alignItems: 'center', gap: 2, padding: '3px 6px', borderRadius: 7, background: 'var(--surface2)', fontSize: 12 }}>
      <span style={{ color: 'var(--text-muted)', display: 'flex', alignItems: 'center' }}>{icon}</span>
      <button onClick={() => onChange(Math.max(0, value - 1))} style={counterBtn}>−</button>
      <span style={{ minWidth: 12, textAlign: 'center', fontWeight: 600, color: value > 0 ? 'var(--primary)' : 'var(--text-muted)' }}>{value}</span>
      <button onClick={() => onChange(Math.min(max, value + 1))} style={counterBtn}>+</button>
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  flex: 1,
  background: 'var(--bg)',
  border: '1.5px solid var(--border)',
  borderRadius: 8,
  padding: '9px 12px',
  color: 'var(--text)',
  outline: 'none',
  fontSize: 13,
  width: '100%',
}

const counterBtn: React.CSSProperties = {
  background: 'none',
  border: 'none',
  color: 'var(--text-muted)',
  cursor: 'pointer',
  fontSize: 15,
  lineHeight: 1,
  padding: '0 2px',
}
