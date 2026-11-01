import type * as React from "react"
import { cn } from "#/lib/utils"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./select"

export type SelectOption = { value: string; label: React.ReactNode; disabled?: boolean }

// Radix can't hold "" as an item value; an empty choice ("All", "None") travels as this.
const EMPTY = "__empty__"
const encode = (v: string) => (v === "" ? EMPTY : v)

/**
 * The project's select box: Select (Radix) driven by a list of options, for the cases a native
 * <select> used to cover. A value no option has shows the placeholder. Never use NativeSelect.
 */
function OptionSelect({
  value,
  onValueChange,
  options,
  placeholder,
  disabled,
  size,
  className,
  id,
  name,
  "aria-label": ariaLabel,
  "aria-invalid": ariaInvalid,
  "aria-describedby": ariaDescribedBy,
}: {
  value: string
  onValueChange: (value: string) => void
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  size?: "sm" | "default"
  className?: string
  id?: string
  name?: string
  "aria-label"?: string
  "aria-invalid"?: boolean
  "aria-describedby"?: string
}) {
  const known = options.some((o) => o.value === value)
  return (
    <Select
      value={known ? encode(value) : ""}
      onValueChange={(v) => onValueChange(v === EMPTY ? "" : v)}
      disabled={disabled}
      name={name}
    >
      <SelectTrigger
        id={id}
        size={size}
        aria-label={ariaLabel}
        aria-invalid={ariaInvalid}
        aria-describedby={ariaDescribedBy}
        className={cn("w-full font-normal", className)}
      >
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent position="popper">
        {options.map((o) => (
          <SelectItem key={o.value} value={encode(o.value)} disabled={o.disabled}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

export { OptionSelect }
