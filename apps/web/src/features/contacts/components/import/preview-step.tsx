import { cn } from '#/lib/utils'
import type { ImportPreview, PreviewRow } from '../../types'
import { formatNumber } from '../kit/format'

const STATUS: Record<PreviewRow['status'], { label: string; tone: string }> = {
  new: { label: 'New', tone: 'bg-brand-soft text-brand-text' },
  update: { label: 'Will update', tone: 'bg-warning-soft text-warning' },
  invalid: { label: 'Skipped', tone: 'bg-destructive-soft text-destructive' },
}

/** Step 3: the counts from the whole file and the first rows. */
export function PreviewStep({ preview }: { preview: ImportPreview }) {
  const stats = [
    { label: 'Rows found', value: preview.rows_found, box: 'bg-muted', text: '' },
    {
      label: 'Ready to import',
      value: preview.ready,
      box: 'bg-brand-soft',
      text: 'text-brand-text',
    },
    {
      label: 'Already exist, will update',
      value: preview.updates,
      box: 'bg-warning-soft',
      text: 'text-warning',
    },
    {
      label: 'Invalid numbers, skipped',
      value: preview.invalid,
      box: 'bg-destructive-soft',
      text: 'text-destructive',
    },
  ]
  return (
    <>
      <dl className="grid grid-cols-[repeat(auto-fit,minmax(130px,1fr))] gap-2.5">
        {stats.map((s) => (
          <div key={s.label} className={cn('flex flex-col-reverse rounded-lg p-3', s.box)}>
            <dt className="text-xs text-muted-foreground">{s.label}</dt>
            <dd className={cn('text-[22px] font-extrabold tabular-nums', s.text)}>
              {formatNumber(s.value)}
            </dd>
          </div>
        ))}
      </dl>
      {preview.sample.length > 0 && (
        <div className="overflow-x-auto rounded-lg border">
          <table className="w-full min-w-[520px] border-collapse text-[13px]">
            <caption className="sr-only">
              The first {preview.sample.length} rows of the file
            </caption>
            <thead>
              <tr className="bg-muted text-left text-xs text-muted-foreground">
                <th scope="col" className="px-3.5 py-2.5 font-bold">
                  Name
                </th>
                <th scope="col" className="px-2.5 py-2.5 font-bold">
                  Phone
                </th>
                <th scope="col" className="px-2.5 py-2.5 font-bold">
                  Email
                </th>
                <th scope="col" className="py-2.5 pr-3.5 pl-2.5 font-bold">
                  Check
                </th>
              </tr>
            </thead>
            <tbody>
              {preview.sample.map((r) => (
                <tr key={r.row} className="border-t">
                  <td className="px-3.5 py-2 font-semibold">{r.name || '—'}</td>
                  <td className="px-2.5 py-2 whitespace-nowrap tabular-nums">{r.phone || '—'}</td>
                  <td className="px-2.5 py-2 text-muted-foreground">{r.email || '—'}</td>
                  <td className="py-2 pr-3.5 pl-2.5">
                    <span
                      className={cn(
                        'rounded-full px-2 py-0.5 text-[11px] font-bold whitespace-nowrap',
                        STATUS[r.status].tone,
                      )}
                    >
                      {r.status === 'invalid' && r.reason ? r.reason : STATUS[r.status].label}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}
