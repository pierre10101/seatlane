import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Armchair, LogIn, LogOut, Moon, Sun } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { errorCopy } from '@/lib/errors'
import { useAuth } from '@/hooks/use-auth'
import { AnimatePresence, motion } from 'framer-motion'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useTheme } from '@/hooks/use-theme'

export function SiteHeader() {
  const { theme, toggle } = useTheme()
  const next = theme === 'dark' ? 'light' : 'dark'
  const { account, ready, setAccount } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()
  const here = location.pathname.startsWith('/sign-') ? '' : `?next=${encodeURIComponent(location.pathname)}`
  async function signOut() {
    try {
      await api.signOut()
      setAccount(null)
      toast('Signed out')
      navigate('/')
    } catch (e) {
      const c = errorCopy(e)
      toast.error(c.title, { description: c.description })
    }
  }
  return (
    <header className="sticky top-0 z-40 border-b border-border/60 bg-background/70 backdrop-blur-xl supports-[backdrop-filter]:bg-background/60">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6">
        <Link to="/" className="group flex items-center gap-2 rounded-lg outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <span className="grid size-8 place-items-center rounded-lg bg-primary text-primary-foreground shadow-sm shadow-primary/30 transition-transform group-hover:-rotate-6">
            <Armchair className="size-4.5" aria-hidden />
          </span>
          <span className="text-[15px] font-semibold tracking-tight">Seatlane</span>
        </Link>
        <div className="flex items-center gap-1.5">
        {ready && account && (
          <>
            <span className="hidden max-w-48 truncate text-sm text-muted-foreground sm:inline" data-testid="signed-in-as">
              {account.email}{account.role !== 'customer' && ` · ${account.role}`}
            </span>
            <Button variant="ghost" size="sm" onClick={signOut} data-testid="sign-out"><LogOut /> Sign out</Button>
          </>
        )}
        {ready && !account && (
          <>
            <Button asChild variant="ghost" size="sm"><Link to={`/sign-in${here}`} data-testid="sign-in"><LogIn /> Sign in</Link></Button>
            <Button asChild variant="outline" size="sm"><Link to={`/sign-up${here}`} data-testid="sign-up">Sign up</Link></Button>
          </>
        )}
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" onClick={toggle} aria-label={`Switch to ${next} mode`} data-testid="theme-toggle">
              <AnimatePresence mode="wait" initial={false}>
                <motion.span
                  key={theme}
                  initial={{ rotate: -90, opacity: 0, scale: 0.6 }}
                  animate={{ rotate: 0, opacity: 1, scale: 1 }}
                  exit={{ rotate: 90, opacity: 0, scale: 0.6 }}
                  transition={{ duration: 0.18 }}
                  className="grid place-items-center"
                >
                  {theme === 'dark' ? <Moon className="size-4" /> : <Sun className="size-4" />}
                </motion.span>
              </AnimatePresence>
            </Button>
          </TooltipTrigger>
          <TooltipContent>Switch to {next} mode</TooltipContent>
        </Tooltip>
        </div>
      </div>
    </header>
  )
}
