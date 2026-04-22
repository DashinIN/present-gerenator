import { ButtonHTMLAttributes, CSSProperties, forwardRef } from 'react'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'ghost' | 'danger'
  size?: 'sm' | 'md' | 'icon'
  loading?: boolean
}

const BASE: CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  gap: 6,
  fontFamily: 'inherit',
  fontWeight: 500,
  borderRadius: 8,
  border: 'none',
  cursor: 'pointer',
  transition: 'background 0.15s, color 0.15s, opacity 0.15s',
  userSelect: 'none',
  flexShrink: 0,
}

const VARIANT_STYLES: Record<string, CSSProperties> = {
  primary: { background: 'var(--primary)', color: '#fff' },
  ghost:   { background: 'transparent', color: 'var(--text-muted)' },
  danger:  { background: 'transparent', color: 'var(--error)' },
}

const SIZE_STYLES: Record<string, CSSProperties> = {
  sm:   { padding: '8px 16px', fontSize: 13 },
  md:   { padding: '10px 20px', fontSize: 14 },
  icon: { padding: '8px', fontSize: 14, borderRadius: 8 },
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading, children, disabled, style, onMouseEnter, onMouseLeave, ...props }, ref) => {
    const isDisabled = disabled || loading
    const variantStyle = VARIANT_STYLES[variant]
    const sizeStyle = SIZE_STYLES[size]

    const handleMouseEnter = (e: React.MouseEvent<HTMLButtonElement>) => {
      if (!isDisabled) {
        const el = e.currentTarget
        if (variant === 'primary') el.style.opacity = '0.88'
        else el.style.background = 'var(--primary-subtle)'
        if (variant === 'ghost') el.style.color = 'var(--text)'
      }
      onMouseEnter?.(e)
    }

    const handleMouseLeave = (e: React.MouseEvent<HTMLButtonElement>) => {
      const el = e.currentTarget
      el.style.opacity = ''
      el.style.background = variantStyle.background as string
      el.style.color = variantStyle.color as string
      onMouseLeave?.(e)
    }

    return (
      <button
        ref={ref}
        disabled={isDisabled}
        style={{
          ...BASE,
          ...variantStyle,
          ...sizeStyle,
          ...(isDisabled ? { opacity: 0.45, pointerEvents: 'none' } : {}),
          ...style,
        }}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        {...props}
      >
        {loading
          ? <span style={{ width: 14, height: 14, border: '2px solid currentColor', borderTopColor: 'transparent', borderRadius: '50%', display: 'inline-block', animation: 'spin 0.8s linear infinite' }} />
          : children}
      </button>
    )
  }
)
Button.displayName = 'Button'
