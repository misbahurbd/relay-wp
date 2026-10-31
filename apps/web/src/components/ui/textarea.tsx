import * as React from "react"
import { cn } from "#/lib/utils"

function Textarea({
  className,
  unstyled = false,
  ...props
}: React.ComponentProps<"textarea"> & { unstyled?: boolean }) {
  return (
    <textarea
      data-slot="textarea"
      className={unstyled ? className : cn(
        "flex field-sizing-content min-h-24 w-full rounded-lg border border-input bg-card px-3.5 py-2.5 text-base shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring focus-visible:outline-solid disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 md:text-sm dark:aria-invalid:ring-destructive/40",
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
