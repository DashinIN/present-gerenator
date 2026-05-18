import { useEffect, useMemo, useRef, useState } from 'react'
import { CheckCircle2, XCircle, Download, ImageIcon, Music2, WandSparkles } from 'lucide-react'
import { AppLogo } from '@/components/AppLogo'
import { AudioPlayer } from '@/components/ui/AudioPlayer'
import type { GenerationRequest } from '@/lib/types'
import { formatDate } from '@/lib/utils'
import { getDisplayImagePrompt } from '@/lib/imagePresets'
import { useI18n } from '@/lib/i18n'

interface ChatThreadProps {
  generations: GenerationRequest[]
  noCreditsAt?: string | null
}

export function ChatThread({ generations, noCreditsAt }: ChatThreadProps) {
  const { t } = useI18n()
  const bottomRef = useRef<HTMLDivElement>(null)
  const lastStatus = generations[generations.length - 1]?.status

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [generations.length, lastStatus])

  if (generations.length === 0) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', flexDirection: 'column', gap: 12, color: 'var(--text-muted)' }}>
        {noCreditsAt ? (
          <div style={{ display: 'flex', gap: 10 }}>
            <BotAvatar />
            <div style={{
              background: 'var(--surface)', border: '1px solid var(--error)',
              borderRadius: '4px 16px 16px 16px', padding: '14px 16px',
              fontSize: 14, color: 'var(--error)',
            }}>
              {t('noCredits')}
              <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 6 }}>{noCreditsAt}</div>
            </div>
          </div>
        ) : (
          <>
            <AppLogo size={56} />
            <div style={{ fontSize: 15, fontWeight: 500 }}>{t('createFirstGreeting')}</div>
            <div style={{ fontSize: 13 }}>{t('addPromptAndSend')}</div>
          </>
        )}
      </div>
    )
  }

  return (
    <div style={{ flex: 1, overflowY: 'auto', padding: '24px 0' }}>
      <div style={{ maxWidth: 720, margin: '0 auto', padding: '0 24px', display: 'flex', flexDirection: 'column', gap: 32 }}>
        {generations.map((gen, index) => (
          <GenerationMessage key={gen.id} gen={gen} isNew={index === generations.length - 1} />
        ))}
        {noCreditsAt && (
          <div className="msg-enter" style={{ display: 'flex', gap: 10 }}>
            <BotAvatar />
            <div style={{
              background: 'var(--surface)', border: '1px solid var(--error)',
              borderRadius: '4px 16px 16px 16px', padding: '14px 16px',
              fontSize: 14, color: 'var(--error)',
            }}>
              {t('noCredits')}
              <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 6 }}>{noCreditsAt}</div>
            </div>
          </div>
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}

function BotAvatar() {
  return (
    <div style={{
      width: 32, height: 32, borderRadius: '50%', flexShrink: 0,
      background: 'var(--surface2)', display: 'flex',
      alignItems: 'center', justifyContent: 'center',
    }}>
      <AppLogo size={20} />
    </div>
  )
}

function GenerationMessage({ gen, isNew }: { gen: GenerationRequest; isNew?: boolean }) {
  return (
    <div className={isNew ? 'msg-enter' : undefined} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <div style={{
          maxWidth: '80%', background: 'var(--primary)',
          borderRadius: '16px 16px 4px 16px', padding: '12px 16px',
        }}>
          <UserPrompt gen={gen} />
        </div>
      </div>

      <div style={{ display: 'flex', gap: 10 }}>
        <BotAvatar />
        <div style={{ flex: 1 }}>
          <GenerationResult gen={gen} />
        </div>
      </div>
    </div>
  )
}

function UserPrompt({ gen }: { gen: GenerationRequest }) {
  const text = gen.image_prompt ? getDisplayImagePrompt(gen.image_prompt) : gen.song_lyrics || gen.song_prompt
  const previewImage = gen.input_photos?.[0]

  return (
    <div style={{ fontSize: 14, color: '#fff' }}>
      {text && (
        <div style={{ whiteSpace: 'pre-wrap', lineHeight: 1.5 }}>
          {text}
        </div>
      )}
      {previewImage && (
        <div style={{ marginTop: 8 }}>
          <img
            src={previewImage}
            alt=""
            style={{
              width: 56,
              height: 56,
              borderRadius: 8,
              objectFit: 'cover',
              display: 'block',
              opacity: 0.95,
              border: '1px solid rgba(255,255,255,0.18)',
            }}
          />
        </div>
      )}
    </div>
  )
}

