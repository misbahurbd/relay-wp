import { useEffect, useState } from 'react'

/** The value, once it has stopped changing for `ms`. */
export function useDebouncedValue<T>(value: T, ms = 300) {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return debounced
}
