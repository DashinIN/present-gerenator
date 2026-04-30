import { useState, useRef, useCallback } from 'react'
import { Paperclip, Send, X, Music, ImageIcon, Sparkles, Loader2 } from 'lucide-react'
import { useTariff, useBalance } from '@/hooks/useAuth'
import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Textarea'
import { api, ApiError } from '@/lib/api'
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

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
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

  const imageInputRef = useRef<HTMLInputElement>(null)
  const audioInputRef = useRef<HTMLInputElement>(null)

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

  const handleFiles = useCallback((picked: FileList | null, acceptedType: 'image' | 'audio') => {
    if (!picked) return
    const next = [...files]
    let nextAudioCount = next.filter(f => f.type === 'audio').length
    let nextImageCount = next.filter(f => f.type === 'image').length
    let changed = false
    for (const f of Array.from(picked)) {
      if (acceptedType === 'image' && f.type.startsWith('image/')) {
        const preview = URL.createObjectURL(f)
        if (nextImageCount < 3) {
          next.push({ file: f, preview, type: 'image' })
          nextImageCount += 1
          changed = true
        } else {
          URL.revokeObjectURL(preview)
          setError('Можно прикрепить максимум 3 фото')
        }
      } else if (acceptedType === 'audio' && f.type.startsWith('audio/')) {
        if (nextAudioCount < 2) {
          next.push({ file: f, type: 'audio' })
          nextAudioCount += 1
          changed = true
        } else {
          setError('Можно прикрепить максимум 2 аудио')
        }
      }
    }
    if (changed) {
      setError('')
      setFiles(next)
    }
  }, [files])

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
      const data = await api.generations.lyrics(p)
      setSongLyrics(data.text ?? '')
    } catch (e: unknown) {
      setError(errorMessage(e, 'Ошибка генерации текста'))
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
    } catch (e: unknown) {
      if (e instanceof ApiError && e.code === 'insufficient_credits') {
        onInsufficientCredits?.()
      } else {
        setError(errorMessage(e, 'Ошибка отправки'))
      }
    } finally {
      setSending(false)
    }
  }

  const truncateName = (name: string, max = 18) =>
    name.length > max ? name.slice(0, max - 1) + '…' : name

  const imageFiles = files.filter(file => file.type === 'image')
  const audioFiles = files.filter(file => file.type === 'audio')

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

          {imageFiles.length > 0 && (
            <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginTop: 8 }}>
              {imageFiles.map((f) => {
                const fileIndex = files.indexOf(f)
                return (
                  <div key={`${f.file.name}-${fileIndex}`} style={{ position: 'relative', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4, maxWidth: 72 }}>
                    {f.preview ? (
                      <img src={f.preview} alt="" style={{ width: 64, height: 64, borderRadius: 10, objectFit: 'cover' }} />
                    ) : null}
                    <span style={{ fontSize: 10, color: 'var(--text-muted)', textAlign: 'center', maxWidth: 72, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {truncateName(f.file.name)}
                    </span>
                    <button
                      onClick={() => removeFile(fileIndex)}
                      style={{ position: 'absolute', top: -5, right: -5, width: 18, height: 18, borderRadius: '50%', background: 'var(--error)', border: 'none', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                    >
                      <X size={10} style={{ color: '#fff' }} />
                    </button>
                  </div>
                )
              })}
            </div>
          )}

          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 8 }}>
            <input
              ref={imageInputRef}
              type="file"
              accept="image/*"
              multiple
              style={{ display: 'none' }}
              onChange={e => {
                handleFiles(e.target.files, 'image')
                e.currentTarget.value = ''
              }}
            />
            <button
              onClick={() => imageInputRef.current?.click()}
              title="Прикрепить фото"
              style={ghostBtnStyle}
            >
              <Paperclip size={14} style={{ color: 'var(--text-muted)' }} />
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>Прикрепить фото</span>
            </button>
            <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{imageFiles.length}/3</span>
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

          {audioFiles.length > 0 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 10 }}>
              {audioFiles.map((f) => {
                const fileIndex = files.indexOf(f)
                return (
                  <div
                    key={`${f.file.name}-${fileIndex}`}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 10,
                      padding: '10px 12px',
                      borderRadius: 10,
                      background: 'var(--surface)',
                      border: '1px solid var(--border)',
                    }}
                  >
                    <div style={{ width: 36, height: 36, borderRadius: 8, background: 'var(--primary-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                      <Music size={16} style={{ color: 'var(--primary)' }} />
                    </div>
                    <div style={{ minWidth: 0, flex: 1 }}>
                      <div style={{ fontSize: 13, color: 'var(--text)' }}>{truncateName(f.file.name, 28)}</div>
                      <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>{formatFileSize(f.file.size)}</div>
                    </div>
                    <button
                      onClick={() => removeFile(fileIndex)}
                      title="Убрать трек"
                      style={{ ...iconBtnStyle, flexShrink: 0 }}
                    >
                      <X size={14} />
                    </button>
                  </div>
                )
              })}
            </div>
          )}

          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 10 }}>
            <input
              ref={audioInputRef}
              type="file"
              accept="audio/*"
              multiple
              style={{ display: 'none' }}
              onChange={e => {
                handleFiles(e.target.files, 'audio')
                e.currentTarget.value = ''
              }}
            />
            <button
              onClick={() => audioInputRef.current?.click()}
              title="Прикрепить музыку"
              style={ghostBtnStyle}
            >
              <Paperclip size={14} style={{ color: 'var(--text-muted)' }} />
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>Прикрепить музыку</span>
            </button>
            <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{audioFiles.length}/2</span>
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

const iconBtnStyle: React.CSSProperties = {
  width: 28,
  height: 28,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  borderRadius: 8,
  border: '1px solid var(--border)',
  background: 'var(--bg)',
  color: 'var(--text-muted)',
  cursor: 'pointer',
}

function formatFileSize(bytes: number) {
  if (bytes < 1024 * 1024) {
    return `${Math.max(1, Math.round(bytes / 1024))} KB`
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
