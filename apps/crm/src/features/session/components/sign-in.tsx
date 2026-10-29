import { type FormEvent, useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { errorMessage } from '#/lib/api'
import { useSignIn } from '../api/session.api'

/** Email and password only: staff sign in with a Relay account that has a password. */
export function SignIn() {
  const signIn = useSignIn()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const submit = (e: FormEvent) => {
    e.preventDefault()
    signIn.mutate({ email: email.trim(), password })
  }
  return (
    <main className="flex min-h-dvh items-center justify-center p-4">
      <form
        onSubmit={submit}
        className="flex w-full max-w-[380px] flex-col gap-4 rounded-xl border bg-card p-6 shadow-card"
      >
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-extrabold tracking-tight">Relay CRM</h1>
          <p className="text-sm text-muted-foreground">
            Staff only. Sign in with your Relay account.
          </p>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="password">Password</Label>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>
        {signIn.isError && (
          <Alert variant="danger">
            <AlertDescription>{errorMessage(signIn.error)}</AlertDescription>
          </Alert>
        )}
        <Button type="submit" disabled={signIn.isPending}>
          {signIn.isPending ? 'Signing in…' : 'Sign in'}
        </Button>
      </form>
    </main>
  )
}
