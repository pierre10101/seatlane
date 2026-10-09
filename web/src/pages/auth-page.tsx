import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { LogIn, UserPlus } from 'lucide-react'
import { api } from '@/lib/api'
import { errorCopy } from '@/lib/errors'
import { useAuth } from '@/hooks/use-auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'

// Sign in and sign up: two fields and the server's answer. Every rule
// (email shape, password length, duplicate email, wrong password, rate
// limit) is the server's; this page shows the copy for its error.id.
export function AuthPage({ mode }: { mode: 'sign-in' | 'sign-up' }) {
  const { setAccount } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const next = params.get('next')?.startsWith('/') ? params.get('next')! : '/'
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const signIn = mode === 'sign-in'

  useEffect(() => {
    document.title = `${signIn ? 'Sign in' : 'Sign up'} · Seatlane`
  }, [signIn])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const account = signIn ? await api.signIn(email, password) : await api.signUp(email, password)
      setAccount(account)
      navigate(next, { replace: true })
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  const copy = error ? errorCopy(error) : null
  const field = 'h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50'
  return (
    <main className="mx-auto max-w-sm px-4 py-12">
      <Card>
        <CardHeader>
          <CardTitle>{signIn ? 'Sign in' : 'Create an account'}</CardTitle>
          <CardDescription>{signIn ? 'Sign in to hold and book seats.' : 'A customer account holds and books seats.'}</CardDescription>
        </CardHeader>
        <form onSubmit={submit} noValidate>
          <CardContent className="flex flex-col gap-3">
            <label className="flex flex-col gap-1.5 text-sm font-medium">
              Email
              <input className={field} type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} required data-testid="email" />
            </label>
            <label className="flex flex-col gap-1.5 text-sm font-medium">
              Password
              <input className={field} type="password" autoComplete={signIn ? 'current-password' : 'new-password'} value={password} onChange={(e) => setPassword(e.target.value)} required data-testid="password" />
            </label>
            {copy && (
              <div role="alert" className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm" data-testid="auth-error">
                <div className="font-medium text-destructive">{copy.title}</div>
                <div className="text-muted-foreground">{copy.description}</div>
              </div>
            )}
          </CardContent>
          <CardFooter className="mt-4 flex flex-col items-stretch gap-3">
            <Button type="submit" disabled={busy} data-testid="submit">
              {signIn ? <LogIn /> : <UserPlus />} {signIn ? 'Sign in' : 'Sign up'}
            </Button>
            <p className="text-center text-sm text-muted-foreground">
              {signIn ? 'New here? ' : 'Already have an account? '}
              <Link className="font-medium text-foreground underline underline-offset-4" to={`/${signIn ? 'sign-up' : 'sign-in'}${next !== '/' ? `?next=${encodeURIComponent(next)}` : ''}`}>
                {signIn ? 'Create an account' : 'Sign in'}
              </Link>
            </p>
          </CardFooter>
        </form>
      </Card>
    </main>
  )
}
