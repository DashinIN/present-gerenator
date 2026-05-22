import {
  ArrowRight,
  Check,
  ChevronDown,
  Clock,
  Crown,
  Download,
  Image as ImageIcon,
  Languages,
  Moon,
  Music,
  Play,
  Sparkles,
  Sun,
  Upload,
  User,
  WandSparkles,
} from 'lucide-react'
import { useEffect } from 'react'
import type { MouseEvent } from 'react'
import { Button } from '@/components/ui/Button'
import { AppLogo } from '@/components/AppLogo'
import { LANGUAGES, useI18n } from '@/lib/i18n'
import { useTheme } from '@/lib/theme'
import './LoginPage.css'

const waveBars = Array.from({ length: 32 }, (_, i) => i)

const steps = [
  ['01', 'landingStep1Title', 'landingStep1Text'],
  ['02', 'landingStep2Title', 'landingStep2Text'],
  ['03', 'landingStep3Title', 'landingStep3Text'],
] as const

const features = [
  [User, 'landingFeaturePersonalTitle', 'landingFeaturePersonalText'],
  [Music, 'landingFeatureSongTitle', 'landingFeatureSongText'],
  [ImageIcon, 'landingFeatureImageTitle', 'landingFeatureImageText'],
  [Crown, 'landingFeatureBrandTitle', 'landingFeatureBrandText'],
] as const

function startLogin() {
  window.location.href = '/api/auth/google/login'
}

function Waveform({ compact = false }: { compact?: boolean }) {
  return (
    <div className={compact ? 'landing-wave landing-wave--compact' : 'landing-wave'} aria-hidden="true">
      {waveBars.map((bar) => <span key={bar} />)}
    </div>
  )
}

function BrandMark() {
  return (
    <div className="landing-brand" aria-label="FunBaza">
      <AppLogo size={36} />
      <span>FunBaza</span>
    </div>
  )
}

function LanguageSwitch() {
  const { language, resolvedLanguage, setLanguage, t } = useI18n()
  const selectedLanguage = LANGUAGES.find(option => option.code === language) ?? LANGUAGES[0]
  const resolved = LANGUAGES.find(option => option.code === resolvedLanguage)
  const chooseLanguage = (event: MouseEvent<HTMLButtonElement>, nextLanguage: typeof language) => {
    setLanguage(nextLanguage)
    event.currentTarget.closest('details')?.removeAttribute('open')
  }

  return (
    <details className="landing-language">
      <summary aria-label={t('language')}>
        <Languages size={14} />
        <span>
          {selectedLanguage.code === 'auto'
            ? `${t('auto')} \u00b7 ${resolved?.nativeName ?? resolvedLanguage.toUpperCase()}`
            : selectedLanguage.nativeName}
        </span>
        <ChevronDown size={14} />
      </summary>

      <div className="landing-language__menu">
        <div className="language-picker" aria-label={t('language')}>
          <div className="language-picker__current">
            <div className="language-picker__current-icon">
              <Languages size={14} />
            </div>
            <div className="language-picker__current-copy">
              <div className="language-picker__current-name">
                {selectedLanguage.code === 'auto' ? t('auto') : selectedLanguage.nativeName}
              </div>
              <div className="language-picker__current-meta">
                {selectedLanguage.code === 'auto' ? t('languageHint') : selectedLanguage.label}
              </div>
            </div>
            <ChevronDown size={14} className="language-picker__current-chevron" />
          </div>

          <div className="language-picker__list" role="listbox" aria-label={t('language')}>
            {LANGUAGES.map(option => {
              const active = option.code === language
              return (
                <button
                  key={option.code}
                  type="button"
                  role="option"
                  aria-selected={active}
                  className={`language-option${active ? ' language-option--active' : ''}`}
                  onClick={(event) => chooseLanguage(event, option.code)}
                >
                  <span className="language-option__copy">
                    <span className="language-option__name">
                      {option.code === 'auto' ? t('auto') : option.nativeName}
                    </span>
                    <span className="language-option__meta">
                      {option.code === 'auto' ? t('languageHint') : option.label}
                    </span>
                  </span>
                  {active && <Check size={14} className="language-option__check" />}
                </button>
              )
            })}
          </div>
        </div>
      </div>
    </details>
  )
}

