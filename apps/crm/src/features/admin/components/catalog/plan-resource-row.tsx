import { CircleCheckIcon, CircleMinusIcon } from 'lucide-react'
import { Badge } from '#/components/ui/badge'
import { Input } from '#/components/ui/input'
import { Switch } from '#/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '#/components/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipTrigger } from '#/components/ui/tooltip'
import { cn } from '#/lib/utils'
import type { Resource } from '../../types'
import { isIncluded, type ResourceEntry } from '../kit/limits'

type Choice = 'off' | 'cap' | 'unlimited'

const CHOICES: { value: Choice; label: string }[] = [
  { value: 'off', label: 'Not included' },
  { value: 'cap', label: 'Limited' },
  { value: 'unlimited', label: 'Unlimited' },
]

// Equal-width segments fill the control column, so two options (required rows) and three line up.
const SEGMENT =
  'h-8 flex-1 rounded-md border-0 px-2 text-[13px] text-muted-foreground hover:bg-card/60 data-[state=on]:shadow-sm'

function plural(unit: string) {
  return unit.endsWith('s') ? unit : `${unit}s`
}

export function PlanResourceRow({
  resource: r,
  entry,
  error,
  onChange,
}: {
  resource: Resource
  entry: ResourceEntry
  error?: string
  onChange: (entry: ResourceEntry) => void
}) {
  const included = isIncluded(entry)
  const id = `plan-resource-${r.key}`
  const choices = CHOICES.filter((c) => !(r.required && c.value === 'off'))

  const choose = (value: string) => {
    if (value === 'cap') onChange({ mode: 'cap', cap: entry.mode === 'cap' ? entry.cap : '' })
    else if (value === 'off' || value === 'unlimited') onChange({ mode: value })
  }

  return (
    <li
      className={cn(
        'grid gap-3 px-4 py-3 transition-colors md:grid-cols-[minmax(0,1fr)_18rem_15rem] md:items-center md:gap-4',
        !included && 'bg-secondary/60',
      )}
    >
      <div className="flex min-w-0 items-start gap-2.5">
        {included ? (
          <CircleCheckIcon aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-brand" />
        ) : (
          <CircleMinusIcon
            aria-hidden="true"
            className="mt-0.5 size-4 shrink-0 text-muted-foreground"
          />
        )}
        <div className="flex min-w-0 flex-col gap-1">
          <span
            id={id}
            className={cn('text-sm font-bold', !included && 'font-semibold text-muted-foreground')}
          >
            {r.label}
          </span>
          <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            {r.required && <span>Every plan has this</span>}
            {!included && <span>Not part of this plan</span>}
            {!r.enforced && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="neutral" tabIndex={0}>
                    Pricing table only
                  </Badge>
                </TooltipTrigger>
                <TooltipContent className="max-w-64">
                  Nothing checks this yet. It shows on the pricing table, but customers can go past
                  it until the feature ships.
                </TooltipContent>
              </Tooltip>
            )}
          </div>
        </div>
      </div>

      {r.kind === 'access' ? (
        <div className="flex items-center gap-2.5 pl-6.5 md:pl-0">
          <Switch
            aria-labelledby={id}
            checked={included}
            onCheckedChange={(on) => onChange({ mode: on ? 'on' : 'off' })}
          />
          <span
            aria-hidden="true"
            className={cn('w-20 text-[13px] font-bold', !included && 'text-muted-foreground')}
          >
            {included ? 'Included' : 'Not included'}
          </span>
        </div>
      ) : (
        <>
          <div className="pl-6.5 md:pl-0">
            <ToggleGroup
              type="single"
              value={entry.mode}
              onValueChange={(v) => v && choose(v)}
              aria-labelledby={id}
              className="w-full rounded-lg bg-secondary p-1"
            >
              {choices.map((c) => (
                <ToggleGroupItem
                  key={c.value}
                  value={c.value}
                  className={cn(
                    SEGMENT,
                    c.value === 'off'
                      ? 'data-[state=on]:bg-card data-[state=on]:text-foreground'
                      : 'data-[state=on]:bg-brand-soft data-[state=on]:text-brand-text',
                  )}
                >
                  {c.label}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          {(entry.mode === 'cap' || (entry.mode === 'unlimited' && r.kind === 'monthly')) && (
            <div className="flex flex-col gap-1.5 pl-6.5 md:pl-0">
              {entry.mode === 'cap' ? (
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    inputMode="numeric"
                    min={1}
                    step={1}
                    aria-label={`${r.label} limit`}
                    aria-invalid={error ? true : undefined}
                    value={entry.cap}
                    placeholder="Amount"
                    onChange={(e) => onChange({ mode: 'cap', cap: e.target.value })}
                    className="h-9 w-24 shrink-0"
                  />
                  <span className="min-w-0 text-[13px] text-muted-foreground">
                    {plural(r.unit)}
                    {r.kind === 'monthly' && ' per month'}
                  </span>
                </div>
              ) : (
                <span className="text-[13px] text-muted-foreground">per month</span>
              )}
              {error && <p className="text-xs text-destructive">{error}</p>}
            </div>
          )}
        </>
      )}
    </li>
  )
}
