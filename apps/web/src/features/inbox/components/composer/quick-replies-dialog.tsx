import { useQuery } from '@tanstack/react-query'
import { Loader2Icon, PencilIcon, PlusIcon, Trash2Icon } from 'lucide-react'
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
import { Label } from '#/components/ui/label'
import { Textarea } from '#/components/ui/textarea'
import { errorMessage } from '#/lib/api'
import {
  useCreateQuickReply,
  useDeleteQuickReply,
  useUpdateQuickReply,
} from '../../api/inbox.mutations'
import { quickRepliesQueryOptions } from '../../api/inbox.queries'
import { type QuickReplyValues, quickReplySchema } from '../../schemas/inbox.schema'
import type { QuickReply } from '../../types'

type Editing = { id: string | null; values: QuickReplyValues }
type Field = keyof QuickReplyValues

const EMPTY: QuickReplyValues = { shortcut: '', title: '', body: '' }

/** Manage the workspace's quick replies (F8): list, add, edit and delete. Needs inbox.reply. */
export function QuickRepliesDialog({
  workspaceId,
  open,
  onOpenChange,
}: {
  workspaceId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const list = useQuery({ ...quickRepliesQueryOptions(workspaceId), enabled: open })
  const [editing, setEditing] = useState<Editing | null>(null)
  const remove = useDeleteQuickReply(workspaceId)

  const close = (next: boolean) => {
    if (!next) setEditing(null)
    onOpenChange(next)
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Quick replies</DialogTitle>
          <DialogDescription>
            Type / and the shortcut in the composer to insert a reply. Everyone in the workspace can
            use them.
          </DialogDescription>
        </DialogHeader>

        {editing ? (
          <QuickReplyForm
            workspaceId={workspaceId}
            editing={editing}
            onDone={() => setEditing(null)}
          />
        ) : (
          <>
            {list.isPending ? (
              <Loader2Icon
                className="mx-auto size-5 animate-spin"
                aria-label="Loading quick replies"
              />
            ) : list.isError ? (
              <Alert variant="danger">
                <AlertDescription>{errorMessage(list.error)}</AlertDescription>
              </Alert>
            ) : list.data.length === 0 ? (
              <p className="text-muted-foreground">
                No quick replies yet. Add the answers your team sends most.
              </p>
            ) : (
              <ul className="flex flex-col divide-y rounded-lg border">
                {list.data.map((r) => (
                  <QuickReplyRow
                    key={r.id}
                    reply={r}
                    deleting={remove.isPending && remove.variables === r.id}
                    onEdit={() =>
                      setEditing({
                        id: r.id,
                        values: { shortcut: r.shortcut, title: r.title, body: r.body },
                      })
                    }
                    onDelete={() =>
                      remove.mutate(r.id, {
                        onSuccess: () => toast.success('Quick reply deleted'),
                        onError: (e) => toast.error(errorMessage(e)),
                      })
                    }
                  />
                ))}
              </ul>
            )}
            <DialogFooter>
              <Button onClick={() => setEditing({ id: null, values: EMPTY })}>
                <PlusIcon aria-hidden="true" />
                Add quick reply
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

function QuickReplyRow({
  reply: r,
  deleting,
  onEdit,
  onDelete,
}: {
  reply: QuickReply
  deleting: boolean
  onEdit: () => void
  onDelete: () => void
}) {
  return (
    <li className="flex items-start gap-3 px-3 py-2.5">
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-sm font-bold">
          /{r.shortcut} <span className="font-semibold text-muted-foreground">· {r.title}</span>
        </span>
        <span className="line-clamp-2 text-[13px] text-muted-foreground">{r.body}</span>
      </span>
      <Button variant="ghost" size="icon-sm" aria-label={`Edit /${r.shortcut}`} onClick={onEdit}>
        <PencilIcon aria-hidden="true" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={`Delete /${r.shortcut}`}
        className="text-destructive hover:text-destructive"
        disabled={deleting}
        onClick={onDelete}
      >
        {deleting ? (
          <Loader2Icon className="animate-spin" aria-hidden="true" />
        ) : (
          <Trash2Icon aria-hidden="true" />
        )}
      </Button>
    </li>
  )
}

function QuickReplyForm({
  workspaceId,
  editing,
  onDone,
}: {
  workspaceId: string
  editing: Editing
  onDone: () => void
}) {
  const [values, setValues] = useState(editing.values)
  const [touched, setTouched] = useState<Partial<Record<Field, boolean>>>({})
  const create = useCreateQuickReply(workspaceId)
  const update = useUpdateQuickReply(workspaceId)
  const mutation = editing.id ? update : create

  const parsed = quickReplySchema.safeParse(values)
  const errors: Partial<Record<Field, string>> = {}
  if (!parsed.success) {
    for (const issue of parsed.error.issues) {
      const k = issue.path[0] as Field
      errors[k] ??= issue.message
    }
  }
  const errorFor = (k: Field) => (touched[k] ? errors[k] : undefined)

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    setTouched({ shortcut: true, title: true, body: true })
    if (!parsed.success) {
      const firstBad = (['shortcut', 'title', 'body'] as const).find((k) => errors[k])
      if (firstBad) document.getElementById(`qr-${firstBad}`)?.focus()
      return
    }
    const done = {
      onSuccess: () => {
        toast.success(editing.id ? 'Quick reply saved' : 'Quick reply added')
        onDone()
      },
    }
    if (editing.id) update.mutate({ id: editing.id, body: parsed.data }, done)
    else create.mutate(parsed.data, done)
  }

  const field = (k: Field, label: string, hint?: string) => ({
    id: `qr-${k}`,
    value: values[k],
    'aria-invalid': !!errorFor(k) || undefined,
    'aria-describedby': errorFor(k) ? `qr-${k}-error` : hint ? `qr-${k}-hint` : undefined,
    onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      setValues((v) => ({ ...v, [k]: e.target.value })),
    onBlur: () => setTouched((t) => ({ ...t, [k]: true })),
    label,
  })

  return (
    <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
      {(['shortcut', 'title'] as const).map((k) => {
        const { label, ...props } = field(k, k === 'shortcut' ? 'Shortcut' : 'Title')
        return (
          <div key={k} className="flex flex-col gap-1.5">
            <Label htmlFor={props.id}>{label}</Label>
            <Input {...props} placeholder={k === 'shortcut' ? 'tracking' : 'Tracking link'} />
            {k === 'shortcut' && !errorFor(k) && (
              <span id="qr-shortcut-hint" className="text-xs text-muted-foreground">
                Typed after / in the composer: lowercase letters, numbers, - and _.
              </span>
            )}
            {errorFor(k) && (
              <span id={`qr-${k}-error`} className="text-[13px] text-destructive">
                {errorFor(k)}
              </span>
            )}
          </div>
        )
      })}
      <div className="flex flex-col gap-1.5">
        {(() => {
          const { label, ...props } = field('body', 'Reply')
          return (
            <>
              <Label htmlFor={props.id}>{label}</Label>
              <Textarea {...props} rows={4} maxLength={1024} />
              {errorFor('body') && (
                <span id="qr-body-error" className="text-[13px] text-destructive">
                  {errorFor('body')}
                </span>
              )}
            </>
          )
        })()}
      </div>
      {mutation.error && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(mutation.error)}</AlertDescription>
        </Alert>
      )}
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          Back
        </Button>
        <Button type="submit" disabled={mutation.isPending}>
          {mutation.isPending && <Loader2Icon className="animate-spin" aria-hidden="true" />}
          {editing.id ? 'Save quick reply' : 'Add quick reply'}
        </Button>
      </DialogFooter>
    </form>
  )
}
