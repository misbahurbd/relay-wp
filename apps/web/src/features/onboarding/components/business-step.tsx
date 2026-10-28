import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useRef } from 'react'
import { describedBy, FormField, fieldError, focusFirstInvalid } from '#/components/form'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Checkbox } from '#/components/ui/checkbox'
import { Input } from '#/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select'
import { ToggleGroup, ToggleGroupItem } from '#/components/ui/toggle-group'
import type { Workspace } from '#/features/workspaces'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { useSaveBusiness } from '../api/onboarding'
import {
  BUSINESS_GOALS,
  businessSchema,
  COUNTRIES,
  guessCountry,
  INDUSTRIES,
  TEAM_SIZES,
} from '../schemas/onboarding'
import type { BusinessGoal, Industry, TeamSize } from '../types'
import { StepFooter } from './step-footer'

type Props = { userId: string; workspace: Workspace; onDone: () => void }

export function BusinessStep({ userId, workspace, onDone }: Props) {
  const save = useSaveBusiness(userId, workspace.id)
  const formRef = useRef<HTMLFormElement>(null)
  // The default "Shakil's workspace" isn't a business name; start empty instead.
  const named = workspace.industry !== null

  const form = useForm({
    defaultValues: {
      name: named ? workspace.name : '',
      industry: (workspace.industry ?? '') as Industry,
      website: workspace.website ?? '',
      country: workspace.country ?? guessCountry(),
      team_size: (workspace.team_size ?? '2_5') as TeamSize,
      goals: (workspace.goals.length
        ? workspace.goals
        : ['marketing', 'support']) as BusinessGoal[],
    },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: async ({ value }) => {
      const ok = await save.mutateAsync({ ...value, website: value.website || undefined }).then(
        () => true,
        () => false,
      )
      if (ok) onDone()
    },
  })

  return (
    <form
      ref={formRef}
      noValidate
      className="flex flex-col gap-5"
      onSubmit={(e) => {
        e.preventDefault()
        form.handleSubmit()
      }}
    >
      <div className="flex flex-col gap-1">
        <h2 id="step-title" className="text-xl">
          Tell us about your business
        </h2>
        <p className="text-muted-foreground">
          We use this to set up your workspace and suggest ready-made templates.
        </p>
      </div>

      {save.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(save.error)}</AlertDescription>
        </Alert>
      )}

      <div className="grid gap-4 sm:grid-cols-2">
        <form.Field name="name" validators={{ onDynamic: businessSchema.shape.name }}>
          {(field) => {
            const error = fieldError(field)
            return (
              <FormField id="name" label="Business name" error={error}>
                <Input
                  id="name"
                  autoComplete="organization"
                  placeholder="e.g. Green Leaf Store"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                  aria-invalid={!!error}
                  aria-describedby={describedBy('name', { error })}
                />
              </FormField>
            )
          }}
        </form.Field>
        <form.Field name="industry" validators={{ onDynamic: businessSchema.shape.industry }}>
          {(field) => {
            const error = fieldError(field)
            return (
              <FormField id="industry" label="Industry" error={error}>
                <Select
                  value={field.state.value || undefined}
                  onValueChange={(v) => {
                    field.handleChange(v as Industry)
                    field.handleBlur()
                  }}
                >
                  <SelectTrigger
                    id="industry"
                    className="w-full"
                    aria-invalid={!!error}
                    aria-describedby={describedBy('industry', { error })}
                  >
                    <SelectValue placeholder="Choose an industry" />
                  </SelectTrigger>
                  <SelectContent>
                    {INDUSTRIES.map((i) => (
                      <SelectItem key={i.id} value={i.id}>
                        {i.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>
            )
          }}
        </form.Field>
        <form.Field name="website" validators={{ onDynamic: businessSchema.shape.website }}>
          {(field) => {
            const error = fieldError(field)
            return (
              <FormField id="website" label="Website (optional)" error={error}>
                <Input
                  id="website"
                  type="text"
                  inputMode="url"
                  autoComplete="url"
                  placeholder="yourstore.com"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                  aria-invalid={!!error}
                  aria-describedby={describedBy('website', { error })}
                />
              </FormField>
            )
          }}
        </form.Field>
        <form.Field name="country" validators={{ onDynamic: businessSchema.shape.country }}>
          {(field) => {
            const error = fieldError(field)
            return (
              <FormField id="country" label="Country" error={error}>
                <Select
                  value={field.state.value || undefined}
                  onValueChange={(v) => {
                    field.handleChange(v)
                    field.handleBlur()
                  }}
                >
                  <SelectTrigger
                    id="country"
                    className="w-full"
                    aria-invalid={!!error}
                    aria-describedby={describedBy('country', { error })}
                  >
                    <SelectValue placeholder="Choose a country" />
                  </SelectTrigger>
                  <SelectContent>
                    {COUNTRIES.map((c) => (
                      <SelectItem key={c.code} value={c.code}>
                        {c.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>
            )
          }}
        </form.Field>
      </div>

      <form.Field name="team_size" validators={{ onDynamic: businessSchema.shape.team_size }}>
        {(field) => (
          <fieldset className="flex flex-col gap-2.5">
            <legend className="mb-2.5 font-bold">How many people will answer chats?</legend>
            <ToggleGroup
              type="single"
              variant="outline"
              spacing={2}
              className="flex-wrap"
              value={field.state.value}
              onValueChange={(v) => v && field.handleChange(v as TeamSize)}
            >
              {TEAM_SIZES.map((t) => (
                <ToggleGroupItem key={t.id} value={t.id} className="min-w-20">
                  {t.label}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </fieldset>
        )}
      </form.Field>

      <form.Field name="goals" validators={{ onDynamic: businessSchema.shape.goals }}>
        {(field) => {
          const error = fieldError(field)
          const toggle = (id: BusinessGoal, on: boolean) => {
            const next = on ? [...field.state.value, id] : field.state.value.filter((g) => g !== id)
            field.handleChange(next)
            field.handleBlur()
          }
          return (
            <fieldset
              className="flex flex-col gap-2.5"
              aria-describedby={error ? 'goals-error' : 'goals-hint'}
            >
              <legend className="mb-2.5 font-bold">
                What do you want to use WhatsApp for?{' '}
                <span className="font-normal text-muted-foreground">Pick any</span>
              </legend>
              <div className="grid gap-2.5 sm:grid-cols-3">
                {BUSINESS_GOALS.map((g) => {
                  const on = field.state.value.includes(g.id)
                  return (
                    <label
                      key={g.id}
                      htmlFor={`goal-${g.id}`}
                      className={cn(
                        'flex items-start gap-3 rounded-lg border p-3.5 transition-colors',
                        on
                          ? 'border-primary bg-brand-soft'
                          : 'border-input bg-card hover:bg-accent',
                      )}
                    >
                      <Checkbox
                        id={`goal-${g.id}`}
                        className="mt-0.5"
                        checked={on}
                        onCheckedChange={(v) => toggle(g.id, v === true)}
                      />
                      <span className="flex flex-col">
                        <span className="font-bold">{g.label}</span>
                        <span className="text-sm text-muted-foreground">{g.hint}</span>
                      </span>
                    </label>
                  )
                })}
              </div>
              {error ? (
                <p id="goals-error" role="alert" className="text-sm text-destructive">
                  {error}
                </p>
              ) : (
                <p id="goals-hint" className="text-sm text-muted-foreground">
                  This only changes which starter templates we suggest.
                </p>
              )}
            </fieldset>
          )
        }}
      </form.Field>

      <StepFooter>
        <span />
        <form.Subscribe selector={(s) => s.isSubmitting}>
          {(submitting) => (
            <Button type="submit" disabled={submitting} className="sm:min-w-36">
              {submitting ? 'Saving…' : 'Continue'}
            </Button>
          )}
        </form.Subscribe>
      </StepFooter>
    </form>
  )
}