function GenerationResult({ gen }: { gen: GenerationRequest }) {
  const isPending = gen.status === 'pending' || gen.status === 'processing_images' || gen.status === 'processing_audio'
  const isFailed = gen.status === 'failed'
  const isCompleted = gen.status === 'completed'
  const imageOnly = isCompleted && (gen.result_images?.length ?? 0) > 0 && !(gen.result_audios?.length)

  if (imageOnly) {
    return (
      <div>
        <CompletedState gen={gen} />
        <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 4 }}>
          {formatDate(gen.created_at)}
        </div>
      </div>
    )
  }

  return (
    <div style={{
      background: 'var(--surface)',
      borderRadius: '4px 16px 16px 16px',
      overflow: 'hidden',
      boxShadow: '0 2px 16px rgba(var(--primary-rgb),0.08)',
    }}>
      {isPending && <SkeletonState gen={gen} />}
      {isFailed && <FailedState message={gen.error_message} />}
      {isCompleted && <CompletedState gen={gen} />}
      <div style={{ fontSize: 11, color: 'var(--text-muted)', padding: '0 16px 12px' }}>
        {formatDate(gen.created_at)}
      </div>
    </div>
  )
}

function SkeletonState({ gen }: { gen: GenerationRequest }) {
  const { t } = useI18n()
  const labels: Partial<Record<GenerationRequest['status'], string>> = {
    pending: t('loaderPending'),
    processing_images: t('loaderProcessingImages'),
    processing_audio: t('loaderProcessingAudio'),
  }
  const stage = gen.status === 'processing_images' || gen.status === 'processing_audio' ? gen.status : 'pending'
  const label = labels[gen.status] ?? t('loaderProcessingFallback')
  const [copyIndex, setCopyIndex] = useState(0)
  const [now, setNow] = useState(() => Date.now())
  const phrasesByStage: Record<typeof stage, string[]> = {
    pending: [t('loadingPending1'), t('loadingPending2'), t('loadingPending3'), t('loadingPending4')],
    processing_images: [t('loadingImages1'), t('loadingImages2'), t('loadingImages3'), t('loadingImages4')],
    processing_audio: [t('loadingAudio1'), t('loadingAudio2'), t('loadingAudio3'), t('loadingAudio4')],
  }
  const phrases = phrasesByStage[stage]

  const readyAudios = gen.result_audios ?? []
  const totalAudioSlots = Math.max(gen.song_count, readyAudios.length, gen.song_count > 0 ? 2 : 0)
  const pendingSlots = totalAudioSlots - readyAudios.length
  const progress = useMemo(() => {
    const createdAt = new Date(gen.created_at).getTime()
    const elapsedSeconds = Number.isFinite(createdAt) ? Math.max(0, (now - createdAt) / 1000) : 0
    const stageFloor = stage === 'pending' ? 6 : stage === 'processing_images' ? 28 : 62
    const stageCap = stage === 'pending' ? 24 : stage === 'processing_images' ? 68 : 92
    const readyBonus = totalAudioSlots > 0 ? (readyAudios.length / totalAudioSlots) * 18 : 0

    return Math.min(stageCap, Math.round(stageFloor + elapsedSeconds * 0.45 + readyBonus))
  }, [gen.created_at, now, readyAudios.length, stage, totalAudioSlots])

  useEffect(() => {
    const interval = window.setInterval(() => {
      setCopyIndex(index => (index + 1) % phrases.length)
    }, 3600)

    return () => window.clearInterval(interval)
  }, [phrases.length])

  useEffect(() => {
    const interval = window.setInterval(() => setNow(Date.now()), 1200)

    return () => window.clearInterval(interval)
  }, [])

  return (
    <div className="generation-loader">
      <div className="generation-loader__header">
        <div className="generation-loader__orb" aria-hidden="true">
          <WandSparkles size={18} />
        </div>
        <div className="generation-loader__copy">
          <div className="generation-loader__title">{label}</div>
          <div className="generation-loader__phrase" key={`${stage}-${copyIndex}`}>
            {phrases[copyIndex]}
          </div>
        </div>
        <div className="generation-loader__percent">{progress}%</div>
      </div>

      <div className="generation-loader__progress" aria-label={t('generationProgressAria').replace('{progress}', String(progress))}>
        <div className="generation-loader__progress-fill" style={{ width: `${progress}%` }} />
      </div>

      {gen.image_count > 0 && (
        <div className="generation-loader__stack" style={{ marginBottom: gen.song_count > 0 ? 16 : 0 }}>
          {Array.from({ length: gen.image_count }).map((_, index) => (
            <VisualImageSlot key={index} index={index} />
          ))}
        </div>
      )}

      {gen.song_count > 0 && (
        <div className="generation-loader__stack">
          {readyAudios.map((url, index) => (
            <AudioPlayer
              key={`ready-${index}`}
              src={url}
              label={t('audioSlotLabel').replace('{index}', String(index + 1))}
            />
          ))}
          {Array.from({ length: pendingSlots }).map((_, index) => (
            <VisualAudioSlot key={`audio-loader-${index}`} index={index} />
          ))}
        </div>
      )}
    </div>
  )
}

