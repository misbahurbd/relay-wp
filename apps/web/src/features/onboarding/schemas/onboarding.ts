import { z } from 'zod'
import type { FirstGoal } from '#/features/workspaces'
import type { BusinessGoal, Industry, TeamSize } from '../types'

export const FIRST_GOALS: { id: FirstGoal; label: string; note: string }[] = [
  {
    id: 'offers',
    label: 'Send offers',
    note: 'We’ll highlight Broadcasts and Templates, so your first campaign is two clicks away.',
  },
  {
    id: 'support',
    label: 'Answer customers',
    note: 'We’ll highlight the Team Inbox and quick replies, so no customer waits.',
  },
  {
    id: 'automate',
    label: 'Automate replies',
    note: 'We’ll highlight Automations and the Chatbot builder, starting with a welcome message.',
  },
]

export const INDUSTRIES: { id: Industry; label: string }[] = [
  { id: 'retail_ecommerce', label: 'Retail and e-commerce' },
  { id: 'food_restaurants', label: 'Food and restaurants' },
  { id: 'education', label: 'Education' },
  { id: 'health_clinics', label: 'Health and clinics' },
  { id: 'travel', label: 'Travel' },
  { id: 'real_estate', label: 'Real estate' },
  { id: 'other', label: 'Other' },
]

export const TEAM_SIZES: { id: TeamSize; label: string }[] = [
  { id: 'just_me', label: 'Just me' },
  { id: '2_5', label: '2–5' },
  { id: '6_20', label: '6–20' },
  { id: '20_plus', label: '20+' },
]

export const BUSINESS_GOALS: { id: BusinessGoal; label: string; hint: string }[] = [
  { id: 'marketing', label: 'Marketing', hint: 'Broadcasts, offers and campaigns' },
  { id: 'support', label: 'Support', hint: 'Answer questions as a team' },
  { id: 'sales', label: 'Sales', hint: 'Catalogs, orders and payments' },
]

export const COUNTRIES: { code: string; name: string; dial: string }[] = [
  { code: 'BD', name: 'Bangladesh', dial: '+880' },
  { code: 'BR', name: 'Brazil', dial: '+55' },
  { code: 'IN', name: 'India', dial: '+91' },
  { code: 'ID', name: 'Indonesia', dial: '+62' },
  { code: 'MY', name: 'Malaysia', dial: '+60' },
  { code: 'MX', name: 'Mexico', dial: '+52' },
  { code: 'NG', name: 'Nigeria', dial: '+234' },
  { code: 'PK', name: 'Pakistan', dial: '+92' },
  { code: 'SA', name: 'Saudi Arabia', dial: '+966' },
  { code: 'ZA', name: 'South Africa', dial: '+27' },
  { code: 'AE', name: 'United Arab Emirates', dial: '+971' },
  { code: 'GB', name: 'United Kingdom', dial: '+44' },
  { code: 'US', name: 'United States', dial: '+1' },
]

/** The country from the browser's locale, if we list it. */
export function guessCountry() {
  const region = new Intl.Locale(navigator.language).maximize().region
  return COUNTRIES.find((c) => c.code === region)?.code ?? ''
}

export const businessSchema = z.object({
  name: z.string().trim().min(2, 'Enter your business name').max(80, 'Use 80 characters or fewer'),
  industry: z.enum(INDUSTRIES.map((i) => i.id) as [Industry, ...Industry[]], {
    error: 'Choose your industry',
  }),
  website: z
    .string()
    .trim()
    .refine(
      // The API adds https:// when it's missing, so "yourstore.com" is fine.
      (v) => v === '' || /^(https?:\/\/)?[^\s./]+\.[^\s]+$/.test(v),
      'Enter a web address like yourstore.com',
    ),
  country: z.string().min(2, 'Choose your country'),
  team_size: z.enum(TEAM_SIZES.map((t) => t.id) as [TeamSize, ...TeamSize[]], {
    error: 'Choose how many people will answer chats',
  }),
  goals: z.array(z.enum(['marketing', 'support', 'sales'])).min(1, 'Pick at least one'),
})

export const testMessageSchema = z.object({
  dial: z.string().min(1),
  number: z
    .string()
    .trim()
    .refine((v) => v.replace(/\D/g, '').length >= 6, 'Enter your WhatsApp number')
    .refine((v) => v.replace(/\D/g, '').length <= 14, 'That number looks too long'),
})
