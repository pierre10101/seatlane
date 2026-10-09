import { cn } from '@/lib/utils'

// A deterministic, decorative backdrop per event: no images to load.
const PALETTES = [
  ['oklch(0.55 0.21 285)', 'oklch(0.70 0.16 330)', 'oklch(0.80 0.12 60)'],
  ['oklch(0.50 0.15 250)', 'oklch(0.65 0.14 200)', 'oklch(0.85 0.10 120)'],
  ['oklch(0.55 0.19 20)', 'oklch(0.72 0.16 55)', 'oklch(0.80 0.10 300)'],
  ['oklch(0.45 0.12 170)', 'oklch(0.68 0.13 140)', 'oklch(0.82 0.11 90)'],
]

export function EventArt({ id, className, children }: { id: number; className?: string; children?: React.ReactNode }) {
  const [a, b, c] = PALETTES[id % PALETTES.length]
  return (
    <div
      aria-hidden={children ? undefined : true}
      className={cn('relative overflow-hidden', className)}
      style={{
        backgroundImage: `radial-gradient(120% 90% at 0% 0%, ${a} 0%, transparent 60%), radial-gradient(90% 120% at 100% 100%, ${b} 0%, transparent 55%), radial-gradient(60% 60% at 70% 20%, ${c} 0%, transparent 60%), linear-gradient(135deg, ${a}, ${b})`,
      }}
    >
      <svg className="absolute inset-0 size-full opacity-[0.18] mix-blend-overlay" aria-hidden>
        <defs>
          <pattern id={`seats-${id}`} width="14" height="14" patternUnits="userSpaceOnUse">
            <rect x="2" y="3" width="10" height="8" rx="2.5" fill="white" />
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill={`url(#seats-${id})`} />
      </svg>
      <div className="absolute inset-0 bg-gradient-to-t from-black/45 via-black/5 to-transparent" />
      {children}
    </div>
  )
}
