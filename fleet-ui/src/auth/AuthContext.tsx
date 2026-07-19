import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { authenticate as apiAuthenticate, logout as apiLogout, userInfo } from '../api/auth'
import { clearToken, getToken, getUsername, setToken } from '../api/token'
import type { UserInfo } from '../api/types'

interface AuthContextValue {
  token: string | null
  username: string | null
  user: UserInfo | null
  isAuthenticated: boolean
  isLoading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  verifyPassword: (password: string) => Promise<boolean>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setTokenState] = useState<string | null>(() => getToken())
  const [username, setUsername] = useState<string | null>(() => getUsername())
  const [user, setUser] = useState<UserInfo | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const t = getToken()
    if (!t) {
      setIsLoading(false)
      return
    }
    userInfo()
      .then(setUser)
      .catch(() => {
        clearToken()
        setTokenState(null)
        setUsername(null)
      })
      .finally(() => setIsLoading(false))
  }, [])

  const login = useCallback(async (u: string, p: string) => {
    const result = await apiAuthenticate(u, p)
    setToken(result.access_token, result.username)
    setTokenState(result.access_token)
    setUsername(result.username)
    const info = await userInfo()
    setUser(info)
  }, [])

  const logout = useCallback(async () => {
    try {
      if (getToken()) await apiLogout()
    } catch {
      /* ignore */
    }
    clearToken()
    setTokenState(null)
    setUsername(null)
    setUser(null)
  }, [])

  const verifyPassword = useCallback(
    async (password: string) => {
      const u = username || getUsername()
      if (!u) return false
      try {
        await apiAuthenticate(u, password)
        return true
      } catch {
        return false
      }
    },
    [username],
  )

  const value = useMemo(
    () => ({
      token,
      username,
      user,
      isAuthenticated: !!token,
      isLoading,
      login,
      logout,
      verifyPassword,
    }),
    [token, username, user, isLoading, login, logout, verifyPassword],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
