export function cn(...classes: (string | undefined | false | null)[]) {
  return classes.filter(Boolean).join(' ')
}

export function formatDate(iso: string) {
  const d = new Date(iso)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60_000) return '\u0442\u043e\u043b\u044c\u043a\u043e \u0447\u0442\u043e'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} \u043c\u0438\u043d \u043d\u0430\u0437\u0430\u0434`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} \u0447 \u043d\u0430\u0437\u0430\u0434`
  return d.toLocaleDateString('ru', { day: 'numeric', month: 'short' })
}

export function formatCredits(n: number) {
  return `${n} \u043a\u0440.`
}
