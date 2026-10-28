import { useNavigate } from '@tanstack/react-router'
import {
  BotIcon,
  CheckIcon,
  ClockIcon,
  MegaphoneIcon,
  MessageCircleIcon,
  PartyPopperIcon,
} from 'lucide-react'
import { useState } from 'react'
import { Logo } from '#/components/layout/logo'
import { ThemeToggle } from '#/components/layout/theme-toggle'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { ToggleGroup, ToggleGroupItem } from '#/components/ui/toggle-group'
import { SignOutButton } from '#/features/auth'
import type { FirstGoal, StartMode } from '#/features/workspaces'
import { useCreateWorkspace } from '#/features/workspaces'
import { errorMessage } from '#/lib/api'
import { FIRST_GOALS } from '../schemas/onboarding'

const SETUP_STEPS = [
  { title: 'Business details', body: 'Your business name, industry and goals', time: '1 min' },
  {
    title: 'Connect WhatsApp',
    body: 'Link your number through Meta’s secure window',
    time: '3 min',
  },
  {
    title: 'Send a test message',
    body: 'See your first message arrive on your phone',
    time: '1 min',
  },
]

const GOAL_ICONS = { offers: MegaphoneIcon, support: MessageCircleIcon, automate: BotIcon }

type Props = { userId: string; firstName: string; email: string }

export function WelcomeScreen({ userId, firstName, email }: Props) {
  const navigate = useNavigate()
  const create = useCreateWorkspace(userId)
  const [goal, setGoal] = useState<FirstGoal>('offers')
  const [starting, setStarting] = useState<StartMode | null>(null)
  const note = FIRST_GOALS.find((g) => g.id === goal)?.note

  const start = (mode: StartMode) => {
    setStarting(mode)
    create.mutate(
      { first_goal: goal, start_mode: mode, name: `${firstName}’s workspace` },
      {
        onSuccess: (ws) =>
          navigate(
            mode === 'setup'
              ? { to: '/w/$workspaceId/setup', params: { workspaceId: ws.id } }
              : { to: '/w/$workspaceId', params: { workspaceId: ws.id } },
          ),
        onSettled: () => setStarting(null),
      },
    )
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="flex items-center justify-between gap-4 border-b bg-card px-4 py-3 sm:px-8">
        <Logo />
        <div className="flex items-center gap-3">
          <span className="hidden text-sm text-muted-foreground sm:inline">
            Signed in as {email}
          </span>
          <ThemeToggle />
          <SignOutButton />
        </div>
      </header>
      <main className="mx-auto grid w-full max-w-6xl flex-1 items-center gap-10 px-4 py-10 sm:px-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.9fr)]">
        <section className="flex flex-col gap-7">
          <div className="flex flex-col gap-3">
            <span className="flex w-fit items-center gap-2 rounded-full bg-brand-soft px-3 py-1 text-sm font-bold text-brand-text">
              <PartyPopperIcon className="size-4" aria-hidden="true" />
              Your 14-day free trial has started. No card needed.
            </span>
            <h1 className="text-[32px] leading-tight">Welcome to Relay, {firstName}</h1>
            <p className="max-w-prose text-[15px] text-muted-foreground">
              Here’s what we’ll set up together. It takes about 5 minutes, and you can stop and come
              back anytime.
            </p>
          </div>

          <ol className="flex flex-col gap-2.5">
            {SETUP_STEPS.map((s, i) => (
              <li
                key={s.title}
                className="flex items-center gap-3.5 rounded-xl border bg-card p-3.5"
              >
                <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-brand-soft font-extrabold text-brand-text">
                  {i + 1}
                </span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="font-bold">{s.title}</span>
                  <span className="text-sm text-muted-foreground">{s.body}</span>
                </span>
                <span className="flex shrink-0 items-center gap-1 text-sm text-muted-foreground">
                  <ClockIcon className="size-3.5" aria-hidden="true" />
                  {s.time}
                </span>
              </li>
            ))}
          </ol>

          <div className="flex flex-col gap-2 rounded-xl bg-muted p-4 text-sm">
            <span className="font-bold">What you’ll need</span>
            <ul className="flex flex-col gap-1.5 text-muted-foreground">
              <li className="flex gap-2">
                <CheckIcon className="mt-0.5 size-4 shrink-0 text-brand-text" aria-hidden="true" />A
                Facebook account with admin access to your business
              </li>
              <li className="flex gap-2">
                <CheckIcon className="mt-0.5 size-4 shrink-0 text-brand-text" aria-hidden="true" />A
                phone number for WhatsApp that can get an SMS or call
              </li>
            </ul>
          </div>

          <fieldset className="flex flex-col gap-3">
            <legend className="mb-3 font-bold">What do you want to do first?</legend>
            <ToggleGroup
              type="single"
              variant="outline"
              spacing={2}
              value={goal}
              onValueChange={(v) => v && setGoal(v as FirstGoal)}
              className="flex-wrap"
            >
              {FIRST_GOALS.map((g) => {
                const Icon = GOAL_ICONS[g.id]
                return (
                  <ToggleGroupItem key={g.id} value={g.id}>
                    <Icon aria-hidden="true" />
                    {g.label}
                  </ToggleGroupItem>
                )
              })}
            </ToggleGroup>
            <p className="text-sm text-muted-foreground" aria-live="polite">
              {note}
            </p>
          </fieldset>

          {create.isError && (
            <Alert variant="danger">
              <AlertDescription>{errorMessage(create.error)}</AlertDescription>
            </Alert>
          )}

          <div className="flex flex-col gap-3 sm:flex-row">
            <Button size="lg" onClick={() => start('setup')} disabled={starting !== null}>
              {starting === 'setup' ? 'Starting…' : 'Start setup'}
            </Button>
            <Button
              size="lg"
              variant="soft"
              onClick={() => start('sample')}
              disabled={starting !== null}
            >
              {starting === 'sample' ? 'Loading sample data…' : 'Explore with sample data first'}
            </Button>
          </div>
        </section>

        <DashboardPeek />
      </main>
    </div>
  )
}

