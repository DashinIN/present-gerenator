import { useState, useRef, useCallback } from 'react'
import { Paperclip, Send, X, Music, ImageIcon, Sparkles, Loader2 } from 'lucide-react'
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

  const [imageEnabled, setImageEnabled] = useState(true)
  const [songEnabled, setSongEnabled] = useState(true)

  const [prompt, setPrompt] = useState('')
  const [songLyrics, setSongLyrics] = useState('')
  const [songStyle, setSongStyle] = useState('')
  const [lyricsPrompt, setLyricsPrompt] = useState('')
  const [generatingLyrics, setGeneratingLyrics] = useState(false)
  const [files, setFiles] = useState<AttachedFile[]>([])
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')

  const fileInputRef = useRef<HTMLInputElement>(null)

  const imageCount = imageEnabled ? 1 : 0
  const songCount = songEnabled ? 1 : 0

  const cost = tariff
    ? tariff.price_per_image * imageCount + tariff.price_per_song * songCount
    : 0

  const canSend = !sending && !disabled
    && (imageEnabled ? prompt.trim() !== '' : true)
    && (songEnabled ? (songLyrics.trim() !== '' || lyricsPrompt.trim() !== '' || prompt.trim() !== '') : true)
    && (imageEnabled || songEnabled)
  const notEnough = balance !== undefined && cost > balance

  const toggleImage = () => {
    if (imageEnabled && !songEnabled) return
    setImageEnabled(v => !v)
  }

  const toggleSong = () => {
    if (songEnabled && !imageEnabled) return
    setSongEnabled(v => !v)
  }

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

  const generateLyrics = async () => {
    const p = lyricsPrompt.trim() || prompt.trim()
    if (!p || generatingLyrics) return
    setGeneratingLyrics(true)
    setError('')
    try {
      const res = await fetch('/api/generations/lyrics', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prompt: p }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data?.error?.message ?? `HTTP ${res.status}`)
      setSongLyrics(data.text ?? '')
    } catch (e: any) {
      setError(e.message ?? 'Ошибка генерации текста')
    } finally {
      setGeneratingLyrics(false)
    }
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
      setPrompt(''); setSongLyrics(''); setSongStyle(''); setLyricsPrompt('')
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

  const truncateName = (name: string, max = 18) =>
    name.length > max ? name.slice(0, max - 1) + '…' : name

  return (
    <div style={{ padding: '14px 24px 20px', background: 'var(--surface)' }}>
      <div style={{ maxWidth: 720, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 10 }}>

        {error && (
          <div style={{ padding: '8px 14px', background: 'rgba(239,68,68,0.08)', borderRadius: 10, fontSize: 13, color: 'var(--error)' }}>
            {error}
          </div>
        )}

        {/* Блок картинки */}
        <SectionBlock
          icon={<ImageIcon size={14} />}
          label="Картинка"
          enabled={imageEnabled}
          onToggle={toggleImage}
          disableToggle={imageEnabled && !songEnabled}
        >
          <Textarea
            value={prompt}
            onChange={e => setPrompt(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() } }}
            placeholder="Опишите поздравление... (Enter — отправить)"
            rows={2}
            disabled={sending}
          />

          {/* Прикреплённые файлы */}
          {files.length > 0 && (
            <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginTop: 8 }}>
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

          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 8 }}>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*,audio/*"
              multiple
              style={{ display: 'none' }}
              onChange={e => handleFiles(e.target.files)}
            />
            <button
              onClick={() => fileInputRef.current?.click()}
              title="Прикрепить фото"
              style={ghostBtnStyle}
            >
              <Paperclip size={14} style={{ color: 'var(--text-muted)' }} />
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>Прикрепить фото</span>
            </button>
          </div>
        </SectionBlock>

        {/* Блок песни */}
        <SectionBlock
          icon={<Music size={14} />}
          label="Песня"
          sublabel="2 варианта"
          enabled={songEnabled}
          onToggle={toggleSong}
          disableToggle={songEnabled && !imageEnabled}
        >
          <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
            <div style={{ flex: 2, display: 'flex', flexDirection: 'column', gap: 6 }}>
              <div style={{ display: 'flex', gap: 6 }}>
                <input
                  value={lyricsPrompt}
                  onChange={e => setLyricsPrompt(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); generateLyrics() } }}
                  placeholder="Промт для текста (или использует промт картинки)..."
                  disabled={generatingLyrics}
                  style={inputStyle}
                />
                <button
                  onClick={generateLyrics}
                  disabled={generatingLyrics || (!lyricsPrompt.trim() && !prompt.trim())}
                  title="Сгенерировать текст песни"
                  style={{
                    display: 'flex', alignItems: 'center', gap: 5,
                    padding: '0 12px', borderRadius: 8, border: 'none',
                    background: 'var(--primary)', color: '#fff',
                    fontSize: 12, fontWeight: 500, cursor: 'pointer',
                    opacity: generatingLyrics || (!lyricsPrompt.trim() && !prompt.trim()) ? 0.5 : 1,
                    whiteSpace: 'nowrap', flexShrink: 0,
                  }}
                >
                  {generatingLyrics
                    ? <><Loader2 size={13} style={{ animation: 'spin 1s linear infinite' }} /> Генерирую...</>
                    : <><Sparkles size={13} /> Сгенерировать{tariff?.price_per_lyrics ? ` (${tariff.price_per_lyrics} кр.)` : ''}</>
                  }
                </button>
              </div>
              <Textarea
                value={songLyrics}
                onChange={e => setSongLyrics(e.target.value)}
                placeholder="Текст песни (введите вручную или сгенерируйте выше)..."
                rows={3}
                disabled={generatingLyrics}
                style={{ resize: 'vertical', opacity: generatingLyrics ? 0.6 : 1 }}
              />
            </div>
            <input
              value={songStyle}
              onChange={e => setSongStyle(e.target.value)}
              placeholder="Стиль (поп, джаз...)"
              style={{ ...inputStyle, flex: 1, alignSelf: 'flex-start' }}
            />
          </div>
        </SectionBlock>

        {/* Отправить */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 12 }}>
          <span style={{ fontSize: 12, color: notEnough ? 'var(--error)' : 'var(--text-muted)' }}>
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
  )
}

