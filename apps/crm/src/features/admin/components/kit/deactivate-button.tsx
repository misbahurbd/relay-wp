import { toast } from 'sonner'
import { Button } from '#/components/ui/button'
import { errorMessage } from '#/lib/api'

/** Asks first, then runs `run`. Deactivating can't be undone from here. */
export function DeactivateButton({
  what,
  run,
  pending,
}: {
  what: string
  run: () => Promise<unknown>
  pending?: boolean
}) {
  const click = () => {
    if (
      !window.confirm(
        `Deactivate ${what}? It goes off sale; what was already bought keeps working.`,
      )
    ) {
      return
    }
    run().then(
      () => toast.success(`Deactivated ${what}`),
      (e) => toast.error(errorMessage(e)),
    )
  }
  return (
    <Button type="button" variant="ghost" size="xs" disabled={pending} onClick={click}>
      Deactivate
    </Button>
  )
}
