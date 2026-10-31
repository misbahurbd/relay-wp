import { FlaskConicalIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '#/components/ui/tooltip'

export const PREVIEW_NOTE = 'Sample data. This part of billing is coming soon.'

/** Marks a section that runs on sample data, so it never passes for the workspace's real numbers. */
export function PreviewBadge() {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="unstyled"
          type="button"
          className="inline-flex h-6 items-center gap-1 rounded-full bg-warning-soft px-2 text-xs font-bold text-warning focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          <FlaskConicalIcon className="size-3.5" aria-hidden="true" />
          Preview
          <span className="sr-only">: {PREVIEW_NOTE}</span>
        </Button>
      </TooltipTrigger>
      <TooltipContent>{PREVIEW_NOTE}</TooltipContent>
    </Tooltip>
  )
}
