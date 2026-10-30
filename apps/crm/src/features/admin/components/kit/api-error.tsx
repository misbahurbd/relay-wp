import { Alert, AlertDescription } from '#/components/ui/alert'
import { errorMessage, isApiError } from '#/lib/api'

/** The API's error with its field details, above a form's buttons. */
export function ApiErrorAlert({ error }: { error: unknown }) {
  if (error == null) return null
  const details = isApiError(error) ? (error.details ?? []) : []
  return (
    <Alert variant="danger">
      <AlertDescription>
        <p>{errorMessage(error)}</p>
        {details.length > 0 && (
          <ul className="mt-1 list-disc pl-4">
            {details.map((d) => (
              <li key={`${d.field}-${d.message}`}>
                {d.field ? `${d.field.replace(/^body\./, '')}: ` : ''}
                {d.message}
              </li>
            ))}
          </ul>
        )}
      </AlertDescription>
    </Alert>
  )
}
