import { XIcon } from 'lucide-react'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'

const STEPS = [
  {
    target: 'inbox',
    title: 'Your team inbox',
    body: 'Every WhatsApp chat lands here. Assign chats, reply, and add private notes for your team.',
  },
  {
    target: 'broadcasts',
    title: 'Broadcasts',
    body: 'Send an approved template to a tag, segment or uploaded list, now or scheduled.',
  },
  {
    target: 'templates',
    title: 'Templates',
    body: 'Create message templates and submit them to Meta. Approval usually takes minutes.',
  },
  {
    target: 'checklist',
    title: 'Your setup checklist',
    body: 'Work through these steps to send your first message. You can hide it any time.',
  },
]

type Rect = { top: number; left: number; width: number; height: number }
const CARD_WIDTH = 320
const GAP = 12

/** Short, skippable tour that points at real elements (data-tour="…"). Escape closes it. */
export function ProductTour({ onClose }: { onClose: () => void }) {
  const [index, setIndex] = useState(0)
  const [rect, setRect] = useState<Rect | null>(null)
  const cardRef = useRef<HTMLDivElement>(null)
  const step = STEPS[index]
  const last = index === STEPS.length - 1

  useLayoutEffect(() => {
    const measure = () => {
      const el = document.querySelector<HTMLElement>(`[data-tour="${step.target}"]`)
      const r = el?.getBoundingClientRect()
      // Hidden targets (the sidebar on phones) get a centred card instead.
      setRect(
        r && r.width > 0 ? { top: r.top, left: r.left, width: r.width, height: r.height } : null,
      )
    }
    document.querySelector(`[data-tour="${step.target}"]`)?.scrollIntoView({ block: 'nearest' })
    measure()
    window.addEventListener('resize', measure)
    window.addEventListener('scroll', measure, true)
    return () => {
      window.removeEventListener('resize', measure)
      window.removeEventListener('scroll', measure, true)
    }
  }, [step.target])

  // biome-ignore lint/correctness/useExhaustiveDependencies: move focus to the card on every step
  useEffect(() => {
    cardRef.current?.focus()
  }, [index])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const position = cardPosition(rect)

  return (
    <>
      <div className="fixed inset-0 z-40 bg-overlay" aria-hidden="true" onClick={onClose} />
      {rect && (
        <div
          aria-hidden="true"
          className="pointer-events-none fixed z-40 rounded-lg ring-4 ring-brand ring-offset-2 ring-offset-background transition-all"
          style={{ top: rect.top, left: rect.left, width: rect.width, height: rect.height }}
        />
      )}
      <div
        ref={cardRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="tour-title"
        aria-describedby="tour-body"
        tabIndex={-1}
        className={cn(
          'fixed z-50 flex flex-col gap-3 rounded-xl border bg-popover p-5 text-popover-foreground shadow-card outline-none',
          !rect && 'top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2',
        )}
        style={{ width: `min(${CARD_WIDTH}px, calc(100vw - 32px))`, ...position }}
      >
        <div className="flex items-center justify-between">
          <span className="text-sm font-bold text-brand-text">
            {index + 1} of {STEPS.length}
          </span>
          <Button variant="ghost" size="icon-sm" aria-label="Skip tour" onClick={onClose}>
            <XIcon />
          </Button>
        </div>
        <h2 id="tour-title" className="text-lg">
          {step.title}
        </h2>
        <p id="tour-body" className="text-muted-foreground">
          {step.body}
        </p>
        <div className="flex items-center justify-between gap-3 pt-1">
          <div className="flex gap-1.5" aria-hidden="true">
            {STEPS.map((s, i) => (
              <span
                key={s.target}
                className={cn(
                  'h-1.5 rounded-full transition-all',
                  i === index ? 'w-5 bg-brand' : 'w-1.5 bg-muted',
                )}
              />
            ))}
          </div>
          <div className="flex gap-2">
            {index > 0 && (
              <Button variant="ghost" size="sm" onClick={() => setIndex((i) => i - 1)}>
                Back
              </Button>
            )}
            <Button size="sm" onClick={() => (last ? onClose() : setIndex((i) => i + 1))}>
              {last ? 'Finish' : 'Next'}
            </Button>
          </div>
        </div>
      </div>
    </>
  )
}

/** Beside the target if there's room on the right, otherwise below it, kept on screen. */
function cardPosition(rect: Rect | null) {
  if (!rect) return {}
  const vw = window.innerWidth
  const vh = window.innerHeight
  if (rect.left + rect.width + GAP + CARD_WIDTH <= vw - 16 && rect.width < vw / 2) {
    return { top: Math.min(Math.max(16, rect.top), vh - 260), left: rect.left + rect.width + GAP }
  }
  const below = rect.top + rect.height + GAP
  return {
    top: below + 240 <= vh ? below : Math.max(16, rect.top - 240 - GAP),
    left: Math.min(Math.max(16, rect.left), vw - CARD_WIDTH - 16),
  }
}
