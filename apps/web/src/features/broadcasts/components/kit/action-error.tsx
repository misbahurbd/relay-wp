import { Link } from '@tanstack/react-router'
import { AlertCircleIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { errorMessage, isApiError } from '#/lib/api'

const TITLE: Record<string, string> = {
  plan_broadcast_limit: 'Monthly broadcasts used up',
  broadcast_not_ready: 'A few things are missing',
  invalid_broadcast: 'Some fields need changes',
  broadcast_empty_audience: 'Nobody to send to',
  template_not_approved: 'Template not approved',
  number_not_connected: 'WhatsApp number not connected',
  broadcast_not_editable: 'This broadcast has started',
}

/** An error from a broadcasts action, kept on the page (DESIGN-SYSTEM §4). */
export function ActionError({ error }: { error: unknown }) {
  const code = isApiError(error) ? error.code : ''
  return (
    <Alert variant="danger">
      <AlertCircleIcon aria-hidden="true" />
      <AlertTitle className="line-clamp-none font-bold">
        {TITLE[code] ?? 'Something went wrong'}
      </AlertTitle>
      <AlertDescription>
        <p>
          {code === 'broadcast_not_ready' || code === 'invalid_broadcast'
            ? 'Fix the highlighted fields, then try again.'
            : code === 'broadcast_empty_audience'
              ? 'No opted-in contacts match this audience. Choose other tags, or ask customers to opt in.'
              : errorMessage(error)}
        </p>
        {code === 'plan_broadcast_limit' && (
          <Link to="/billing" className="font-bold text-brand-text underline underline-offset-2">
            See plans
          </Link>
        )}
        {code === 'number_not_connected' && (
          <Link
            to="/setup"
            search={{ step: 'whatsapp' }}
            className="font-bold text-brand-text underline underline-offset-2"
          >
            Connect WhatsApp
          </Link>
        )}
      </AlertDescription>
    </Alert>
  )
}
