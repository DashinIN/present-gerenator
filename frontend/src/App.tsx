import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useCurrentUser } from '@/hooks/useAuth'
import { LoginPage } from '@/pages/LoginPage'
import { ChatPage } from '@/pages/ChatPage'
import { ApiError } from '@/lib/api'
import { ThemeProvider } from '@/lib/theme'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (count, err) => {
        if (err instanceof ApiError && (err.status === 401 || err.status === 403)) return false
        return count < 2
      },
    },
  },
})

function AppRouter() {
  const { data: user, isLoading, error } = useCurrentUser()

  if (isLoading) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <div className="spinner" />
      </div>
    )
  }

  const isUnauth = error instanceof ApiError && error.status === 401
  if (!user || isUnauth) return <LoginPage />
  return <ChatPage />
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AppRouter />
      </ThemeProvider>
    </QueryClientProvider>
  )
}
