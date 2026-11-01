import { XIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { OptionSelect } from '#/components/ui/option-select'
import {
  blankCondition,
  conditionComplete,
  fieldOf,
  OPT_IN_VALUES,
  SEGMENT_FIELDS,
  SOURCE_VALUES,
} from '../../schemas/segments.schema'
import type { SegmentCondition, SegmentField, SegmentOp, Tag } from '../../types'

/** One condition: field, condition, value, remove. */
export function ConditionRow({
  condition: c,
  joiner,
  tags,
  readOnly,
  error,
  onChange,
  onRemove,
}: {
  condition: SegmentCondition
  joiner: string
  tags: Tag[]
  readOnly: boolean
  error?: string
  onChange: (c: SegmentCondition) => void
  onRemove: () => void
}) {
  const f = fieldOf(c.field)
  const show = !conditionComplete(c) ? 'Enter a value to use this condition.' : error
  const select = 'h-10 w-auto min-w-[140px]'
  return (
    <div className="flex flex-col gap-1">
      <div className="flex flex-wrap items-center gap-2">
        <span className="w-10 text-xs font-bold text-muted-foreground uppercase">{joiner}</span>
        <OptionSelect
          aria-label="Field"
          size="sm"
          className={select}
          value={c.field}
          disabled={readOnly}
          onValueChange={(v) => onChange(blankCondition(v as SegmentField, tags))}
          options={SEGMENT_FIELDS}
        />
        <OptionSelect
          aria-label="Condition"
          size="sm"
          className={select}
          value={c.op}
          disabled={readOnly}
          onValueChange={(v) => onChange({ ...c, op: v as SegmentOp })}
          options={f.ops}
        />
        {f.kind === 'tag' && (
          <OptionSelect
            aria-label="Tag"
            size="sm"
            className={select}
            value={c.value}
            disabled={readOnly}
            placeholder="Choose a tag"
            onValueChange={(v) => onChange({ ...c, value: v })}
            options={tags.map((t) => ({ value: t.id, label: t.name }))}
          />
        )}
        {(f.kind === 'opt_in' || f.kind === 'source') && (
          <OptionSelect
            aria-label="Value"
            size="sm"
            className={select}
            value={c.value}
            disabled={readOnly}
            onValueChange={(v) => onChange({ ...c, value: v })}
            options={f.kind === 'opt_in' ? OPT_IN_VALUES : SOURCE_VALUES}
          />
        )}
        {f.kind === 'days' && c.op !== 'never' && (
          <span className="flex items-center gap-2">
            <Input
              type="number"
              min={1}
              max={3650}
              aria-label="Days"
              className="h-10 w-24"
              value={c.value}
              disabled={readOnly}
              onChange={(e) => onChange({ ...c, value: e.target.value })}
            />
            <span className="text-sm text-muted-foreground">
              days{c.op === 'over' ? ' ago' : ''}
            </span>
          </span>
        )}
        {f.kind === 'attribute' && (
          <Input
            aria-label="Custom field"
            placeholder="Field, like city"
            className="h-10 w-36"
            value={c.key ?? ''}
            disabled={readOnly}
            onChange={(e) => onChange({ ...c, key: e.target.value })}
          />
        )}
        {(f.kind === 'text' || f.kind === 'code' || f.kind === 'attribute') && (
          <Input
            aria-label="Value"
            placeholder={f.kind === 'code' ? '+44' : f.kind === 'text' ? 'en' : 'Type a value'}
            className="h-10 w-36"
            value={c.value}
            disabled={readOnly}
            onChange={(e) => onChange({ ...c, value: e.target.value })}
          />
        )}
        {!readOnly && (
          <Button variant="ghost" size="icon-sm" aria-label="Remove condition" onClick={onRemove}>
            <XIcon aria-hidden="true" />
          </Button>
        )}
      </div>
      {show && <span className="pl-12 text-xs font-semibold text-destructive">{show}</span>}
    </div>
  )
}
