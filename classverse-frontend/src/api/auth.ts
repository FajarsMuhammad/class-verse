import { api } from './client'
import type { AccessTokenResponse, Me, RegisterRequest } from './types'

export type { Me } from './types'
export type RegisterInput = RegisterRequest

export const authApi = {
  register: (input: RegisterInput) =>
    api<void>('/api/auth/register', { method: 'POST', body: input, auth: false }),
  resendOtp: (email: string) =>
    api<void>('/api/auth/resend-otp', { method: 'POST', body: { email }, auth: false }),
  verifyEmail: (email: string, code: string) =>
    api<AccessTokenResponse>('/api/auth/verify-email', {
      method: 'POST',
      body: { email, code },
      auth: false,
    }),
  login: (email: string, password: string) =>
    api<AccessTokenResponse>('/api/auth/login', {
      method: 'POST',
      body: { email, password },
      auth: false,
    }),
  logout: () => api<void>('/api/auth/logout', { method: 'POST', auth: false }),
  me: () => api<Me>('/api/me'),
  updateMe: (patch: { display_name?: string; birth_date?: string; avatar_key?: string }) =>
    api<Me>('/api/me', { method: 'PATCH', body: patch }),
}
