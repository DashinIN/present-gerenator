/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'

export type LanguageCode =
  | 'auto'
  | 'ru'
  | 'en'
  | 'es'
  | 'pt'
  | 'fr'
  | 'de'
  | 'it'
  | 'tr'
  | 'ar'
  | 'hi'
  | 'zh'
  | 'ja'
  | 'ko'
  | 'uk'
  | 'be'
  | 'kk'
  | 'uz'
  | 'az'
  | 'hy'
  | 'ka'
  | 'ky'
  | 'tg'
  | 'ro'

export const LANGUAGES: Array<{ code: LanguageCode; label: string; nativeName: string }> = [
  { code: 'auto', label: 'Auto', nativeName: 'Auto' },
  { code: 'ru', label: 'Russian', nativeName: '\u0420\u0443\u0441\u0441\u043a\u0438\u0439' },
  { code: 'en', label: 'English', nativeName: 'English' },
  { code: 'es', label: 'Spanish', nativeName: 'Espa\u00f1ol' },
  { code: 'pt', label: 'Portuguese', nativeName: 'Portugu\u00eas' },
  { code: 'fr', label: 'French', nativeName: 'Fran\u00e7ais' },
  { code: 'de', label: 'German', nativeName: 'Deutsch' },
  { code: 'it', label: 'Italian', nativeName: 'Italiano' },
  { code: 'tr', label: 'Turkish', nativeName: 'T\u00fcrk\u00e7e' },
  { code: 'ar', label: 'Arabic', nativeName: '\u0627\u0644\u0639\u0631\u0628\u064a\u0629' },
  { code: 'hi', label: 'Hindi', nativeName: '\u0939\u093f\u0928\u094d\u0926\u0940' },
  { code: 'zh', label: 'Chinese', nativeName: '\u4e2d\u6587' },
  { code: 'ja', label: 'Japanese', nativeName: '\u65e5\u672c\u8a9e' },
  { code: 'ko', label: 'Korean', nativeName: '\ud55c\uad6d\uc5b4' },
  { code: 'uk', label: 'Ukrainian', nativeName: '\u0423\u043a\u0440\u0430\u0457\u043d\u0441\u044c\u043a\u0430' },
  { code: 'be', label: 'Belarusian', nativeName: '\u0411\u0435\u043b\u0430\u0440\u0443\u0441\u043a\u0430\u044f' },
  { code: 'kk', label: 'Kazakh', nativeName: '\u049a\u0430\u0437\u0430\u049b\u0448\u0430' },
  { code: 'uz', label: 'Uzbek', nativeName: 'O\u02bbzbekcha' },
  { code: 'az', label: 'Azerbaijani', nativeName: 'Az\u0259rbaycanca' },
  { code: 'hy', label: 'Armenian', nativeName: '\u0540\u0561\u0575\u0565\u0580\u0565\u0576' },
  { code: 'ka', label: 'Georgian', nativeName: '\u10e5\u10d0\u10e0\u10d7\u10e3\u10da\u10d8' },
  { code: 'ky', label: 'Kyrgyz', nativeName: '\u041a\u044b\u0440\u0433\u044b\u0437\u0447\u0430' },
  { code: 'tg', label: 'Tajik', nativeName: '\u0422\u043e\u04b7\u0438\u043a\u04e3' },
  { code: 'ro', label: 'Romanian', nativeName: 'Rom\u00e2n\u0103' },
]

type Messages = Record<string, string>

