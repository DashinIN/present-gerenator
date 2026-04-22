import { useEffect, useRef } from 'react'
import { ImageIcon, Music, CheckCircle2, XCircle, Download } from 'lucide-react'
import type { GenerationRequest } from '@/lib/types'
import { formatDate } from '@/lib/utils'

interface ChatThreadProps {
  generations: GenerationRequest[]
  noCreditsAt?: string | null
}

export function ChatThread({ generations, noCreditsAt }: ChatThreadProps) {
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [generations.length, generations[generations.length - 1]?.status])

  if (generations.length === 0) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', flexDirection: 'column', gap: 12, color: 'var(--text-muted)' }}>
        <div style={{ fontSize: 40 }}>🎉</div>
        <div style={{ fontSize: 15, fontWeight: 500 }}>Создайте первое поздравление</div>
        <div style={{ fontSize: 13 }}>Добавьте промпт и нажмите отправить</div>
      </div>
    )
  }

  return (
    <div style={{ flex: 1, overflowY: 'auto', padding: '24px 0' }}>
      <div style={{ maxWidth: 720, margin: '0 auto', padding: '0 24px', display: 'flex', flexDirection: 'column', gap: 32 }}>
        {generations.map((gen, i) => (
          <GenerationMessage key={gen.id} gen={gen} isNew={i === generations.length - 1} />
        ))}
        {noCreditsAt && (
          <div className="msg-enter" style={{ display: 'flex', gap: 10 }}>
            <BotAvatar />
            <div style={{
              background: 'var(--surface)', border: '1px solid var(--error)',
              borderRadius: '4px 16px 16px 16px', padding: '14px 16px',
              fontSize: 14, color: 'var(--error)',
            }}>
              Кредиты закончились. Пополните баланс, чтобы продолжить.
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
      alignItems: 'center', justifyContent: 'center', fontSize: 16,
    }}>
      🎁
    </div>
  )
}

function GenerationMessage({ gen, isNew }: { gen: GenerationRequest; isNew?: boolean }) {
  return (
    <div className={isNew ? 'msg-enter' : undefined} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* Пузырь пользователя */}
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <div style={{
          maxWidth: '80%', background: 'var(--primary)',
          borderRadius: '16px 16px 4px 16px', padding: '12px 16px',
        }}>
          <UserPrompt gen={gen} />
        </div>
      </div>

      {/* Ответ системы */}
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
  const prompt = gen.image_prompt || gen.song_lyrics || ''
  return (
    <div style={{ fontSize: 14, color: '#fff' }}>
      {prompt && <div>{prompt}</div>}
      <div style={{ display: 'flex', gap: 12, marginTop: 8, fontSize: 12, opacity: 0.8 }}>
        {gen.image_count > 0 && (
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <ImageIcon size={12} /> {gen.image_count} {plural(gen.image_count, 'картинка', 'картинки', 'картинок')}
          </span>
        )}
        {gen.song_count > 0 && (
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <Music size={12} /> {gen.song_count} {plural(gen.song_count, 'песня', 'песни', 'песен')}
          </span>
        )}
        <span style={{ marginLeft: 'auto' }}>−{gen.credits_spent} кр.</span>
      </div>
      {gen.input_photos?.length > 0 && (
        <div style={{ display: 'flex', gap: 6, marginTop: 8, flexWrap: 'wrap' }}>
          {gen.input_photos.map((url, i) => (
            <img key={i} src={url} alt="" style={{ width: 48, height: 48, borderRadius: 6, objectFit: 'cover', opacity: 0.9 }} />
          ))}
        </div>
      )}
    </div>
  )
}

function GenerationResult({ gen }: { gen: GenerationRequest }) {
  const isPending = gen.status === 'pending' || gen.status === 'processing_images' || gen.status === 'processing_audio'
  const isFailed = gen.status === 'failed'
  const isCompleted = gen.status === 'completed'

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
  const label = {
    pending: 'В очереди...',
    processing_images: 'Генерирую картинки...',
    processing_audio: 'Генерирую музыку...',
  }[gen.status] ?? 'Обрабатываю...'

  return (
    <div style={{ padding: '16px 16px 8px' }}>
      {/* Статус */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
        <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--primary)', display: 'inline-block', animation: 'pulse 1.4s ease-in-out infinite' }} />
        <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>{label}</span>
      </div>

      {/* Скелетоны картинок */}
      {gen.image_count > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: gen.song_count > 0 ? 16 : 0 }}>
          {Array.from({ length: gen.image_count }).map((_, i) => (
            <div key={i} className="skeleton" style={{ width: '100%', aspectRatio: '16/9', borderRadius: 12 }} />
          ))}
        </div>
      )}

      {/* Скелетоны аудио */}
      {gen.song_count > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {Array.from({ length: gen.song_count }).map((_, i) => (
            <div key={i} className="skeleton" style={{ width: '100%', height: 48, borderRadius: 10 }} />
          ))}
        </div>
      )}
    </div>
  )
}

function FailedState({ message }: { message?: string }) {
  return (
    <div style={{ padding: '16px 16px 8px', display: 'flex', alignItems: 'flex-start', gap: 8 }}>
      <XCircle size={16} style={{ color: 'var(--error)', flexShrink: 0, marginTop: 2 }} />
      <div>
        <div style={{ fontSize: 14, color: 'var(--error)' }}>Ошибка генерации</div>
        {message && <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 4 }}>{message}</div>}
      </div>
    </div>
  )
}

function CompletedState({ gen }: { gen: GenerationRequest }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column' }}>
      {/* Картинки — на всю ширину */}
      {gen.result_images?.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {gen.result_images.map((url, i) => (
            <div key={i} style={{ position: 'relative' }}>
              <img
                src={url}
                alt={`result ${i + 1}`}
                style={{
                  width: '100%', display: 'block',
                  borderRadius: gen.result_images.length === 1 ? '4px 16px 0 0' : i === 0 ? '4px 16px 0 0' : i === gen.result_images.length - 1 && !gen.result_audios?.length ? '0 0 0 0' : '0',
                  objectFit: 'cover',
                  maxHeight: 420,
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
                <Download size={13} /> Скачать
              </a>
            </div>
          ))}
        </div>
      )}

      {/* Аудио */}
      {gen.result_audios?.length > 0 && (
        <div style={{ padding: '14px 16px 8px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: 5, marginBottom: 4 }}>
            <CheckCircle2 size={13} style={{ color: 'var(--success)' }} /> Готово
          </div>
          {gen.result_audios.map((url, i) => (
            <div key={i} style={{
              display: 'flex', alignItems: 'center', gap: 10,
              background: 'var(--surface2)', borderRadius: 10, padding: '10px 14px',
            }}>
              <audio controls src={url} style={{ flex: 1, height: 36 }} />
              <a href={url} download style={{ color: 'var(--text-muted)', display: 'flex', flexShrink: 0 }} title="Скачать">
                <Download size={15} />
              </a>
            </div>
          ))}
        </div>
      )}

      {/* Если только картинки — показываем статус внизу */}
      {gen.result_images?.length > 0 && !gen.result_audios?.length && (
        <div style={{ padding: '10px 16px 8px', display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--success)' }}>
          <CheckCircle2 size={13} /> Готово
        </div>
      )}
    </div>
  )
}

function plural(n: number, one: string, few: string, many: string) {
  if (n % 10 === 1 && n % 100 !== 11) return one
  if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)) return few
  return many
}
