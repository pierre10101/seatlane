import { AnimatePresence, motion } from 'framer-motion'
import { AlertTriangle, Armchair, Check, CircleCheck, CircleX, ListChecks, Loader2, RotateCcw, Ticket, X } from 'lucide-react'
import type { Seat } from '@/lib/api'
import { clock, formatMoney, seatLabel } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { CountdownRing } from '@/components/countdown-ring'
import type { Pending } from '@/components/seat-map'
import { copyFor } from '@/lib/errors'
import { cn } from '@/lib/utils'

export type HoldRow = {
  seatId: number
  seat?: Seat
  /** The server's flag: false once expires_at is no later than its now. */
  active: boolean
  /** Seconds left: (expires_at - now) as the server reported, minus the time since. */
  remaining: number
  /** expires_at - held_at, as the server reported. */
  total: number
}

/** What one confirm_hold call answered during "Confirm each seat". */
export type ConfirmResult = {
  seatId: number
  seat?: Seat
  ok: boolean
  /** The server's error.id when ok is false (F2, F3, ...). */
  errorId?: string
}

type Props = {
  holds: HoldRow[]
  tickets: Seat[]
  pending: ReadonlyMap<number, Pending>
  busy: boolean
  onConfirm: (seatId: number) => void
  onConfirmEach: () => void
  onRelease: (seatId: number) => void
  onHoldAgain: (seatId: number) => void
  /** The per-seat outcome of the last "Confirm each seat", if any. */
  results: ConfirmResult[] | null
  onDismissResults: () => void
}

