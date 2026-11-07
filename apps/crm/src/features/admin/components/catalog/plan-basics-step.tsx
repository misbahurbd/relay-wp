import { ChevronRightIcon } from 'lucide-react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Label } from '#/components/ui/label'
import { Switch } from '#/components/ui/switch'
import type { Plan } from '../../types'
import { FREE_PLAN } from '../kit/limits'
import { TextField } from '../kit/text-field'
import {
  type Errors,
  nextVersionOf,
  type PlanDraft,
  type PlanEditorMode,
  slugify,
} from './plan-draft'

export function PlanBasicsStep({
  mode,
  draft,
  errors,
  plans,
  onChange,
}: {
  mode: PlanEditorMode
  draft: PlanDraft
  errors: Errors
  plans: Plan[]
  onChange: (patch: Partial<PlanDraft>) => void
}) {
  const creating = mode.kind === 'create'
  const free = draft.code === FREE_PLAN
  const taken = creating ? nextVersionOf(draft.code, plans) : undefined

  return (
    <div className="mx-auto flex w-full max-w-[560px] flex-col gap-5">
      <TextField
        id="plan-name"
        label="Name"
        value={draft.name}
        onChange={(name) =>
          onChange({ name, ...(creating && !draft.codeEdited ? { code: slugify(name) } : {}) })
        }
        placeholder="Growth"
        autoComplete="off"
        autoFocus={creating}
        error={errors.name}
        hint="What customers see on the pricing table."
      />
      <TextField
        id="plan-code"
        label="Code"
        value={draft.code}
        onChange={(v) =>
          onChange({
            code: v.toLowerCase().replace(/[^a-z0-9_]+/g, '_'),
            codeEdited: v !== '',
          })
        }
        disabled={!creating}
        placeholder="growth"
        autoComplete="off"
        spellCheck={false}
        className="font-mono"
        error={errors.code}
        hint={
          creating
            ? 'Made from the name. Used by the API and TRIAL_PLAN; it cannot change later.'
            : 'The code ties versions of one plan together, so it stays as it is.'
        }
      />
      {taken && (
        <Alert variant="info">
          <AlertDescription>
            <p>
              A plan with the code <code>{draft.code}</code> exists. This publishes it as version{' '}
              {taken}; everyone on the earlier versions keeps what they have.
            </p>
          </AlertDescription>
        </Alert>
      )}
      {!free && (
        <>
          <TextField
            id="plan-description"
            label={
              <>
                Description <span className="font-normal text-muted-foreground">(optional)</span>
              </>
            }
            value={draft.description}
            onChange={(description) => onChange({ description })}
            maxLength={200}
            placeholder="For growing teams running regular campaigns."
            hint="One line under the plan name."
          />
          <div className="flex items-start justify-between gap-4 rounded-lg border bg-card p-4">
            <div className="flex min-w-0 flex-col gap-0.5">
              <Label htmlFor="plan-recommended">Recommended</Label>
              <p className="text-xs text-muted-foreground">
                Gives this plan the Recommended badge on the pricing table.
              </p>
            </div>
            <Switch
              id="plan-recommended"
              checked={draft.recommended}
              onCheckedChange={(recommended) => onChange({ recommended })}
            />
          </div>
        </>
      )}
      <details open={!!errors.sortOrder} className="group">
        <summary className="flex w-fit cursor-pointer list-none items-center gap-1 rounded-md py-1 text-sm font-bold text-muted-foreground outline-none hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
          <ChevronRightIcon
            aria-hidden="true"
            className="size-4 transition-transform group-open:rotate-90"
          />
          Advanced
        </summary>
        <div className="pt-3">
          <TextField
            id="plan-sort"
            label="Sort order"
            type="number"
            min={0}
            value={draft.sortOrder}
            onChange={(sortOrder) => onChange({ sortOrder })}
            className="w-32"
            error={errors.sortOrder}
            hint="Lower shows first on the pricing table."
          />
        </div>
      </details>
    </div>
  )
}
