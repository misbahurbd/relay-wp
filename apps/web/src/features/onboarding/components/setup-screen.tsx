import { CheckIcon, LifeBuoyIcon, LightbulbIcon } from 'lucide-react'
import { useState } from 'react'
import { Logo } from '#/components/layout/logo'
import { ThemeToggle } from '#/components/layout/theme-toggle'
import { SignOutButton } from '#/features/auth'
import type { Workspace } from '#/features/workspaces'
import { cn } from '#/lib/utils'
import type { Onboarding } from '../types'
import { BusinessStep } from './business-step'
import { TestMessageStep } from './test-message-step'
import { WhatsAppStep } from './whatsapp-step'

export type StepId = 'business' | 'whatsapp' | 'test_message'

const STEPS: { id: StepId; label: string; sub: string; minutesLeft: string }[] = [
  { id: 'business', label: 'Business details', sub: 'About you', minutesLeft: '4 minutes' },
  { id: 'whatsapp', label: 'Connect WhatsApp', sub: 'With Meta', minutesLeft: '3 minutes' },
  { id: 'test_message', label: 'Send test message', sub: 'See it work', minutesLeft: '1 minute' },
]

const TIPS: Record<StepId, { title: string; tips: string[] }> = {
  business: {
    title: 'Tips for this step',
    tips: [
      'Use the same business name customers know you by. It helps Meta approve your display name faster.',
      'You can invite teammates later from Team settings.',
      'Picking goals only changes the starter templates we suggest. Nothing is locked.',
    ],
  },
  whatsapp: {
    title: 'Before you connect',
    tips: [
      'Using the number on the WhatsApp app today? Back up your chats, then delete that WhatsApp account before connecting.',
      'No spare number? A new SIM or a landline works. Meta can verify by voice call.',
      'Display names should match your brand, like “Green Leaf Store”, not “Best Deals 24/7”.',
    ],
  },
  test_message: {
    title: 'Good to know',
    tips: [
      'Replies to your test show up in your Inbox, so you can see the whole loop.',
      'Marketing templates need Meta approval, usually within a few minutes to a few hours.',
      'Next up: import your contacts and send your first broadcast.',
    ],
  },
}

type Props = {
  userId: string
  firstName: string
  workspace: Workspace
  onboarding: Onboarding
  /** Open an earlier step than the current one, e.g. "Connect now" from the dashboard. */
  initialStep?: StepId
}

export function SetupScreen({ userId, firstName, workspace, onboarding, initialStep }: Props) {
  const current = onboarding.current_step === 'done' ? 'test_message' : onboarding.current_step
  // Back can show an earlier step; progress itself always comes from the server.
  const [viewing, setViewing] = useState<StepId>(initialStep ?? current)
  const step =
    STEPS.findIndex((s) => s.id === current) < STEPS.findIndex((s) => s.id === viewing)
      ? current
      : viewing
  const index = STEPS.findIndex((s) => s.id === step)
  const goTo = (id: StepId) => {
    setViewing(id)
    window.scrollTo({ top: 0 })
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="flex items-center justify-between gap-4 border-b bg-card px-4 py-3 sm:px-8">
        <Logo />
        <div className="flex items-center gap-3">
          <a
            href="mailto:support@relay.app?subject=Setup%20call"
            className="hidden text-sm font-bold sm:inline"
          >
            Need help? Book a free setup call
          </a>
          <ThemeToggle />
          <SignOutButton />
        </div>
      </header>

      <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 px-4 py-8 sm:px-8">
        <div className="flex flex-col gap-1.5">
          <p className="text-sm font-bold text-brand-text">
            Step {index + 1} of 3 · about {STEPS[index].minutesLeft} left
          </p>
          <h1 className="text-[28px] leading-tight">Welcome, {firstName}. Let’s get you set up.</h1>
        </div>

        <Stepper current={current} viewing={step} />

        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
          <section
            className="rounded-xl border bg-card p-5 shadow-card sm:p-7"
            aria-labelledby="step-title"
          >
            {step === 'business' && (
              <BusinessStep userId={userId} workspace={workspace} onDone={() => goTo('whatsapp')} />
            )}
            {step === 'whatsapp' && (
              <WhatsAppStep
                userId={userId}
                workspaceId={workspace.id}
                onBack={() => goTo('business')}
                onDone={() => goTo('test_message')}
              />
            )}
            {step === 'test_message' && (
              <TestMessageStep
                userId={userId}
                workspace={workspace}
                onBack={() => goTo('whatsapp')}
              />
            )}
          </section>
          <TipsCard step={step} />
        </div>
      </main>
    </div>
  )
}

function Stepper({ current, viewing }: { current: StepId; viewing: StepId }) {
  const reached = STEPS.findIndex((s) => s.id === current)
  return (
    <ol aria-label="Setup progress" className="grid grid-cols-3 gap-2">
      {STEPS.map((s, i) => {
        const done = i < reached
        const isViewing = s.id === viewing
        return (
          <li
            key={s.id}
            aria-current={isViewing ? 'step' : undefined}
            className="flex min-w-0 flex-col gap-2"
          >
            <span
              className={cn('h-1.5 rounded-full', i <= reached ? 'bg-brand' : 'bg-muted')}
              aria-hidden="true"
            />
            <span className="flex items-center gap-2">
              <span
                className={cn(
                  'flex size-6 shrink-0 items-center justify-center rounded-full border-2 text-xs font-extrabold',
                  done && 'border-primary bg-primary text-primary-foreground',
                  !done && isViewing && 'border-primary bg-brand-soft text-brand-text',
                  !done && !isViewing && 'border-input text-muted-foreground',
                )}
              >
                {done ? <CheckIcon className="size-3.5" aria-hidden="true" /> : i + 1}
              </span>
              <span className="flex min-w-0 flex-col leading-tight">
                <span
                  className={cn(
                    'truncate text-sm font-bold',
                    !done && !isViewing && 'text-muted-foreground',
                  )}
                >
                  {s.label}
                  {done && <span className="sr-only"> (done)</span>}
                </span>
                <span className="hidden text-xs text-muted-foreground sm:block">{s.sub}</span>
              </span>
            </span>
          </li>
        )
      })}
    </ol>
  )
}

function TipsCard({ step }: { step: StepId }) {
  const { title, tips } = TIPS[step]
  return (
    <aside className="flex flex-col gap-4 rounded-xl border bg-card p-5" aria-label="Helpful tips">
      <h2 className="flex items-center gap-2 text-base">
        <LightbulbIcon className="size-4.5 text-warning" aria-hidden="true" />
        {title}
      </h2>
      <ul className="flex flex-col gap-3 text-sm text-muted-foreground">
        {tips.map((t) => (
          <li key={t} className="flex gap-2">
            <span className="mt-2 size-1.5 shrink-0 rounded-full bg-brand" aria-hidden="true" />
            {t}
          </li>
        ))}
      </ul>
      <div className="flex items-start gap-3 rounded-lg bg-muted p-3 text-sm">
        <LifeBuoyIcon className="mt-0.5 size-4.5 shrink-0 text-brand-text" aria-hidden="true" />
        <span>
          Stuck? Our team can help.{' '}
          <a href="mailto:support@relay.app" className="font-bold">
            Chat with support
          </a>
        </span>
      </div>
    </aside>
  )
}
