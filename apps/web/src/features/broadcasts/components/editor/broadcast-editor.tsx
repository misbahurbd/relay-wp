import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  ArrowLeftIcon,
  CalendarClockIcon,
  ChevronRightIcon,
  Trash2Icon,
  XCircleIcon,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { useAccess } from '#/features/team'
import { type Template, templatesQueryOptions } from '#/features/templates'
import { useCancelBroadcast, useDeleteBroadcast } from '../../api/broadcasts.mutations'
import { audienceQueryOptions, sendingNumbersQueryOptions } from '../../api/broadcasts.queries'
import { useBroadcastEditor } from '../../hooks/use-broadcast-editor'
import {
  audienceEmpty,
  emptyForm,
  fitVariables,
  formFrom,
  templateVariables,
} from '../../schemas/broadcasts.schema'
import type { Broadcast, Schedule } from '../../types'
import { ActionError } from '../kit/action-error'
import { formatCount, formatWhen } from '../kit/format'
import { StatusBadge } from '../kit/status-badge'
import { zonedToIso, zoneLabel } from '../kit/time-zone'
import { AudienceStep } from './audience-step'
import { type AddedList, ListUpload } from './list-upload'
import { ReviewStep } from './review-step'
import { ScheduleStep } from './schedule-step'
import { SentView } from './sent-view'
import { Stepper } from './stepper'
import { SummaryPanel } from './summary-panel'
import { TemplateStep } from './template-step'

type Props = {
  workspaceId: string
  broadcast?: Broadcast
  tagIds?: string[]
  segmentIds?: string[]
  templateId?: string
}

/**
 * Create or edit a draft or scheduled broadcast in four steps: Audience, Template, Schedule,
 * Review & send, with the summary beside them. New drafts can start from a tag (Contacts'
 * "Send broadcast") or a template (Templates' "Use in broadcast"). "Create another" starts over.
 */
export function BroadcastEditor(props: Props) {
  const navigate = useNavigate()
  const [round, setRound] = useState(0)
  return (
    <Wizard
      key={round}
      {...props}
      onAnother={() => {
        if (props.broadcast) navigate({ to: '/broadcasts/new' })
        else setRound((r) => r + 1)
      }}
    />
  )
}

/** "2 tags, 1 segment, 1 list" */
function sources(tags: number, lists: number, segments: number) {
  const t = tags - lists
  const parts = [
    t > 0 && `${t} ${t === 1 ? 'tag' : 'tags'}`,
    segments > 0 && `${segments} ${segments === 1 ? 'segment' : 'segments'}`,
    lists > 0 && `${lists} ${lists === 1 ? 'list' : 'lists'}`,
  ].filter(Boolean)
  return parts.join(', ') || 'no source'
}

function whenText(s: Schedule) {
  if (s.when === 'now') return 'Immediately'
  if (!s.date || !s.time) return 'Pick a date and time'
  return `${formatWhen(zonedToIso(s.date, s.time, s.zone))} · ${zoneLabel(s.zone)}`
}

