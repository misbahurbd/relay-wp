import { useNavigate } from '@tanstack/react-router'
import { LogOutIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { useLogout } from '../api/auth'

/** For pages outside the app shell (onboarding): signs out and goes to the login page. */
export function SignOutButton() {
  const navigate = useNavigate()
  const logout = useLogout()
  return (
    <Button
      variant="ghost"
      size="sm"
      disabled={logout.isPending}
      onClick={() => logout.mutate({}, { onSettled: () => navigate({ to: '/login' }) })}
    >
      <LogOutIcon aria-hidden="true" />
      Log out
    </Button>
  )
}
