import { useId, useRef, useState } from 'react'
import type { ChangeEvent } from 'react'
import { Download, Pause, Play, Volume2 } from 'lucide-react'
import { useI18n } from '@/lib/i18n'

interface AudioPlayerProps {
  src: string
  label?: string
}

function formatTime(value: number) {
  if (!Number.isFinite(value) || value < 0) return '0:00'

  const totalSeconds = Math.floor(value)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60

  return `${minutes}:${String(seconds).padStart(2, '0')}`
}

export function AudioPlayer({ src, label }: AudioPlayerProps) {
  const { t } = useI18n()
  const audioRef = useRef<HTMLAudioElement>(null)
  const timelineId = useId()
  const [isPlaying, setIsPlaying] = useState(false)
  const [duration, setDuration] = useState(0)
  const [currentTime, setCurrentTime] = useState(0)

  const progress = duration > 0 ? (currentTime / duration) * 100 : 0

  const togglePlayback = async () => {
    const audio = audioRef.current
    if (!audio) return

    if (audio.paused) {
      try {
        await audio.play()
      } catch {
        setIsPlaying(false)
      }
      return
    }

    audio.pause()
  }

  const handleSeek = (event: ChangeEvent<HTMLInputElement>) => {
    const audio = audioRef.current
    const nextTime = Number(event.target.value)

    setCurrentTime(nextTime)
    if (audio) audio.currentTime = nextTime
  }

  return (
    <div className={`audio-player${isPlaying ? ' audio-player--playing' : ''}`}>
      <audio
        ref={audioRef}
        preload="metadata"
        src={src}
        onPlay={() => setIsPlaying(true)}
        onPause={() => setIsPlaying(false)}
        onEnded={() => {
          setIsPlaying(false)
          setCurrentTime(0)
        }}
        onTimeUpdate={event => setCurrentTime(event.currentTarget.currentTime)}
        onLoadedMetadata={event => {
          setDuration(event.currentTarget.duration || 0)
          setCurrentTime(event.currentTarget.currentTime || 0)
        }}
        onDurationChange={event => setDuration(event.currentTarget.duration || 0)}
      />

      <button
        type="button"
        className="audio-player__play"
        onClick={togglePlayback}
        aria-label={isPlaying ? t('pauseAudio') : t('playAudio')}
      >
        {isPlaying ? <Pause size={18} /> : <Play size={18} />}
      </button>

      <div className="audio-player__body">
        <div className="audio-player__topline">
          <div className="audio-player__meta">
            <span className="audio-player__eyebrow">
              <Volume2 size={13} />
              {t('songSection')}
            </span>
            <span className="audio-player__label">{label ?? t('generatedTrack')}</span>
          </div>

          <a
            href={src}
            download
            className="audio-player__download"
            title={t('download')}
            aria-label={t('download')}
          >
            <Download size={15} />
          </a>
        </div>

        <label className="audio-player__timeline" htmlFor={timelineId}>
          <span className="audio-player__time">{formatTime(currentTime)}</span>
          <input
            id={timelineId}
            className="audio-player__range"
            type="range"
            min={0}
            max={duration || 0}
            step={0.1}
            value={Math.min(currentTime, duration || 0)}
            onChange={handleSeek}
            aria-label={t('audioProgress')}
            style={{ ['--audio-progress' as string]: `${progress}%` }}
          />
          <span className="audio-player__time">{formatTime(duration)}</span>
        </label>

        <div className="audio-player__visualizer" aria-hidden="true">
          {Array.from({ length: 20 }).map((_, index) => (
            <span key={index} style={{ animationDelay: `${index * 70}ms` }} />
          ))}
        </div>
      </div>
    </div>
  )
}