function Wizard({
  workspaceId,
  broadcast,
  tagIds,
  segmentIds,
  templateId,
  onAnother,
}: Props & { onAnother: () => void }) {
  const navigate = useNavigate()
  const { access, can } = useAccess(workspaceId)
  const numbers = useQuery(sendingNumbersQueryOptions(workspaceId))
  const templatesQuery = useInfiniteQuery(
    templatesQueryOptions(workspaceId, { status: 'approved' }),
  )
  const templates = (templatesQuery.data?.pages.flatMap((p) => p.items) ?? []) as Template[]
  const editor = useBroadcastEditor({
    workspaceId,
    broadcast,
    initial: broadcast
      ? formFrom(broadcast)
      : { ...emptyForm(tagIds, segmentIds), template_id: templateId },
  })
  const { form, setForm, errors, step, setStep, schedule } = editor
  const template = templates.find((t) => t.id === form.template_id)
  const empty = audienceEmpty(form.audience)
  const audience = useQuery({
    ...audienceQueryOptions(workspaceId, form.audience, form.number_id),
    enabled: !empty,
  })
  const cancel = useCancelBroadcast(workspaceId)
  const remove = useDeleteBroadcast(workspaceId)
  const [lists, setLists] = useState<AddedList[]>([])

  // While access loads, keep the form usable; the API checks every request anyway.
  const readOnly = !!access && !can('broadcasts.create')
  const scheduled = broadcast?.status === 'scheduled'
  const sent = editor.sent

  // Default to the first connected number, and fit the blanks to a template picked from a link.
  useEffect(() => {
    const first = numbers.data?.[0]?.id
    if (!form.number_id && first) setForm((f) => ({ ...f, number_id: first }))
  }, [numbers.data, form.number_id, setForm])
  useEffect(() => {
    if (template && !broadcast) {
      setForm((f) => (f.body.length || f.header ? f : fitVariables(f, template)))
    }
  }, [template, broadcast, setForm])

  const included = audience.data?.included ?? 0
  const tags = form.audience.tag_ids.length
  const blocked =
    (step === 1 && (empty || (audience.isSuccess && included === 0))) ||
    (step === 2 && !template) ||
    (step === 3 && schedule.when === 'later' && (!schedule.date || !schedule.time))
  const tv = templateVariables(template)
  const reviewRows = [
    { label: 'Campaign', value: form.name.trim() || 'Untitled broadcast', step: 1 },
    {
      label: 'Audience',
      value: form.audience.all
        ? `Everyone who opted in · ${formatCount(included)} contacts`
        : `${formatCount(included)} contacts from ${sources(tags, lists.length, form.audience.segment_ids.length)}`,
      step: 1,
    },
    {
      label: 'Template',
      value: template
        ? `${template.name} · ${tv.body + (tv.header ? 1 : 0)} variables`
        : 'None chosen',
      step: 2,
    },
    {
      label: 'Send time',
      value:
        whenText(schedule) +
        (form.audience.skip_recent_marketing ? ' · recently messaged contacts skipped' : ''),
      step: 3,
    },
  ]

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-1">
          <Link
            to="/broadcasts"
            className="inline-flex items-center gap-1 text-[13px] font-bold text-brand-text no-underline"
          >
            <ArrowLeftIcon className="size-3.5" aria-hidden="true" />
            Broadcasts
          </Link>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-[26px] font-extrabold tracking-[-0.02em] break-words">
              {broadcast ? broadcast.name : 'New broadcast'}
            </h1>
            {broadcast && <StatusBadge status={broadcast.status} />}
          </div>
        </div>
        {!sent && (
          <div className="flex flex-wrap gap-2.5">
            {broadcast?.status === 'draft' && can('broadcasts.create') && (
              <Button
                variant="outline"
                className="text-destructive"
                disabled={remove.isPending}
                onClick={() =>
                  remove.mutate(broadcast.id, {
                    onSuccess: () => {
                      toast.success(`${broadcast.name} deleted`)
                      navigate({ to: '/broadcasts' })
                    },
                  })
                }
              >
                <Trash2Icon aria-hidden="true" />
                Delete
              </Button>
            )}
            {!readOnly && (
              <Button
                variant="outline"
                disabled={!!editor.busy}
                onClick={async () => {
                  if (await editor.save())
                    toast.success(scheduled ? 'Changes saved' : 'Draft saved')
                }}
              >
                {editor.busy === 'save' ? 'Saving…' : scheduled ? 'Save changes' : 'Save as draft'}
              </Button>
            )}
          </div>
        )}
      </div>

      {scheduled && broadcast && !sent && (
        <Alert variant="info">
          <CalendarClockIcon aria-hidden="true" />
          <AlertTitle className="font-bold">
            Scheduled for {formatWhen(broadcast.scheduled_at)}
          </AlertTitle>
          <AlertDescription className="flex flex-wrap items-center gap-3">
            You can still change it. The audience is taken when it starts sending.
            {can('broadcasts.send') && (
              <Button
                variant="outline"
                size="sm"
                disabled={cancel.isPending}
                onClick={() =>
                  cancel.mutate(broadcast.id, {
                    onSuccess: () => toast.success('Broadcast cancelled'),
                  })
                }
              >
                <XCircleIcon aria-hidden="true" />
                Cancel broadcast
              </Button>
            )}
          </AlertDescription>
        </Alert>
      )}

      <Stepper step={step} done={!!sent} onStep={setStep} />

      {(editor.error || cancel.error || remove.error) && !sent && (
        <ActionError error={editor.error ?? cancel.error ?? remove.error} />
      )}

      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_320px]">
        <section
          aria-label="Broadcast step"
          className="flex min-w-0 flex-col gap-5 rounded-2xl border bg-card p-5 shadow-card sm:p-6"
        >
          {sent ? (
            <SentView broadcast={sent} recipients={audience.data?.included} onAnother={onAnother} />
          ) : (
            <>
              {step === 1 && (
                <AudienceStep
                  workspaceId={workspaceId}
                  audience={form.audience}
                  readOnly={readOnly}
                  error={errors['audience.tag_ids'] ?? errors.audience}
                  hideTagIds={lists.map((l) => l.tagId)}
                  onChange={(a) => setForm({ ...form, audience: a })}
                  extra={
                    can('contacts.import') && (
                      <ListUpload
                        workspaceId={workspaceId}
                        lists={lists}
                        readOnly={readOnly}
                        onAdd={(l) => {
                          setLists((ls) => [...ls.filter((x) => x.tagId !== l.tagId), l])
                          setForm((f) => ({
                            ...f,
                            audience: {
                              ...f.audience,
                              all: false,
                              tag_ids: [
                                ...f.audience.tag_ids.filter((t) => t !== l.tagId),
                                l.tagId,
                              ],
                            },
                          }))
                        }}
                        onRemove={(tagId) => {
                          setLists((ls) => ls.filter((x) => x.tagId !== tagId))
                          setForm((f) => ({
                            ...f,
                            audience: {
                              ...f.audience,
                              tag_ids: f.audience.tag_ids.filter((t) => t !== tagId),
                            },
                          }))
                        }}
                      />
                    )
                  }
                />
              )}
              {step === 2 && (
                <TemplateStep
                  templates={templates}
                  loading={templatesQuery.isPending}
                  form={form}
                  template={template}
                  readOnly={readOnly}
                  errors={errors}
                  senderName={numbers.data?.find((n) => n.id === form.number_id)?.verified_name}
                  onChange={setForm}
                />
              )}
              {step === 3 && (
                <ScheduleStep
                  schedule={schedule}
                  readOnly={readOnly}
                  error={errors.scheduled_at}
                  onChange={editor.setSchedule}
                />
              )}
              {step === 4 && (
                <ReviewStep
                  rows={reviewRows}
                  marketing={template?.category === 'marketing'}
                  onEdit={setStep}
                />
              )}

              <div className="flex items-center justify-between gap-3 border-t pt-4">
                <Button variant="outline" disabled={step === 1} onClick={() => setStep(step - 1)}>
                  Back
                </Button>
                <span className="text-[13px] text-muted-foreground">Step {step} of 4</span>
                {step < 4 ? (
                  <Button disabled={blocked} onClick={() => setStep(step + 1)}>
                    Continue
                    <ChevronRightIcon aria-hidden="true" />
                  </Button>
                ) : (
                  <Button
                    disabled={!!editor.busy || readOnly || (!!access && !can('broadcasts.send'))}
                    onClick={() => editor.send()}
                  >
                    {editor.busy
                      ? 'Sending…'
                      : schedule.when === 'now'
                        ? 'Send broadcast'
                        : 'Schedule broadcast'}
                    <ChevronRightIcon aria-hidden="true" />
                  </Button>
                )}
              </div>
              {step === 4 && access && !can('broadcasts.send') && (
                <p className="text-right text-[13px] text-muted-foreground">
                  Your role can save drafts. Ask an owner or admin to send it.
                </p>
              )}
            </>
          )}
        </section>

        <SummaryPanel
          name={form.name}
          numberId={form.number_id}
          numbers={numbers.data}
          count={audience.data}
          empty={empty}
          templateName={template?.name}
          category={template?.category}
          when={whenText(schedule)}
          readOnly={readOnly || !!sent}
          errors={errors}
          onName={(name) => setForm({ ...form, name })}
          onNumber={(number_id) => setForm({ ...form, number_id })}
        />
      </div>
    </main>
  )
}
