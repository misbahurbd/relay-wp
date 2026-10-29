import { CircleAlertIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { errorMessage } from '#/lib/api'

export function LoadError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return (
    <Alert variant="danger">
      <CircleAlertIcon aria-hidden="true" />
      <AlertTitle>Couldn’t load this</AlertTitle>
      <AlertDescription>
        <p>{errorMessage(error)}</p>
        <Button variant="outline" size="sm" className="mt-2" onClick={onRetry}>
          Try again
        </Button>
      </AlertDescription>
    </Alert>
  )
}
