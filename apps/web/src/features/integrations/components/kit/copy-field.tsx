import { CheckIcon, CopyIcon, EyeIcon, EyeOffIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { cn } from '#/lib/utils'

/** A secret in monospace with Copy, and Show/Hide when it starts hidden. */
export function CopyField({
  value,
  label,
  hideable,
  className,
}: {
  value: string
  label: string
  hideable?: boolean
  className?: string
}) {
  const [shown, setShown] = useState(!hideable)
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error("Couldn't copy. Select the text and copy it yourself.")
    }
  }

  return (
    <div
      className={cn('flex items-center gap-2 rounded-[10px] border bg-muted p-1.5 pl-3', className)}
    >
      <span className="sr-only">{label}:</span>
      <code className="min-w-0 flex-1 truncate font-mono text-[13px] text-foreground select-all">
        {shown ? value : `${value.slice(0, 10)}${'•'.repeat(18)}${value.slice(-4)}`}
      </code>
      {hideable && (
        <button
          type="button"
          onClick={() => setShown((s) => !s)}
          aria-label={shown ? `Hide ${label}` : `Show ${label}`}
          className="flex size-8 flex-none items-center justify-center rounded-lg text-muted-foreground hover:bg-card hover:text-foreground"
        >
          {shown ? (
            <EyeOffIcon className="size-4" aria-hidden="true" />
          ) : (
            <EyeIcon className="size-4" aria-hidden="true" />
          )}
        </button>
      )}
      <button
        type="button"
        onClick={copy}
        className="inline-flex h-8 flex-none items-center gap-1.5 rounded-lg bg-card px-2.5 text-[13px] font-bold shadow-xs hover:bg-background"
      >
        {copied ? (
          <CheckIcon className="size-3.5 text-brand-text" aria-hidden="true" />
        ) : (
          <CopyIcon className="size-3.5" aria-hidden="true" />
        )}
        {copied ? 'Copied' : 'Copy'}
      </button>
    </div>
  )
}
