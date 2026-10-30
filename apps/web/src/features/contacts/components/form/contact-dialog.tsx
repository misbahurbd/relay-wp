import { revalidateLogic, useForm } from '@tanstack/react-form'
import { Loader2Icon, PlusIcon } from 'lucide-react'
import { type ComponentProps, useRef, useState } from 'react'
import { toast } from 'sonner'
import { describedBy, FormField, fieldError, focusFirstInvalid } from '#/components/form'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select'
import { errorMessage, isApiError } from '#/lib/api'
import { useCreateContact, useUpdateContact } from '../../api/contacts.mutations'
import { useTags } from '../../hooks/use-tags'
import {
  type ContactFormValues,
  contactFormSchema,
  LANGUAGES,
  OPT_IN_LABEL,
  OPT_INS,
} from '../../schemas/contacts.schema'
import type { ContactDetail, OptIn, TagRef } from '../../types'
import { Modal } from '../kit/modal'
import { TagChip } from '../kit/tag-chip'
import { TagPicker } from '../kit/tag-picker'

type FieldName = 'name' | 'phone' | 'email' | 'language'
type ServerErrors = Partial<Record<FieldName, string>>
const FIELDS: FieldName[] = ['name', 'phone', 'email', 'language']

/** Puts the API's per-field problems on the field they belong to. */
function serverFieldErrors(error: unknown): ServerErrors | null {
  if (!isApiError(error)) return null
  if (error.code === 'invalid_phone' || error.code === 'contact_exists') {
    return { phone: error.details?.[0]?.message ?? error.message }
  }
  const out: ServerErrors = {}
  for (const d of error.details ?? []) {
    const name = d.field?.replace(/^body\./, '') as FieldName | undefined
    if (name && FIELDS.includes(name)) out[name] ??= d.message
  }
  return Object.keys(out).length ? out : null
}

/** Add a contact, or edit one when `contact` is given. */
export function ContactDialog({
  workspaceId,
  contact,
  open,
  onClose,
  onSaved,
}: {
  workspaceId: string
  contact?: ContactDetail
  open: boolean
  onClose: () => void
  onSaved?: (contactId: string) => void
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={contact ? 'Edit contact' : 'Add contact'}
      description={contact ? contact.phone_display : 'Add someone you can message on WhatsApp.'}
    >
      {open && (
        <ContactForm
          workspaceId={workspaceId}
          contact={contact}
          onClose={onClose}
          onSaved={onSaved}
        />
      )}
    </Modal>
  )
}