interface SectionBlockProps {
  icon: React.ReactNode
  label: string
  sublabel?: string
  enabled: boolean
  onToggle: () => void
  disableToggle: boolean
  children: React.ReactNode
}

function SectionBlock({ icon, label, sublabel, enabled, onToggle, disableToggle, children }: SectionBlockProps) {
  return (
    <div style={{
      border: `1.5px solid ${enabled ? 'rgba(var(--primary-rgb),0.3)' : 'var(--border)'}`,
      borderRadius: 14,
      overflow: 'hidden',
      transition: 'border-color 0.2s',
      background: 'var(--bg)',
    }}>
      {/* Заголовок */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 8,
        padding: '10px 14px',
        background: enabled ? 'rgba(var(--primary-rgb),0.04)' : 'var(--surface2)',
        borderBottom: enabled ? '1px solid rgba(var(--primary-rgb),0.1)' : '1px solid transparent',
        transition: 'background 0.2s',
      }}>
        <span style={{ color: enabled ? 'var(--primary)' : 'var(--text-muted)', display: 'flex' }}>{icon}</span>
        <span style={{ fontSize: 13, fontWeight: 600, color: enabled ? 'var(--text)' : 'var(--text-muted)' }}>{label}</span>
        {sublabel && (
          <span style={{ fontSize: 11, color: 'var(--text-muted)', background: 'var(--surface2)', padding: '1px 7px', borderRadius: 20 }}>
            {sublabel}
          </span>
        )}
        <div style={{ flex: 1 }} />
        <button
          onClick={onToggle}
          disabled={disableToggle}
          title={disableToggle ? 'Нельзя отключить оба блока' : enabled ? 'Отключить' : 'Включить'}
          style={{
            position: 'relative', width: 36, height: 20, borderRadius: 10,
            background: enabled ? 'var(--primary)' : 'var(--border)',
            border: 'none', cursor: disableToggle ? 'not-allowed' : 'pointer',
            opacity: disableToggle ? 0.4 : 1,
            transition: 'background 0.2s', flexShrink: 0,
          }}
        >
          <span style={{
            position: 'absolute', top: 3, left: enabled ? 19 : 3,
            width: 14, height: 14, borderRadius: '50%', background: '#fff',
            transition: 'left 0.2s',
            boxShadow: '0 1px 3px rgba(0,0,0,0.2)',
          }} />
        </button>
      </div>

      {/* Тело */}
      <div style={{
        padding: enabled ? '12px 14px' : '0 14px',
        maxHeight: enabled ? 400 : 0,
        overflow: 'hidden',
        transition: 'max-height 0.25s ease, padding 0.25s ease',
      }}>
        {children}
      </div>
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  flex: 1,
  background: 'var(--surface)',
  border: '1.5px solid var(--border)',
  borderRadius: 8,
  padding: '9px 12px',
  color: 'var(--text)',
  outline: 'none',
  fontSize: 13,
  width: '100%',
}

const ghostBtnStyle: React.CSSProperties = {
  display: 'flex', alignItems: 'center', gap: 5,
  background: 'none', border: 'none', cursor: 'pointer',
  padding: '4px 8px', borderRadius: 7,
}
