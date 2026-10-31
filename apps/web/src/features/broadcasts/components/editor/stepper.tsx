import { CheckIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'

export const STEPS = ['Audience', 'Template', 'Schedule', 'Review & send'] as const

/** "Step 1 · Audience" … as buttons: a done or current step can be revisited. */
export function Stepper({
  step,
  done,
  onStep,
}: {
  step: number
  done: boolean
  onStep: (n: number) => void
}) {
  return (
    <nav aria-label="Broadcast steps" className="grid grid-cols-2 gap-2.5 md:grid-cols-4">
      {STEPS.map((label, i) => {
        const n = i + 1
        const complete = done || n < step
        const current = !done && n === step
        return (
          <Button
            key={label}
            variant="unstyled"
            type="button"
            aria-current={current ? 'step' : undefined}
            disabled={done}
            onClick={() => onStep(n)}
            className={cn(
              'flex h-auto items-center justify-start gap-3 rounded-[14px] border bg-card px-3.5 py-3 text-left',
              current ? 'border-primary' : 'border-border hover:border-primary/60',
            )}
          >
            <span
              className={cn(
                'flex size-8 flex-none items-center justify-center rounded-full font-extrabold',
                complete
                  ? 'bg-primary text-primary-foreground'
                  : current
                    ? 'bg-brand-soft text-brand-text'
                    : 'bg-muted text-muted-foreground',
              )}
            >
              {complete ? <CheckIcon className="size-4" strokeWidth={3} aria-hidden="true" /> : n}
            </span>
            <span className="flex flex-col leading-tight">
              <span className="text-xs font-semibold text-muted-foreground">Step {n}</span>
              <span className="font-bold">{label}</span>
            </span>
          </Button>
        )
      })}
    </nav>
  )
}
