import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { api, setAuthToken, getAuthToken } from '../lib/api'
import type { AuthUser, LoginResponse } from '../lib/types'

interface AuthContextType {
  user: AuthUser | null
  isAuthenticated: boolean
  isLoading: boolean
  login: (email: string, password: string) => Promise<void>
  signup: (email: string, password: string, firstName: string, lastName: string, orgName: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const token = getAuthToken()
    if (token) {
      api.get<AuthUser>('/auth/me')
        .then(setUser)
        .catch(() => {
          setAuthToken(null)
        })
        .finally(() => setIsLoading(false))
    } else {
      setIsLoading(false)
    }
  }, [])

  const login = async (email: string, password: string) => {
    const res = await api.post<LoginResponse>('/auth/login', { email, password })
    setAuthToken(res.token)
    setUser(res.user)
  }

  const signup = async (email: string, password: string, firstName: string, lastName: string, orgName: string) => {
    const res = await api.post<LoginResponse>('/auth/signup', {
      email, password, first_name: firstName, last_name: lastName, org_name: orgName,
    })
    setAuthToken(res.token)
    setUser(res.user)
  }

  const logout = () => {
    setAuthToken(null)
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, isAuthenticated: !!user, isLoading, login, signup, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
