import { Link } from '@tanstack/react-router'
import { AlertCircleIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { errorMessage, isApiError } from '#/lib/api'

const TITLE: Record<string, string> = {
  whatsapp_not_connected: 'Connect WhatsApp first',
  whatsapp_disconnected: 'WhatsApp is disconnected',
  meta_template_refused: 'Meta refused this template',
  whatsapp_unavailable: 'Meta isn’t responding',
  template_in_review: 'Meta is reviewing this template',
  template_not_editable: 'This template can’t be changed',
  invalid_template: 'Some fields need changes',
}

/**
 * An error from a templates action that needs the user to do something, kept on the page
 * (DESIGN-SYSTEM §4). A missing or broken WhatsApp connection links to the setup step.
 */
export function ActionError({ workspaceId, error }: { workspaceId: string; error: unknown }) {
  const code = isApiError(error) ? error.code : ''
  const whatsapp = code === 'whatsapp_not_connected' || code === 'whatsapp_disconnected'
  return (
    <Alert variant={code === 'whatsapp_unavailable' ? 'warning' : 'danger'}>
      <AlertCircleIcon aria-hidden="true" />
      <AlertTitle className="line-clamp-none font-bold">
        {TITLE[code] ?? 'Something went wrong'}
      </AlertTitle>
      <AlertDescription>
        <p>
          {code === 'invalid_template'
            ? 'Fix the highlighted fields, then try again.'
            : code === 'whatsapp_unavailable'
              ? 'Meta’s servers didn’t answer. Try again in a few minutes.'
              : errorMessage(error)}
        </p>
        {whatsapp && (
          <Link
            to="/w/$workspaceId/setup"
            params={{ workspaceId }}
            search={{ step: 'whatsapp' }}
            className="font-bold text-brand-text underline underline-offset-2"
          >
            {code === 'whatsapp_disconnected' ? 'Reconnect WhatsApp' : 'Connect WhatsApp'}
          </Link>
        )}
      </AlertDescription>
    </Alert>
  )
}
