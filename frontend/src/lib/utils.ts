export function cn(...classes: (string | undefined | false | null)[]) {
  return classes.filter(Boolean).join(' ')
}

export function formatDate(iso: string) {
  const d = new Date(iso)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60_000) return 'только что'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} мин назад`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} ч назад`
  return d.toLocaleDateString('ru', { day: 'numeric', month: 'short' })
}

export function formatCredits(n: number) {
  return `${n} кр.`
}
