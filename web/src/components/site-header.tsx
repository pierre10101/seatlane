import { Link } from 'react-router-dom'
import { Armchair, Moon, Sun } from 'lucide-react'
import { AnimatePresence, motion } from 'framer-motion'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useTheme } from '@/hooks/use-theme'

export function SiteHeader() {
  const { theme, toggle } = useTheme()
  const next = theme === 'dark' ? 'light' : 'dark'
  return (
    <header className="sticky top-0 z-40 border-b border-border/60 bg-background/70 backdrop-blur-xl supports-[backdrop-filter]:bg-background/60">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6">
        <Link to="/" className="group flex items-center gap-2 rounded-lg outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <span className="grid size-8 place-items-center rounded-lg bg-primary text-primary-foreground shadow-sm shadow-primary/30 transition-transform group-hover:-rotate-6">
            <Armchair className="size-4.5" aria-hidden />
          </span>
          <span className="text-[15px] font-semibold tracking-tight">Seatlane</span>
        </Link>
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
    </header>
  )
}
