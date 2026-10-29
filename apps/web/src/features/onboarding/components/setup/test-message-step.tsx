import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { CheckCheckIcon, CircleCheckIcon, CircleXIcon, LoaderCircleIcon } from 'lucide-react'
import { useRef, useState } from 'react'
import { describedBy, FormField, fieldError, focusFirstInvalid } from '#/components/form'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select'
import type { Workspace } from '#/features/workspaces'
import { errorMessage } from '#/lib/api'
import {
  numbersQueryOptions,
  testMessageQueryOptions,
  useCompleteOnboarding,
  useSendTestMessage,
} from '../../api/onboarding.api'
import { COUNTRIES, testMessageSchema } from '../../schemas/onboarding.schema'
import { StepFooter } from './step-footer'

type Props = { userId: string; workspace: Workspace; onBack: () => void }

export function TestMessageStep({ userId, workspace, onBack }: Props) {
  const navigate = useNavigate()
  const numbers = useQuery(numbersQueryOptions(userId, workspace.id))
  const send = useSendTestMessage(userId, workspace.id)
  const complete = useCompleteOnboarding(userId, workspace.id)
  const [messageId, setMessageId] = useState<string | null>(null)
  // Shown as the user typed it ("+1 415 555 0199"), not as raw digits.
  const [sentTo, setSentTo] = useState('')
  const message = useQuery({
    ...testMessageQueryOptions(userId, workspace.id, messageId ?? ''),
    enabled: messageId !== null,
  })
  const formRef = useRef<HTMLFormElement>(null)
  const number = numbers.data?.find((n) => n.status === 'connected')
  const defaultDial = COUNTRIES.find((c) => c.code === workspace.country)?.dial ?? '+1'

  const form = useForm({
    defaultValues: { dial: defaultDial, number: '' },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: async ({ value }) => {
      if (!number) return
      const to = `${value.dial}${value.number.replace(/\D/g, '')}`
      const sent = await send.mutateAsync({ numberId: number.id, to }).catch(() => null)
      if (sent) {
        setMessageId(sent.id)
        setSentTo(`${value.dial} ${value.number.trim()}`)
      }
    },
  })

  const finish = () =>
    complete.mutate(undefined, {
      onSuccess: () => navigate({ to: '/' }),
    })

  const status = message.data?.status
  const delivered = status === 'delivered' || status === 'read'
  const sameNumber = send.error && 'code' in send.error && send.error.code === 'same_number'

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-1">
        <h2 id="step-title" className="text-xl">
          Send your first test message
        </h2>
        <p className="text-muted-foreground">
          Send a message to your own phone to see exactly what customers will get.
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-[minmax(0,1fr)_260px]">
        <form
          ref={formRef}
          noValidate
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault()
            form.handleSubmit()
          }}
        >
          <form.Field name="number" validators={{ onDynamic: testMessageSchema.shape.number }}>
            {(field) => {
              const error = fieldError(field) ?? (sameNumber ? errorMessage(send.error) : undefined)
              return (
                <FormField
                  id="phone"
                  label="Your WhatsApp number"
                  error={error}
                  description="Use a different number from the one you just connected."
                >
                  <div className="flex gap-2">
                    <form.Field name="dial">
                      {(dial) => (
                        <Select value={dial.state.value} onValueChange={dial.handleChange}>
                          <SelectTrigger className="w-28 shrink-0" aria-label="Country code">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {[...new Set(COUNTRIES.map((c) => c.dial))].map((d) => (
                              <SelectItem key={d} value={d}>
                                {d}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      )}
                    </form.Field>
                    <Input
                      id="phone"
                      type="tel"
                      inputMode="tel"
                      autoComplete="tel-national"
                      placeholder="1712 345 678"
                      value={field.state.value}
                      onChange={(e) => {
                        field.handleChange(e.target.value)
                        if (send.isError) send.reset()
                      }}
                      onBlur={field.handleBlur}
                      aria-invalid={!!error}
                      aria-describedby={describedBy('phone', { error, hint: true })}
                    />
                  </div>
                </FormField>
              )
            }}
          </form.Field>

          <div className="flex items-center justify-between gap-3 rounded-lg bg-muted p-3.5">
            <span className="flex flex-col">
              <span className="text-xs font-bold text-muted-foreground">Template</span>
              <span className="font-mono text-sm font-bold">hello_world</span>
              <span className="text-xs text-muted-foreground">
                Pre-approved by Meta, so you can send it right away.
              </span>
            </span>
            <Badge variant="success">Approved</Badge>
          </div>

          {send.isError && !sameNumber && (
            <Alert variant="danger">
              <AlertDescription>{errorMessage(send.error)}</AlertDescription>
            </Alert>
          )}

          <DeliveryStatus status={status} to={sentTo} />

          {!delivered && (
            <form.Subscribe selector={(s) => s.isSubmitting}>
              {(submitting) => (
                <Button
                  type="submit"
                  variant={messageId ? 'outline' : 'default'}
                  disabled={submitting || !number || status === 'sent'}
                >
                  {submitting
                    ? 'Sending…'
                    : status === 'failed'
                      ? 'Try again'
                      : messageId
                        ? 'Send again'
                        : 'Send test message'}
                </Button>
              )}
            </form.Subscribe>
          )}
        </form>

        <PhonePreview
          businessName={number?.verified_name ?? workspace.name}
          delivered={delivered}
        />
      </div>

      {complete.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(complete.error)}</AlertDescription>
        </Alert>
      )}

      <StepFooter>
        <Button variant="ghost" onClick={onBack}>
          Back
        </Button>
        <Button
          variant={delivered ? 'default' : 'ghost'}
          onClick={finish}
          disabled={complete.isPending}
          className="sm:min-w-36"
        >
          {complete.isPending
            ? 'Opening…'
            : delivered
              ? 'Go to dashboard'
              : 'Skip test and go to dashboard'}
        </Button>
      </StepFooter>
    </div>
  )
}

