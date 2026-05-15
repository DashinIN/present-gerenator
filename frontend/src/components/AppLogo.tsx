import type { CSSProperties } from 'react'
import logoUrl from '@/assets/minimal_ai_creative_logo.svg'

interface AppLogoProps {
  size?: number
  alt?: string
  style?: CSSProperties
}

export function AppLogo({ size = 24, alt = 'FunBaza logo', style }: AppLogoProps) {
  return (
    <img
      src={logoUrl}
      alt={alt}
      width={size}
      height={size}
      style={{
        width: size,
        height: size,
        display: 'block',
        objectFit: 'contain',
        ...style,
      }}
    />
  )
}
