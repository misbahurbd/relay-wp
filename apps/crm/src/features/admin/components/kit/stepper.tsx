import { CheckIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'

export type StepperItem = { id: string; label: string; done: boolean; reachable: boolean }

/**
 * The steps of a form as a row of numbered buttons. A step you can reach is a button; on a phone
 * only the current step shows its label, the others keep it for screen readers.
 */
export function Stepper({
  steps,
  current,
  onSelect,
  label,
}: {
  steps: StepperItem[]
  current: string
  onSelect: (id: string) => void
  label: string
}) {
  return (
    <nav aria-label={label}>
      <ol className="flex items-center gap-1.5 sm:gap-2">
        {steps.map((s, i) => {
          const active = s.id === current
          return (
            <li key={s.id} className="flex min-w-0 items-center gap-1.5 sm:gap-2">
              {i > 0 && <span aria-hidden="true" className="h-px w-3 bg-border sm:w-6" />}
              <Button
                type="button"
                variant={active ? 'soft' : 'ghost'}
                size="sm"
                disabled={!s.reachable && !active}
                aria-current={active ? 'step' : undefined}
                onClick={() => onSelect(s.id)}
                className="min-w-0 gap-2 px-2 sm:px-3"
              >
                <span
                  aria-hidden="true"
                  className={cn(
                    'grid size-5 shrink-0 place-content-center rounded-full border text-[11px] font-extrabold',
                    active && 'border-primary bg-primary text-primary-foreground',
                    s.done && !active && 'border-transparent bg-brand-soft text-brand-text',
                    !active && !s.done && 'border-input text-muted-foreground',
                  )}
                >
                  {s.done && !active ? <CheckIcon className="size-3" /> : i + 1}
                </span>
                <span className={cn('truncate', !active && 'max-sm:sr-only')}>{s.label}</span>
                {s.done && !active && <span className="sr-only">(done)</span>}
              </Button>
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
