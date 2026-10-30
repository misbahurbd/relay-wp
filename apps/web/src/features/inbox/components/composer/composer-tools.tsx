import { FileTextIcon, ImageIcon, MusicIcon, PaperclipIcon, SmileIcon } from 'lucide-react'
import { useRef, useState } from 'react'
import { Button } from '#/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '#/components/ui/dropdown-menu'
import { Popover, PopoverContent, PopoverTrigger } from '#/components/ui/popover'
import { type AttachOption, EMOJI } from '../../schemas/inbox.schema'

export function EmojiPicker({
  disabled,
  onPick,
}: {
  disabled?: boolean
  onPick: (emoji: string) => void
}) {
  const [open, setOpen] = useState(false)
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon-sm" disabled={disabled} aria-label="Emoji">
          <SmileIcon aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" side="top" className="grid w-auto grid-cols-6 gap-0.5 p-1.5">
        {EMOJI.map((e) => (
          <button
            key={e}
            type="button"
            aria-label={`Insert ${e}`}
            onClick={() => {
              onPick(e)
              setOpen(false)
            }}
            className="flex size-10 items-center justify-center rounded-md text-xl hover:bg-muted"
          >
            {e}
          </button>
        ))}
      </PopoverContent>
    </Popover>
  )
}

const OPTION_ICON = { Audio: MusicIcon, Document: FileTextIcon } as const

/** "Photo or video", "Audio" and "Document", as the channel allows: each opens its own file picker. */
export function AttachMenu({
  options,
  disabled,
  onFile,
}: {
  options: AttachOption[]
  disabled?: boolean
  onFile: (file: File, option: AttachOption) => void
}) {
  const inputs = useRef<(HTMLInputElement | null)[]>([])
  if (options.length === 0) return null
  const pick = (option: AttachOption) => (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (file) onFile(file, option)
  }
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon-sm" disabled={disabled} aria-label="Attach a file">
            <PaperclipIcon aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" side="top" aria-label="Attach">
          {options.map((o, i) => {
            const Icon = OPTION_ICON[o.label as keyof typeof OPTION_ICON] ?? ImageIcon
            return (
              <DropdownMenuItem
                key={o.label}
                className="min-h-10"
                onSelect={() => inputs.current[i]?.click()}
              >
                <Icon aria-hidden="true" />
                {o.label}
              </DropdownMenuItem>
            )
          })}
        </DropdownMenuContent>
      </DropdownMenu>
      {options.map((o, i) => (
        <input
          key={o.label}
          ref={(el) => {
            inputs.current[i] = el
          }}
          type="file"
          accept={o.accept}
          hidden
          onChange={pick(o)}
        />
      ))}
    </>
  )
}
