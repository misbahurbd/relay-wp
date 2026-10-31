import { Link } from '@tanstack/react-router'
import {
  AlertTriangleIcon,
  CheckIcon,
  ChevronDownIcon,
  CircleCheckIcon,
  CopyIcon,
  FlaskConicalIcon,
  InboxIcon,
  MegaphoneIcon,
  PlayCircleIcon,
  UsersIcon,
} from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Progress } from '#/components/ui/progress'
import { trialDaysLeft, type Workspace } from '#/features/workspaces'
import { cn } from '#/lib/utils'
import { useUpdateMemberState } from '../../api/onboarding.api'
import type { ChecklistItem, Onboarding, WhatsAppNumber } from '../../types'
import { ProductTour } from './product-tour'

const CHECKLIST: Record<
  ChecklistItem['id'],
  { title: string; why: string; time: string; cta: string; available?: boolean }
> = {
  account: { title: 'Create your account', why: 'Done. Welcome to Relay.', time: 'Done', cta: '' },
  whatsapp: {
    title: 'Connect your WhatsApp number',
    why: 'Needed before you can send or receive any message.',
    time: '5 min',
    cta: 'Connect',
    available: true,
  },
  template: {
    title: 'Create your first template',
    why: 'Meta must approve a template before you can message customers first.',
    time: '3 min',
    cta: 'Create template',
  },
  contacts: {
    title: 'Import contacts',
    why: 'Upload a CSV or Excel file of customers who opted in.',
    time: '2 min',
    cta: 'Import',
  },
  broadcast: {
    title: 'Send your first broadcast',
    why: 'Reach your contacts with an approved template.',
    time: '5 min',
    cta: 'New broadcast',
  },
  team: {
    title: 'Invite a teammate',
    why: 'Share the inbox so replies never wait.',
    time: '1 min',
    cta: 'Invite',
  },
}

type Props = {
  userId: string
  firstName: string
  workspace: Workspace
  onboarding: Onboarding
  number: WhatsAppNumber | undefined
}

export function FirstRunDashboard({ userId, firstName, workspace, onboarding, number }: Props) {
  const member = useUpdateMemberState(userId, workspace.id)
  const [tourOpen, setTourOpen] = useState(!onboarding.tour_seen)
  const daysLeft = trialDaysLeft(onboarding.trial_ends_at)
  const sample = onboarding.sample_data && !number

  return (
    <main className="mx-auto flex w-full max-w-6xl flex-col gap-6 px-4 py-6 sm:px-6">
      {onboarding.whatsapp_skipped || (onboarding.sample_data && !number) ? (
        <ConnectBanner sample={onboarding.sample_data} />
      ) : null}

      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1.5">
          {daysLeft !== null && (
            <Badge variant="info" className="w-fit">
              Free trial: {daysLeft} {daysLeft === 1 ? 'day' : 'days'} left · No card needed
            </Badge>
          )}
          <h1 className="text-[26px] leading-tight">
            Welcome, {firstName}. Let’s get your first message out.
          </h1>
          <p className="text-muted-foreground">
            Follow these steps. Most teams send their first broadcast in under 20 minutes.
          </p>
        </div>
        <Button variant="outline" onClick={() => setTourOpen(true)}>
          Take the tour
        </Button>
      </div>

      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        <Checklist
          items={onboarding.checklist}
          hidden={onboarding.checklist_hidden}
          onHiddenChange={(hidden) => member.mutate({ checklist_hidden: hidden })}
        />
        <SetupVideo />
      </div>

      <Stats sample={sample} />

      <div className="grid gap-4 md:grid-cols-3">
        <InboxEmpty link={number?.wa_link} />
        <EmptyCard
          icon={MegaphoneIcon}
          title="Recent broadcasts"
          heading="No broadcasts yet"
          body="Send an approved template to many contacts at once."
          cta="Send your first broadcast"
        />
        <EmptyCard
          icon={UsersIcon}
          title="Contacts"
          heading="No contacts yet"
          body="Upload a CSV or Excel file. We’ll help you match the columns."
          cta="Import contacts"
        />
      </div>

      {tourOpen && (
        <ProductTour
          onClose={() => {
            setTourOpen(false)
            if (!onboarding.tour_seen) member.mutate({ tour_seen: true })
          }}
        />
      )}
    </main>
  )
}

