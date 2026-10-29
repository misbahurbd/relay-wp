import { ClockIcon, InfoIcon, LockIcon, XCircleIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import type { Template } from '../../types'

/** Why the template is (or isn't) editable, from its status (AD-8). */
export function StatusNotice({ template, canEdit }: { template: Template; canEdit: boolean }) {
  switch (template.status) {
    case 'pending':
      return (
        <Alert variant="warning">
          <ClockIcon aria-hidden="true" />
          <AlertTitle className="font-bold">Meta is reviewing this template</AlertTitle>
          <AlertDescription>
            You can change it again once Meta approves or rejects it. Meta usually replies within 24
            hours.
          </AlertDescription>
        </Alert>
      )
    case 'rejected':
      return (
        <Alert variant="danger">
          <XCircleIcon aria-hidden="true" />
          <AlertTitle className="font-bold">Meta rejected this template</AlertTitle>
          <AlertDescription>
            {template.rejected_reason && <p>{template.rejected_reason}</p>}
            {canEdit && <p>Fix it below, then submit it again.</p>}
          </AlertDescription>
        </Alert>
      )
    case 'approved':
      return canEdit ? (
        <Alert variant="info">
          <InfoIcon aria-hidden="true" />
          <AlertTitle className="font-bold">This template is approved</AlertTitle>
          <AlertDescription>
            Saving changes keeps the approved version in use until you submit them and Meta approves
            them.
          </AlertDescription>
        </Alert>
      ) : null
    case 'paused':
    case 'disabled':
      return (
        <Alert variant="warning">
          <LockIcon aria-hidden="true" />
          <AlertTitle className="font-bold">
            Meta {template.status === 'paused' ? 'paused' : 'disabled'} this template
          </AlertTitle>
          <AlertDescription>
            {template.rejected_reason ?? 'Paused or disabled templates can’t be changed.'}
          </AlertDescription>
        </Alert>
      )
    default:
      return null
  }
}
