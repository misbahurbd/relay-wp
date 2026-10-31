import { useQuery } from '@tanstack/react-query'
import { Loader2Icon, SearchIcon, SendIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '#/components/ui/dialog'
import { Input } from '#/components/ui/input'
import { cn } from '#/lib/utils'
import { useSendMessage, useUploadMedia } from '../../api/inbox.mutations'
import { templatesQueryOptions } from '../../api/inbox.queries'
import { CATEGORY_LABEL, CHANNEL_COPY, TEMPLATE_PRICE } from '../../schemas/inbox.schema'
import type { ChannelType, InboxTemplate, Upload } from '../../types'
import { inboxErrorMessage } from '../kit/errors'
import type { WindowState } from '../kit/window'
import { TemplateFields, type TemplateValues } from './template-fields'
import { TemplatePreview } from './template-preview'

const EMPTY: TemplateValues = { body: [], header: '', media: null, urls: {} }

/** Fills a new template's variables: {{1}} usually greets the customer by first name. */
function initialValues(t: InboxTemplate, first: string): TemplateValues {
  return {
    ...EMPTY,
    body: Array.from({ length: t.body_variables }, (_, i) => (i === 0 ? first : '')),
  }
}

/** What's still missing before the template can go out, or null when it's complete. */
function missing(t: InboxTemplate, v: TemplateValues) {
  if (v.body.slice(0, t.body_variables).some((x) => !x.trim()) || v.body.length < t.body_variables)
    return 'Fill in every variable.'
  if (t.header_variable && !v.header.trim()) return 'Fill in the header variable.'
  if (t.header_media && !v.media) return `Add the header ${t.header_type}.`
  if (t.url_buttons.some((i) => !v.urls[i]?.trim())) return 'Fill in every button link.'
  return null
}

/** "Send a template to …" (F4): pick an approved template, fill its variables, preview, send. */
export function TemplateDialog({
  workspaceId,
  conversationId,
  channelType,
  name,
  window: win,
  open,
  onOpenChange,
}: {
  workspaceId: string
  conversationId: string
  channelType: ChannelType
  name: string
  window: WindowState
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const query = useQuery({ ...templatesQueryOptions(workspaceId, channelType), enabled: open })
  const { metaPricing } = CHANNEL_COPY[channelType]
  const first = win.first
  const [q, setQ] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [values, setValues] = useState<TemplateValues>(EMPTY)
  const [showMissing, setShowMissing] = useState(false)
  const send = useSendMessage(workspaceId)
  const upload = useUploadMedia(workspaceId)

  const templates = query.data ?? []
  const needle = q.trim().toLowerCase()
  const shown = templates.filter(
    (t) => !needle || `${t.name} ${t.body} ${t.category}`.toLowerCase().includes(needle),
  )
  const selected = templates.find((t) => t.id === selectedId) ?? null
  const problem = selected ? missing(selected, values) : 'Choose a template.'

  const pick = (t: InboxTemplate) => {
    setSelectedId(t.id)
    setValues(initialValues(t, first))
    setShowMissing(false)
    send.reset()
  }

  const close = (next: boolean) => {
    if (!next) {
      setQ('')
      setSelectedId(null)
      setValues(EMPTY)
      setShowMissing(false)
      send.reset()
    }
    onOpenChange(next)
  }

  const submit = () => {
    if (!selected) return
    if (problem) {
      setShowMissing(true)
      return
    }
    send.mutate(
      {
        conversationId,
        body: {
          type: 'template',
          template_id: selected.id,
          media_key: values.media?.key,
          params: {
            body: values.body.slice(0, selected.body_variables).map((x) => x.trim()),
            header_text: selected.header_variable ? values.header.trim() : undefined,
            url_buttons: selected.url_buttons.map((index) => ({
              index,
              value: values.urls[index].trim(),
            })),
          },
        },
      },
      {
        onSuccess: () => {
          toast.success(
            metaPricing
              ? `Template sent · ${CATEGORY_LABEL[selected.category]} ${TEMPLATE_PRICE[selected.category]} est. Meta charge`
              : `Template sent · ${CATEGORY_LABEL[selected.category]}`,
          )
          close(false)
        },
      },
    )
  }

  const onHeaderFile = (file: File) =>
    upload.mutate(file, {
      onSuccess: (media: Upload) => setValues((v) => ({ ...v, media })),
      onError: (e) => toast.error(inboxErrorMessage(e, 'The upload failed. Please try again.')),
    })

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="flex max-h-[92dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-4xl">
        <DialogHeader className="border-b px-5 py-4 text-left">
          <DialogTitle>Send a template to {name}</DialogTitle>
          <DialogDescription>
            {win.limited
              ? `Only ${metaPricing ? 'Meta-approved' : 'approved'} templates can be sent outside the ${win.label}.`
              : 'Pick an approved template and fill in its variables.'}
          </DialogDescription>
        </DialogHeader>

        <div className="grid min-h-0 flex-1 gap-5 overflow-y-auto px-5 py-4 md:grid-cols-[1fr_300px]">
          <div className="flex min-w-0 flex-col gap-3">
            <label className="flex h-10 items-center gap-2 rounded-md border border-input bg-muted px-3 text-muted-foreground">
              <SearchIcon className="size-4" aria-hidden="true" />
              <span className="sr-only">Search templates</span>
              <Input
                unstyled
                type="search"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="Search approved templates"
                className="min-w-0 flex-1 bg-transparent text-base text-foreground outline-none md:text-sm"
              />
            </label>

            {query.isPending ? (
              <Loader2Icon
                className="mx-auto my-6 size-5 animate-spin"
                aria-label="Loading templates"
              />
            ) : query.isError ? (
              <Alert variant="danger">
                <AlertDescription>
                  {inboxErrorMessage(query.error, 'We couldn’t load templates.')}
                </AlertDescription>
              </Alert>
            ) : (
              <div
                role="radiogroup"
                aria-label="Approved templates"
                className="flex flex-col gap-1.5"
              >
                {shown.map((t) => (
                  <label
                    key={t.id}
                    className={cn(
                      'flex cursor-pointer items-start gap-3 rounded-lg border px-3 py-2.5 has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-ring',
                      t.id === selectedId ? 'border-brand bg-brand-soft' : 'hover:bg-muted',
                    )}
                  >
                    <Input
                      unstyled
                      type="radio"
                      name="inbox-template"
                      value={t.id}
                      checked={t.id === selectedId}
                      onChange={() => pick(t)}
                      className="sr-only"
                    />
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate text-sm font-bold">{t.name}</span>
                      <span className="line-clamp-1 text-xs text-muted-foreground">{t.body}</span>
                    </span>
                    <span className="flex flex-none flex-col items-end gap-0.5 text-xs">
                      <span className="rounded-sm bg-neutral-soft px-1.5 font-bold text-neutral">
                        {CATEGORY_LABEL[t.category]}
                      </span>
                      {metaPricing && (
                        <span className="text-muted-foreground">
                          est. Meta charge {TEMPLATE_PRICE[t.category]}
                        </span>
                      )}
                    </span>
                  </label>
                ))}
                {shown.length === 0 && (
                  <span className="py-4 text-center text-muted-foreground">
                    No approved template matches.
                  </span>
                )}
              </div>
            )}

            {selected && (
              <TemplateFields
                template={selected}
                values={values}
                onChange={setValues}
                uploading={upload.isPending}
                onHeaderFile={onHeaderFile}
              />
            )}
          </div>

          <TemplatePreview
            template={selected}
            values={values}
            window={win}
            metaPricing={metaPricing}
          />
        </div>

        <div className="flex flex-col gap-2 border-t px-5 py-3">
          {showMissing && problem && (
            <p role="alert" className="text-[13px] text-destructive">
              {problem}
            </p>
          )}
          {send.error && (
            <Alert variant="danger">
              <AlertDescription>{inboxErrorMessage(send.error)}</AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => close(false)}>
              Cancel
            </Button>
            <Button onClick={submit} disabled={!selected || send.isPending || upload.isPending}>
              {send.isPending ? (
                <Loader2Icon className="animate-spin" aria-hidden="true" />
              ) : (
                <SendIcon aria-hidden="true" />
              )}
              Send template
              {selected && metaPricing ? ` · ${TEMPLATE_PRICE[selected.category]}` : ''}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  )
}
