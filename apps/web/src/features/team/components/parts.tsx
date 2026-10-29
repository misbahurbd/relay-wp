import { Link } from '@tanstack/react-router'
import { ChevronLeftIcon, XIcon } from 'lucide-react'
import type { ComponentProps, ReactNode } from 'react'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '#/components/ui/dialog'
import { cn } from '#/lib/utils'
import { TONE, toneFor } from '../schemas/team'
import type { RoleColor } from '../types'

/** The prototype's .card: 16px radius, surface, border, soft shadow. */
export function Panel({ className, ...props }: ComponentProps<'section'>) {
  return (
    <section
      className={cn(
        'flex flex-col rounded-xl border bg-card px-5 py-5 shadow-card sm:px-[22px]',
        className,
      )}
      {...props}
    />
  )
}

export function PanelHeader({
  title,
  description,
  action,
  id,
}: {
  title: ReactNode
  description?: ReactNode
  action?: ReactNode
  id?: string
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="flex flex-col gap-0.5">
        <h2 id={id} className="text-[17px] font-bold tracking-normal">
          {title}
        </h2>
        {description && <span className="text-[13px] text-muted-foreground">{description}</span>}
      </div>
      {action}
    </div>
  )
}

/** Uppercase section label (.h-sec). */
export function SectionLabel({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        'text-xs font-extrabold tracking-[0.05em] text-muted-foreground uppercase',
        className,
      )}
    >
      {children}
    </span>
  )
}

export function Breadcrumb({ workspaceId, current }: { workspaceId: string; current: string }) {
  return (
    <nav
      aria-label="Breadcrumb"
      className="flex items-center gap-2 text-[13px] font-semibold text-muted-foreground"
    >
      <Link
        to="/w/$workspaceId/team"
        params={{ workspaceId }}
        className="inline-flex items-center gap-1.5 text-brand-text no-underline hover:text-foreground"
      >
        <ChevronLeftIcon className="size-4" aria-hidden="true" />
        Team
      </Link>
      <span aria-hidden="true">/</span>
      <span aria-current="page" className="text-foreground">
        {current}
      </span>
    </nav>
  )
}

export function initials(name: string) {
  return name
    .split(/[\s@.]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase())
    .join('')
}

export function PersonAvatar({
  name,
  seed,
  size = 38,
  tone,
  className,
  children,
}: {
  name: string
  seed?: string
  size?: number
  tone?: RoleColor
  className?: string
  children?: ReactNode
}) {
  const t = TONE[tone ?? toneFor(seed ?? name)]
  return (
    <span
      aria-hidden="true"
      style={{ width: size, height: size, fontSize: size >= 60 ? 22 : size >= 38 ? 13 : 11 }}
      className={cn(
        'relative flex flex-none items-center justify-center rounded-full font-extrabold',
        t.soft,
        t.text,
        className,
      )}
    >
      {initials(name)}
      {children}
    </span>
  )
}

/** Small rounded label: role names, states, badges. */
export function Pill({
  tone = 'gray',
  children,
  className,
  square,
}: {
  tone?: RoleColor
  children: ReactNode
  className?: string
  square?: boolean
}) {
  const t = TONE[tone]
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 py-0.5 text-xs font-bold whitespace-nowrap',
        square ? 'rounded-md px-2' : 'rounded-full px-2.5',
        t.soft,
        t.text,
        className,
      )}
    >
      {children}
    </span>
  )
}

/** The prototype's pill switch (aria-pressed button). */
export function Toggle({
  on,
  onChange,
  label,
  disabled,
  title,
}: {
  on: boolean
  onChange: (on: boolean) => void
  label: string
  disabled?: boolean
  title?: string
}) {
  return (
    <button
      type="button"
      aria-pressed={on}
      aria-label={label}
      title={title}
      disabled={disabled}
      onClick={() => onChange(!on)}
      className={cn(
        'relative h-7 w-[46px] flex-none rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-50',
        on ? 'bg-primary' : 'bg-input/60',
      )}
    >
      <span
        className={cn(
          'absolute top-1 size-5 rounded-full bg-white shadow-[0_1px_2px_rgba(0,0,0,.25)] transition-[left]',
          on ? 'left-[22px]' : 'left-1',
        )}
      />
    </button>
  )
}

export function RadioDot({ checked }: { checked: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'mt-0.5 box-border size-[18px] flex-none rounded-full bg-card',
        checked ? 'border-[6px] border-primary' : 'border-2 border-input',
      )}
    />
  )
}

/**
 * A native radio drawn as anything: the input is visually hidden and the label carries the look,
 * so arrow keys, forms and screen readers behave as with any radio group.
 */
export function RadioOption({
  name,
  checked,
  onSelect,
  label,
  disabled,
  className,
  children,
}: {
  name: string
  checked: boolean
  onSelect: () => void
  label?: string
  disabled?: boolean
  className?: string
  children: ReactNode
}) {
  return (
    <label
      className={cn(
        'cursor-pointer has-[:disabled]:cursor-default has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring',
        className,
      )}
    >
      <input
        type="radio"
        className="sr-only"
        name={name}
        checked={checked}
        disabled={disabled}
        aria-label={label}
        onChange={onSelect}
      />
      {children}
    </label>
  )
}

