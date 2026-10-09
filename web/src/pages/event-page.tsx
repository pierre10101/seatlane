import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { motion } from 'framer-motion'
import { toast } from 'sonner'
import { ArrowLeft, CalendarDays, Clock3, MapPin, RefreshCw, SearchX, TicketX, WifiOff } from 'lucide-react'
import { api, type MyHold, type Seat, type SeatMap as SeatMapData } from '@/lib/api'
import { errorCopy, errorId, groupCopyFor } from '@/lib/errors'
import { clock, formatDate, formatRange, formatTime, seatLabel } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EventArt } from '@/components/event-art'
import { Legend, SeatMap, type Pending } from '@/components/seat-map'
import { CheckoutPanel, type HoldRow } from '@/components/checkout-panel'
import { SuccessState } from '@/components/success-state'
import { StateCard } from '@/components/states'
import { useTicker } from '@/hooks/use-ticker'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'

const POLL_MS = 4000

/** One of my holds as the server last reported it, with when that answer
 * arrived (performance.now()). The countdown is (expires_at - now) from the
 * server minus the time elapsed since; the browser's clock is never read. */
type HoldView = MyHold & { serverNow: number; receivedAt: number }

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ready' }

export function EventPage() {
  const eventId = Number(useParams().id)
  const [load, setLoad] = useState<Load>({ status: 'loading' })
  const [map, setMap] = useState<SeatMapData | null>(null)
  const [holds, setHolds] = useState<HoldView[]>([])
  const [pending, setPending] = useState<ReadonlyMap<number, Pending>>(new Map())
  const [stale, setStale] = useState(false)
  const [success, setSuccess] = useState<number[] | null>(null)
  const [busyAll, setBusyAll] = useState(false)
  const [message, setMessage] = useState('')
  const [askRelease, setAskRelease] = useState<number | null>(null)
  const holdsRef = useRef<HoldView[]>([])
  const seq = useRef(0)

  const announce = useCallback((text: string) => {
    setMessage('')
    window.setTimeout(() => setMessage(text), 30)
  }, [])

  const setPendingFor = useCallback((id: number, p: Pending | null) => {
    setPending((prev) => {
      const next = new Map(prev)
      if (p) next.set(id, p)
      else next.delete(id)
      return next
    })
  }, [])

  /** Reads the seat map and my holds. Answers that arrive out of order are
   * dropped, so the screen always shows the latest server answer. */
  const refresh = useCallback(async () => {
    const mine = ++seq.current
    try {
      const [m, h] = await Promise.all([api.seatMap(eventId), api.myHolds(eventId)])
      if (mine !== seq.current) return
      const receivedAt = performance.now()
      const next = h.holds.map((x) => ({ ...x, serverNow: h.now, receivedAt }))
      // A hold the server now reports inactive was active a moment ago:
      // tell the visitor (the server decided; we only noticed).
      for (const x of next) {
        const before = holdsRef.current.find((p) => p.seat_id === x.seat_id)
        if (before?.active && !x.active) {
          const seat = m.seats.find((s) => s.seat_id === x.seat_id)
          const name = seat ? seatLabel(seat) : `seat ${x.seat_id}`
          toast.warning(`Your hold on ${name} expired`, { description: 'Holds last 10 minutes. Hold it again if it is still free.' })
          announce(`Your hold on ${name} expired.`)
        }
      }
      holdsRef.current = next
      setMap(m)
      setHolds(next)
      setStale(false)
      setLoad({ status: 'ready' })
    } catch (e) {
      if (mine !== seq.current) return
      setLoad((prev) => (prev.status === 'ready' ? prev : { status: 'error', error: e }))
      setStale(true)
    }
  }, [eventId, announce])

  useEffect(() => {
    setLoad({ status: 'loading' })
    setMap(null)
    holdsRef.current = []
    setHolds([])
    refresh()
    const id = window.setInterval(() => document.visibilityState === 'visible' && refresh(), POLL_MS)
    const onVisible = () => document.visibilityState === 'visible' && refresh()
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      window.clearInterval(id)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [refresh])

  useEffect(() => {
    if (map) document.title = `${map.event.name} · Seatlane`
  }, [map])

  const fail = useCallback(
    (e: unknown, what: string) => {
      const copy = errorCopy(e)
      const id = errorId(e)
      toast.error(copy.title, { id: `err-${id}-${what}`, description: copy.description })
      announce(`${what}: ${copy.title}. ${copy.description}`)
    },
    [announce],
  )

  const seatById = useMemo(() => new Map((map?.seats ?? []).map((s) => [s.seat_id, s])), [map])
  const nameOf = useCallback((id: number) => {
    const s = seatById.get(id)
    return s ? `${s.section} ${seatLabel(s)}` : `Seat ${id}`
  }, [seatById])

  const hold = useCallback(
    async (seatId: number) => {
      setPendingFor(seatId, 'hold')
      try {
        const res = await api.hold(seatId)
        // Show the server's answer at once; the refresh below reconciles.
        const view: HoldView = { seat_id: res.seat_id, held_at: res.held_at, expires_at: res.expires_at, active: true, serverNow: res.now, receivedAt: performance.now() }
        holdsRef.current = [view, ...holdsRef.current.filter((h) => h.seat_id !== seatId)]
        setHolds(holdsRef.current)
        announce(`${nameOf(seatId)} is held for you. You have ${Math.round((res.expires_at - res.now) / 60)} minutes to confirm.`)
      } catch (e) {
        fail(e, `Could not hold ${nameOf(seatId)}`)
      } finally {
        await refresh()
        setPendingFor(seatId, null)
      }
    },
    [announce, fail, nameOf, refresh, setPendingFor],
  )

  const release = useCallback(
    async (seatId: number) => {
      setPendingFor(seatId, 'release')
      try {
        await api.release(seatId)
        announce(`${nameOf(seatId)} released.`)
        toast(`${nameOf(seatId)} released`, { description: 'The seat is back on sale.' })
      } catch (e) {
        fail(e, `Could not release ${nameOf(seatId)}`)
      } finally {
        await refresh()
        setPendingFor(seatId, null)
      }
    },
    [announce, fail, nameOf, refresh, setPendingFor],
  )

  const confirmSeats = useCallback(
    async (ids: number[]) => {
      const sold: number[] = []
      for (const id of ids) {
        setPendingFor(id, 'confirm')
        try {
          await api.confirm(id)
          sold.push(id)
        } catch (e) {
          fail(e, `Could not confirm ${nameOf(id)}`)
        }
      }
      await refresh()
      ids.forEach((id) => setPendingFor(id, null))
      if (sold.length) {
        setSuccess(sold)
        announce(`Booked: ${sold.map(nameOf).join(', ')}.`)
      }
    },
    [announce, fail, nameOf, refresh, setPendingFor],
  )

  // "Confirm all N seats": ONE confirm_holds call with every seat shown in
  // the panel (live and expired: the server decides each one). The server
  // books all of them together or none: on any failure (F2 a hold ran out,
  // F13 a seat is not yours) nothing is booked, and the toast says so.
  const confirmAll = useCallback(async () => {
    const ids = [...holdsRef.current].map((h) => h.seat_id).sort((a, b) => a - b)
    if (ids.length === 0) return
    setBusyAll(true)
    ids.forEach((id) => setPendingFor(id, 'confirm'))
    try {
      await api.confirmAll(ids)
      setSuccess(ids)
      announce(`Booked together: ${ids.map(nameOf).join(', ')}.`)
    } catch (e) {
      const id = errorId(e)
      const copy = groupCopyFor(id)
      toast.error(copy.title, { id: `err-${id}-confirm-all`, description: copy.description })
      announce(`Could not confirm your ${ids.length === 1 ? 'seat' : `${ids.length} seats`}: ${copy.title}. ${copy.description}`)
    } finally {
      await refresh()
      ids.forEach((id) => setPendingFor(id, null))
      setBusyAll(false)
    }
  }, [announce, nameOf, refresh, setPendingFor])

  // Tapping your own held seat on a touch screen asks first (a stray tap
  // while scrolling must not give a seat away); a mouse click or Enter
  // releases at once, as the tooltip says.
  const onSeat = useCallback(
    (s: Seat, via: { touch: boolean }) => {
      if (s.available) hold(s.seat_id)
      else if (s.held_by_me && via.touch) setAskRelease(s.seat_id)
      else if (s.held_by_me) release(s.seat_id)
    },
    [hold, release],
  )

  // Countdown: tick while I hold anything; refetch as soon as an active hold
  // reaches zero (the server decides whether it has really expired).
  const t = useTicker(holds.length > 0)
  const rows: HoldRow[] = useMemo(
    () =>
      holds
        .map((h) => ({
          seatId: h.seat_id,
          seat: seatById.get(h.seat_id),
          active: h.active,
          remaining: Math.max(0, h.expires_at - h.serverNow - Math.max(0, t - h.receivedAt) / 1000),
          total: h.expires_at - h.held_at,
        }))
        .sort((a, b) => Number(b.active) - Number(a.active) || a.remaining - b.remaining || a.seatId - b.seatId),
    [holds, seatById, t],
  )
  const fired = useRef(new Set<string>())
  useEffect(() => {
    for (const h of holds) {
      const key = `${h.seat_id}:${h.expires_at}:${h.serverNow}`
      const left = h.expires_at - h.serverNow - Math.max(0, t - h.receivedAt) / 1000
      if (h.active && left <= 0 && !fired.current.has(key)) {
        fired.current.add(key)
        refresh()
      }
    }
  }, [t, holds, refresh])

  const tickets = useMemo(() => (map?.seats ?? []).filter((s) => s.sold_to_me).sort((a, b) => a.section_rank - b.section_rank || a.row.localeCompare(b.row) || a.number - b.number), [map])
  const successSeats = useMemo(() => (success ?? []).map((id) => seatById.get(id)).filter((s): s is Seat => !!s), [success, seatById])
  const soonest = rows.find((r) => r.active)

  if (load.status === 'loading' || (load.status === 'ready' && !map)) return <EventSkeleton />
  if (load.status === 'error') {
    const id = errorId(load.error)
    const copy = errorCopy(load.error)
    return (
      <main className="mx-auto max-w-7xl px-4 py-16 sm:px-6">
        <StateCard
          icon={id === 'F10' ? SearchX : id === 'network' ? WifiOff : TicketX}
          tone={id === 'F10' ? 'muted' : 'error'}
          title={copy.title}
          action={
            <div className="flex gap-2">
              <Button asChild variant="outline"><Link to="/"><ArrowLeft /> All events</Link></Button>
              {id !== 'F10' && <Button onClick={() => { setLoad({ status: 'loading' }); refresh() }}><RefreshCw /> Try again</Button>}
            </div>
          }
        >
          {copy.description}
        </StateCard>
      </main>
    )
  }

  const e = map!.event
  return (
    <main className="mx-auto max-w-7xl px-4 pb-28 sm:px-6 lg:pb-16">
      <div aria-live="polite" aria-atomic="true" className="sr-only" data-testid="live">{message}</div>

      <motion.section initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="pt-6">
        <Link to="/" className="inline-flex items-center gap-1.5 rounded-md text-sm text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50">
          <ArrowLeft className="size-4" aria-hidden /> All events
        </Link>
        <EventArt id={e.event_id} className="mt-3 rounded-2xl">
          <div className="relative flex flex-col gap-4 p-5 text-white sm:flex-row sm:items-end sm:justify-between sm:p-7">
            <div className="max-w-2xl">
              <h1 className="text-2xl font-semibold tracking-tight text-balance drop-shadow-sm sm:text-3xl">{e.name}</h1>
              <p className="mt-1.5 text-sm text-white/85 sm:text-base">{e.tagline}</p>
              <div className="mt-4 flex flex-wrap gap-x-5 gap-y-1.5 text-sm text-white/90">
                <span className="flex items-center gap-1.5"><MapPin className="size-4" aria-hidden />{e.venue}, {e.city}</span>
                <span className="flex items-center gap-1.5"><CalendarDays className="size-4" aria-hidden />{formatDate(e.starts_at)}</span>
                <span className="flex items-center gap-1.5"><Clock3 className="size-4" aria-hidden />{formatTime(e.starts_at)}</span>
              </div>
            </div>
            <div className="shrink-0 self-start rounded-xl bg-black/30 px-4 py-2.5 backdrop-blur sm:self-end">
              <div className="text-[11px] font-medium tracking-wide text-white/70 uppercase">Tickets</div>
              <div className="text-lg font-semibold tabular">{formatRange(e.from_price, e.to_price)}</div>
            </div>
          </div>
        </EventArt>
      </motion.section>

      <div className="mt-6 grid grid-cols-[minmax(0,1fr)] items-start gap-6 lg:grid-cols-[minmax(0,1fr)_370px]">
        <section aria-labelledby="map-title" className="rounded-2xl border bg-card p-3 shadow-sm sm:p-6" data-testid="seat-map">
          <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
            <h2 id="map-title" className="font-semibold tracking-tight">Choose your seats</h2>
            {stale && (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-500/15 px-2.5 py-1 text-xs font-medium text-amber-700 dark:text-amber-300">
                <WifiOff className="size-3.5" aria-hidden /> Reconnecting…
              </span>
            )}
          </div>
          <SeatMap seats={map!.seats} pending={pending} onActivate={onSeat} />
          <div className="mt-6 border-t pt-4">
            <Legend seats={map!.seats} />
          </div>
        </section>

        <aside className="lg:sticky lg:top-20">
          <CheckoutPanel
            holds={rows}
            tickets={tickets}
            pending={pending}
            busy={busyAll}
            onConfirm={(id) => confirmSeats([id])}
            onConfirmAll={confirmAll}
            onRelease={release}
            onHoldAgain={hold}
          />
        </aside>
      </div>

      {soonest && (
        <motion.div initial={{ y: 80 }} animate={{ y: 0 }} className="fixed inset-x-0 bottom-0 z-30 border-t bg-background/85 px-4 py-3 backdrop-blur-xl lg:hidden" data-testid="mobile-bar">
          <div className="mx-auto flex max-w-7xl items-center justify-between gap-3">
            <div className="text-sm">
              <span className="font-semibold">{rows.filter((r) => r.active).length} held</span>
              <span className="text-muted-foreground tabular"> · next ends in {clock(soonest.remaining)}</span>
            </div>
            <Button size="lg" className="h-10" onClick={() => document.getElementById('checkout')?.scrollIntoView({ behavior: 'smooth', block: 'start' })}>
              Review
            </Button>
          </div>
        </motion.div>
      )}

      <AlertDialog open={askRelease !== null} onOpenChange={(open) => !open && setAskRelease(null)}>
        <AlertDialogContent data-testid="release-confirm">
          <AlertDialogHeader>
            <AlertDialogTitle>Release {askRelease !== null ? nameOf(askRelease) : 'this seat'}?</AlertDialogTitle>
            <AlertDialogDescription>
              It goes back on sale straight away, and someone else can take it.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel data-testid="release-keep">Keep it</AlertDialogCancel>
            <AlertDialogAction
              data-testid="release-yes"
              onClick={() => {
                if (askRelease !== null) release(askRelease)
                setAskRelease(null)
              }}
            >
              Release
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <SuccessState open={successSeats.length > 0} event={e} seats={successSeats} onClose={() => setSuccess(null)} />
    </main>
  )
}

function EventSkeleton() {
  return (
    <main className="mx-auto max-w-7xl px-4 pb-16 sm:px-6" aria-busy="true" aria-label="Loading the seat map">
      <div className="pt-6">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="mt-3 h-44 rounded-2xl" />
      </div>
      <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_370px]">
        <div className="rounded-2xl border bg-card p-6">
          <Skeleton className="mx-auto h-10 w-full max-w-xl rounded-b-[50%]" />
          <div className="mt-8 flex flex-col items-center gap-2">
            {Array.from({ length: 9 }, (_, r) => (
              <div key={r} className="flex gap-1.5">
                {Array.from({ length: 14 }, (_, c) => (
                  <Skeleton key={c} className="size-5 rounded-md sm:size-7" style={{ animationDelay: `${(r + c) * 30}ms` }} />
                ))}
              </div>
            ))}
          </div>
        </div>
        <div className="space-y-3 rounded-2xl border bg-card p-5">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-16 w-full rounded-xl" />
          <Skeleton className="h-16 w-full rounded-xl" />
        </div>
      </div>
    </main>
  )
}
