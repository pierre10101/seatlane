import { motion } from 'framer-motion'
import { clock } from '@/lib/format'
import { cn } from '@/lib/utils'

/** A ring that empties as the hold runs out. `remaining` and `total` come
 * from the server's answer (expires_at - now, expires_at - held_at). */
export function CountdownRing({ remaining, total, expired, size = 52 }: { remaining: number; total: number; expired: boolean; size?: number }) {
  const stroke = 4
  const r = (size - stroke) / 2
  const c = 2 * Math.PI * r
  const frac = expired || total <= 0 ? 0 : Math.min(1, Math.max(0, remaining / total))
  const tone = expired ? 'text-muted-foreground' : remaining <= 60 ? 'text-destructive' : remaining <= 180 ? 'text-amber-500' : 'text-primary'
  return (
    <div className={cn('relative grid shrink-0 place-items-center', tone)} style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90" aria-hidden>
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="currentColor" strokeOpacity={0.15} strokeWidth={stroke} />
        <motion.circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="currentColor"
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={c}
          animate={{ strokeDashoffset: c * (1 - frac) }}
          transition={{ duration: 0.3, ease: 'linear' }}
        />
      </svg>
      <span className={cn('absolute text-[11px] font-semibold tabular', remaining <= 60 && !expired && 'animate-pulse')}>
        {expired ? '0:00' : clock(remaining).replace(/^0/, '')}
      </span>
    </div>
  )
}
