import { createContext, useContext, useEffect, useState, ReactNode } from 'react'

type Theme = 'dark' | 'light'

interface ThemeCtx {
  theme: Theme
  accent: string
  setTheme: (t: Theme) => void
  setAccent: (c: string) => void
}

const Ctx = createContext<ThemeCtx>({} as ThemeCtx)

export const ACCENT_PRESETS = [
  { name: 'Фиолетовый', value: '#7c5cfc' },
  { name: 'Синий',      value: '#3b82f6' },
  { name: 'Циан',       value: '#06b6d4' },
  { name: 'Изумруд',    value: '#10b981' },
  { name: 'Оранжевый',  value: '#f97316' },
  { name: 'Розовый',    value: '#ec4899' },
]

function hexToRgb(hex: string): [number, number, number] | null {
  const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})/i.exec(hex)
  if (!m) return null
  return [parseInt(m[1], 16), parseInt(m[2], 16), parseInt(m[3], 16)]
}

// Blend base color with accent at given ratio (0–1)
function blendHex(base: [number, number, number], accent: [number, number, number], ratio: number): string {
  const r = Math.round(base[0] * (1 - ratio) + accent[0] * ratio)
  const g = Math.round(base[1] * (1 - ratio) + accent[1] * ratio)
  const b = Math.round(base[2] * (1 - ratio) + accent[2] * ratio)
  return `rgb(${r},${g},${b})`
}

function applyTheme(theme: Theme, accent: string) {
  const root = document.documentElement
  const rgb = hexToRgb(accent) ?? [124, 92, 252]
  const [r, g, b] = rgb

  // Accent vars
  root.style.setProperty('--primary', accent)
  root.style.setProperty('--primary-hover', `rgba(${r},${g},${b},0.85)`)
  root.style.setProperty('--primary-subtle', `rgba(${r},${g},${b},0.12)`)
  root.style.setProperty('--primary-rgb', `${r},${g},${b}`)
  // Border: accent-tinted, very subtle
  root.style.setProperty('--border', `rgba(${r},${g},${b},0.18)`)

  if (theme === 'dark') {
    const darkBase: [number, number, number] = [10, 10, 12]
    root.style.setProperty('--bg',       blendHex(darkBase,       rgb, 0.06))
    root.style.setProperty('--surface',  blendHex([20, 20, 24],   rgb, 0.07))
    root.style.setProperty('--surface2', blendHex([30, 30, 36],   rgb, 0.09))
    root.style.setProperty('--text',         '#e8e8ee')
    root.style.setProperty('--text-muted',   '#72728a')
    root.style.setProperty('--success',      '#22c55e')
    root.style.setProperty('--error',        '#ef4444')
  } else {
    root.style.setProperty('--bg',       blendHex([245, 245, 248], rgb, 0.04))
    root.style.setProperty('--surface',  blendHex([255, 255, 255], rgb, 0.03))
    root.style.setProperty('--surface2', blendHex([238, 238, 244], rgb, 0.06))
    root.style.setProperty('--text',         '#111118')
    root.style.setProperty('--text-muted',   '#7a7a96')
    root.style.setProperty('--success',      '#16a34a')
    root.style.setProperty('--error',        '#dc2626')
  }
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(
    () => (localStorage.getItem('theme') as Theme) ?? 'dark'
  )
  const [accent, setAccentState] = useState(
    () => localStorage.getItem('accent') ?? '#7c5cfc'
  )

  const setTheme = (t: Theme) => { setThemeState(t); localStorage.setItem('theme', t) }
  const setAccent = (c: string) => { setAccentState(c); localStorage.setItem('accent', c) }

  useEffect(() => { applyTheme(theme, accent) }, [theme, accent])
  useEffect(() => { applyTheme(theme, accent) }, []) // mount

  return <Ctx.Provider value={{ theme, accent, setTheme, setAccent }}>{children}</Ctx.Provider>
}

export const useTheme = () => useContext(Ctx)
