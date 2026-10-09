import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { Check, Loader2, Lock, X } from 'lucide-react'
import type { Seat } from '@/lib/api'
import { formatMoney, formatRange, seatLabel } from '@/lib/format'
import { layout, seatState, STATE_TEXT, type SeatState } from '@/lib/seats'
import { cn } from '@/lib/utils'

export type Pending = 'hold' | 'release' | 'confirm'

type Props = {
  seats: Seat[]
  pending: ReadonlyMap<number, Pending>
  onActivate: (seat: Seat) => void
}

const SEAT_STYLE: Record<SeatState, string> = {
  available:
    'bg-seat-free border-seat-free-border hover:border-primary hover:bg-primary/15 hover:-translate-y-0.5 hover:shadow-md hover:shadow-primary/20 cursor-pointer',
  mine: 'bg-seat-mine border-seat-mine text-primary-foreground shadow-md shadow-primary/35 cursor-pointer hover:brightness-110',
  other: 'bg-seat-other/75 border-seat-other seat-stripes cursor-not-allowed',
  'sold-mine': 'bg-seat-sold-mine border-seat-sold-mine text-white cursor-default',
  'sold-other': 'bg-seat-sold border-transparent cursor-not-allowed opacity-80',
}

export function SeatMap({ seats, pending, onActivate }: Props) {
  const sections = useMemo(() => layout(seats), [seats])
  // Every row on the map, top to bottom, for arrow-key navigation.
  const rows = useMemo(() => sections.flatMap((s) => s.rows.map((r) => r.seats)), [sections])
  const position = useMemo(() => {
    const m = new Map<number, [number, number]>()
    rows.forEach((r, ri) => r.forEach((s, ci) => m.set(s.seat_id, [ri, ci])))
    return m
  }, [rows])

  const [focusId, setFocusId] = useState<number | null>(null)
  const [tipAt, setTip] = useState<{ id: number; x: number; y: number } | null>(null)
  const byId = useMemo(() => new Map(seats.map((s) => [s.seat_id, s])), [seats])
  // The tooltip always shows the seat as the server last reported it.
  const tipSeat = tipAt ? byId.get(tipAt.id) : undefined
  const tip = tipAt && tipSeat ? { seat: tipSeat, x: tipAt.x, y: tipAt.y } : null
  const canvas = useRef<HTMLDivElement>(null)
  const scroller = useRef<HTMLDivElement>(null)
  const [wide, setWide] = useState(false)
  // On narrow screens a wide hall scrolls sideways: start centred on the stage.
  useEffect(() => {
    const el = scroller.current
    if (!el) return
    const check = () => {
      const over = el.scrollWidth > el.clientWidth + 1
      setWide(over)
      if (over && el.scrollLeft === 0) el.scrollLeft = (el.scrollWidth - el.clientWidth) / 2
    }
    check()
    const ro = new ResizeObserver(check)
    ro.observe(el)
    return () => ro.disconnect()
  }, [sections.length])

  // Roving tabindex: one seat is in the tab order (the last focused one, or
  // the first available seat, or the first seat).
  const tabId = useMemo(() => {
    if (focusId !== null && position.has(focusId)) return focusId
    return (seats.find((s) => s.available) ?? rows[0]?.[0])?.seat_id ?? null
  }, [focusId, position, seats, rows])

  const showTip = useCallback((seat: Seat, el: HTMLElement) => {
    const box = canvas.current?.getBoundingClientRect()
    const r = el.getBoundingClientRect()
    if (!box) return
    setTip({ id: seat.seat_id, x: r.left - box.left + r.width / 2, y: r.top - box.top })
  }, [])

  const focusSeat = useCallback((id: number) => {
    setFocusId(id)
    canvas.current?.querySelector<HTMLButtonElement>(`[data-seat-id="${id}"]`)?.focus()
  }, [])

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      const id = Number((e.target as HTMLElement).dataset.seatId)
      const at = position.get(id)
      if (!at) return
      const [r, c] = at
      let next: Seat | undefined
      const across = (to: number) => {
        const from = rows[r]
        const target = rows[to]
        if (!target) return undefined
        const ratio = from.length > 1 ? c / (from.length - 1) : 0
        return target[Math.round(ratio * (target.length - 1))]
      }
      switch (e.key) {
        case 'ArrowRight': next = rows[r][c + 1]; break
        case 'ArrowLeft': next = rows[r][c - 1]; break
        case 'ArrowDown': next = across(r + 1); break
        case 'ArrowUp': next = across(r - 1); break
        case 'Home': next = e.ctrlKey ? rows[0][0] : rows[r][0]; break
        case 'End': next = e.ctrlKey ? rows[rows.length - 1].at(-1) : rows[r].at(-1); break
        default: return
      }
      e.preventDefault()
      if (next) focusSeat(next.seat_id)
    },
    [position, rows, focusSeat],
  )

  return (
    <div className="relative">
      {wide && (
        <p className="mb-2 text-center text-[11px] text-muted-foreground sm:hidden" aria-hidden>
          Swipe sideways to see every seat
        </p>
      )}
      <div ref={scroller} className="overflow-x-auto overscroll-x-contain pb-2 [scrollbar-width:thin]">
        <div ref={canvas} className="relative mx-auto flex w-max min-w-full flex-col items-center gap-7 px-1 pt-1 sm:px-2" onKeyDown={onKeyDown} onMouseLeave={() => setTip(null)}>
          <Stage />
          <div role="grid" aria-label="Seat map. Use the arrow keys to move between seats, Enter to hold or release." aria-rowcount={rows.length} className="flex flex-col items-center gap-7">
            {sections.map((sec) => {
              const prices = sec.rows.flatMap((r) => r.seats.map((s) => s.price))
              const lo = prices.reduce((a, b) => (b.cents < a.cents ? b : a))
              const hi = prices.reduce((a, b) => (b.cents > a.cents ? b : a))
              return (
                <div key={`${sec.rank}-${sec.name}`} role="rowgroup" aria-label={`${sec.name}, ${formatRange(lo, hi)}`} className="flex flex-col items-center gap-1.5 sm:gap-2">
                  <div className="mb-1 flex items-center gap-2 text-xs font-medium text-muted-foreground" aria-hidden>
                    <span className="h-px w-6 bg-border sm:w-10" />
                    <span className="tracking-wide uppercase">{sec.name}</span>
                    <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-foreground tabular">{formatRange(lo, hi)}</span>
                    <span className="h-px w-6 bg-border sm:w-10" />
                  </div>
                  {sec.rows.map((row) => (
                    <div key={row.label} role="row" className="flex items-center gap-1 sm:gap-2">
                      <RowLabel label={row.label} />
                      <div className="flex gap-[3px] sm:gap-1.5">
                        {row.seats.map((s) => (
                          <SeatButton
                            key={s.seat_id}
                            seat={s}
                            pending={pending.get(s.seat_id)}
                            tabbable={s.seat_id === tabId}
                            onActivate={onActivate}
                            onShow={showTip}
                            onHide={() => setTip(null)}
                            onFocusSeat={setFocusId}
                          />
                        ))}
                      </div>
                      <RowLabel label={row.label} />
                    </div>
                  ))}
                </div>
              )
            })}
          </div>
          <SeatTooltip tip={tip} pending={tip ? pending.get(tip.seat.seat_id) : undefined} />
        </div>
      </div>
    </div>
  )
}