function ContactForm({
  workspaceId,
  contact,
  onClose,
  onSaved,
}: {
  workspaceId: string
  contact?: ContactDetail
  onClose: () => void
  onSaved?: (contactId: string) => void
}) {
  const create = useCreateContact(workspaceId)
  const update = useUpdateContact(workspaceId)
  const save = contact ? update : create
  const { tags, create: createTag } = useTags(workspaceId)
  const formRef = useRef<HTMLFormElement>(null)
  const [server, setServer] = useState<ServerErrors>({})
  const [picked, setPicked] = useState<TagRef[]>(contact?.tags ?? [])

  const form = useForm({
    defaultValues: {
      name: contact?.name ?? '',
      phone: contact?.phone_display ?? '',
      email: contact?.email ?? '',
      opt_in: contact?.opt_in ?? 'pending',
      language: contact?.language ?? '',
      tag_ids: contact?.tags.map((t) => t.id) ?? [],
    } as ContactFormValues,
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: async ({ value }) => {
      const v = contactFormSchema.parse(value)
      try {
        const saved = contact
          ? await update.mutateAsync({
              contactId: contact.id,
              body: {
                name: v.name,
                phone: v.phone,
                email: v.email,
                opt_in: v.opt_in,
                language: v.language.trim(),
                tag_ids: v.tag_ids,
              },
            })
          : await create.mutateAsync({
              name: v.name,
              phone: v.phone,
              email: v.email || undefined,
              opt_in: v.opt_in,
              language: v.language.trim() || undefined,
              tag_ids: v.tag_ids,
            })
        toast.success(contact ? 'Contact saved' : 'Contact added')
        onSaved?.(saved.id)
        onClose()
      } catch (error) {
        const fields = serverFieldErrors(error)
        if (fields) {
          setServer(fields)
          focusFirstInvalid(formRef.current)
        }
      }
    },
  })

  const serverOnly = save.isError && !serverFieldErrors(save.error)

  const text = (
    name: FieldName,
    label: string,
    props: ComponentProps<typeof Input>,
    hint?: string,
  ) => (
    <form.Field
      name={name}
      validators={{
        onDynamic: ({ value }) =>
          contactFormSchema.shape[name].safeParse(value).error?.issues[0]?.message,
      }}
    >
      {(field) => {
        const error = fieldError(field) ?? server[name]
        const id = `contact-${name}`
        return (
          <FormField id={id} label={label} error={error} description={hint}>
            <Input
              id={id}
              value={field.state.value}
              onChange={(e) => {
                field.handleChange(e.target.value)
                if (server[name]) setServer((s) => ({ ...s, [name]: undefined }))
              }}
              onBlur={field.handleBlur}
              aria-invalid={!!error}
              aria-describedby={describedBy(id, { error, hint: !!hint })}
              {...props}
            />
          </FormField>
        )
      }}
    </form.Field>
  )

  return (
    <form
      ref={formRef}
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault()
        void form.handleSubmit()
      }}
    >
      {serverOnly && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(save.error)}</AlertDescription>
        </Alert>
      )}
      {text('name', 'Name', { autoComplete: 'name', placeholder: 'e.g. Ayesha Rahman' })}
      {text(
        'phone',
        'WhatsApp number',
        { type: 'tel', autoComplete: 'tel', placeholder: '+44 7700 900412' },
        'Include the country code, starting with +.',
      )}
      {text('email', 'Email (optional)', {
        type: 'email',
        autoComplete: 'email',
        placeholder: 'name@example.com',
      })}
      <div className="grid gap-4 sm:grid-cols-2">
        <form.Field name="opt_in">
          {(field) => (
            <FormField id="contact-opt-in" label="WhatsApp opt-in">
              <Select
                value={field.state.value}
                onValueChange={(v) => field.handleChange(v as OptIn)}
              >
                <SelectTrigger id="contact-opt-in" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {OPT_INS.map((o) => (
                    <SelectItem key={o} value={o}>
                      {OPT_IN_LABEL[o]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </FormField>
          )}
        </form.Field>
        {text('language', 'Language (optional)', {
          list: 'contact-languages',
          placeholder: 'e.g. English',
          autoComplete: 'off',
        })}
        <datalist id="contact-languages">
          {LANGUAGES.map((l) => (
            <option key={l} value={l} />
          ))}
        </datalist>
      </div>
      <form.Field name="tag_ids">
        {(field) => {
          const toggle = (tag: TagRef) => {
            const on = field.state.value.includes(tag.id)
            field.handleChange(
              on ? field.state.value.filter((id) => id !== tag.id) : [...field.state.value, tag.id],
            )
            setPicked((p) => (on ? p.filter((t) => t.id !== tag.id) : [...p, tag]))
          }
          return (
            <fieldset className="flex flex-col gap-2">
              <legend className="mb-2 text-sm font-bold">Tags (optional)</legend>
              <div className="flex flex-wrap items-center gap-1.5">
                {picked.map((t) => (
                  <TagChip key={t.id} name={t.name} round onRemove={() => toggle(t)} />
                ))}
                <TagPicker
                  multi
                  tags={tags}
                  selectedIds={field.state.value}
                  onPick={toggle}
                  onCreate={createTag}
                  trigger={
                    <button
                      type="button"
                      className="inline-flex h-7 items-center gap-1 rounded-full border border-dashed border-input px-2.5 text-xs font-bold text-brand-text hover:bg-muted"
                    >
                      <PlusIcon className="size-3" aria-hidden="true" />
                      Add tag
                    </button>
                  }
                />
              </div>
            </fieldset>
          )
        }}
      </form.Field>
      <div className="flex justify-end gap-2.5 pt-2">
        <Button type="button" variant="outline" onClick={onClose}>
          Cancel
        </Button>
        <form.Subscribe selector={(s) => s.isSubmitting}>
          {(submitting) => (
            <Button type="submit" disabled={submitting}>
              {submitting && <Loader2Icon className="animate-spin" aria-hidden="true" />}
              {contact ? 'Save contact' : 'Add contact'}
            </Button>
          )}
        </form.Subscribe>
      </div>
    </form>
  )
}
