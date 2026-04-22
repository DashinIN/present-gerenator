import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/lib/api'

export function useCurrentUser() {
  return useQuery({
    queryKey: ['me'],
    queryFn: api.auth.me,
    retry: (count, err) => {
      if (err instanceof ApiError && err.status === 401) return false
      return count < 2
    },
    staleTime: 5 * 60_000,
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.auth.logout,
    onSuccess: () => {
      qc.setQueryData(['me'], null)
      qc.removeQueries({ queryKey: ['balance'] })
      qc.removeQueries({ queryKey: ['sessions'] })
    },
  })
}

export function useBalance() {
  return useQuery({
    queryKey: ['balance'],
    queryFn: () => api.billing.balance().then(r => r.balance),
    refetchInterval: 30_000,
    staleTime: 10_000,
  })
}

export function useTariff() {
  return useQuery({
    queryKey: ['tariff'],
    queryFn: api.billing.tariff,
    staleTime: 60_000,
  })
}
