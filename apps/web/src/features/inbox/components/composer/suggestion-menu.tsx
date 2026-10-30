import { cn } from '#/lib/utils'

export type Suggestion = { id: string; primary: string; secondary?: string }

/**
 * The "/" quick-reply and "@" mention lists above the composer. Focus stays in the text box;
 * arrows move the highlight, Enter picks it and Esc closes (handled by the composer).
 */
export function SuggestionMenu({
  id,
  label,
  heading,
  empty,
  items,
  active,
  onPick,
}: {
  id: string
  label: string
  heading: string
  empty: string
  items: Suggestion[]
  active: number
  onPick: (index: number) => void
}) {
  return (
    <div className="absolute bottom-full left-0 z-30 mb-2 flex max-h-72 w-full max-w-md flex-col overflow-y-auto rounded-lg border bg-popover p-1.5 shadow-card">
      <span className="px-2.5 py-1.5 text-[11px] font-extrabold tracking-wider text-muted-foreground uppercase">
        {heading}
      </span>
      <div id={id} role="listbox" aria-label={label}>
        {items.map((item, i) => (
          <div
            key={item.id}
            id={`${id}-${i}`}
            role="option"
            aria-selected={i === active}
            tabIndex={-1}
            // Keep focus in the text box: pick on mouse down, before it would blur.
            onMouseDown={(e) => {
              e.preventDefault()
              onPick(i)
            }}
            className={cn(
              'flex min-h-10 cursor-pointer flex-col justify-center rounded-md px-2.5 py-1.5 text-[13px]',
              i === active ? 'bg-muted' : 'hover:bg-muted',
            )}
          >
            <span className="font-bold">{item.primary}</span>
            {item.secondary && (
              <span className="line-clamp-1 text-xs text-muted-foreground">{item.secondary}</span>
            )}
          </div>
        ))}
      </div>
      {items.length === 0 && (
        <span className="px-2.5 py-2 text-[13px] text-muted-foreground">{empty}</span>
      )}
    </div>
  )
}