export function CheckoutPanel({ holds, tickets, pending, busy, onConfirm, onConfirmEach, onRelease, onHoldAgain, results, onDismissResults }: Props) {
  const live = holds.filter((h) => h.active)
  const total = live.reduce((sum, h) => sum + (h.seat?.price.cents ?? 0), 0)
  const currency = live[0]?.seat?.price.currency ?? 'ZAR'
  return (
    <section id="checkout" aria-labelledby="checkout-title" className="overflow-hidden rounded-2xl border bg-card shadow-sm" data-testid="checkout">
      <header className="flex items-center justify-between border-b px-5 py-4">
        <h2 id="checkout-title" className="flex items-center gap-2 font-semibold tracking-tight">
          <Armchair className="size-4 text-primary" aria-hidden /> Your seats
        </h2>
        <Badge variant="secondary" className="tabular">{holds.length} held</Badge>
      </header>

      <div className="px-3 py-3">
        {holds.length === 0 ? (
          <div className="flex flex-col items-center gap-2 px-4 py-8 text-center">
            <span className="grid size-11 place-items-center rounded-full bg-primary/10 text-primary">
              <Armchair className="size-5" aria-hidden />
            </span>
            <p className="text-sm font-medium">No seats held yet</p>
            <p className="max-w-[16rem] text-xs text-muted-foreground">Pick any open seat on the map. It is yours for 10 minutes while you decide.</p>
          </div>
        ) : (
          <ul className="flex flex-col gap-2" aria-label="Seats you are holding">
            <AnimatePresence initial={false}>
              {holds.map((h) => (
                <motion.li
                  key={h.seatId}
                  layout
                  initial={{ opacity: 0, height: 0, scale: 0.98 }}
                  animate={{ opacity: 1, height: 'auto', scale: 1 }}
                  exit={{ opacity: 0, height: 0, scale: 0.98 }}
                  transition={{ duration: 0.22 }}
                >
                  <HoldItem h={h} pending={pending.get(h.seatId)} onConfirm={onConfirm} onRelease={onRelease} onHoldAgain={onHoldAgain} />
                </motion.li>
              ))}
            </AnimatePresence>
          </ul>
        )}
      </div>

      {results && results.length > 0 && <ConfirmResults results={results} pending={pending} onHoldAgain={onHoldAgain} onDismiss={onDismissResults} />}

      {live.length > 0 && (
        <footer className="space-y-3 border-t bg-muted/30 px-5 py-4">
          <div className="flex items-baseline justify-between">
            <span className="text-sm text-muted-foreground">{live.length} {live.length === 1 ? 'seat' : 'seats'} on hold</span>
            <span className="text-lg font-semibold tabular">{formatMoney({ cents: total, currency })}</span>
          </div>
          <Button size="lg" className="h-11 w-full text-sm shadow-lg shadow-primary/25" disabled={busy} onClick={onConfirmEach} data-testid="confirm-each">
            {busy ? <Loader2 className="animate-spin" /> : <ListChecks />} {holds.length === 1 ? 'Confirm seat' : `Confirm ${holds.length} seats one by one`}
          </Button>
          {holds.length > 1 && (
            <p className="text-center text-[11px] text-muted-foreground" data-testid="confirm-each-note">
              Each seat is booked on its own. If a hold runs out first, that seat is not booked; the others still are.
            </p>
          )}
        </footer>
      )}

      {tickets.length > 0 && (
        <div className="border-t px-5 py-4">
          <h3 className="mb-2 flex items-center gap-2 text-sm font-semibold"><Ticket className="size-4 text-seat-sold-mine" aria-hidden /> Your tickets</h3>
          <ul className="flex flex-wrap gap-1.5" data-testid="tickets">
            {tickets.map((s) => (
              <li key={s.seat_id} className="inline-flex items-center gap-1 rounded-full bg-seat-sold-mine/15 px-2.5 py-1 text-xs font-medium text-foreground">
                <Check className="size-3 text-seat-sold-mine" strokeWidth={3} aria-hidden /> {s.section} {seatLabel(s)}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  )
}

function HoldItem({ h, pending, onConfirm, onRelease, onHoldAgain }: { h: HoldRow; pending?: Pending; onConfirm: (id: number) => void; onRelease: (id: number) => void; onHoldAgain: (id: number) => void }) {
  const checking = h.active && h.remaining <= 0
  const label = h.seat ? seatLabel(h.seat) : `Seat ${h.seatId}`
  return (
    <div
      className={cn('flex items-center gap-3 rounded-xl border bg-background/60 p-2.5 transition-colors', !h.active && 'border-dashed opacity-75')}
      data-testid="hold-item"
      data-active={h.active}
    >
      <CountdownRing remaining={h.remaining} total={h.total} expired={!h.active} />
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-1.5">
          <span className="font-semibold">{label}</span>
          {h.seat && <span className="truncate text-xs text-muted-foreground">{h.seat.section}</span>}
        </div>
        <div className="text-xs text-muted-foreground tabular" aria-live="off">
          {!h.active ? (
            <span className="font-medium text-destructive">Hold expired</span>
          ) : checking ? (
            <span className="inline-flex items-center gap-1"><Loader2 className="size-3 animate-spin" aria-hidden /> Checking with the box office…</span>
          ) : (
            <>
              {h.seat && <span className="font-medium text-foreground">{formatMoney(h.seat.price)}</span>}
              <span> · {clock(h.remaining)} left</span>
            </>
          )}
        </div>
      </div>
      <div className="flex items-center gap-1">
        {h.active ? (
          <Button size="sm" onClick={() => onConfirm(h.seatId)} disabled={!!pending} aria-label={`Confirm ${label}`} data-testid="confirm">
            {pending === 'confirm' ? <Loader2 className="animate-spin" /> : <Check />} Confirm
          </Button>
        ) : (
          <Button size="sm" variant="outline" onClick={() => onHoldAgain(h.seatId)} disabled={!!pending} aria-label={`Hold ${label} again`}>
            {pending === 'hold' ? <Loader2 className="animate-spin" /> : <RotateCcw />} Hold again
          </Button>
        )}
        <Button size="icon-sm" variant="ghost" onClick={() => onRelease(h.seatId)} disabled={!!pending} aria-label={`Release ${label}`} data-testid="release">
          {pending === 'release' ? <Loader2 className="animate-spin" /> : <X />}
        </Button>
      </div>
    </div>
  )
}

/** "2 of 3 seats confirmed; A17's hold had expired" */
function confirmSummary(results: ConfirmResult[]): string {
  const n = results.length
  const failed = results.filter((r) => !r.ok)
  const done = n - failed.length
  if (failed.length === 0) return n === 1 ? 'Your seat is confirmed' : `All ${n} seats confirmed`
  const head = `${done} of ${n} ${n === 1 ? 'seat' : 'seats'} confirmed`
  const why = failed.map((r) => {
    const label = r.seat ? seatLabel(r.seat) : `Seat ${r.seatId}`
    if (r.errorId === 'F2') return `${label}'s hold had expired`
    return `${label} was not booked (${copyFor(r.errorId ?? 'internal').title.toLowerCase()})`
  })
  return `${head}; ${why.join('; ')}`
}

/** F2 (hold ran out) and F3 (no hold) can be fixed by holding the seat again. */
const holdAgainFixes = new Set(['F2', 'F3'])

function ConfirmResults({ results, pending, onHoldAgain, onDismiss }: { results: ConfirmResult[]; pending: ReadonlyMap<number, Pending>; onHoldAgain: (id: number) => void; onDismiss: () => void }) {
  const failed = results.filter((r) => !r.ok).length
  const tone = failed === 0 ? 'ok' : failed === results.length ? 'error' : 'partial'
  return (
    <div
      className={cn(
        'mx-3 mb-3 rounded-xl border p-3',
        tone === 'ok' && 'border-emerald-500/40 bg-emerald-500/10',
        tone === 'partial' && 'border-amber-500/50 bg-amber-500/10',
        tone === 'error' && 'border-destructive/40 bg-destructive/10',
      )}
      role="status"
      data-testid="confirm-results"
      data-outcome={tone}
    >
      <div className="flex items-start gap-2">
        {tone === 'ok' ? (
          <CircleCheck className="mt-0.5 size-4 shrink-0 text-emerald-600 dark:text-emerald-400" aria-hidden />
        ) : (
          <AlertTriangle className={cn('mt-0.5 size-4 shrink-0', tone === 'partial' ? 'text-amber-600 dark:text-amber-400' : 'text-destructive')} aria-hidden />
        )}
        <p className="flex-1 text-sm font-semibold" data-testid="confirm-summary">{confirmSummary(results)}</p>
        <Button size="icon-sm" variant="ghost" className="-mt-1 -mr-1" onClick={onDismiss} aria-label="Dismiss these results" data-testid="confirm-results-dismiss">
          <X />
        </Button>
      </div>
      <ul className="mt-2 flex flex-col gap-1.5" aria-label="Result for each seat">
        {results.map((r) => {
          const label = r.seat ? `${r.seat.section} ${seatLabel(r.seat)}` : `Seat ${r.seatId}`
          const copy = r.ok ? null : copyFor(r.errorId ?? 'internal')
          const p = pending.get(r.seatId)
          return (
            <li key={r.seatId} className="flex items-start gap-2 rounded-lg bg-background/70 px-2.5 py-2 text-xs" data-testid="confirm-result" data-ok={r.ok} data-error-id={r.errorId ?? ''}>
              {r.ok ? <CircleCheck className="mt-px size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" aria-hidden /> : <CircleX className="mt-px size-3.5 shrink-0 text-destructive" aria-hidden />}
              <div className="min-w-0 flex-1">
                <div className="font-medium">
                  {label} <span className={r.ok ? 'text-emerald-700 dark:text-emerald-300' : 'text-destructive'}>{r.ok ? 'confirmed' : `not booked: ${copy!.title}`}</span>
                </div>
                {copy && <div className="mt-0.5 text-muted-foreground">{copy.description}</div>}
              </div>
              {!r.ok && holdAgainFixes.has(r.errorId ?? '') && (
                <Button size="xs" variant="outline" className="shrink-0" onClick={() => onHoldAgain(r.seatId)} disabled={!!p} aria-label={`Hold ${label} again`} data-testid="result-hold-again">
                  {p === 'hold' ? <Loader2 className="animate-spin" /> : <RotateCcw />} Hold again
                </Button>
              )}
            </li>
          )
        })}
      </ul>
    </div>
  )
}