/** A drawn preview of the dashboard; decorative, so hidden from screen readers. */
function DashboardPeek() {
  return (
    <section
      aria-label="What you get"
      className="hidden flex-col gap-4 rounded-2xl bg-panel p-6 text-panel-foreground lg:flex"
    >
      <span className="text-sm font-bold text-panel-muted">A peek at your dashboard</span>
      <div
        className="flex flex-col gap-3 rounded-xl bg-card p-4 text-foreground shadow-card"
        aria-hidden="true"
      >
        <div className="grid grid-cols-3 gap-2">
          {[
            ['Delivered', '1,284'],
            ['Read rate', '87%'],
            ['Replies', '312'],
          ].map(([label, value]) => (
            <div key={label} className="flex flex-col rounded-lg bg-muted p-2.5">
              <span className="text-xs text-muted-foreground">{label}</span>
              <span className="text-lg font-extrabold">{value}</span>
            </div>
          ))}
        </div>
        <div className="flex h-24 items-end gap-1.5 rounded-lg bg-muted p-2.5">
          {[40, 65, 45, 80, 60, 90, 72, 95, 70, 85].map((h, i) => (
            <span
              // biome-ignore lint/suspicious/noArrayIndexKey: static decorative bars
              key={i}
              className="flex-1 rounded-sm bg-brand"
              style={{ height: `${h}%` }}
            />
          ))}
        </div>
      </div>
      <ul className="flex flex-col gap-2.5 text-sm">
        {[
          'Official WhatsApp Business API',
          'Broadcasts, shared inbox and chatbots in one place',
          'Help from a real person anytime, in chat',
        ].map((t) => (
          <li key={t} className="flex gap-2">
            <CheckIcon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            {t}
          </li>
        ))}
      </ul>
    </section>
  )
}
