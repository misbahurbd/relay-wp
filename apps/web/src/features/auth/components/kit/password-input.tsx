import { CheckIcon, CircleIcon, EyeIcon, EyeOffIcon } from 'lucide-react'
import { type ComponentProps, useState } from 'react'
import { Input } from '#/components/ui/input'
import { cn } from '#/lib/utils'
import { PASSWORD_RULES } from '../../schemas/auth.schema'

/** Password field with a show/hide toggle. Paste and password managers always work. */
export function PasswordInput({ className, ...props }: Omit<ComponentProps<'input'>, 'type'>) {
  const [shown, setShown] = useState(false)
  return (
    <div className="relative">
      <Input type={shown ? 'text' : 'password'} className={cn('pr-12', className)} {...props} />
      <button
        type="button"
        onClick={() => setShown((s) => !s)}
        aria-label={shown ? 'Hide password' : 'Show password'}
        aria-pressed={shown}
        className="absolute inset-y-0 right-0 flex w-11 items-center justify-center rounded-r-lg text-muted-foreground hover:text-foreground"
      >
        {shown ? <EyeOffIcon className="size-4.5" /> : <EyeIcon className="size-4.5" />}
      </button>
    </div>
  )
}

/** The password rules, ticked off live as the user types. Shown before they start. */
export function PasswordRules({ id, value }: { id: string; value: string }) {
  return (
    <ul id={id} className="grid grid-cols-2 gap-x-3 gap-y-1 text-sm">
      {PASSWORD_RULES.map((rule) => {
        const met = rule.test(value)
        return (
          <li
            key={rule.id}
            className={cn(
              'flex items-center gap-1.5',
              met ? 'text-brand-text' : 'text-muted-foreground',
            )}
          >
            {met ? (
              <CheckIcon className="size-3.5" aria-hidden="true" />
            ) : (
              <CircleIcon className="size-3" aria-hidden="true" />
            )}
            {rule.label}
            <span className="sr-only">{met ? '(done)' : '(not yet)'}</span>
          </li>
        )
      })}
    </ul>
  )
}