const DEFAULT_MESSAGES: Messages = {
  language: 'Language',
  languageHint: 'App interface',
  settings: 'Settings',
  appearance: 'Appearance',
  theme: 'Theme',
  themeDark: 'Dark',
  themeLight: 'Light',
  accent: 'Accent',
  auto: 'Auto',
  loginSubtitle: 'Personal AI greetings: images and music in minutes',
  loginGoogle: 'Continue with Google',
  newGreeting: 'New greeting',
  sessions: 'History',
  noSessions: 'No greetings yet',
  balance: 'Balance',
  logout: 'Log out',
  rename: 'Rename',
  untitled: 'Untitled',
  hideSidebar: 'Hide sidebar',
  showSidebar: 'Show sidebar',
  greetingCount_one: 'greeting',
  greetingCount_many: 'greetings',
  generating: 'generating...',
  describeGreeting: 'Describe what you want to create',
  noCredits: 'You are out of credits. Top up your balance to continue.',
  createFirstGreeting: 'Create your first greeting',
  createModalTitle: 'Create a greeting',
  createModalSubtitle: 'Choose what to generate first',
  createModeSongTitle: 'Song',
  createModeSongDesc: 'Generate lyrics and music for your greeting.',
  createModeImageTitle: 'Image',
  createModeImageDesc: 'Create a unique image in the style you need.',
  createModeHint: 'You will describe the idea on the next step.',
  continue: 'Continue',
  addPromptAndSend: 'Add a prompt and press send',
  generationError: 'Generation error',
  download: 'Download',
  imageSection: 'Image',
  songSection: 'Song',
  promptPlaceholder: 'Describe what to create... (Enter to send)',
  model: 'Model',
  attachPhoto: 'Attach photo',
  photo: 'Photo',
  clear: 'Clear',
  songText: 'Song lyrics',
  songStylePlaceholder: 'Style: pop, jazz, rock...',
  noSongTextSelected: 'No song text selected yet',
  selectedSongText: 'Selected lyrics',
  insufficient: 'insufficient',
  send: 'Send',
  imageFilters: 'Image filters',
  imageFilterTheme: 'Theme and style',
  noFilter: 'No filter',
  onlyPrompt: 'Only your prompt',
  done: 'Done',
  lyricsPromptLabel: 'Prompt for lyrics',
  lyricsPromptPlaceholder: 'Describe what the song lyrics should be about. If left empty, the main prompt will be used.',
  generatingLyrics: 'Generating...',
  generateThreeVariants: 'Generate 3 variants',
  lyricsGenerationCost: 'Generation cost',
  generatingThreeVariants: 'Generating 3 lyric variants',
  variant: 'Variant',
  editSelectedLyrics: 'Edit selected lyrics',
  editSelectedLyricsPlaceholder: 'You can refine the selected lyrics here',
  close: 'Close',
  chooseLyrics: 'Choose lyrics',
  maxThreePhotos: 'You can attach up to 3 photos',
  textGenerationError: 'Failed to generate lyrics',
  sendError: 'Failed to send request',
  filterNone: 'Filter: none',
  filterLabel: 'Filter',
  transactionsTitle: 'Transaction history',
  loading: 'Loading...',
  noTransactions: 'No transactions',
  initial_grant: 'Initial balance',
  daily_grant: 'Daily grant',
  generation_charge: 'Generation',
  generation_refund: 'Error refund',
  purchase: 'Top up',
  creditsShort: 'cr.',
  loaderPending: 'In queue',
  loaderProcessingImages: 'Generating images',
  loaderProcessingAudio: 'Generating music',
  loaderProcessingFallback: 'Working on it',
  loadingPending1: 'Putting the idea in line and getting the models ready.',
  loadingPending2: 'Checking whether the confetti reserves are sufficient.',
  loadingPending3: 'Warming up the prompt before the main run.',
  loadingPending4: 'Looking for the hidden make-it-beautiful button.',
  loadingImages1: 'Picking the angle where everything looks its best.',
  loadingImages2: 'Convincing the pixels to cooperate.',
  loadingImages3: 'Adding glow without going overboard.',
  loadingImages4: 'Discarding the suspiciously awkward versions.',
  loadingAudio1: 'Tuning the chorus before it gets stuck in your head.',
  loadingAudio2: 'Making sure the beat did not run off behind the cake.',
  loadingAudio3: 'Blending verses, chorus, and a little magic into one track.',
  loadingAudio4: 'Aiming the vocals at the mood, not the next room.',
  imageSlotLabel: 'Frame {index}: finding the best angle',
  audioSlotLabel: 'Version {index}: mixing the greeting anthem',
  generationProgressAria: 'Generation progress {progress}%',
  categoryPeople: 'For people',
  categorySocial: 'For social media',
  categoryProducts: 'For products',
  categoryCharacters: 'For characters',
  categoryEvents: 'For couples and events',
  presetPhotoshoot: 'Photoshoot',
  presetAvatar: 'Avatar',
  presetBusiness: 'Business portrait',
  presetMagazine: 'Magazine cover',
  presetSportsBroadcast: 'Sports broadcast',
  presetCinematic: 'Cinematic frame',
  presetStreet: 'Street style',
  presetPaparazzi: 'Paparazzi',
  presetRedCarpet: 'Red carpet',
  presetMeme: 'Meme / reaction',
  presetStreamer: 'Streamer setup',
  presetEsports: 'Esports poster',
  presetAlbumCover: 'Album cover',
  presetMoviePoster: 'Movie poster',
  presetProductPhoto: 'Product photo',
  presetLifestyleProduct: 'Lifestyle product',
  presetLuxuryAd: 'Luxury ad',
  presetSticker: 'Sticker',
  presetIcon3d: '3D icon',
  presetMascotLogo: 'Mascot logo',
  presetAnime: 'Anime',
  presetCyberpunk: 'Cyberpunk',
  presetFantasy: 'Fantasy hero',
  presetSuperhero: 'Superhero',
  presetGameCharacter: 'Game character',
  presetCrimeGame: 'Crime game poster',
  presetRetro90s: '90s retro',
  presetPolaroid: 'Polaroid',
  presetY2k: 'Y2K',
  presetHorror: 'Horror',
  presetTravel: 'Travel blogger',
  presetRomantic: 'Romantic shoot',
  presetWedding: 'Wedding shoot',
  presetToy: 'Toy figure',
  presetInterior: 'Interior design',
}