function RowLabel({ label }: { label: string }) {
  return (
    <span aria-hidden className="w-3 text-center text-[10px] font-medium text-muted-foreground/80 tabular sm:w-5 sm:text-[11px]">
      {label}
    </span>
  )
}

function Stage() {
  return (
    <div className="relative w-full max-w-xl" aria-hidden>
      <div className="absolute inset-x-8 -bottom-6 h-10 rounded-full bg-stage/25 blur-2xl" />
      <div className="relative h-11 rounded-b-[50%_100%] rounded-t-md border border-stage/30 bg-gradient-to-b from-stage/35 via-stage/15 to-transparent">
        <span className="absolute inset-0 grid place-items-center text-[11px] font-semibold tracking-[0.35em] text-stage uppercase">Stage</span>
      </div>
    </div>
  )
}

type SeatButtonProps = {
  seat: Seat
  pending?: Pending
  tabbable: boolean
  onActivate: (s: Seat) => void
  onShow: (s: Seat, el: HTMLElement) => void
  onHide: () => void
  onFocusSeat: (id: number) => void
}

const SeatButton = memo(function SeatButton({ seat, pending, tabbable, onActivate, onShow, onHide, onFocusSeat }: SeatButtonProps) {
  const state = seatState(seat)
  const actionable = state === 'available' || state === 'mine'
  const label = `${seat.section}, row ${seat.row}, seat ${seat.number}, ${formatMoney(seat.price)}, ${pending ? pendingText(pending) : STATE_TEXT[state].toLowerCase()}`
  return (
    <span role="gridcell" className="contents">
      <motion.button
        type="button"
        data-seat-id={seat.seat_id}
        data-state={state}
        data-pending={pending ?? undefined}
        aria-label={label}
        aria-pressed={state === 'mine' ? true : state === 'available' ? false : undefined}
        aria-disabled={!actionable || !!pending}
        aria-busy={!!pending}
        tabIndex={tabbable ? 0 : -1}
        whileTap={actionable && !pending ? { scale: 0.85 } : undefined}
        onClick={() => actionable && !pending && onActivate(seat)}
        onMouseEnter={(e) => onShow(seat, e.currentTarget)}
        onFocus={(e) => {
          onFocusSeat(seat.seat_id)
          onShow(seat, e.currentTarget)
        }}
        onBlur={onHide}
        className={cn(
          'relative grid size-5 place-items-center rounded-[6px] rounded-b-[3px] border-[1.5px] transition-all duration-150 outline-none focus-visible:ring-3 focus-visible:ring-ring/60 focus-visible:ring-offset-2 focus-visible:ring-offset-background sm:size-7 sm:rounded-[9px] sm:rounded-b-[5px]',
          SEAT_STYLE[state],
          pending === 'hold' && 'border-primary bg-primary/40 text-primary-foreground',
          pending === 'release' && 'opacity-50',
          pending === 'confirm' && 'animate-pulse',
        )}
      >
        <SeatGlyph state={state} pending={pending} />
      </motion.button>
    </span>
  )
})

