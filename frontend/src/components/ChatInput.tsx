import { useState, useRef, useCallback } from 'react'
import { Paperclip, Send, X, Music, ImageIcon, Sparkles, Loader2, ChevronDown, Check } from 'lucide-react'
import { useTariff, useBalance } from '@/hooks/useAuth'
import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Textarea'
import { api, ApiError, type LyricsVariant } from '@/lib/api'
import {
  composeImagePrompt,
  imagePromptCategories,
  imagePromptPresets,
  noImagePresetId,
} from '@/lib/imagePresets'
import { useQueryClient } from '@tanstack/react-query'
import type React from 'react'
import { useI18n } from '@/lib/i18n'

interface ChatInputProps {
  sessionId: string | null
  parentId: string | null
  onSent: (genId: string, sessionId: string) => void
  onInsufficientCredits?: () => void
  disabled?: boolean
  initialMode?: 'image' | 'song'
  initialModeVersion?: number
}

interface AttachedFile {
  file: File
  preview?: string
  type: 'image'
}

type ImageModel = 'gpt-image-2' | 'flux-2-flex' | 'seedream-5-lite'

const imageModels: Array<{ value: ImageModel; label: string; hint: string }> = [
  { value: 'gpt-image-2', label: 'Best quality', hint: 'GPT Image 2' },
  { value: 'flux-2-flex', label: 'Balanced', hint: 'Flux 2 Flex' },
  { value: 'seedream-5-lite', label: 'Fast', hint: 'Seedream 5 Lite' },
]

const categoryTranslationKeys: Record<string, string> = {
  'For people': 'categoryPeople',
  'For social media': 'categorySocial',
  'For products': 'categoryProducts',
  'For characters': 'categoryCharacters',
  'For couples and events': 'categoryEvents',
}

const presetTranslationKeys: Record<string, string> = {
  photoshoot: 'presetPhotoshoot',
  avatar: 'presetAvatar',
  business: 'presetBusiness',
  magazine: 'presetMagazine',
  'sports-broadcast': 'presetSportsBroadcast',
  cinematic: 'presetCinematic',
  street: 'presetStreet',
  paparazzi: 'presetPaparazzi',
  'red-carpet': 'presetRedCarpet',
  meme: 'presetMeme',
  streamer: 'presetStreamer',
  esports: 'presetEsports',
  'album-cover': 'presetAlbumCover',
  'movie-poster': 'presetMoviePoster',
  'product-photo': 'presetProductPhoto',
  'lifestyle-product': 'presetLifestyleProduct',
  'luxury-ad': 'presetLuxuryAd',
  sticker: 'presetSticker',
  'icon-3d': 'presetIcon3d',
  'mascot-logo': 'presetMascotLogo',
  anime: 'presetAnime',
  cyberpunk: 'presetCyberpunk',
  fantasy: 'presetFantasy',
  superhero: 'presetSuperhero',
  'game-character': 'presetGameCharacter',
  'crime-game': 'presetCrimeGame',
  'retro-90s': 'presetRetro90s',
  polaroid: 'presetPolaroid',
  y2k: 'presetY2k',
  horror: 'presetHorror',
  travel: 'presetTravel',
  romantic: 'presetRomantic',
  wedding: 'presetWedding',
  toy: 'presetToy',
  interior: 'presetInterior',
}

function translateLabel(t: (key: string) => string, key: string | undefined, fallback: string) {
  if (!key) return fallback
  const translated = t(key)
  return translated === key ? fallback : translated
}

function categoryLabel(category: string, t: (key: string) => string) {
  return translateLabel(t, categoryTranslationKeys[category], category)
}

function presetLabel(id: string, fallback: string, t: (key: string) => string) {
  return translateLabel(t, presetTranslationKeys[id], fallback)
}

function imageModelLabel(model: ImageModel) {
  return imageModels.find(option => option.value === model)?.hint ?? 'GPT Image 2'
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}

