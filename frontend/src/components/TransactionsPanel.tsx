import { useQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { api } from '@/lib/api'
import { formatDate } from '@/lib/utils'
import { useI18n } from '@/lib/i18n'

interface TransactionsPanelProps {
  onClose: () => void
}

function useTransactions() {
  return useQuery({
    queryKey: ['transactions'],
    queryFn: () => api.billing.transactions(50).then(r => r.transactions ?? []),
    staleTime: 30_000,
  })
}

export function TransactionsPanel({ onClose }: TransactionsPanelProps) {
  const { data: txs, isLoading } = useTransactions()
  const { t } = useI18n()

  return (
    <div
      style={{
        position: 'fixed', inset: 0, zIndex: 200,
        display: 'flex', alignItems: 'flex-end', justifyContent: 'flex-start',
      }}
      onClick={onClose}
    >
      <div
        style={{
          marginLeft: 260, width: 360, maxHeight: '70vh',
          background: 'var(--surface)', borderRadius: '0 16px 0 0',
          boxShadow: '0 -4px 32px rgba(0,0,0,0.22)',
          display: 'flex', flexDirection: 'column', overflow: 'hidden',
        }}
        onClick={event => event.stopPropagation()}
      >
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '14px 16px', borderBottom: '1px solid var(--border)',
          flexShrink: 0,
        }}>
          <span style={{ fontWeight: 600, fontSize: 14 }}>{t('transactionsTitle')}</span>
          <button
            onClick={onClose}
            style={{ border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--text-muted)', display: 'flex' }}
          >
            <X size={16} />
          </button>
        </div>

        <div style={{ overflowY: 'auto', flex: 1, padding: '8px 0' }}>
          {isLoading && (
            <div style={{ padding: '24px 16px', color: 'var(--text-muted)', fontSize: 13, textAlign: 'center' }}>
              {t('loading')}
            </div>
          )}
          {!isLoading && (!txs || txs.length === 0) && (
            <div style={{ padding: '24px 16px', color: 'var(--text-muted)', fontSize: 13, textAlign: 'center' }}>
              {t('noTransactions')}
            </div>
          )}
          {txs?.map(tx => (
            <div key={tx.id} style={{
              display: 'flex', alignItems: 'center', gap: 10,
              padding: '10px 16px',
              borderBottom: '1px solid var(--border)',
            }}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {txTypeLabel(tx.type, t)}
                </div>
                <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 2 }}>
                  {formatDate(tx.created_at)}
                </div>
              </div>
              <div style={{
                fontWeight: 600, fontSize: 14, flexShrink: 0,
                color: tx.amount > 0 ? 'var(--success)' : 'var(--error)',
              }}>
                {tx.amount > 0 ? '+' : ''}{tx.amount}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function txTypeLabel(type: string, t: (key: string) => string): string {
  switch (type) {
    case 'initial_grant': return t('initial_grant')
    case 'daily_grant': return t('daily_grant')
    case 'generation_charge': return t('generation_charge')
    case 'generation_refund': return t('generation_refund')
    case 'purchase': return t('purchase')
    default: return type
  }
}