function DeliveryStatus({ status, to }: { status?: string; to?: string }) {
  if (!status) return null
  if (status === 'delivered' || status === 'read') {
    return (
      <Alert variant="success" role="status">
        <CircleCheckIcon />
        <AlertTitle>Delivered to {to}</AlertTitle>
        <AlertDescription>
          Check your phone. Reply to it and the reply will show up in your Inbox.
        </AlertDescription>
      </Alert>
    )
  }
  if (status === 'failed') {
    return (
      <Alert variant="danger" role="alert">
        <CircleXIcon />
        <AlertTitle>We couldn’t deliver it</AlertTitle>
        <AlertDescription>
          Check the number is on WhatsApp and try again, or skip this step.
        </AlertDescription>
      </Alert>
    )
  }
  return (
    <p className="flex items-center gap-2 text-sm text-muted-foreground" role="status">
      <LoaderCircleIcon className="size-4 animate-spin" aria-hidden="true" />
      Sent. Waiting for WhatsApp to deliver it…
    </p>
  )
}

function PhonePreview({ businessName, delivered }: { businessName: string; delivered: boolean }) {
  const initials = businessName
    .split(/\s+/)
    .map((w) => w[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
  return (
    <figure
      className="flex flex-col overflow-hidden rounded-[28px] border-4 border-foreground/80 bg-chat"
      aria-label="Preview of the test message on WhatsApp"
    >
      <div className="flex items-center gap-2.5 bg-panel px-3.5 py-3 text-panel-foreground">
        <span className="flex size-8 items-center justify-center rounded-full bg-white/15 text-xs font-extrabold">
          {initials}
        </span>
        <span className="flex flex-col leading-tight">
          <span className="text-sm font-bold">{businessName}</span>
          <span className="text-xs text-panel-muted">Business account</span>
        </span>
      </div>
      <div className="flex min-h-64 flex-col gap-2 p-3">
        <span className="mx-auto rounded-md bg-card px-2 py-0.5 text-xs text-muted-foreground">
          Today
        </span>
        <div className="flex max-w-[92%] flex-col gap-1 rounded-lg rounded-tl-sm bg-bubble-in p-2.5 text-sm shadow-card">
          <span className="font-bold">Hello World</span>
          <span>
            Welcome and congratulations!! This message demonstrates your ability to send a WhatsApp
            message notification from the Cloud API, hosted by Meta.
          </span>
          <span className="text-xs text-muted-foreground">
            WhatsApp Business Platform sample message
          </span>
          <span className="flex items-center justify-end gap-1 text-xs text-muted-foreground">
            {delivered ? 'Now' : 'Preview'}
            {delivered && (
              <CheckCheckIcon className="size-3.5 text-tick-read" aria-label="Delivered" />
            )}
          </span>
        </div>
      </div>
    </figure>
  )
}
