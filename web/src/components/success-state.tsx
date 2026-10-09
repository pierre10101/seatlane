import { useEffect, useRef } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { Link } from 'react-router-dom'
import { CalendarDays, MapPin, PartyPopper } from 'lucide-react'
import type { EventCard, Seat } from '@/lib/api'
import { formatDate, formatMoney, formatTime, seatLabel } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { EventArt } from '@/components/event-art'

export function SuccessState({ open, event, seats, onClose }: { open: boolean; event?: EventCard; seats: Seat[]; onClose: () => void }) {
  const done = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!open) return
    done.current?.focus()
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])
  const total = seats.reduce((s, x) => s + x.price.cents, 0)
  return (
    <AnimatePresence>
      {open && event && (
        <motion.div className="fixed inset-0 z-50 grid place-items-center bg-background/70 p-4 backdrop-blur-sm" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={onClose}>
          <motion.div
            role="dialog"
            aria-modal="true"
            aria-labelledby="success-title"
            data-testid="success"
            className="w-full max-w-md overflow-hidden rounded-3xl border bg-card shadow-2xl"
            initial={{ opacity: 0, y: 24, scale: 0.94 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 12, scale: 0.97 }}
            transition={{ type: 'spring', stiffness: 260, damping: 22 }}
            onClick={(e) => e.stopPropagation()}
          >
            <EventArt id={event.event_id} className="h-28">
              <motion.div className="absolute inset-0 grid place-items-center" initial={{ scale: 0, rotate: -20 }} animate={{ scale: 1, rotate: 0 }} transition={{ delay: 0.15, type: 'spring', stiffness: 300, damping: 14 }}>
                <span className="grid size-14 place-items-center rounded-full bg-white text-emerald-600 shadow-lg">
                  <PartyPopper className="size-7" aria-hidden />
                </span>
              </motion.div>
              <Confetti />
            </EventArt>
            <div className="space-y-4 p-6">
              <div className="text-center">
                <h2 id="success-title" className="text-xl font-semibold tracking-tight">You're going!</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {seats.length === 1 ? 'Your seat is booked' : `${seats.length} seats are booked`} for <span className="font-medium text-foreground">{event.name}</span>.
                </p>
              </div>
              <div className="relative rounded-2xl border border-dashed bg-muted/40 p-4">
                <span className="absolute top-1/2 -left-2.5 size-5 -translate-y-1/2 rounded-full border bg-card" aria-hidden />
                <span className="absolute top-1/2 -right-2.5 size-5 -translate-y-1/2 rounded-full border bg-card" aria-hidden />
                <ul className="flex flex-wrap justify-center gap-1.5">
                  {seats.map((s) => (
                    <li key={s.seat_id} className="rounded-lg bg-seat-sold-mine/15 px-2.5 py-1 text-sm font-semibold">
                      {s.section} · {seatLabel(s)}
                    </li>
                  ))}
                </ul>
                <div className="mt-3 grid gap-1 text-xs text-muted-foreground">
                  <span className="flex items-center justify-center gap-1.5"><MapPin className="size-3.5" aria-hidden /> {event.venue}, {event.city}</span>
                  <span className="flex items-center justify-center gap-1.5"><CalendarDays className="size-3.5" aria-hidden /> {formatDate(event.starts_at)} · {formatTime(event.starts_at)}</span>
                </div>
                {seats.length > 0 && <div className="mt-3 text-center text-base font-semibold tabular">{formatMoney({ cents: total, currency: seats[0].price.currency })}</div>}
              </div>
              <div className="flex flex-col gap-2 sm:flex-row">
                <Button ref={done} className="h-10 flex-1" onClick={onClose} data-testid="success-done">Back to the seat map</Button>
                <Button asChild variant="outline" className="h-10 flex-1">
                  <Link to="/">More events</Link>
                </Button>
              </div>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

function Confetti() {
  const bits = Array.from({ length: 18 }, (_, i) => i)
  const colors = ['#fde047', '#f472b6', '#a78bfa', '#34d399', '#60a5fa']
  return (
    <div className="pointer-events-none absolute inset-0" aria-hidden>
      {bits.map((i) => (
        <motion.span
          key={i}
          className="absolute top-1/2 left-1/2 h-2 w-1 rounded-sm"
          style={{ background: colors[i % colors.length] }}
          initial={{ x: 0, y: 0, opacity: 1, rotate: 0 }}
          animate={{ x: Math.cos((i / bits.length) * Math.PI * 2) * (90 + (i % 3) * 30), y: Math.sin((i / bits.length) * Math.PI * 2) * (40 + (i % 4) * 12), opacity: 0, rotate: 180 + i * 20 }}
          transition={{ duration: 1.1, delay: 0.2, ease: 'easeOut' }}
        />
      ))}
    </div>
  )
}
