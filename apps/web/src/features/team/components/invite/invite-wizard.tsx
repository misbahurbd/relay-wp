import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  ArrowRightIcon,
  CheckIcon,
  ChevronDownIcon,
  InfoIcon,
  SlidersHorizontalIcon,
  UploadIcon,
  XIcon,
} from 'lucide-react'
import { type ClipboardEvent, type KeyboardEvent, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { OptionSelect } from '#/components/ui/option-select'
import { Skeleton } from '#/components/ui/skeleton'
import { Textarea } from '#/components/ui/textarea'
import { numbersQueryOptions } from '#/features/onboarding'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { useInvite, useResendInvitation, useRevokeInvitation } from '../../api/team.mutations'
import {
  permissionsQueryOptions,
  rolesQueryOptions,
  seatsQueryOptions,
  teamsQueryOptions,
} from '../../api/team.queries'
import { EXPIRIES, GROUPS, PERMISSION_HINTS, parseEmails, SCOPES } from '../../schemas/team.schema'
import type { ChatScope, Invitation, Permission, Role } from '../../types'
import { PersonAvatar } from '../kit/avatar'
import { Btn } from '../kit/btn'
import { ErrorText } from '../kit/field'
import { formatShortDate } from '../kit/format'
import { SectionLabel } from '../kit/panel'
import { Pill } from '../kit/pill'
import { RadioOption } from '../kit/radio'
import { Toggle } from '../kit/toggle'

const STEPS = [
  { label: 'People', hint: 'Who to invite' },
  { label: 'Role', hint: 'What they can do' },
  { label: 'Access', hint: 'Teams and numbers' },
  { label: 'Review', hint: 'Check and send' },
]

type Expiry = (typeof EXPIRIES)[number]['value']

export function InviteWizard({ workspaceId }: { workspaceId: string }) {
  const roles = useQuery(rolesQueryOptions(workspaceId))
  const catalog = useQuery(permissionsQueryOptions())
  const seats = useQuery(seatsQueryOptions(workspaceId))
  const teams = useQuery(teamsQueryOptions(workspaceId))
  const numbers = useQuery(numbersQueryOptions('', workspaceId))
  const invite = useInvite(workspaceId)

  const [step, setStep] = useState(1)
  const [emails, setEmails] = useState<string[]>([])
  const [roleId, setRoleId] = useState<string>()
  const [toggles, setToggles] = useState<Record<string, boolean>>({})
  const [teamIds, setTeamIds] = useState<string[]>([])
  const [numberIds, setNumberIds] = useState<string[]>([])
  const [scope, setScope] = useState<ChatScope>()
  const [message, setMessage] = useState('')
  const [expiry, setExpiry] = useState<Expiry>('7d')
  const [twoFactor, setTwoFactor] = useState(false)
  const [error, setError] = useState<string>()
  const [sent, setSent] = useState<Invitation[] | null>(null)

  if (!roles.data || !catalog.data || !seats.data) {
    return (
      <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:px-7 sm:pt-6 sm:pb-10">
        <Skeleton className="h-16 w-full rounded-xl" />
        <Skeleton className="h-96 w-full rounded-xl" />
      </main>
    )
  }

  const choices = roles.data.filter((r) => r.system_key !== 'owner')
  const role =
    choices.find((r) => r.id === roleId) ??
    choices.find((r) => r.system_key === 'agent') ??
    choices[0]
  const rolePerms = new Set(role?.permissions ?? [])
  const isOn = (k: string) => toggles[k] ?? rolePerms.has(k)
  const overrides = catalog.data
    .filter((p) => toggles[p.key] !== undefined && toggles[p.key] !== rolePerms.has(p.key))
    .map((p) => ({
      permission_key: p.key,
      effect: toggles[p.key] ? ('grant' as const) : ('deny' as const),
    }))
  const chatScope = scope ?? role?.chat_scope ?? 'assigned'
  const taken = seats.data.used + seats.data.reserved
  const adding = role?.uses_seat ? emails.length : 0
  const after = taken + adding
  const limit = seats.data.limit
  // A negative limit: the plan has no member cap.
  const over = limit < 0 ? 0 : Math.max(0, after - limit)

  function next() {
    setError(undefined)
    if (step === 1 && emails.length === 0) {
      setError('Add at least one email address.')
      return
    }
    if (step === 1 && over > 0) {
      setError(`That’s ${over} more than your plan’s ${limit} seats. Remove someone or add seats.`)
      return
    }
    if (step < 4) {
      setStep(step + 1)
      return
    }
    if (!role) return
    invite.mutate(
      {
        emails,
        role_id: role.id,
        team_ids: teamIds,
        number_ids: numberIds,
        chat_scope: scope && scope !== role.chat_scope ? scope : undefined,
        overrides: overrides.length ? overrides : undefined,
        personal_message: message.trim() || undefined,
        expires_in: expiry,
        require_2fa: twoFactor,
      },
      {
        onSuccess: (list) => {
          setSent(list)
          toast.success(list.length === 1 ? 'Invite sent' : `${list.length} invites sent`)
        },
        onError: (e) => setError(errorMessage(e)),
      },
    )
  }

  function reset() {
    setSent(null)
    setStep(1)
    setEmails([])
    setToggles({})
    setMessage('')
    setError(undefined)
  }

  const teamNames = (teams.data ?? []).filter((t) => teamIds.includes(t.id)).map((t) => t.name)
  const numberNames = (numbers.data ?? [])
    .filter((n) => numberIds.includes(n.id))
    .map((n) => n.verified_name)

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:px-7 sm:pt-6 sm:pb-10">
      <div className="flex flex-col gap-1.5">
        <nav
          aria-label="Breadcrumb"
          className="flex items-center gap-1.5 text-[13px] text-muted-foreground"
        >
          <Link to="/team" className="font-bold">
            Team
          </Link>
          <span aria-hidden="true">/</span>
          <span aria-current="page">Invite teammates</span>
        </nav>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Invite teammates</h1>
          <Link
            to="/team"
            className="inline-flex h-10 items-center gap-1.5 rounded-[10px] border bg-card px-3.5 font-bold text-foreground no-underline hover:bg-muted"
          >
            <XIcon className="size-4" aria-hidden="true" />
            {sent ? 'Close' : 'Cancel'}
          </Link>
        </div>
      </div>

      {!sent && (
        <ol
          aria-label="Invite steps"
          className="grid grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-1.5 rounded-[14px] border bg-card p-1.5"
        >
          {STEPS.map((s, i) => {
            const n = i + 1
            const current = n === step
            const done = n < step
            return (
              <li key={s.label} className="flex">
                <Button
                  variant="unstyled"
                  type="button"
                  aria-current={current ? 'step' : undefined}
                  disabled={n > step}
                  onClick={() => setStep(n)}
                  className={cn(
                    'flex min-h-12 flex-1 items-center gap-2.5 rounded-[10px] px-3 py-1.5 text-left text-foreground disabled:cursor-default',
                    current ? 'bg-brand-soft' : 'bg-transparent',
                  )}
                >
                  <span
                    className={cn(
                      'flex size-7 flex-none items-center justify-center rounded-full text-[13px] font-extrabold',
                      current || done
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-neutral-soft text-neutral',
                    )}
                  >
                    {done ? <CheckIcon className="size-4" aria-label="Done" /> : n}
                  </span>
                  <span className="flex flex-col leading-[1.25]">
                    <span className="font-bold">{s.label}</span>
                    <span className="text-xs text-muted-foreground">{s.hint}</span>
                  </span>
                </Button>
              </li>
            )
          })}
        </ol>
      )}

      <div className="flex flex-wrap items-start gap-5">
        <section className="flex min-w-0 flex-[2_1_560px] flex-col gap-5 rounded-xl border bg-card p-5 shadow-card sm:p-6">
          {sent ? (
            <Done workspaceId={workspaceId} sent={sent} expiry={expiry} onMore={reset} />
          ) : (
            <>
              {step === 1 && (
                <PeopleStep
                  emails={emails}
                  onEmails={setEmails}
                  seats={seats.data}
                  adding={adding}
                  usesSeat={!!role?.uses_seat}
                />
              )}
              {step === 2 && (
                <RoleStep
                  roles={choices}
                  role={role}
                  catalog={catalog.data}
                  isOn={isOn}
                  changed={overrides.length}
                  onRole={(id) => {
                    setRoleId(id)
                    setToggles({})
                    setScope(undefined)
                  }}
                  onToggle={(k, v) => setToggles({ ...toggles, [k]: v })}
                  onReset={(k) => {
                    const { [k]: _, ...rest } = toggles
                    setToggles(rest)
                  }}
                />
              )}
              {step === 3 && (
                <div className="flex flex-col gap-[22px]">
                  <StepTitle
                    title="What can they reach?"
                    text="Pick their teams and WhatsApp numbers, then which chats they can open."
                  />
                  <fieldset className="flex flex-col gap-2.5">
                    <legend className="mb-2.5">
                      <SectionLabel>Teams</SectionLabel>
                    </legend>
                    {teams.data?.length === 0 && (
                      <span className="text-sm text-muted-foreground">
                        No teams yet. You can add them to teams later.
                      </span>
                    )}
                    <div className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-2.5">
                      {teams.data?.map((t) => (
                        <CheckCard
                          key={t.id}
                          on={teamIds.includes(t.id)}
                          onChange={(on) =>
                            setTeamIds(on ? [...teamIds, t.id] : teamIds.filter((x) => x !== t.id))
                          }
                          title={t.name}
                          meta={`${t.member_ids.length} ${t.member_ids.length === 1 ? 'member' : 'members'}`}
                        />
                      ))}
                    </div>
                  </fieldset>
                  <fieldset className="flex flex-col gap-2.5">
                    <legend className="mb-2.5">
                      <SectionLabel>WhatsApp numbers</SectionLabel>
                    </legend>
                    {numbers.data?.length === 0 && (
                      <span className="text-sm text-muted-foreground">
                        No WhatsApp numbers are connected yet.
                      </span>
                    )}
                    <div className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-2.5">
                      {numbers.data?.map((n) => (
                        <CheckCard
                          key={n.id}
                          on={numberIds.includes(n.id)}
                          onChange={(on) =>
                            setNumberIds(
                              on ? [...numberIds, n.id] : numberIds.filter((x) => x !== n.id),
                            )
                          }
                          title={n.verified_name}
                          meta={n.display_phone_number}
                          dot={n.status === 'connected'}
                        />
                      ))}
                    </div>
                    {numbers.data && numbers.data.length > 0 && (
                      <span className="text-xs text-muted-foreground">
                        None ticked means every number their teams use.
                      </span>
                    )}
                  </fieldset>
                  <fieldset className="flex flex-col gap-2.5">
                    <legend className="mb-2.5">
                      <SectionLabel>Which chats can they open?</SectionLabel>
                    </legend>
                    <div
                      role="radiogroup"
                      aria-label="Chat access"
                      className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-2.5"
                    >
                      {SCOPES.map((s) => {
                        const checked = chatScope === s.value
                        return (
                          <RadioOption
                            key={s.value}
                            name="invite-scope"
                            checked={checked}
                            onSelect={() => setScope(s.value)}
                            className={cn(
                              'flex flex-col gap-1.5 rounded-lg border-2 p-3.5 text-left text-foreground',
                              checked
                                ? 'border-primary bg-brand-soft'
                                : 'border-border bg-card hover:bg-muted',
                            )}
                          >
                            <span className="flex items-center gap-2">
                              <Dot checked={checked} />
                              <span className="font-extrabold">{s.label}</span>
                            </span>
                            <span className="text-[13px] text-muted-foreground">
                              {s.desc}
                              {s.value === role?.chat_scope && ' The role’s default.'}
                            </span>
                          </RadioOption>
                        )
                      })}
                    </div>
                  </fieldset>
                </div>
              )}
              {step === 4 && role && (
                <div className="flex flex-col gap-[18px]">
                  <StepTitle
                    title="Review and send"
                    text="Each person gets an email with a private link to join this workspace."
                  />
                  <div className="overflow-x-auto rounded-lg border">
                    <table className="w-full min-w-[640px] border-collapse text-[13px]">
                      <thead>
                        <tr className="bg-muted text-left text-xs text-muted-foreground">
                          <th className="px-3.5 py-2.5 font-bold">Invitee</th>
                          <th className="p-2.5 font-bold">Role</th>
                          <th className="p-2.5 font-bold">Teams</th>
                          <th className="p-2.5 font-bold">Numbers</th>
                          <th className="p-2.5 font-bold">Chats</th>
                          <th className="px-3.5 py-2.5 font-bold">Seat</th>
                        </tr>
                      </thead>
                      <tbody>
                        {emails.map((e) => (
                          <tr key={e} className="border-t">
                            <td className="px-3.5 py-3 font-bold">{e}</td>
                            <td className="px-2.5 py-3">
                              <Pill tone={role.color}>
                                {role.name}
                                {overrides.length > 0 && ' (customized)'}
                              </Pill>
                            </td>
                            <td className="px-2.5 py-3">{teamNames.join(', ') || '—'}</td>
                            <td className="px-2.5 py-3">
                              {numberNames.join(', ') || 'Their teams’ numbers'}
                            </td>
                            <td className="px-2.5 py-3">
                              {SCOPES.find((s) => s.value === chatScope)?.label}
                            </td>
                            <td className="px-3.5 py-3 text-muted-foreground">
                              {role.uses_seat ? 'Uses 1 seat' : 'Free'}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-[13px] font-bold">
                      Personal message{' '}
                      <span className="font-medium text-muted-foreground">
                        (optional, shown in the email)
                      </span>
                    </span>
                    <Textarea
                      unstyled
                      rows={3}
                      maxLength={500}
                      value={message}
                      onChange={(e) => setMessage(e.target.value)}
                      placeholder="Hi! I’m adding you to our WhatsApp inbox so you can help with orders and returns."
                      className="resize-y rounded-[10px] border bg-card p-3 text-base text-foreground placeholder:text-muted-foreground md:text-sm"
                    />
                  </label>
                  <div className="flex flex-wrap gap-3.5">
                    <label className="flex flex-[1_1_200px] flex-col gap-1.5">
                      <span className="text-[13px] font-bold">Invite link expires after</span>
                      <OptionSelect
                        className="bg-muted"
                        value={expiry}
                        onValueChange={(v) => setExpiry(v as Expiry)}
                        options={EXPIRIES.map((x) => ({ value: x.value, label: x.label }))}
                      />
                    </label>
                    <label className="flex min-h-11 flex-[1_1_240px] cursor-pointer items-center gap-2.5 self-end text-[13px]">
                      <Input
                        unstyled
                        type="checkbox"
                        className="size-[18px] accent-primary"
                        checked={twoFactor}
                        onChange={(e) => setTwoFactor(e.target.checked)}
                      />
                      Require two-factor sign-in for these people
                    </label>
                  </div>
                </div>
              )}
              <ErrorText>{error}</ErrorText>
              <div className="flex flex-wrap items-center justify-between gap-2.5 border-t pt-4">
                <Btn
                  variant="outline"
                  className="rounded-[10px] px-4"
                  disabled={step === 1}
                  onClick={() => setStep(step - 1)}
                >
                  Back
                </Btn>
                <span className="text-[13px] text-muted-foreground">Step {step} of 4</span>
                <Btn className="rounded-[10px] px-5" disabled={invite.isPending} onClick={next}>
                  {step < 4
                    ? 'Continue'
                    : invite.isPending
                      ? 'Sending…'
                      : emails.length === 1
                        ? 'Send invite'
                        : `Send ${emails.length} invites`}
                  {step < 4 && <ArrowRightIcon className="size-4" aria-hidden="true" />}
                </Btn>
              </div>
            </>
          )}
        </section>

        {!sent && role && (
          <Summary
            role={role}
            catalog={catalog.data}
            isOn={isOn}
            count={emails.length}
            teams={teamNames}
            numbers={numberNames}
            scope={chatScope}
            after={after}
            limit={seats.data.limit}
          />
        )}
      </div>
    </main>
  )
}

function StepTitle({ title, text }: { title: string; text: string }) {
  return (
    <div className="flex flex-col gap-1">
      <h2 className="text-xl font-extrabold">{title}</h2>
      <p className="text-muted-foreground">{text}</p>
    </div>
  )
}

function Dot({ checked }: { checked: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex size-[18px] flex-none items-center justify-center rounded-full border-2',
        checked ? 'border-primary' : 'border-input',
      )}
    >
      {checked && <span className="size-2 rounded-full bg-primary" />}
    </span>
  )
}

function CheckCard({
  on,
  onChange,
  title,
  meta,
  dot,
}: {
  on: boolean
  onChange: (on: boolean) => void
  title: string
  meta: string
  dot?: boolean
}) {
  return (
    <label
      className={cn(
        'flex min-h-14 cursor-pointer items-center gap-3 rounded-lg border-2 px-3.5 py-2.5',
        on ? 'border-primary bg-brand-soft' : 'border-border bg-card hover:bg-muted',
      )}
    >
      <Input
        unstyled
        type="checkbox"
        className="size-[18px] accent-primary"
        checked={on}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span className="flex flex-1 flex-col leading-[1.3]">
        <span className="font-bold">{title}</span>
        <span className="text-xs text-muted-foreground">{meta}</span>
      </span>
      {dot && <span aria-hidden="true" className="size-2 rounded-full bg-brand" />}
    </label>
  )
}

// ---------- Step 1 ----------

function PeopleStep({
  emails,
  onEmails,
  seats,
  adding,
  usesSeat,
}: {
  emails: string[]
  onEmails: (e: string[]) => void
  seats: { used: number; reserved: number; limit: number }
  adding: number
  usesSeat: boolean
}) {
  const [draft, setDraft] = useState('')
  const [invalid, setInvalid] = useState<string[]>([])
  const file = useRef<HTMLInputElement>(null)

  function add(text: string) {
    const { valid, invalid: bad } = parseEmails(text)
    onEmails([...emails, ...valid.filter((e) => !emails.includes(e))])
    setInvalid(bad)
    setDraft(bad.join(', '))
  }
  function onKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      if (draft.trim()) add(draft)
    } else if (e.key === 'Backspace' && !draft && emails.length) {
      onEmails(emails.slice(0, -1))
    }
  }
  function onPaste(e: ClipboardEvent<HTMLInputElement>) {
    const text = e.clipboardData.getData('text')
    if (/[\s,;]/.test(text.trim())) {
      e.preventDefault()
      add(text)
    }
  }

  const taken = seats.used + seats.reserved
  const after = taken + adding
  // A negative limit: the plan has no member cap, so there is no meter.
  const unlimited = seats.limit < 0
  const over = unlimited ? 0 : after - seats.limit
  const tone = over > 0 ? 'red' : !unlimited && after === seats.limit ? 'amber' : 'green'
  const title = unlimited
    ? 'No seat limit on your plan'
    : over > 0
      ? `${over} over your seat limit`
      : after === seats.limit
        ? 'This fills your last seats'
        : `Room for ${seats.limit - after} more`
  const cells = Math.max(seats.limit, after)

  return (
    <div className="flex flex-col gap-[18px]">
      <StepTitle
        title="Who are you inviting?"
        text="Add one or more work emails. Paste a list separated by commas or new lines."
      />
      <div className="flex flex-col gap-2">
        <label htmlFor="inv-email" className="text-[13px] font-bold">
          Email addresses
        </label>
        <div className="flex min-h-[52px] flex-wrap items-center gap-2 rounded-lg border bg-muted p-2">
          {emails.map((e) => (
            <span
              key={e}
              className="inline-flex h-8 items-center gap-1.5 rounded-full border bg-card pr-1 pl-2.5 text-[13px] font-semibold"
            >
              {e}
              <Button
                variant="unstyled"
                type="button"
                aria-label={`Remove ${e}`}
                onClick={() => onEmails(emails.filter((x) => x !== e))}
                className="flex size-6 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground"
              >
                <XIcon className="size-3.5" aria-hidden="true" />
              </Button>
            </span>
          ))}
          <Input
            unstyled
            id="inv-email"
            type="email"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={onKey}
            onPaste={onPaste}
            onBlur={() => draft.trim() && add(draft)}
            placeholder="name@company.com"
            aria-invalid={invalid.length > 0}
            aria-describedby={invalid.length ? 'inv-email-error' : undefined}
            className="h-8 min-w-0 flex-[1_1_180px] border-0 bg-transparent px-1 text-base text-foreground outline-none placeholder:text-muted-foreground md:text-sm"
          />
          <Button
            variant="unstyled"
            type="button"
            onClick={() => draft.trim() && add(draft)}
            className="h-[34px] rounded-lg bg-brand-soft px-3 font-bold text-brand-text hover:opacity-80"
          >
            Add
          </Button>
        </div>
        <ErrorText id="inv-email-error">
          {invalid.length > 0 &&
            `Check ${invalid.length === 1 ? 'this address' : 'these addresses'}: ${invalid.join(', ')}`}
        </ErrorText>
        <div className="flex flex-wrap items-center gap-3.5 text-[13px]">
          <Button
            variant="unstyled"
            type="button"
            onClick={() => file.current?.click()}
            className="inline-flex items-center gap-1.5 font-bold text-brand-text underline"
          >
            <UploadIcon className="size-4" aria-hidden="true" />
            Import from CSV
          </Button>
          <Input
            unstyled
            ref={file}
            type="file"
            accept=".csv,text/csv,text/plain"
            className="sr-only"
            tabIndex={-1}
            aria-hidden="true"
            onChange={async (e) => {
              const f = e.target.files?.[0]
              if (f) add(await f.text())
              e.target.value = ''
            }}
          />
          <span className="text-muted-foreground">
            {emails.length} {emails.length === 1 ? 'person' : 'people'} added
          </span>
        </div>
      </div>

      <div
        className={cn(
          'flex flex-col gap-2.5 rounded-lg p-4',
          tone === 'red'
            ? 'bg-destructive-soft'
            : tone === 'amber'
              ? 'bg-warning-soft'
              : 'bg-brand-soft',
        )}
      >
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span
            className={cn(
              'font-extrabold',
              tone === 'red'
                ? 'text-destructive'
                : tone === 'amber'
                  ? 'text-warning'
                  : 'text-brand-text',
            )}
          >
            {title}
          </span>
          {!unlimited && (
            <span className="text-[13px] text-muted-foreground">
              {seats.limit} seats on your plan
            </span>
          )}
        </div>
        {unlimited ? null : cells <= 30 ? (
          <div
            aria-hidden="true"
            className="grid gap-1"
            style={{ gridTemplateColumns: `repeat(${cells}, minmax(0, 1fr))` }}
          >
            {Array.from({ length: cells }, (_, i) => (
              <span
                // biome-ignore lint/suspicious/noArrayIndexKey: seat cells have no identity
                key={i}
                className={cn(
                  'h-2 rounded-[4px]',
                  i >= seats.limit
                    ? 'bg-destructive'
                    : i < taken
                      ? 'bg-brand'
                      : i < after
                        ? 'bg-info'
                        : 'bg-card',
                )}
              />
            ))}
          </div>
        ) : (
          <div aria-hidden="true" className="h-2 rounded-[4px] bg-card">
            <div
              className={cn('h-2 rounded-[4px]', over > 0 ? 'bg-destructive' : 'bg-brand')}
              style={{ width: `${Math.min(100, (after / seats.limit) * 100)}%` }}
            />
          </div>
        )}
        <span className="text-[13px]">
          {taken} used + {adding} new = {after}
          {unlimited ? ' seats.' : ` of ${seats.limit} seats.`}
        </span>
        <span className="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground">
          <InfoIcon className="size-4" aria-hidden="true" />
          {usesSeat ? 'Viewers don’t use a seat.' : 'This role is free and doesn’t use a seat.'}
        </span>
      </div>
    </div>
  )
}

