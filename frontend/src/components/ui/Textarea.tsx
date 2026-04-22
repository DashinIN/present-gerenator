import { TextareaHTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/utils'

export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaHTMLAttributes<HTMLTextAreaElement>
>(({ className, ...props }, ref) => (
  <textarea
    ref={ref}
    className={cn(
      'w-full bg-transparent resize-none outline-none text-[var(--text)] placeholder:text-[var(--text-muted)]',
      className
    )}
    {...props}
  />
))
Textarea.displayName = 'Textarea'
