import { formatShortDate } from './format'

type LogLike = {
  action: string
  target: string
  before: unknown
  after: unknown
  note: string
  subject_name: string
  subject_email: string | null
}

function field(v: unknown, key: string): string | undefined {
  if (v && typeof v === 'object' && key in v) {
    const x = (v as Record<string, unknown>)[key]
    if (typeof x === 'string') return x
  }
  return undefined
}

/** "granted “Export contacts”, expires Nov 12": an entry as the rest of a sentence after the actor. */
export function describeLog(e: LogLike, label: (key: string) => string) {
  const who = e.subject_name || e.subject_email || 'someone'
  const perm = `“${label(e.target)}”`
  const expires = field(e.after, 'expires_at')
  switch (e.action) {
    case 'role_changed':
      return `changed ${who}’s role from ${field(e.before, 'role')} to ${field(e.after, 'role')}`
    case 'permission_granted':
      return `granted ${perm} to ${who}${expires ? `, expires ${formatShortDate(expires)}` : ''}`
    case 'permission_denied':
      return `removed ${perm} from ${who}${expires ? ` until ${formatShortDate(expires)}` : ''}`
    case 'permission_revoked':
      return `revoked ${perm} from ${who}`
    case 'permission_expired':
      return `${perm} expired for ${who}`
    case 'invited':
      return `invited ${who} as ${field(e.after, 'role')}`
    case 'invite_resent':
      return `resent the invite to ${who}`
    case 'invite_revoked':
      return `revoked the invite for ${who}`
    case 'invite_accepted':
      return `accepted the invite and joined as ${field(e.after, 'role')}`
    case 'removed':
      return `removed ${who} from the workspace`
    case 'left':
      return 'left the workspace'
    case 'suspended':
      return `suspended ${who}’s access`
    case 'restored':
      return `restored ${who}’s access`
    case 'access_changed':
      return `changed ${who}’s teams and numbers`
    case 'team_changed':
      return e.note
        ? `updated a team · ${e.note}`
        : `updated team ${field(e.after, 'name') ?? ''}`.trim()
    case 'role_created':
      return `created the role “${field(e.after, 'name')}”`
    case 'role_updated':
      return `edited the role “${field(e.after, 'name')}”`
    case 'role_deleted':
      return `deleted the role “${field(e.before, 'name')}”`
    default:
      return e.action.replace(/_/g, ' ')
  }
}

/** What changed, as "before → after" for the access log table. */
export function logChange(e: LogLike, label: (key: string) => string) {
  const status = (v: unknown) => {
    const s = field(v, 'status')
    return s && s.charAt(0).toUpperCase() + s.slice(1)
  }
  switch (e.action) {
    case 'role_changed':
      return { target: 'Role', before: field(e.before, 'role'), after: field(e.after, 'role') }
    case 'permission_granted': {
      const expires = field(e.after, 'expires_at')
      return {
        target: label(e.target),
        before: 'Not allowed',
        after: expires ? `Granted until ${formatShortDate(expires)}` : 'Granted',
      }
    }
    case 'permission_denied':
      return { target: label(e.target), before: 'Allowed', after: 'Removed' }
    case 'permission_revoked':
      return { target: label(e.target), before: 'Granted', after: 'Revoked' }
    case 'permission_expired':
      return { target: label(e.target), before: 'Granted', after: 'Expired' }
    case 'suspended':
    case 'restored':
      return { target: 'Access', before: status(e.before), after: status(e.after) }
    case 'invited':
      return { target: 'Invite', after: field(e.after, 'role') }
    case 'invite_accepted':
      return { target: 'Membership', after: field(e.after, 'role') }
    case 'role_created':
    case 'role_updated':
      return { target: `Role “${field(e.after, 'name')}”` }
    case 'role_deleted':
      return { target: `Role “${field(e.before, 'name')}”` }
    case 'team_changed':
      return { target: `Team ${field(e.after, 'name') ?? field(e.before, 'name') ?? ''}`.trim() }
    case 'access_changed':
      return { target: 'Teams and numbers' }
    default:
      return { target: e.target && !/^[0-9a-f-]{36}$/.test(e.target) ? e.target : '' }
  }
}
