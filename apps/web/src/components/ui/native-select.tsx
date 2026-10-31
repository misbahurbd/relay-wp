import * as React from "react"
import { cn } from "#/lib/utils"

function NativeSelect({
  className,
  unstyled = false,
  ...props
}: React.ComponentProps<"select"> & { unstyled?: boolean }) {
  return (
    <select
      data-slot="native-select"
      className={
        unstyled
          ? className
          : cn(
              "h-11.5 w-full min-w-0 rounded-lg border border-input bg-card px-3.5 py-1 text-base shadow-xs transition-[color,box-shadow] outline-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 focus-visible:border-ring focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring focus-visible:outline-solid md:text-sm dark:aria-invalid:ring-destructive/40",
              className
            )
      }
      {...props}
    />
  )
}

export { NativeSelect }
