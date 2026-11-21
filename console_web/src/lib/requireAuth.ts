import { redirect } from '@tanstack/react-router'
import { getToken } from './auth'

export function requireAuth() {
  if (!getToken()) {
    throw redirect({ to: '/login' })
  }
}