// ---------- Step 2 ----------

function RoleStep({
  roles,
  role,
  catalog,
  isOn,
  changed,
  onRole,
  onToggle,
  onReset,
}: {
  roles: Role[]
  role?: Role
  catalog: Permission[]
  isOn: (k: string) => boolean
  changed: number
  onRole: (id: string) => void
  onToggle: (k: string, v: boolean) => void
  onReset: (k: string) => void
}) {
  const [open, setOpen] = useState(changed > 0)
  const label = (k: string) => catalog.find((p) => p.key === k)?.label ?? k
  const order = new Map(GROUPS.map((g, i) => [g.key, i]))
  const perms = [...catalog].sort((a, b) => (order.get(a.group) ?? 0) - (order.get(b.group) ?? 0))
  return (
    <div className="flex flex-col gap-[18px]">
      <div className="flex flex-wrap items-end justify-between gap-2.5">
        <StepTitle
          title="Choose a role"
          text="A role is a set of permissions. Everyone in this invite gets the same role."
        />
        <Link to="/team/roles" className="text-[13px] font-bold">
          Manage roles
        </Link>
      </div>
      <div
        role="radiogroup"
        aria-label="Role"
        className="grid grid-cols-[repeat(auto-fit,minmax(230px,1fr))] gap-3"
      >
        {roles.map((r) => {
          const checked = r.id === role?.id
          const scope = SCOPES.find((s) => s.value === r.chat_scope)?.label
          const bullets = [`Sees ${scope?.toLowerCase()}`, ...r.permissions.slice(0, 3).map(label)]
          return (
            <RadioOption
              key={r.id}
              name="invite-role"
              checked={checked}
              onSelect={() => onRole(r.id)}
              className={cn(
                'flex flex-col gap-2.5 rounded-[14px] border-2 p-4 text-left text-foreground',
                checked ? 'border-primary bg-brand-soft' : 'border-border bg-card hover:bg-muted',
              )}
            >
              <span className="flex w-full items-center gap-2.5">
                <span
                  aria-hidden="true"
                  className={cn(
                    'flex size-5 flex-none items-center justify-center rounded-full border-2',
                    checked ? 'border-primary' : 'border-input',
                  )}
                >
                  {checked && <span className="size-2.5 rounded-full bg-primary" />}
                </span>
                <span className="flex-1 text-[15px] font-extrabold">{r.name}</span>
                <Pill tone={r.system_key ? r.color : 'gray'} className="px-2 text-[11px]">
                  {r.system_key ? (r.uses_seat ? 'System' : 'Free') : 'Custom'}
                </Pill>
              </span>
              <span className="flex flex-col gap-1.5 text-[13px] text-muted-foreground">
                {bullets.map((b) => (
                  <span key={b} className="flex gap-1.5">
                    <CheckIcon
                      className="mt-0.5 size-3.5 flex-none text-brand-text"
                      aria-hidden="true"
                    />
                    {b}
                  </span>
                ))}
                {r.permissions.length > 3 && (
                  <span className="pl-5">+{r.permissions.length - 3} more</span>
                )}
              </span>
              <span className="text-xs font-bold text-muted-foreground">
                {r.uses_seat ? 'Uses a seat' : 'Free, no seat'}
              </span>
            </RadioOption>
          )
        })}
      </div>

      <div className="rounded-lg border">
        <Button
          variant="unstyled"
          type="button"
          aria-expanded={open}
          onClick={() => setOpen(!open)}
          className="flex min-h-[52px] w-full items-center gap-2.5 rounded-lg px-4 text-left text-foreground"
        >
          <SlidersHorizontalIcon className="size-[18px] text-muted-foreground" aria-hidden="true" />
          <span className="flex-1 font-bold">Customize permissions for this invite</span>
          {changed > 0 && (
            <span className="rounded-full bg-warning-soft px-2 py-0.5 text-[11px] font-extrabold text-warning">
              {changed} changed
            </span>
          )}
          <ChevronDownIcon
            className={cn('size-[18px] transition-transform', open && 'rotate-180')}
            aria-hidden="true"
          />
        </Button>
        {open && (
          <div className="flex flex-col px-4 pb-3">
            <p className="mb-2 text-[13px] text-muted-foreground">
              Changes apply only to these people. The {role?.name} role itself stays the same.
            </p>
            {perms.map((p) => {
              const on = isOn(p.key)
              const base = role?.permissions.includes(p.key) ?? false
              return (
                <div key={p.key} className="flex items-center gap-3 border-t py-2.5">
                  <span className="flex flex-1 flex-col leading-[1.3]">
                    <span className="font-semibold">{p.label}</span>
                    <span className="text-xs text-muted-foreground">{PERMISSION_HINTS[p.key]}</span>
                  </span>
                  {on !== base && (
                    <Button
                      variant="unstyled"
                      type="button"
                      onClick={() => onReset(p.key)}
                      className="h-7 rounded-md px-2 text-xs font-bold text-muted-foreground underline"
                    >
                      Reset
                    </Button>
                  )}
                  <Toggle on={on} label={p.label} onChange={(v) => onToggle(p.key, v)} />
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}

// ---------- Aside ----------

function Summary({
  role,
  catalog,
  isOn,
  count,
  teams,
  numbers,
  scope,
  after,
  limit,
}: {
  role: Role
  catalog: Permission[]
  isOn: (k: string) => boolean
  count: number
  teams: string[]
  numbers: string[]
  scope: ChatScope
  after: number
  limit: number
}) {
  const yes = catalog.filter((p) => isOn(p.key))
  const no = catalog.filter((p) => !isOn(p.key))
  const scopeLabel = SCOPES.find((s) => s.value === scope)?.label
  return (
    <aside
      aria-label="What they will be able to do"
      className="flex min-w-0 flex-[1_1_300px] flex-col gap-3.5 rounded-xl border bg-card p-5 shadow-card"
    >
      <div className="flex flex-col gap-0.5">
        <h2 className="text-[17px] font-extrabold">They will be able to…</h2>
        <span className="text-[13px] text-muted-foreground">
          {count} {count === 1 ? 'person' : 'people'} ·{' '}
          <strong className="text-foreground">{role.name}</strong>
        </span>
      </div>
      <ul className="flex flex-col gap-2 text-[13px]">
        <li className="flex items-center gap-2.5">
          <span className="flex size-5 flex-none items-center justify-center rounded-full bg-brand-soft text-brand-text">
            <CheckIcon className="size-3" aria-hidden="true" />
          </span>
          Read chats: {scopeLabel?.toLowerCase()}
        </li>
        {yes.map((p) => (
          <li key={p.key} className="flex items-center gap-2.5">
            <span className="flex size-5 flex-none items-center justify-center rounded-full bg-brand-soft text-brand-text">
              <CheckIcon className="size-3" aria-hidden="true" />
            </span>
            {p.label}
          </li>
        ))}
      </ul>
      {no.length > 0 && (
        <div className="flex flex-col gap-2 border-t pt-3.5">
          <SectionLabel>Not allowed</SectionLabel>
          <ul className="flex flex-col gap-2 text-[13px] text-muted-foreground">
            {no.map((p) => (
              <li key={p.key} className="flex items-center gap-2.5">
                <span className="flex size-5 flex-none items-center justify-center rounded-full bg-neutral-soft text-neutral">
                  <XIcon className="size-3" aria-hidden="true" />
                </span>
                {p.label}
              </li>
            ))}
          </ul>
        </div>
      )}
      <dl className="flex flex-col gap-2.5 rounded-lg bg-muted px-3 py-3 text-[13px]">
        <div className="flex justify-between gap-3">
          <dt className="text-muted-foreground">Teams</dt>
          <dd className="text-right font-bold">{teams.join(', ') || 'None yet'}</dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-muted-foreground">Numbers</dt>
          <dd className="text-right font-bold">{numbers.join(', ') || 'Their teams’ numbers'}</dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-muted-foreground">Chats</dt>
          <dd className="text-right font-bold">{scopeLabel}</dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-muted-foreground">Seats after invite</dt>
          <dd
            className={cn(
              'font-bold',
              limit >= 0 && after > limit
                ? 'text-destructive'
                : after === limit
                  ? 'text-warning'
                  : 'text-foreground',
            )}
          >
            {limit < 0 ? after : `${after} of ${limit}`}
          </dd>
        </div>
      </dl>
      <p className="text-xs text-muted-foreground">
        You can change anyone’s role or remove access later from the Team page. Removing someone
        signs them out right away.
      </p>
    </aside>
  )
}

// ---------- Done ----------

function Done({
  workspaceId,
  sent,
  expiry,
  onMore,
}: {
  workspaceId: string
  sent: Invitation[]
  expiry: Expiry
  onMore: () => void
}) {
  const [revoked, setRevoked] = useState<string[]>([])
  const days = EXPIRIES.find((x) => x.value === expiry)?.label
  return (
    <div className="flex flex-col gap-[18px]">
      <div className="flex items-center gap-3.5 rounded-[14px] bg-brand-soft p-4">
        <span className="flex size-11 flex-none items-center justify-center rounded-full bg-primary text-primary-foreground">
          <CheckIcon className="size-5" aria-hidden="true" />
        </span>
        <span className="flex flex-col leading-[1.35]">
          <span className="text-[17px] font-extrabold text-brand-text">
            {sent.length === 1 ? 'Invite sent' : `${sent.length} invites sent`}
          </span>
          <span className="text-muted-foreground">
            They’ll show as Pending on the Team page until they accept. Links expire in {days}.
          </span>
        </span>
      </div>
      <SectionLabel>Pending invites</SectionLabel>
      <div className="flex flex-col gap-2.5">
        {sent.map((i) => (
          <PendingRow
            key={i.id}
            workspaceId={workspaceId}
            invite={i}
            revoked={revoked.includes(i.id)}
            onRevoked={() => setRevoked([...revoked, i.id])}
          />
        ))}
      </div>
      <div className="flex flex-wrap gap-2.5">
        <Link
          to="/team"
          className="inline-flex h-11 items-center rounded-[10px] bg-primary px-[18px] font-bold text-primary-foreground no-underline hover:bg-primary/90 hover:text-primary-foreground"
        >
          Back to Team
        </Link>
        <Btn variant="outline" className="rounded-[10px] px-4" onClick={onMore}>
          Invite more people
        </Btn>
      </div>
    </div>
  )
}

function PendingRow({
  workspaceId,
  invite,
  revoked,
  onRevoked,
}: {
  workspaceId: string
  invite: Invitation
  revoked: boolean
  onRevoked: () => void
}) {
  const resend = useResendInvitation(workspaceId)
  const revoke = useRevokeInvitation(workspaceId)
  const [confirming, setConfirming] = useState(false)
  return (
    <div className="flex flex-col gap-2.5 rounded-lg border bg-card p-3.5">
      <div className="flex flex-wrap items-center gap-3">
        <PersonAvatar name={invite.email} tone="gray" size={40} />
        <span className="flex min-w-0 flex-[1_1_200px] flex-col leading-[1.3]">
          <span className="font-bold">{invite.email}</span>
          <span className="text-xs text-muted-foreground">
            {invite.role_name} · expires {formatShortDate(invite.expires_at)}
          </span>
        </span>
        <Pill tone={revoked ? 'gray' : 'amber'} className="py-[3px]">
          {revoked ? 'Revoked' : 'Pending'}
        </Pill>
        {!revoked && (
          <span className="flex flex-wrap gap-1.5">
            <Btn
              variant="outline"
              size="sm"
              className="h-9 rounded-lg font-semibold"
              disabled={resend.isPending}
              onClick={() =>
                resend.mutate(invite.id, {
                  onSuccess: () => toast.success(`New link sent to ${invite.email}`),
                  onError: (e) => toast.error(errorMessage(e)),
                })
              }
            >
              Resend
            </Btn>
            <Btn
              variant="danger-outline"
              size="sm"
              className="h-9 rounded-lg"
              onClick={() => setConfirming(true)}
            >
              Revoke invite
            </Btn>
          </span>
        )}
      </div>
      {confirming && !revoked && (
        <div
          role="alertdialog"
          aria-label="Confirm revoke"
          className="flex flex-wrap items-center gap-2.5 rounded-[10px] bg-destructive-soft p-3"
        >
          <span className="flex-[1_1_220px] text-[13px]">
            Revoke the invite for <strong>{invite.email}</strong>? The link stops working right away
            and the seat is freed.
          </span>
          <Btn
            variant="outline"
            size="sm"
            className="h-9 rounded-lg"
            onClick={() => setConfirming(false)}
          >
            Keep invite
          </Btn>
          <Btn
            variant="danger"
            size="sm"
            className="h-9 rounded-lg"
            disabled={revoke.isPending}
            onClick={() =>
              revoke.mutate(invite.id, {
                onSuccess: () => {
                  toast.success(`Invite for ${invite.email} revoked`)
                  onRevoked()
                },
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
          >
            Revoke
          </Btn>
        </div>
      )}
    </div>
  )
}
