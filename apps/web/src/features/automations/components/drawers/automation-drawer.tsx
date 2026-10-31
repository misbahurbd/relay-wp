import { XIcon } from 'lucide-react'
import { type FormEvent, useState } from 'react'
import { toast } from 'sonner'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from '#/components/ui/sheet'
import { errorMessage, isApiError } from '#/lib/api'
import { useSaveAutomation } from '../../api/automations.mutations'
import { description, KINDS } from '../../schemas/automations.schema'
import type { Automation, AutomationSettings, FieldErrors } from '../../types'
import { AwayForm } from './away-form'
import { CartForm } from './cart-form'
import { DripForm } from './drip-form'
import { KeywordForm } from './keyword-form'
import { OrderForm } from './order-form'
import { WelcomeForm } from './welcome-form'

/** The settings drawer: one form per kind, saved whole. */
export function AutomationDrawer({
  workspaceId,
  automation,
  canEdit,
  onClose,
}: {
  workspaceId: string
  automation: Automation
  canEdit: boolean
  onClose: () => void
}) {
  const save = useSaveAutomation(workspaceId)
  const [settings, setSettings] = useState<AutomationSettings>(automation.settings)
  const [errors, setErrors] = useState<FieldErrors>({})
  const kind = KINDS[automation.kind]
  const patch = (next: AutomationSettings) => setSettings({ ...settings, ...next })
  const form = { workspaceId, settings, errors, onChange: patch, firstSave: !automation.updated_at }

  function submit(e: FormEvent) {
    e.preventDefault()
    setErrors({})
    save.mutate(
      { kind: automation.kind, settings },
      {
        onSuccess: () => {
          toast.success('Settings saved')
          onClose()
        },
        onError: (err) => {
          if (isApiError(err)) {
            const found: FieldErrors = {}
            for (const d of err.details ?? []) if (d.field) found[d.field] = d.message
            setErrors(found)
          }
          toast.error(errorMessage(err))
        },
      },
    )
  }

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent
        side="right"
        showCloseButton={false}
        className="flex h-full w-full flex-col gap-0 p-0 sm:max-w-[540px]"
      >
        <form onSubmit={submit} className="flex min-h-0 flex-1 flex-col">
          <div className="flex items-center gap-3 border-b px-[22px] py-[18px]">
            <div className="flex flex-1 flex-col gap-0.5">
              <SheetTitle className="text-lg font-extrabold">{kind.title}</SheetTitle>
              <SheetDescription className="text-[13px] text-muted-foreground">
                {description(automation)}
              </SheetDescription>
            </div>
            <SheetClose
              aria-label="Close"
              className="flex size-10 items-center justify-center rounded-[10px] text-muted-foreground hover:bg-muted hover:text-foreground"
            >
              <XIcon className="size-[18px]" aria-hidden="true" />
            </SheetClose>
          </div>
          <div className="flex flex-1 flex-col gap-[18px] overflow-y-auto p-[22px] text-sm">
            {automation.kind === 'welcome' && <WelcomeForm {...form} />}
            {automation.kind === 'away' && <AwayForm {...form} />}
            {automation.kind === 'keyword' && <KeywordForm {...form} />}
            {automation.kind === 'cart' && <CartForm {...form} />}
            {automation.kind === 'order' && <OrderForm {...form} />}
            {automation.kind === 'drip' && <DripForm {...form} />}
          </div>
          <div className="flex justify-end gap-2.5 border-t px-[22px] py-4">
            <button
              type="button"
              onClick={onClose}
              className="h-11 rounded-[10px] border bg-card px-[18px] font-bold hover:bg-muted"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!canEdit || save.isPending}
              title={canEdit ? undefined : "Your role can't edit automations"}
              className="h-11 rounded-[10px] bg-primary px-5 font-bold text-primary-foreground hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {save.isPending ? 'Saving…' : 'Save changes'}
            </button>
          </div>
        </form>
      </SheetContent>
    </Sheet>
  )
}

export type FormProps = {
  workspaceId: string
  settings: AutomationSettings
  errors: FieldErrors
  onChange: (next: AutomationSettings) => void
  /** Nothing has been saved yet, so defaults may follow the browser. */
  firstSave: boolean
}
