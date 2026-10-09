import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { api, ApiError, type Account } from '@/lib/api'

// Who is signed in, as the server says (GET /api/me). The sign-in cookie is
// HttpOnly, so the page asks instead of guessing; 401 means signed out.
type Auth = {
  account: Account | null
  ready: boolean
  refresh: () => Promise<void>
  setAccount: (a: Account | null) => void
}

const AuthContext = createContext<Auth | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [account, setAccount] = useState<Account | null>(null)
  const [ready, setReady] = useState(false)
  const refresh = useCallback(async () => {
    try {
      setAccount(await api.me())
    } catch (e) {
      if (e instanceof ApiError && e.id === 'unauthorized') setAccount(null)
    } finally {
      setReady(true)
    }
  }, [])
  useEffect(() => {
    refresh()
  }, [refresh])
  return <AuthContext.Provider value={{ account, ready, refresh, setAccount }}>{children}</AuthContext.Provider>
}

export function useAuth(): Auth {
  const a = useContext(AuthContext)
  if (!a) throw new Error('useAuth outside AuthProvider')
  return a
}