function ConnectBanner({ sample }: { sample: boolean }) {
  return (
    <Alert variant="warning" className="items-center">
      {sample ? <FlaskConicalIcon /> : <AlertTriangleIcon />}
      <AlertTitle>
        {sample
          ? 'You’re viewing sample data. Connect your number to go live.'
          : 'Connect WhatsApp to start sending messages'}
      </AlertTitle>
      <AlertDescription className="flex flex-wrap items-center justify-between gap-3">
        <span>
          {sample
            ? 'Everything here is a demo store. Your real chats appear once you connect.'
            : 'You skipped this during setup. It takes about 5 minutes with your Facebook account.'}
        </span>
        <Button size="sm" asChild>
          <Link to="/setup" search={{ step: 'whatsapp' }}>
            Connect now
          </Link>
        </Button>
      </AlertDescription>
    </Alert>
  )
}

function Checklist({
  items,
  hidden,
  onHiddenChange,
}: {
  items: ChecklistItem[]
  hidden: boolean
  onHiddenChange: (hidden: boolean) => void
}) {
  const [collapsed, setCollapsed] = useState(false)
  const done = items.filter((i) => i.done).length
  const all = done === items.length

  if (hidden) {
    return (
      <section
        aria-label="Getting started checklist"
        className="flex items-center justify-between gap-3 rounded-xl border bg-card p-4"
      >
        <span className="text-sm text-muted-foreground">
          Checklist hidden · {done} of {items.length} done
        </span>
        <Button variant="ghost" size="sm" onClick={() => onHiddenChange(false)}>
          Show checklist
        </Button>
      </section>
    )
  }

  return (
    <section
      aria-labelledby="checklist-title"
      data-tour="checklist"
      className="flex flex-col gap-4 rounded-xl border bg-card p-5 shadow-card"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-col">
          <h2 id="checklist-title" className="text-lg">
            Getting started
          </h2>
          <span className="text-sm text-muted-foreground">
            {done} of {items.length} done
          </span>
        </div>
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="sm"
            aria-expanded={!collapsed}
            aria-controls="checklist-items"
            onClick={() => setCollapsed((c) => !c)}
          >
            {collapsed ? 'Show steps' : 'Collapse'}
            <ChevronDownIcon className={cn('transition-transform', !collapsed && 'rotate-180')} />
          </Button>
          <Button variant="ghost" size="sm" onClick={() => onHiddenChange(true)}>
            Hide checklist
          </Button>
        </div>
      </div>
      <Progress value={(done / items.length) * 100} aria-label="Setup progress" />
      {all && (
        <Alert variant="success">
          <CircleCheckIcon />
          <AlertTitle>You’re all set</AlertTitle>
          <AlertDescription>
            Your channel is live. Your dashboard will fill up as messages go out.
          </AlertDescription>
        </Alert>
      )}
      {!collapsed && (
        <ul id="checklist-items" className="flex flex-col">
          {items.map((item) => {
            const meta = CHECKLIST[item.id]
            return (
              <li
                key={item.id}
                className="flex items-center gap-3.5 border-t py-3.5 first:border-t-0"
              >
                <span
                  className={cn(
                    'flex size-6 shrink-0 items-center justify-center rounded-full border-2',
                    item.done
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'border-input',
                  )}
                  aria-hidden="true"
                >
                  {item.done && <CheckIcon className="size-3.5" />}
                </span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span
                    className={cn('font-bold', item.done && 'text-muted-foreground line-through')}
                  >
                    {meta.title}
                    <span className="sr-only">{item.done ? ' (done)' : ' (to do)'}</span>
                  </span>
                  <span className="text-sm text-muted-foreground">{meta.why}</span>
                </span>
                {item.id !== 'account' && !item.done && (
                  <>
                    <span className="hidden text-sm text-muted-foreground sm:inline">
                      {meta.time}
                    </span>
                    {meta.available ? (
                      <Button size="sm" variant="outline" asChild>
                        <Link to="/setup" search={{ step: 'whatsapp' }}>
                          {meta.cta}
                        </Link>
                      </Button>
                    ) : (
                      <Badge variant="neutral">Coming soon</Badge>
                    )}
                  </>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}

function SetupVideo() {
  return (
    <section
      aria-labelledby="video-title"
      className="flex flex-col gap-3 rounded-xl border bg-card p-5"
    >
      <h2 id="video-title" className="text-base">
        Watch the 2-minute setup video
      </h2>
      <div className="flex aspect-video items-center justify-center rounded-lg bg-panel text-panel-foreground">
        <PlayCircleIcon className="size-12 opacity-80" aria-hidden="true" />
        <span className="sr-only">Video coming soon</span>
      </div>
      <p className="text-sm text-muted-foreground">
        See how to create a template, import contacts and send your first broadcast.
      </p>
      <a href="mailto:support@relay.app?subject=Onboarding%20call" className="text-sm font-bold">
        Book a free 15-minute onboarding call
      </a>
    </section>
  )
}

function Stats({ sample }: { sample: boolean }) {
  const stats = [
    ['Messages sent', sample ? '2,418' : '0', 'Appears after your first broadcast'],
    ['Delivered', sample ? '98%' : '—', 'Appears after your first broadcast'],
    ['Read rate', sample ? '87%' : '—', 'Appears after your first broadcast'],
    ['Replies', sample ? '312' : '0', 'Counted when customers reply'],
    ['Active chats', sample ? '14' : '0', 'Open chats in your inbox'],
  ]
  return (
    <section
      aria-label="Key stats"
      className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"
    >
      {stats.map(([label, value, help]) => (
        <div key={label} className="flex flex-col gap-1 rounded-xl border bg-card p-4">
          <span className="text-sm text-muted-foreground">{label}</span>
          <span className="text-2xl font-extrabold">{value}</span>
          {!sample && <span className="text-xs text-muted-foreground">{help}</span>}
        </div>
      ))}
    </section>
  )
}

function InboxEmpty({ link }: { link?: string }) {
  return (
    <section
      aria-labelledby="inbox-title"
      className="flex flex-col gap-3 rounded-xl border bg-card p-5"
    >
      <h2 id="inbox-title" className="flex items-center gap-2 text-base">
        <InboxIcon className="size-4.5 text-muted-foreground" aria-hidden="true" />
        Inbox
      </h2>
      <p className="font-bold">No chats yet</p>
      {link ? (
        <>
          <p className="text-sm text-muted-foreground">
            Share your WhatsApp link so customers can message you.
          </p>
          <div className="flex gap-2">
            <label htmlFor="wa-link" className="sr-only">
              Your chat link
            </label>
            <Input
              unstyled
              id="wa-link"
              readOnly
              value={link}
              className="h-9 min-w-0 flex-1 rounded-md border border-input bg-muted px-3 text-sm"
            />
            <Button
              size="sm"
              variant="outline"
              onClick={() =>
                navigator.clipboard
                  .writeText(`https://${link}`)
                  .then(() => toast.success('Link copied'))
                  .catch(() => toast.error('Couldn’t copy. Select the link and copy it instead.'))
              }
            >
              <CopyIcon aria-hidden="true" />
              Copy
            </Button>
          </div>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">
          Chats appear here once your WhatsApp number is connected.
        </p>
      )}
    </section>
  )
}

function EmptyCard({
  icon: Icon,
  title,
  heading,
  body,
  cta,
}: {
  icon: typeof InboxIcon
  title: string
  heading: string
  body: string
  cta: string
}) {
  const id = `${title.toLowerCase().replace(/\s+/g, '-')}-title`
  return (
    <section aria-labelledby={id} className="flex flex-col gap-3 rounded-xl border bg-card p-5">
      <h2 id={id} className="flex items-center gap-2 text-base">
        <Icon className="size-4.5 text-muted-foreground" aria-hidden="true" />
        {title}
      </h2>
      <p className="font-bold">{heading}</p>
      <p className="text-sm text-muted-foreground">{body}</p>
      <div className="mt-auto flex items-center gap-2">
        <Button size="sm" variant="outline" disabled aria-describedby={`${id}-soon`}>
          {cta}
        </Button>
        <span id={`${id}-soon`} className="text-xs text-muted-foreground">
          Coming soon
        </span>
      </div>
    </section>
  )
}