export function ChatInput({
  sessionId,
  parentId,
  onSent,
  onInsufficientCredits,
  disabled,
  initialMode = 'image',
}: ChatInputProps) {
  const { data: tariff } = useTariff()
  const { data: balance } = useBalance()
  const qc = useQueryClient()
  const { t } = useI18n()

  const [imageEnabled, setImageEnabled] = useState(() => initialMode !== 'song')
  const [songEnabled, setSongEnabled] = useState(() => initialMode === 'song')
  const [prompt, setPrompt] = useState('')
  const [imageModel, setImageModel] = useState<ImageModel>('gpt-image-2')
  const [imageModelOpen, setImageModelOpen] = useState(false)
  const [filterModalOpen, setFilterModalOpen] = useState(false)
  const [presetCategory, setPresetCategory] = useState<string>(imagePromptCategories[0])
  const [imagePresetId, setImagePresetId] = useState(noImagePresetId)
  const [songLyrics, setSongLyrics] = useState('')
  const [songStyle, setSongStyle] = useState('')
  const [lyricsPrompt, setLyricsPrompt] = useState('')
  const [lyricsVariants, setLyricsVariants] = useState<LyricsVariant[]>([])
  const [selectedLyricsIndex, setSelectedLyricsIndex] = useState(0)
  const [lyricsModalOpen, setLyricsModalOpen] = useState(false)
  const [generatingLyrics, setGeneratingLyrics] = useState(false)
  const [files, setFiles] = useState<AttachedFile[]>([])
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')

  const imageInputRef = useRef<HTMLInputElement>(null)
  const imageCount = imageEnabled ? 1 : 0
  const songCount = songEnabled ? 1 : 0
  const cost = tariff ? tariff.price_per_image * imageCount + tariff.price_per_song * songCount : 0
  const canSend = !sending && !disabled
    && (imageEnabled ? prompt.trim() !== '' : true)
    && (songEnabled ? songLyrics.trim() !== '' : true)
    && (imageEnabled || songEnabled)
  const notEnough = balance !== undefined && cost > balance

  const toggleImage = () => {
    if (imageEnabled && !songEnabled) return
    setImageEnabled(value => !value)
  }

  const toggleSong = () => {
    if (songEnabled && !imageEnabled) return
    setSongEnabled(value => !value)
  }

  const handleFiles = useCallback((picked: FileList | null) => {
    if (!picked) return
    const next = [...files]
    let nextImageCount = next.filter(file => file.type === 'image').length
    let changed = false

    for (const file of Array.from(picked)) {
      if (!file.type.startsWith('image/')) continue
      const preview = URL.createObjectURL(file)
      if (nextImageCount < 3) {
        next.push({ file, preview, type: 'image' })
        nextImageCount += 1
        changed = true
      } else {
        URL.revokeObjectURL(preview)
        setError(t('maxThreePhotos'))
      }
    }

    if (changed) {
      setError('')
      setFiles(next)
    }
  }, [files, t])

  const removeFile = (index: number) => {
    setFiles(prev => {
      const next = [...prev]
      if (next[index]?.preview) URL.revokeObjectURL(next[index].preview!)
      next.splice(index, 1)
      return next
    })
  }

  const generateLyrics = async () => {
    const effectivePrompt = lyricsPrompt.trim() || prompt.trim()
    if (!effectivePrompt || generatingLyrics) return

    setGeneratingLyrics(true)
    setError('')
    try {
      const data = await api.generations.lyrics(effectivePrompt, 3)
      const variants = data.variants?.length
        ? data.variants
        : [{ title: data.title ?? `${t('variant')} 1`, text: data.text ?? '' }]
      setLyricsVariants(variants)
      setSelectedLyricsIndex(0)
      setSongLyrics(variants[0]?.text ?? '')
    } catch (e: unknown) {
      setError(errorMessage(e, t('textGenerationError')))
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
    form.append('image_prompt', imageEnabled ? composeImagePrompt(prompt, imagePresetId) : '')
    form.append('image_model', imageModel)
    form.append('song_prompt', lyricsPrompt.trim() || prompt.trim())
    form.append('song_lyrics', songLyrics)
    form.append('song_style', songStyle)
    form.append('image_count', String(imageCount))
    form.append('song_count', String(songCount))

    for (const file of files) form.append('photos', file.file)

    try {
      const result = await api.generations.create(form)
      setPrompt('')
      setSongLyrics('')
      setSongStyle('')
      setLyricsPrompt('')
      setLyricsVariants([])
      setSelectedLyricsIndex(0)
      setFiles([])
      setImageModelOpen(false)
      setFilterModalOpen(false)
      setLyricsModalOpen(false)
      qc.invalidateQueries({ queryKey: ['sessions'] })
      qc.invalidateQueries({ queryKey: ['balance'] })
      onSent(result.id, result.session_id)
    } catch (e: unknown) {
      if (e instanceof ApiError && e.code === 'insufficient_credits') {
        onInsufficientCredits?.()
      } else {
        setError(errorMessage(e, t('sendError')))
      }
    } finally {
      setSending(false)
    }
  }

  const truncateName = (name: string, max = 18) => (
    name.length > max ? `${name.slice(0, max - 1)}...` : name
  )

  const imageFiles = files.filter(file => file.type === 'image')
  const categoryPresets = imagePromptPresets.filter(preset => preset.category === presetCategory)
  const selectedPreset = imagePromptPresets.find(preset => preset.id === imagePresetId)
  const songSummary = songLyrics.trim()
    ? songLyrics.trim().replace(/\s+/g, ' ').slice(0, 100)
    : t('noSongTextSelected')

  const updateSelectedLyrics = (text: string) => {
    setSongLyrics(text)
    setLyricsVariants(prev => prev.map((variant, index) => (
      index === selectedLyricsIndex ? { ...variant, text } : variant
    )))
  }

  const selectLyricsVariant = (index: number) => {
    setSelectedLyricsIndex(index)
    setSongLyrics(lyricsVariants[index]?.text ?? '')
  }

  const clearSongState = () => {
    setSongLyrics('')
    setLyricsVariants([])
    setSelectedLyricsIndex(0)
  }

  const costLabel = t('creditsShort')

  return (
    <>
      <div style={{ padding: '14px 24px 20px', background: 'var(--surface)' }}>
        <div style={{ maxWidth: 720, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 10 }}>
          {error && (
            <div style={{ padding: '8px 14px', background: 'rgba(239,68,68,0.08)', borderRadius: 10, fontSize: 13, color: 'var(--error)' }}>
              {error}
            </div>
          )}

          <SectionBlock
            icon={<ImageIcon size={14} />}
            label={t('imageSection')}
            enabled={imageEnabled}
            onToggle={toggleImage}
            disableToggle={imageEnabled && !songEnabled}
          >
            <Textarea
              value={prompt}
              onChange={event => setPrompt(event.target.value)}
              onKeyDown={event => {
                if (event.key === 'Enter' && !event.shiftKey) {
                  event.preventDefault()
                  send()
                }
              }}
              placeholder={t('promptPlaceholder')}
              rows={2}
              disabled={sending}
            />

            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 10, flexWrap: 'wrap' }}>
              <div style={{ position: 'relative', zIndex: imageModelOpen ? 120 : 1 }}>
                <button type="button" onClick={() => setImageModelOpen(value => !value)} style={ghostBtnStyle}>
                  <span style={{ fontSize: 12, color: 'var(--text)' }}>{t('model')}: {imageModelLabel(imageModel)}</span>
                  <ChevronDown size={14} style={{ color: 'var(--text-muted)' }} />
                </button>
                {imageModelOpen && (
                  <div style={{
                    position: 'absolute',
                    bottom: 'calc(100% + 8px)',
                    left: 0,
                    zIndex: 120,
                    minWidth: 220,
                    background: 'var(--surface)',
                    border: '1px solid var(--border)',
                    borderRadius: 10,
                    boxShadow: '0 16px 40px rgba(0,0,0,0.26)',
                    padding: 8,
                    overflow: 'hidden',
                    isolation: 'isolate',
                  }}>
                    {imageModels.map(option => (
                      <button
                        key={option.value}
                        type="button"
                        onClick={() => {
                          setImageModel(option.value)
                          setImageModelOpen(false)
                        }}
                        style={{
                          width: '100%',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          gap: 10,
                          padding: '10px 12px',
                          borderRadius: 10,
                          border: 'none',
                          background: imageModel === option.value ? 'rgba(var(--primary-rgb),0.08)' : 'transparent',
                          cursor: 'pointer',
                          color: 'var(--text)',
                          textAlign: 'left',
                        }}
                      >
                        <span>
                          <span style={{ display: 'block', fontSize: 13, fontWeight: 600 }}>{option.hint}</span>
                          <span style={{ display: 'block', fontSize: 11, color: 'var(--text-muted)' }}>{option.label}</span>
                        </span>
                        {imageModel === option.value && <Check size={14} style={{ color: 'var(--primary)' }} />}
                      </button>
                    ))}
                  </div>
                )}
              </div>

              <input
                ref={imageInputRef}
                type="file"
                accept="image/*"
                multiple
                style={{ display: 'none' }}
                onChange={event => {
                  handleFiles(event.target.files)
                  event.currentTarget.value = ''
                }}
              />
              <button type="button" onClick={() => imageInputRef.current?.click()} title={t('attachPhoto')} style={ghostBtnStyle}>
                <Paperclip size={14} style={{ color: 'var(--text-muted)' }} />
                <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{t('photo')}</span>
              </button>
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{imageFiles.length}/3</span>
              <button
                type="button"
                onClick={() => setFilterModalOpen(true)}
                style={{
                  ...ghostBtnStyle,
                  border: '1px solid var(--border)',
                  background: imagePresetId === noImagePresetId ? 'var(--surface)' : 'rgba(var(--primary-rgb),0.08)',
                  color: 'var(--text)',
                  padding: '7px 10px',
                }}
              >
                <Sparkles size={14} style={{ color: 'var(--primary)' }} />
                <span style={{ fontSize: 12, color: 'var(--text)' }}>
                  {imagePresetId === noImagePresetId
                    ? `${t('filterLabel')}: ${t('noFilter')}`
                    : `${t('filterLabel')}: ${presetLabel(selectedPreset?.id ?? '', selectedPreset?.label ?? '', t)}`}
                </span>
              </button>
            </div>

            {imageFiles.length > 0 && (
              <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginTop: 10 }}>
                {imageFiles.map(file => {
                  const fileIndex = files.indexOf(file)
                  return (
                    <div key={`${file.file.name}-${fileIndex}`} style={{ position: 'relative', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4, maxWidth: 72 }}>
                      {file.preview ? <img src={file.preview} alt="" style={{ width: 64, height: 64, borderRadius: 10, objectFit: 'cover' }} /> : null}
                      <span style={{ fontSize: 10, color: 'var(--text-muted)', textAlign: 'center', maxWidth: 72, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {truncateName(file.file.name)}
                      </span>
                      <button
                        type="button"
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
          </SectionBlock>

          <SectionBlock
            icon={<Music size={14} />}
            label={t('songSection')}
            enabled={songEnabled}
            onToggle={toggleSong}
            disableToggle={songEnabled && !imageEnabled}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setLyricsModalOpen(true)}
                style={{ border: '1px solid var(--border)', color: 'var(--text)', background: 'var(--surface)' }}
              >
                <Music size={14} />
                {t('songText')}
              </Button>
              <input
                value={songStyle}
                onChange={event => setSongStyle(event.target.value)}
                placeholder={t('songStylePlaceholder')}
                style={{ ...inputStyle, minWidth: 220, flex: '1 1 240px' }}
              />
              {songLyrics.trim() && (
                <button type="button" onClick={clearSongState} style={{ ...ghostBtnStyle, padding: '6px 8px' }}>
                  <X size={14} style={{ color: 'var(--text-muted)' }} />
                  <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{t('clear')}</span>
                </button>
              )}
            </div>

            <div style={{
              marginTop: 10,
              padding: '10px 12px',
              borderRadius: 10,
              background: 'var(--surface)',
              border: '1px solid var(--border)',
            }}>
              <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 4 }}>
                {songLyrics.trim() ? t('selectedSongText') : t('noSongTextSelected')}
              </div>
              <div style={{ fontSize: 13, color: songLyrics.trim() ? 'var(--text)' : 'var(--text-muted)', lineHeight: 1.5 }}>
                {songSummary}
              </div>
            </div>
          </SectionBlock>

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 12 }}>
            <span style={{ fontSize: 12, color: notEnough ? 'var(--error)' : 'var(--text-muted)' }}>
              {cost > 0 ? `${cost} ${costLabel}` : ''}
              {notEnough ? ` - ${t('insufficient')}` : ''}
            </span>
            <Button onClick={send} disabled={!canSend || notEnough} loading={sending} size="sm" style={{ gap: 6 }}>
              <Send size={14} />
              {t('send')}
            </Button>
          </div>
        </div>
      </div>

      <SimpleModal open={filterModalOpen} onClose={() => setFilterModalOpen(false)} title={t('imageFilters')}>
        <div className="image-filter-panel image-filter-panel--modal">
          <div className="image-filter-panel__head">
            <Sparkles size={14} />
            <span>{t('imageFilterTheme')}</span>
          </div>

          <div className="image-filter-categories" role="tablist" aria-label="Image filter categories">
            {imagePromptCategories.map(category => {
              const active = presetCategory === category
              return (
                <button
                  key={category}
                  type="button"
                  className={`image-filter-category${active ? ' image-filter-category--active' : ''}`}
                  onClick={() => {
                    setPresetCategory(category)
                    setImagePresetId(noImagePresetId)
                  }}
                >
                  {categoryLabel(category, t)}
                </button>
              )
            })}
          </div>

          <div className="image-filter-grid">
            <button
              type="button"
              className={`image-filter-card${imagePresetId === noImagePresetId ? ' image-filter-card--active' : ''}`}
              onClick={() => setImagePresetId(noImagePresetId)}
            >
              <span className="image-filter-card__title">{t('noFilter')}</span>
              <span className="image-filter-card__meta">{t('onlyPrompt')}</span>
            </button>

            {categoryPresets.map(preset => {
              const active = imagePresetId === preset.id
              return (
                <button
                  key={preset.id}
                  type="button"
                  className={`image-filter-card${active ? ' image-filter-card--active' : ''}`}
                  onClick={() => setImagePresetId(preset.id)}
                >
                  <span className="image-filter-card__title">{presetLabel(preset.id, preset.label, t)}</span>
                  <span className="image-filter-card__meta">
                    {[preset.aspectRatio, ...(preset.bestFor ?? []).slice(0, 2)].filter(Boolean).join(' / ')}
                  </span>
                </button>
              )
            })}
          </div>

          {selectedPreset && (
            <div className="image-filter-selected">
              <Check size={13} />
              <span>
                {presetLabel(selectedPreset.id, selectedPreset.label, t)}
                {selectedPreset.aspectRatio ? ` / ${selectedPreset.aspectRatio}` : ''}
                {selectedPreset.bestFor?.length ? ` / ${selectedPreset.bestFor.join(', ')}` : ''}
              </span>
            </div>
          )}
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 14 }}>
          <Button size="sm" onClick={() => setFilterModalOpen(false)}>{t('done')}</Button>
        </div>
      </SimpleModal>

      <SimpleModal open={lyricsModalOpen} onClose={() => setLyricsModalOpen(false)} title={t('songText')}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <div style={fieldLabelStyle}>{t('lyricsPromptLabel')}</div>
            <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) auto', gap: 10, alignItems: 'start' }}>
              <Textarea
                value={lyricsPrompt}
                onChange={event => setLyricsPrompt(event.target.value)}
                placeholder={t('lyricsPromptPlaceholder')}
                rows={3}
                style={{ minHeight: 88 }}
                disabled={generatingLyrics}
              />
              <Button
                onClick={generateLyrics}
                disabled={generatingLyrics || (!lyricsPrompt.trim() && !prompt.trim())}
                size="sm"
                style={{ gap: 6, minWidth: 190, alignSelf: 'flex-start' }}
              >
                {generatingLyrics ? <Loader2 size={14} style={{ animation: 'spin 1s linear infinite' }} /> : <Sparkles size={14} />}
                {generatingLyrics ? t('generatingLyrics') : t('generateThreeVariants')}
              </Button>
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 8 }}>
              {tariff?.price_per_lyrics
                ? `${t('lyricsGenerationCost')}: ${tariff.price_per_lyrics} ${costLabel}`
                : t('generatingThreeVariants')}
            </div>
          </div>

          {lyricsVariants.length > 0 && (
            <>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 10 }}>
                {lyricsVariants.map((variant, index) => (
                  <button
                    key={`${variant.title}-${index}`}
                    type="button"
                    onClick={() => selectLyricsVariant(index)}
                    style={{
                      width: '100%',
                      borderRadius: 12,
                      border: selectedLyricsIndex === index ? '1px solid rgba(var(--primary-rgb),0.4)' : '1px solid var(--border)',
                      background: selectedLyricsIndex === index ? 'rgba(var(--primary-rgb),0.06)' : 'var(--surface)',
                      padding: '12px 14px',
                      cursor: 'pointer',
                      textAlign: 'left',
                      color: 'var(--text)',
                      minHeight: 220,
                      display: 'flex',
                      flexDirection: 'column',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginBottom: 8 }}>
                      <div style={{ fontSize: 13, fontWeight: 600 }}>{variant.title || `${t('variant')} ${index + 1}`}</div>
                      {selectedLyricsIndex === index && <Check size={14} style={{ color: 'var(--primary)' }} />}
                    </div>
                    <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.5, whiteSpace: 'pre-wrap' }}>
                      {variant.text}
                    </div>
                  </button>
                ))}
              </div>

              <div>
                <div style={fieldLabelStyle}>{t('editSelectedLyrics')}</div>
                <Textarea
                  value={songLyrics}
                  onChange={event => updateSelectedLyrics(event.target.value)}
                  placeholder={t('editSelectedLyricsPlaceholder')}
                  rows={10}
                  style={{ minHeight: 260 }}
                />
              </div>
            </>
          )}

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
            <Button variant="ghost" size="sm" onClick={() => setLyricsModalOpen(false)}>{t('close')}</Button>
            <Button size="sm" onClick={() => setLyricsModalOpen(false)} disabled={!songLyrics.trim()}>{t('chooseLyrics')}</Button>
          </div>
        </div>
      </SimpleModal>
    </>
  )
}

