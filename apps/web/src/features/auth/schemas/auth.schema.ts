import { z } from 'zod'

const email = z.email('Enter an email like you@yourbusiness.com').trim()

// Mirrors the API's rules (docs/PHASE-2-AUTH.md). The API also rejects breached passwords.
export const PASSWORD_RULES = [
  { id: 'length', label: '8 or more characters', test: (v: string) => v.length >= 8 },
  {
    id: 'case',
    label: 'Upper and lower case',
    test: (v: string) => /[a-z]/.test(v) && /[A-Z]/.test(v),
  },
  { id: 'digit', label: 'A number', test: (v: string) => /\d/.test(v) },
  { id: 'symbol', label: 'A symbol', test: (v: string) => /[^A-Za-z0-9]/.test(v) },
] as const

export const newPassword = z
  .string()
  .max(128, 'Use 128 characters or fewer')
  .refine((v) => PASSWORD_RULES.every((r) => r.test(v)), 'Meet all the password rules below')

export const signupSchema = z.object({
  fullName: z.string().trim().min(1, 'Enter your name').max(100, 'Use 100 characters or fewer'),
  email,
  password: newPassword,
  acceptTerms: z.literal(true, { error: 'Accept the terms to create your account' }),
})

export const loginSchema = z.object({
  email,
  password: z.string().min(1, 'Enter your password'),
})

export const emailSchema = z.object({ email })

export const resetPasswordSchema = z.object({ password: newPassword })

export const codeSchema = z.object({
  code: z.string().regex(/^\d{6}$/, 'Enter all 6 digits'),
})

/** "Shakil Ahmed Khan" → first "Shakil", last "Ahmed Khan". */
export function splitName(fullName: string) {
  const [first, ...rest] = fullName.trim().split(/\s+/)
  return { first_name: first ?? '', last_name: rest.join(' ') || undefined }
}