const supported = new Set(LANGUAGES.map(language => language.code).filter(code => code !== 'auto'))
const cache = new Map<string, Messages>()

function detectLanguage(): Exclude<LanguageCode, 'auto'> {
  const raw = navigator.language?.split('-')[0] as Exclude<LanguageCode, 'auto'> | undefined
  return raw && supported.has(raw) ? raw : 'en'
}

async function loadMessages(language: Exclude<LanguageCode, 'auto'>): Promise<Messages> {
  if (cache.has(language)) return cache.get(language) as Messages

  const response = await fetch(`/locales/${language}.json`, { cache: 'no-cache' })
  if (!response.ok) throw new Error(`Failed to load locale: ${language}`)

  const loaded = await response.json() as Messages
  const merged = { ...DEFAULT_MESSAGES, ...loaded }
  cache.set(language, merged)
  return merged
}

interface I18nCtx {
  language: LanguageCode
  resolvedLanguage: Exclude<LanguageCode, 'auto'>
  setLanguage: (language: LanguageCode) => void
  t: (key: string) => string
}

const Ctx = createContext<I18nCtx>({} as I18nCtx)

export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<LanguageCode>(
    () => (localStorage.getItem('language') as LanguageCode | null) ?? 'auto'
  )
  const [messages, setMessages] = useState<Messages>(DEFAULT_MESSAGES)
  const resolvedLanguage = language === 'auto' ? detectLanguage() : language

  useEffect(() => {
    let cancelled = false

    loadMessages(resolvedLanguage)
      .then(nextMessages => {
        if (!cancelled) setMessages(nextMessages)
      })
      .catch(() => {
        if (!cancelled) setMessages(DEFAULT_MESSAGES)
      })

    return () => {
      cancelled = true
    }
  }, [resolvedLanguage])

  useEffect(() => {
    document.documentElement.lang = resolvedLanguage
  }, [resolvedLanguage])

  const setLanguage = (nextLanguage: LanguageCode) => {
    setLanguageState(nextLanguage)
    localStorage.setItem('language', nextLanguage)
  }

  const t = useMemo(
    () => (key: string) => messages[key] ?? DEFAULT_MESSAGES[key] ?? key,
    [messages]
  )

  return (
    <Ctx.Provider value={{ language, resolvedLanguage, setLanguage, t }}>
      {children}
    </Ctx.Provider>
  )
}

export const useI18n = () => useContext(Ctx)