/** A selectable card used as a radio (role, scope, reassign choices). */
export function RadioCard({
  name,
  checked,
  onSelect,
  children,
  className,
  disabled,
}: {
  name: string
  checked: boolean
  onSelect: () => void
  children: ReactNode
  className?: string
  disabled?: boolean
}) {
  return (
    <RadioOption
      name={name}
      checked={checked}
      onSelect={onSelect}
      disabled={disabled}
      className={cn(
        'flex items-start gap-3 rounded-lg border-2 px-3.5 py-3 text-left text-foreground has-[:disabled]:opacity-50',
        checked ? 'border-primary bg-brand-soft' : 'border-border bg-card hover:bg-muted',
        className,
      )}
    >
      <RadioDot checked={checked} />
      {children}
    </RadioOption>
  )
}

/** The prototype's modal: 18px radius, title + description, close icon. */
export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  width = 'sm:max-w-[560px]',
  icon,
}: {
  open: boolean
  onClose: () => void
  title: ReactNode
  description?: ReactNode
  children: ReactNode
  width?: string
  icon?: ReactNode
}) {
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        showCloseButton={false}
        // Without a description, don't point aria-describedby at a missing element.
        {...(description ? {} : { 'aria-describedby': undefined })}
        className={cn(
          'flex max-h-[calc(100dvh-64px)] flex-col gap-4 overflow-y-auto rounded-[18px] border-0 bg-card p-6',
          width,
        )}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            {icon}
            <div className="flex flex-col gap-0.5">
              <DialogTitle className="text-xl font-extrabold">{title}</DialogTitle>
              {description && (
                <DialogDescription className="text-[13px] text-muted-foreground">
                  {description}
                </DialogDescription>
              )}
            </div>
          </div>
          <button
            type="button"
            aria-label="Close"
            onClick={onClose}
            className="flex size-10 flex-none items-center justify-center rounded-[10px] text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <XIcon className="size-[18px]" aria-hidden="true" />
          </button>
        </div>
        {children}
      </DialogContent>
    </Dialog>
  )
}

/** Buttons in the prototype's sizes: lg 44px/12px radius, md 40px/10px, sm 34px. */
export function Btn({
  variant = 'primary',
  size = 'lg',
  className,
  type = 'button',
  ...props
}: ComponentProps<'button'> & {
  variant?: 'primary' | 'outline' | 'danger' | 'danger-outline' | 'danger-soft' | 'soft' | 'ghost'
  size?: 'lg' | 'md' | 'sm'
}) {
  return (
    <button
      type={type}
      className={cn(
        'inline-flex items-center justify-center gap-2 font-bold whitespace-nowrap no-underline disabled:cursor-not-allowed disabled:opacity-50',
        size === 'lg' && 'h-11 rounded-lg px-[18px]',
        size === 'md' && 'h-10 rounded-[10px] px-3.5 text-[13px]',
        size === 'sm' && 'h-[34px] rounded-[10px] px-3 text-[13px]',
        variant === 'primary' && 'bg-primary text-primary-foreground hover:bg-primary/90',
        variant === 'outline' && 'border bg-card text-foreground hover:bg-muted',
        variant === 'danger' &&
          'bg-destructive text-white hover:bg-destructive/90 dark:text-background',
        variant === 'danger-outline' &&
          'border border-destructive bg-card text-destructive hover:bg-destructive-soft',
        variant === 'danger-soft' && 'bg-destructive-soft text-destructive hover:opacity-80',
        variant === 'soft' && 'bg-brand-soft text-brand-text hover:opacity-80',
        variant === 'ghost' && 'bg-transparent text-brand-text hover:bg-muted',
        className,
      )}
      {...props}
    />
  )
}

export const inputClass =
  'h-11 w-full rounded-[10px] border border-input bg-muted px-3 text-base text-foreground placeholder:text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60 aria-invalid:border-destructive md:text-sm'

export const selectClass =
  'h-11 w-full rounded-[10px] border border-input bg-muted px-2.5 text-base text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60 md:text-sm'

export function ErrorText({ id, children }: { id?: string; children?: ReactNode }) {
  if (!children) return null
  return (
    <p id={id} role="alert" className="text-[13px] text-destructive">
      {children}
    </p>
  )
}

// ---------- formatting ----------

const MIN = 60_000

export function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function formatShortDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

export function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
}

export function formatDateTime(iso: string) {
  return `${formatShortDate(iso)}, ${formatTime(iso)}`
}

/** "Now", "12 min ago", "3 h ago", "Yesterday, 6:40 PM", "Oct 24". */
export function relativeTime(iso: string, now = Date.now()) {
  const t = Date.parse(iso)
  const diff = now - t
  if (diff < 2 * MIN) return 'Now'
  if (diff < 60 * MIN) return `${Math.round(diff / MIN)} min ago`
  const d = new Date(t)
  if (d.toDateString() === new Date(now).toDateString())
    return `${Math.round(diff / (60 * MIN))} h ago`
  if (d.toDateString() === new Date(now - 86_400_000).toDateString())
    return `Yesterday, ${formatTime(iso)}`
  return formatShortDate(iso)
}

/** "Chrome · macOS" from a user agent. */
export function deviceLabel(ua: string) {
  if (!ua) return ''
  const browser = /Edg\//.test(ua)
    ? 'Edge'
    : /Firefox\//.test(ua)
      ? 'Firefox'
      : /Chrome\//.test(ua)
        ? 'Chrome'
        : /Safari\//.test(ua)
          ? 'Safari'
          : 'Browser'
  const os = /iPhone|iPad/.test(ua)
    ? 'iPhone'
    : /Android/.test(ua)
      ? 'Android'
      : /Mac OS X/.test(ua)
        ? 'macOS'
        : /Windows/.test(ua)
          ? 'Windows'
          : /Linux/.test(ua)
            ? 'Linux'
            : ''
  return [browser, os].filter(Boolean).join(' · ')
}

// ---------- access log wording ----------

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
