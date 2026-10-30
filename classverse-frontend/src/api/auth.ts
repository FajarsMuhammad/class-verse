import { api } from './client'

export interface Me {
  id: string
  email: string
  display_name: string
  birth_date: string | null
  avatar_key: string
  needs_profile: boolean
  orgs: unknown[]
}

interface TokenResponse {
  access_token: string
}

export interface RegisterInput {
  email: string
  password: string
  display_name: string
  birth_date: string
}

export const authApi = {
  register: (input: RegisterInput) =>
    api<void>('/api/auth/register', { method: 'POST', body: input, auth: false }),
  resendOtp: (email: string) =>
    api<void>('/api/auth/resend-otp', { method: 'POST', body: { email }, auth: false }),
  verifyEmail: (email: string, code: string) =>
    api<TokenResponse>('/api/auth/verify-email', {
      method: 'POST',
      body: { email, code },
      auth: false,
    }),
  login: (email: string, password: string) =>
    api<TokenResponse>('/api/auth/login', { method: 'POST', body: { email, password }, auth: false }),
  logout: () => api<void>('/api/auth/logout', { method: 'POST', auth: false }),
  me: () => api<Me>('/api/me'),
  updateMe: (patch: { display_name?: string; birth_date?: string }) =>
    api<Me>('/api/me', { method: 'PATCH', body: patch }),
}
