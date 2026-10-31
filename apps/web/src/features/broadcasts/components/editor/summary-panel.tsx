import type { NumberDto } from '@relay/api-client'
import { AlertTriangleIcon, CheckIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Input } from '#/components/ui/input'
import { Skeleton } from '#/components/ui/skeleton'
import { cn } from '#/lib/utils'
import type { AudienceCount } from '../../types'
import { controlClass, Select } from '../kit/field'
import { formatCount } from '../kit/format'

const QUALITY = { green: 'High', yellow: 'Medium', red: 'Low' } as const

function Check({ ok, children }: { ok: boolean; children: ReactNode }) {
  return (
    <span className={cn('flex items-center gap-2', ok ? 'text-muted-foreground' : 'text-warning')}>
      {ok ? (
        <CheckIcon className="size-[15px] flex-none text-brand-text" aria-hidden="true" />
      ) : (
        <AlertTriangleIcon className="size-[15px] flex-none" aria-hidden="true" />
      )}
      {children}
    </span>
  )
}

/** The panel beside every step: name, number, who gets it, and what to check before sending. */
export function SummaryPanel({
  name,
  numberId,
  numbers,
  count,
  empty,
  templateName,
  category,
  when,
  readOnly,
  errors,
  onName,
  onNumber,
}: {
  name: string
  numberId?: string
  numbers?: NumberDto[]
  count?: AudienceCount
  empty: boolean
  templateName?: string
  category?: string
  when: string
  readOnly: boolean
  errors: Record<string, string>
  onName: (name: string) => void
  onNumber: (id?: string) => void
}) {
  const number = numbers?.find((n) => n.id === numberId)
  const quality = number?.quality_rating
  const over = !!count && count.tier_limit > 0 && count.included > count.tier_limit
  const label = 'text-xs font-extrabold tracking-[0.05em] text-muted-foreground uppercase'

  return (
    <aside
      aria-label="Broadcast summary"
      className="flex flex-col gap-4 rounded-2xl border bg-card p-5 shadow-card lg:sticky lg:top-5"
    >
      <label className="flex flex-col gap-1.5">
        <span className={label}>Campaign name</span>
        <Input
          unstyled
          className={controlClass}
          value={name}
          maxLength={120}
          disabled={readOnly}
          placeholder="Halloween flash sale"
          onChange={(e) => onName(e.target.value)}
        />
        {errors.name && (
          <span className="text-xs font-semibold text-destructive">{errors.name}</span>
        )}
      </label>
      {/* biome-ignore lint/a11y/noLabelWithoutControl: the control is inside */}
      <label className="flex flex-col gap-1.5">
        <span className={label}>Send from</span>
        <Select
          value={numberId ?? ''}
          disabled={readOnly}
          onValueChange={(v) => onNumber(v || undefined)}
          placeholder={numbers?.length === 0 ? 'No connected number' : 'Choose a number'}
          options={(numbers ?? []).map((n) => ({
            value: n.id,
            label: `${n.verified_name} · ${n.display_phone_number}`,
          }))}
        />
        {errors.number_id && (
          <span className="text-xs font-semibold text-destructive">{errors.number_id}</span>
        )}
      </label>

      <div className="flex flex-col gap-1 rounded-[14px] bg-brand-soft p-4" aria-live="polite">
        <span className="text-xs font-bold text-brand-text">Recipients</span>
        {empty ? (
          <span className="text-[32px] leading-tight font-extrabold tracking-[-0.02em]">0</span>
        ) : count ? (
          <span className="text-[32px] leading-tight font-extrabold tracking-[-0.02em] tabular-nums">
            {formatCount(count.included)}
          </span>
        ) : (
          <Skeleton className="h-9 w-24" />
        )}
        <span className="text-xs text-muted-foreground">
          {empty
            ? 'Pick everyone, a tag or a list'
            : count && count.left_out > 0
              ? `Unique, opted-in contacts · ${formatCount(count.left_out)} not opted in left out`
              : 'Unique, opted-in contacts'}
        </span>
      </div>

      <dl className="flex flex-col text-sm">
        {[
          ['Template', templateName ?? '—'],
          ['Category', category ? category[0].toUpperCase() + category.slice(1) : '—'],
          ['Send time', when],
          ['Meta cost', 'Billed by Meta'],
        ].map(([k, v]) => (
          <div key={k} className="flex justify-between gap-3 border-b py-2.5 last:border-b-0">
            <dt className="text-muted-foreground">{k}</dt>
            <dd className="text-right font-bold">{v}</dd>
          </div>
        ))}
      </dl>

      <div className="flex flex-col gap-2 rounded-xl bg-muted p-3.5 text-[13px]">
        <span className="font-bold">Before you send</span>
        <Check ok>Only opted-in contacts are included</Check>
        {quality && <Check ok={quality === 'green'}>Quality rating is {QUALITY[quality]}</Check>}
        {count && count.tier_limit > 0 && (
          <Check ok={!over}>
            {over
              ? `Over your ${formatCount(count.tier_limit)} / 24h limit: Meta will refuse the rest`
              : `Within your ${formatCount(count.tier_limit)} / 24h limit`}
          </Check>
        )}
      </div>
    </aside>
  )
}