interface SectionBlockProps {
  icon: React.ReactNode
  label: string
  enabled: boolean
  onToggle: () => void
  disableToggle: boolean
  children: React.ReactNode
}

function SectionBlock({ icon, label, enabled, onToggle, disableToggle, children }: SectionBlockProps) {
  return (
    <div style={{
      border: `1.5px solid ${enabled ? 'rgba(var(--primary-rgb),0.3)' : 'var(--border)'}`,
      borderRadius: 14,
      overflow: enabled ? 'visible' : 'hidden',
      transition: 'border-color 0.2s',
      background: 'var(--bg)',
      position: 'relative',
    }}>
      <div
        style={{
          display: 'flex', alignItems: 'center', gap: 8,
          padding: '10px 14px',
          background: enabled ? 'rgba(var(--primary-rgb),0.04)' : 'var(--surface2)',
          borderBottom: enabled ? '1px solid rgba(var(--primary-rgb),0.1)' : '1px solid transparent',
          cursor: disableToggle ? 'not-allowed' : 'pointer',
        }}
        onClick={() => {
          if (!disableToggle) onToggle()
        }}
      >
        <span style={{ color: enabled ? 'var(--primary)' : 'var(--text-muted)', display: 'flex' }}>{icon}</span>
        <span style={{ fontSize: 13, fontWeight: 600, color: enabled ? 'var(--text)' : 'var(--text-muted)' }}>{label}</span>
        <div style={{ flex: 1 }} />
        <button
          type="button"
          onClick={event => {
            event.stopPropagation()
            onToggle()
          }}
          disabled={disableToggle}
          style={{
            position: 'relative', width: 36, height: 20, borderRadius: 10,
            background: enabled ? 'var(--primary)' : 'var(--border)',
            border: 'none', cursor: disableToggle ? 'not-allowed' : 'pointer',
            opacity: disableToggle ? 0.4 : 1,
            flexShrink: 0,
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

      <div style={{
        padding: enabled ? '12px 14px' : '0 14px',
        maxHeight: enabled ? 920 : 0,
        overflow: enabled ? 'visible' : 'hidden',
        transition: 'max-height 0.25s ease, padding 0.25s ease',
      }}>
        {children}
      </div>
    </div>
  )
}

interface SimpleModalProps {
  open: boolean
  onClose: () => void
  title: string
  children: React.ReactNode
}

function SimpleModal({ open, onClose, title, children }: SimpleModalProps) {
  if (!open) return null

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 100,
        background: 'rgba(8,6,13,0.48)',
        backdropFilter: 'blur(6px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 20,
      }}
    >
      <div
        onClick={event => event.stopPropagation()}
        style={{
          width: 'min(760px, 100%)',
          maxHeight: '85vh',
          overflowY: 'auto',
          background: 'var(--bg)',
          border: '1px solid var(--border)',
          borderRadius: 18,
          boxShadow: '0 20px 60px rgba(0,0,0,0.18)',
        }}
      >
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 12,
          padding: '18px 20px 14px',
          borderBottom: '1px solid var(--border)',
        }}>
          <div style={{ fontSize: 18, fontWeight: 700 }}>{title}</div>
          <button type="button" onClick={onClose} style={ghostBtnStyle}>
            <X size={16} style={{ color: 'var(--text-muted)' }} />
          </button>
        </div>
        <div style={{ padding: 20 }}>
          {children}
        </div>
      </div>
    </div>
  )
}

const fieldLabelStyle: React.CSSProperties = {
  fontSize: 12,
  color: 'var(--text-muted)',
  marginBottom: 6,
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
  display: 'inline-flex',
  alignItems: 'center',
  gap: 5,
  background: 'none',
  border: 'none',
  cursor: 'pointer',
  padding: '4px 8px',
  borderRadius: 7,
}
