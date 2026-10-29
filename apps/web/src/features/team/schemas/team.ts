import { z } from 'zod'
import type { ChatScope, RoleColor } from '../types'

export const SCOPES: { value: ChatScope; label: string; long: string; desc: string }[] = [
  {
    value: 'assigned',
    label: 'Only assigned chats',
    long: 'Only assigned to them',
    desc: 'They see a chat once it’s assigned to them.',
  },
  {
    value: 'team',
    label: 'Their team’s chats',
    long: 'Their teams’ chats',
    desc: 'All chats routed to their teams, assigned or not.',
  },
  {
    value: 'all',
    label: 'All chats',
    long: 'All chats',
    desc: 'Every chat on the selected numbers.',
  },
]

export function scopeLabel(scope: ChatScope) {
  return SCOPES.find((s) => s.value === scope)?.long ?? scope
}

/** Tailwind classes per colour, mapped to design tokens (the prototype's --primary/--blue/…). */
export const TONE = {
  green: { soft: 'bg-brand-soft', text: 'text-brand-text', solid: 'bg-brand', ring: 'ring-brand' },
  blue: { soft: 'bg-info-soft', text: 'text-info', solid: 'bg-info', ring: 'ring-info' },
  amber: {
    soft: 'bg-warning-soft',
    text: 'text-warning',
    solid: 'bg-warning',
    ring: 'ring-warning',
  },
  red: {
    soft: 'bg-destructive-soft',
    text: 'text-destructive',
    solid: 'bg-destructive',
    ring: 'ring-destructive',
  },
  gray: {
    soft: 'bg-neutral-soft',
    text: 'text-neutral',
    solid: 'bg-neutral',
    ring: 'ring-neutral',
  },
} as const satisfies Record<RoleColor, Record<string, string>>

export const COLORS: { value: RoleColor; label: string }[] = [
  { value: 'green', label: 'Green' },
  { value: 'blue', label: 'Blue' },
  { value: 'amber', label: 'Amber' },
  { value: 'red', label: 'Red' },
  { value: 'gray', label: 'Gray' },
]

/** A stable avatar colour for a person. */
export function toneFor(seed: string): RoleColor {
  const tones: RoleColor[] = ['green', 'blue', 'amber', 'gray', 'red']
  let h = 0
  for (const c of seed) h = (h * 31 + c.charCodeAt(0)) >>> 0
  return tones[h % tones.length]
}

export const ROUTING = [
  { value: 'round_robin', label: 'Round-robin' },
  { value: 'least_busy', label: 'Least busy' },
  { value: 'manual', label: 'Manual' },
] as const

export const EXPIRIES = [
  { value: '24h', label: '24 hours' },
  { value: '7d', label: '7 days' },
  { value: '30d', label: '30 days' },
] as const

/** Permission groups in the order and wording of the Roles editor. */
export const GROUPS: { key: string; title: string; tone: RoleColor }[] = [
  { key: 'inbox', title: 'Inbox', tone: 'green' },
  { key: 'contacts', title: 'Contacts', tone: 'blue' },
  { key: 'broadcasts', title: 'Broadcasts', tone: 'amber' },
  { key: 'templates', title: 'Templates', tone: 'gray' },
  { key: 'automations', title: 'Chatbots and automations', tone: 'blue' },
  { key: 'integrations', title: 'Integrations and API keys', tone: 'gray' },
  { key: 'team', title: 'Team', tone: 'green' },
  { key: 'billing', title: 'Billing and wallet', tone: 'amber' },
]

export const PERMISSION_HINTS: Record<string, string> = {
  'inbox.reply': 'Send free-form messages inside the 24-hour window',
  'inbox.send_template': 'Restart conversations with approved templates (Meta charges apply)',
  'inbox.close_reassign': 'Resolve chats or move them to another agent',
  'inbox.notes': 'Leave notes only teammates can see',
  'contacts.view': 'See names, numbers, tags and history',
  'contacts.edit': 'Change names, tags and custom fields',
  'contacts.import': 'Upload CSV or Excel lists',
  'contacts.export': 'Download lists with phone numbers',
  'contacts.delete': 'Permanently remove contacts and their chats',
  'broadcasts.create': 'Draft campaigns and choose audiences',
  'broadcasts.send': 'Send or schedule campaigns (uses wallet balance)',
  'broadcasts.reports': 'See delivered, read and reply stats',
  'templates.create': 'Write new message templates',
  'templates.submit': 'Send templates for Meta approval',
  'templates.delete': 'Remove templates from your account',
  'automations.edit': 'Change bot flows, keywords and away messages',
  'automations.publish': 'Make bot changes live for customers',
  'integrations.manage': 'Connect Shopify, Google Sheets, Zapier and webhooks',
  'integrations.api_keys': 'Full programmatic access to your account',
  'team.invite': 'Send invites by email (uses a seat)',
  'team.roles': 'Edit roles, permissions and people’s access',
  'team.remove': 'Suspend or remove a teammate’s access',
  'billing.view': 'See plan, invoices and Meta costs',
  'billing.pay': 'Update card, pay invoices, change plan',
  'billing.topup': 'Add money for Meta message costs',
}

export const LOG_ACTIONS: Record<string, { label: string; tone: RoleColor }> = {
  role_changed: { label: 'Role changed', tone: 'blue' },
  permission_granted: { label: 'Permission granted', tone: 'green' },
  permission_denied: { label: 'Permission removed', tone: 'red' },
  permission_revoked: { label: 'Permission revoked', tone: 'red' },
  permission_expired: { label: 'Permission expired', tone: 'gray' },
  invited: { label: 'Invited', tone: 'amber' },
  invite_resent: { label: 'Invite resent', tone: 'amber' },
  invite_revoked: { label: 'Invite revoked', tone: 'red' },
  invite_accepted: { label: 'Joined', tone: 'green' },
  removed: { label: 'Removed', tone: 'red' },
  left: { label: 'Left', tone: 'gray' },
  suspended: { label: 'Suspended', tone: 'amber' },
  restored: { label: 'Restored', tone: 'green' },
  access_changed: { label: 'Access changed', tone: 'blue' },
  team_changed: { label: 'Team changed', tone: 'blue' },
  role_created: { label: 'Role created', tone: 'green' },
  role_updated: { label: 'Role edited', tone: 'blue' },
  role_deleted: { label: 'Role deleted', tone: 'red' },
}

const EMAIL = z.email()

/** Splits pasted text (commas, spaces, new lines, a CSV column) into unique addresses. */
export function parseEmails(text: string) {
  const valid: string[] = []
  const invalid: string[] = []
  for (const raw of text.split(/[\s,;]+/)) {
    const email = raw
      .trim()
      .replace(/^["'<]+|["'>]+$/g, '')
      .toLowerCase()
    if (!email || valid.includes(email)) continue
    if (EMAIL.safeParse(email).success) valid.push(email)
    else invalid.push(raw.trim())
  }
  return { valid, invalid }
}

export const roleSchema = z.object({
  name: z.string().trim().min(1, 'Enter a role name').max(50, 'Use up to 50 characters'),
  description: z.string().trim().max(200, 'Use up to 200 characters'),
})

export const teamSchema = z.object({
  name: z.string().trim().min(1, 'Enter a team name').max(50, 'Use up to 50 characters'),
  description: z.string().trim().max(200, 'Use up to 200 characters'),
})

export const passwordSchema = z.string().min(8, 'Use at least 8 characters')
