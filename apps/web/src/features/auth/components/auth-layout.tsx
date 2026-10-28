import { CheckIcon, ShieldCheckIcon, SparklesIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Logo } from '#/components/layout/logo'
import { ThemeToggle } from '#/components/layout/theme-toggle'

type Props = {
  title: string
  description?: ReactNode
  /** Which marketing panel to show beside the form on wide screens. */
  panel?: 'signup' | 'login' | 'none'
  children: ReactNode
}

/** Split screen: the form on the left, a value panel on the right (hidden on small screens). */
export function AuthLayout({ title, description, panel = 'signup', children }: Props) {
  return (
    <div className="flex min-h-dvh bg-card">
      <div className="flex min-w-0 flex-1 flex-col px-4 py-6 sm:px-10">
        <header className="flex items-center justify-between">
          <Logo to="/login" />
          <ThemeToggle />
        </header>
        <main className="mx-auto flex w-full max-w-[420px] flex-1 flex-col justify-center gap-6 py-10">
          <div className="flex flex-col gap-1.5">
            <h1 className="text-[28px] leading-tight">{title}</h1>
            {description && <p className="text-[15px] text-muted-foreground">{description}</p>}
          </div>
          {children}
        </main>
        <footer className="text-center text-xs text-muted-foreground">
          © {new Date().getFullYear()} Relay · Built on the official WhatsApp Business Platform
        </footer>
      </div>
      {panel === 'signup' && <SignupPanel />}
      {panel === 'login' && <LoginPanel />}
    </div>
  )
}

function PanelShell({ children }: { children: ReactNode }) {
  return (
    <aside className="hidden w-[46%] max-w-[680px] shrink-0 flex-col justify-center gap-8 bg-panel p-12 text-panel-foreground lg:flex">
      {children}
    </aside>
  )
}

function SignupPanel() {
  return (
    <PanelShell>
      <div className="flex flex-col gap-3">
        <span className="w-fit rounded-full bg-white/10 px-3 py-1 text-xs font-bold">
          Trusted by 2,000+ small businesses
        </span>
        <h2 className="text-[34px] leading-tight">
          Talk to your customers where they already are.
        </h2>
        <p className="text-[15px] text-panel-muted">
          Broadcasts, a shared team inbox and simple chatbots, all on WhatsApp.
        </p>
      </div>
      <ChatMock />
      <ul className="grid grid-cols-2 gap-4">
        {[
          ['Official API', 'WhatsApp Business Platform, no ban risk'],
          ['3-minute setup', 'Connect your number with Meta in a few clicks'],
          ['14-day free trial', 'Every feature on the Growth plan'],
          ['No card needed', 'Add billing only when you’re ready'],
        ].map(([title, body]) => (
          <li key={title} className="flex gap-2.5">
            <CheckIcon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            <span className="flex flex-col">
              <span className="font-bold">{title}</span>
              <span className="text-sm text-panel-muted">{body}</span>
            </span>
          </li>
        ))}
      </ul>
    </PanelShell>
  )
}

function ChatMock() {
  return (
    <div
      role="img"
      aria-label="Example WhatsApp chat between a store and a customer"
      className="flex flex-col gap-2 rounded-xl bg-chat p-4 text-foreground shadow-card"
    >
      <div className="mb-1 flex items-center gap-2.5 border-b pb-3">
        <span className="flex size-8 items-center justify-center rounded-full bg-brand-soft text-xs font-extrabold text-brand-text">
          GL
        </span>
        <span className="flex flex-col leading-tight">
          <span className="text-sm font-bold">Green Leaf Store</span>
          <span className="text-xs text-muted-foreground">Business account</span>
        </span>
      </div>
      <p className="max-w-[80%] rounded-lg rounded-tl-sm bg-bubble-out px-3 py-2 text-sm">
        Hi Maya, your order #4821 is on its way. Track it here: glstore.co/t/4821
      </p>
      <p className="max-w-[80%] self-end rounded-lg rounded-tr-sm bg-bubble-in px-3 py-2 text-sm">
        Amazing, thank you! Can I add a gift note?
      </p>
    </div>
  )
}

function LoginPanel() {
  return (
    <PanelShell>
      <div className="flex flex-col gap-3">
        <h2 className="text-[34px] leading-tight">
          Your customers are waiting. Let’s get back to them.
        </h2>
        <p className="text-[15px] text-panel-muted">
          Your inbox, broadcasts and chatbots pick up right where you left off.
        </p>
      </div>
      <div className="flex flex-col gap-2 rounded-xl bg-white/10 p-5">
        <span className="flex items-center gap-2 text-xs font-extrabold tracking-wide uppercase">
          <SparklesIcon className="size-4" aria-hidden="true" />
          What’s new
        </span>
        <span className="font-bold">AI reply block for chatbots</span>
        <span className="text-sm text-panel-muted">
          Let your bot answer common questions from your FAQ, and hand over to an agent when it
          isn’t sure.
        </span>
      </div>
      <p className="flex items-center gap-2 text-sm text-panel-muted">
        <ShieldCheckIcon className="size-4" aria-hidden="true" />
        Encrypted connections and secure sessions
      </p>
    </PanelShell>
  )
}