function ThemeToggle() {
  const { theme, setTheme, setAccent } = useTheme()
  const { t } = useI18n()
  const isLight = theme === 'light'

  const setLandingTheme = (nextTheme: 'light' | 'dark') => {
    setTheme(nextTheme)
    setAccent(nextTheme === 'light' ? '#0f66ff' : '#6aa8ff')
  }

  return (
    <div className="landing-theme-toggle" aria-label={t('theme')}>
      <button
        type="button"
        className={isLight ? 'is-active' : ''}
        onClick={() => setLandingTheme('light')}
        aria-label={t('themeLight')}
      >
        <Sun size={16} />
      </button>
      <button
        type="button"
        className={!isLight ? 'is-active' : ''}
        onClick={() => setLandingTheme('dark')}
        aria-label={t('themeDark')}
      >
        <Moon size={16} />
      </button>
    </div>
  )
}

function DemoBoard() {
  const { t } = useI18n()

  return (
    <div className="landing-demo" aria-label={t('landingDemoAria')}>
      <div className="landing-panel landing-panel--nav">
        <div className="panel-top">
          <AppLogo size={24} />
          <strong>FunBaza</strong>
        </div>
        <button type="button" className="panel-primary"><Sparkles size={14} /> {t('landingDemoNewProject')}</button>
        <span>{t('sessions')}</span>
        <span>{t('landingDemoTemplates')}</span>
        <span>{t('landingDemoFavorites')}</span>
      </div>

      <div className="landing-panel landing-panel--idea">
        <div className="panel-title">{t('landingDemoChooseIdea')}</div>
        <div className="idea-grid">
          <div>
            <Music size={24} />
            <strong>{t('createModeSongTitle')}</strong>
            <Waveform compact />
          </div>
          <div>
            <ImageIcon size={24} />
            <strong>{t('createModeImageTitle')}</strong>
            <div className="mini-image" />
          </div>
        </div>
        <label>{t('landingDemoDescribeIdea')}</label>
        <div className="fake-input">{t('landingDemoPrompt')}</div>
        <button type="button">{t('continue')} <ArrowRight size={14} /></button>
      </div>

      <div className="landing-panel landing-panel--lyrics">
        <div className="panel-title">
          <span><WandSparkles size={16} /> {t('landingDemoLyricsTitle')}</span>
          <button type="button">{t('landingDemoGenerate')}</button>
        </div>
        <div className="lyrics-grid">
          <article>
            <b>{t('landingDemoVerse1')}</b>
            <p>{t('landingDemoVerse1Text')}</p>
          </article>
          <article>
            <b>{t('landingDemoChorus')}</b>
            <p>{t('landingDemoChorusText')}</p>
          </article>
          <article>
            <b>{t('landingDemoVerse2')}</b>
            <p>{t('landingDemoVerse2Text')}</p>
          </article>
        </div>
      </div>

      <div className="landing-panel landing-panel--result">
        <div className="panel-title">{t('landingDemoReady')}</div>
        <div className="result-art">
          <div className="city" />
          <div className="car" />
          <div className="figure figure--one" />
          <div className="figure figure--two" />
          <div className="figure figure--three" />
        </div>
        <div className="result-player">
          <button type="button" aria-label={t('playAudio')}><Play size={15} fill="currentColor" /></button>
          <span>0:17 / 1:50</span>
          <Waveform compact />
        </div>
        <div className="result-actions">
          <button type="button"><Download size={14} /> {t('download')}</button>
          <button type="button"><Upload size={14} /> {t('landingShare')}</button>
        </div>
      </div>
    </div>
  )
}