function pendingText(p: Pending) {
  return p === 'hold' ? 'holding…' : p === 'release' ? 'releasing…' : 'confirming…'
}

function SeatGlyph({ state, pending }: { state: SeatState; pending?: Pending }) {
  if (pending === 'hold' || pending === 'release') return <Loader2 className="size-3 animate-spin sm:size-3.5" aria-hidden />
  switch (state) {
    case 'mine':
      return <motion.span initial={{ scale: 0 }} animate={{ scale: 1 }} className="size-1.5 rounded-full bg-current sm:size-2" aria-hidden />
    case 'sold-mine':
      return <Check className="size-3 sm:size-3.5" strokeWidth={3} aria-hidden />
    case 'sold-other':
      return <X className="size-2.5 text-muted-foreground/70 sm:size-3" aria-hidden />
    case 'other':
      return <Lock className="size-2.5 text-black/50 sm:size-3" aria-hidden />
    default:
      return null
  }
}

function SeatTooltip({ tip, pending }: { tip: { seat: Seat; x: number; y: number } | null; pending?: Pending }) {
  return (
    <AnimatePresence>
      {tip && (
        <motion.div
          key="tip"
          role="tooltip"
          initial={{ opacity: 0, y: 4, scale: 0.96 }}
          animate={{ opacity: 1, y: 0, scale: 1, left: tip.x, top: tip.y }}
          exit={{ opacity: 0, y: 4, scale: 0.96 }}
          transition={{ duration: 0.12, left: { duration: 0.08 }, top: { duration: 0.08 } }}
          style={{ left: tip.x, top: tip.y }}
          className="pointer-events-none absolute z-30 -translate-x-1/2 -translate-y-[calc(100%+10px)] rounded-xl border bg-popover px-3 py-2 text-popover-foreground shadow-xl"
          data-testid="seat-tooltip"
        >
          <div className="flex items-baseline gap-2 whitespace-nowrap">
            <span className="text-sm font-semibold">{seatLabel(tip.seat)}</span>
            <span className="text-xs text-muted-foreground">{tip.seat.section}</span>
            <span className="ml-auto pl-3 text-sm font-semibold tabular">{formatMoney(tip.seat.price)}</span>
          </div>
          <div className="mt-0.5 flex items-center gap-1.5 text-xs text-muted-foreground">
            <LegendSwatch state={seatState(tip.seat)} small />
            {pending ? pendingText(pending) : STATE_TEXT[seatState(tip.seat)]}
            {seatState(tip.seat) === 'available' && !pending && <span className="text-primary">· click to hold</span>}
            {seatState(tip.seat) === 'mine' && !pending && <span className="text-primary">· click to release</span>}
          </div>
          <span className="absolute -bottom-[5px] left-1/2 size-2.5 -translate-x-1/2 rotate-45 border-r border-b bg-popover" />
        </motion.div>
      )}
    </AnimatePresence>
  )
}

export function LegendSwatch({ state, small }: { state: SeatState; small?: boolean }) {
  return <span aria-hidden className={cn('inline-block shrink-0 rounded-[4px] border-[1.5px]', small ? 'size-2.5' : 'size-3.5', SEAT_STYLE[state], 'hover:translate-y-0 hover:shadow-none cursor-default')} />
}

export function Legend({ seats }: { seats: Seat[] }) {
  const counts = useMemo(() => {
    const c: Record<SeatState, number> = { available: 0, mine: 0, other: 0, 'sold-mine': 0, 'sold-other': 0 }
    for (const s of seats) c[seatState(s)]++
    return c
  }, [seats])
  const order: SeatState[] = ['available', 'mine', 'other', 'sold-mine', 'sold-other']
  return (
    <ul className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-muted-foreground" aria-label="Legend">
      {order.map((s) => (
        <li key={s} className="flex items-center gap-1.5">
          <LegendSwatch state={s} />
          <span>{STATE_TEXT[s]}</span>
          <span className="text-foreground/70 tabular">{counts[s]}</span>
        </li>
      ))}
    </ul>
  )
}
