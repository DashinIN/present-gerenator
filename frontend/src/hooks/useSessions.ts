import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'

export function useSessions() {
  return useQuery({
    queryKey: ['sessions'],
    queryFn: () => api.sessions.list().then(r => r.sessions),
    staleTime: 10_000,
  })
}

export function useSession(id: string | null) {
  return useQuery({
    queryKey: ['session', id],
    queryFn: () => api.sessions.get(id!),
    enabled: !!id,
    refetchInterval: (query) => {
      const gens = query.state.data?.generations ?? []
      const hasPending = gens.some(g =>
        g.status === 'pending' || g.status === 'processing_images' || g.status === 'processing_audio'
      )
      return hasPending ? 2000 : false
    },
  })
}

export function useRenameSession() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, title }: { id: string; title: string }) =>
      api.sessions.rename(id, title),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['sessions'] })
    },
  })
}