export function LoginPage() {
  const { theme, setTheme, setAccent } = useTheme()
  const { t } = useI18n()

  useEffect(() => {
    if (!localStorage.getItem('theme')) {
      setTheme('light')
      setAccent('#0f66ff')
    }
  }, [setAccent, setTheme])

  return (
    <main className="landing-page" data-landing-theme={theme}>
      <div className="landing-frame" />
      <header className="landing-header">
        <BrandMark />
        <nav aria-label={t('landingNavAria')}>
          <a href="#how">{t('landingNavHow')}</a>
          <a href="#features">{t('landingNavFeatures')}</a>
          <a href="#use-cases">{t('landingNavCases')}</a>
        </nav>
        <div className="landing-header-actions">
          <LanguageSwitch />
          <ThemeToggle />
          <Button
            className="landing-login-button"
            variant="ghost"
            onClick={startLogin}
          >
            <User size={16} />
            {t('landingLogin')}
          </Button>
          <Button onClick={startLogin}>
            {t('landingCreate')}
            <Sparkles size={16} />
          </Button>
        </div>
      </header>

      <section className="landing-hero">
        <div className="landing-serpent" aria-hidden="true" />
        <div className="landing-hero-copy">
          <div className="landing-eyebrow"><Sparkles size={14} /> {t('landingEyebrow')}</div>
          <h1>{t('landingHeroTitle')}</h1>
          <p>{t('landingHeroText')}</p>
          <div className="landing-hero-actions">
            <Button onClick={startLogin} style={{ minHeight: 48, paddingInline: 22 }}>
              {t('landingCreate')}
              <Sparkles size={17} />
            </Button>
            <a className="landing-demo-link" href="#how">
              <Play size={16} fill="currentColor" />
              {t('landingWatchProcess')}
            </a>
          </div>
        </div>
        <DemoBoard />
      </section>

      <section className="landing-strip" aria-label={t('landingCoreFeaturesAria')}>
        <article>
          <Music size={28} />
          <div><strong>{t('createModeSongTitle')}</strong><span>{t('landingStripSong')}</span></div>
        </article>
        <article>
          <ImageIcon size={28} />
          <div><strong>{t('createModeImageTitle')}</strong><span>{t('landingStripImage')}</span></div>
        </article>
        <article>
          <Clock size={28} />
          <div><strong>{t('landingStripFastTitle')}</strong><span>{t('landingStripFastText')}</span></div>
        </article>
        <article>
          <Crown size={28} />
          <div><strong>{t('landingStripUseTitle')}</strong><span>{t('landingStripUseText')}</span></div>
        </article>
      </section>

      <section className="landing-section" id="how">
        <div className="landing-section-title">
          <h2>{t('landingHowTitle')}</h2>
          <Sparkles size={18} />
        </div>
        <div className="landing-steps">
          {steps.map(([num, titleKey, textKey]) => (
            <article key={num}>
              <span>{num}</span>
              <div>
                <h3>{t(titleKey)}</h3>
                <p>{t(textKey)}</p>
              </div>
            </article>
          ))}
        </div>
      </section>

      <section className="landing-section" id="features">
        <div className="landing-section-title">
          <h2>{t('landingWhyTitle')}</h2>
          <Sparkles size={18} />
        </div>
        <div className="landing-features">
          {features.map(([Icon, titleKey, textKey]) => (
            <article key={titleKey}>
              <Icon size={26} />
              <h3>{t(titleKey)}</h3>
              <p>{t(textKey)}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="landing-showcase" id="use-cases">
        <div>
          <h2>{t('landingShowcaseTitle')}</h2>
          <p>{t('landingShowcaseText')}</p>
        </div>
        <div className="showcase-list">
          <span><Check size={16} /> {t('landingUseCase1')}</span>
          <span><Check size={16} /> {t('landingUseCase2')}</span>
          <span><Check size={16} /> {t('landingUseCase3')}</span>
        </div>
      </section>

      <footer className="landing-footer">
        <BrandMark />
        <span>&copy; 2026 FunBaza</span>
        <button type="button" onClick={startLogin}>{t('landingStartProject')} <ArrowRight size={16} /></button>
      </footer>
    </main>
  )
}