function VisualImageSlot({ index }: { index: number }) {
  const { t } = useI18n()

  return (
    <div className="image-loader-slot">
      <div className="image-loader-slot__sky" />
      <div className="image-loader-slot__sun" />
      <div className="image-loader-slot__mountain image-loader-slot__mountain--back" />
      <div className="image-loader-slot__mountain image-loader-slot__mountain--front" />
      <div className="image-loader-slot__scan" />
      <div className="image-loader-slot__label">
        <ImageIcon size={14} />
        {t('imageSlotLabel').replace('{index}', String(index + 1))}
      </div>
    </div>
  )
}

function VisualAudioSlot({ index }: { index: number }) {
  const { t } = useI18n()
  const bars = [22, 34, 18, 42, 28, 48, 24, 38, 20, 32, 44, 26]

  return (
    <div className="audio-loader-slot">
      <div className="audio-loader-slot__icon">
        <Music2 size={16} />
      </div>
      <div className="audio-loader-slot__content">
        <div className="audio-loader-slot__label">{t('audioSlotLabel').replace('{index}', String(index + 1))}</div>
        <div className="audio-loader-slot__wave" aria-hidden="true">
          {bars.map((height, barIndex) => (
            <span key={barIndex} style={{ height, animationDelay: `${barIndex * 90}ms` }} />
          ))}
        </div>
      </div>
    </div>
  )
}

function FailedState({ message }: { message?: string }) {
  const { t } = useI18n()

  return (
    <div style={{ padding: '16px 16px 8px', display: 'flex', alignItems: 'flex-start', gap: 8 }}>
      <XCircle size={16} style={{ color: 'var(--error)', flexShrink: 0, marginTop: 2 }} />
      <div>
        <div style={{ fontSize: 14, color: 'var(--error)' }}>{t('generationError')}</div>
        {message && <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 4 }}>{message}</div>}
      </div>
    </div>
  )
}

function CompletedState({ gen }: { gen: GenerationRequest }) {
  const { t } = useI18n()

  return (
    <div style={{ display: 'flex', flexDirection: 'column' }}>
      {gen.result_images?.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {gen.result_images.map((url, index) => (
            <div key={index} style={{ position: 'relative' }}>
              <img
                src={url}
                alt={`result ${index + 1}`}
                style={{
                  width: '70%', display: 'block',
                  borderRadius: '4px 16px 16px 16px',
                }}
              />
              <a
                href={url}
                download
                style={{
                  position: 'absolute', bottom: 10, right: 10,
                  background: 'rgba(0,0,0,0.55)', backdropFilter: 'blur(4px)',
                  borderRadius: 8, padding: '6px 8px',
                  display: 'flex', alignItems: 'center', gap: 5,
                  color: '#fff', fontSize: 12, textDecoration: 'none',
                }}
              >
                <Download size={13} /> {t('download')}
              </a>
            </div>
          ))}
        </div>
      )}

      {gen.result_audios?.length > 0 && (
        <div style={{ padding: '14px 16px 8px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: 5, marginBottom: 4 }}>
            <CheckCircle2 size={13} style={{ color: 'var(--success)' }} />
          </div>
          {gen.result_audios.map((url, index) => (
            <AudioPlayer
              key={index}
              src={url}
              label={t('audioSlotLabel').replace('{index}', String(index + 1))}
            />
          ))}
        </div>
      )}

      {gen.result_images?.length > 0 && !gen.result_audios?.length && (
        <div style={{ padding: '10px 16px 8px', display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--success)' }}>
          <CheckCircle2 size={13} />
        </div>
      )}
    </div>
  )
}
