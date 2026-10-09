import type { LucideIcon } from 'lucide-react'
import { motion } from 'framer-motion'
import { cn } from '@/lib/utils'

export function StateCard({
  icon: Icon,
  title,
  children,
  action,
  tone = 'muted',
  className,
}: {
  icon: LucideIcon
  title: string
  children?: React.ReactNode
  action?: React.ReactNode
  tone?: 'muted' | 'error'
  className?: string
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      className={cn('mx-auto flex max-w-md flex-col items-center gap-3 rounded-2xl border border-dashed bg-card/60 px-6 py-12 text-center', className)}
      role={tone === 'error' ? 'alert' : undefined}
    >
      <span
        className={cn(
          'grid size-12 place-items-center rounded-full',
          tone === 'error' ? 'bg-destructive/10 text-destructive' : 'bg-primary/10 text-primary',
        )}
      >
        <Icon className="size-5" aria-hidden />
      </span>
      <h2 className="text-base font-semibold">{title}</h2>
      {children && <p className="text-sm text-muted-foreground">{children}</p>}
      {action && <div className="mt-2">{action}</div>}
    </motion.div>
  )
}
