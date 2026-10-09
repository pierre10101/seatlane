import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import { ArrowRight, CalendarDays, Clock3, MapPin, RefreshCw, ShieldCheck, Ticket, TicketX, Timer } from 'lucide-react'
import { api, type EventCard } from '@/lib/api'
import { errorCopy } from '@/lib/errors'
import { dateParts, formatDate, formatRange, formatTime } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EventArt } from '@/components/event-art'
import { StateCard } from '@/components/states'

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ready'; events: EventCard[] }

export function EventsPage() {
  const [load, setLoad] = useState<Load>({ status: 'loading' })
  const fetchEvents = useCallback(() => {
    setLoad({ status: 'loading' })
    api.listEvents().then(
      (events) => setLoad({ status: 'ready', events }),
      (error) => setLoad({ status: 'error', error }),
    )
  }, [])
  useEffect(() => {
    document.title = 'Seatlane · Pick your seat'
    fetchEvents()
  }, [fetchEvents])

  return (
    <main className="mx-auto max-w-7xl px-4 pb-24 sm:px-6">
      <section className="relative pt-12 pb-10 sm:pt-16 sm:pb-14">
        <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.4 }} className="max-w-2xl">
          <span className="inline-flex items-center gap-1.5 rounded-full border bg-card/70 px-3 py-1 text-xs font-medium text-muted-foreground backdrop-blur">
            <Ticket className="size-3.5 text-primary" aria-hidden /> Live seat maps · South Africa
          </span>
          <h1 className="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-5xl">
            Pick your seat.{' '}
            <span className="bg-gradient-to-r from-primary to-[oklch(0.70_0.16_330)] bg-clip-text text-transparent sm:block">Keep it for ten minutes.</span>
          </h1>
          <p className="mt-4 max-w-xl text-base text-pretty text-muted-foreground sm:text-lg">
            Every seat you see is exactly what the box office sees. Hold one, take a breath, then confirm — nobody else can grab it while your timer runs.
          </p>
          <ul className="mt-6 flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted-foreground">
            <li className="flex items-center gap-2"><Timer className="size-4 text-primary" aria-hidden /> 10-minute holds</li>
            <li className="flex items-center gap-2"><ShieldCheck className="size-4 text-primary" aria-hidden /> No double bookings</li>
            <li className="flex items-center gap-2"><Ticket className="size-4 text-primary" aria-hidden /> Prices in rand</li>
          </ul>
        </motion.div>
      </section>

      <div className="flex items-baseline justify-between pb-4">
        <h2 className="text-lg font-semibold tracking-tight">On sale now</h2>
        {load.status === 'ready' && load.events.length > 0 && (
          <span className="text-sm text-muted-foreground">{load.events.length} events</span>
        )}
      </div>

      {load.status === 'loading' && <EventGridSkeleton />}
      {load.status === 'error' && (
        <StateCard
          icon={TicketX}
          tone="error"
          title={errorCopy(load.error).title}
          action={<Button variant="outline" onClick={fetchEvents}><RefreshCw /> Try again</Button>}
        >
          {errorCopy(load.error).description}
        </StateCard>
      )}
      {load.status === 'ready' && load.events.length === 0 && (
        <StateCard icon={CalendarDays} title="No events on sale right now">
          New shows are added every week. Check back soon.
        </StateCard>
      )}
      {load.status === 'ready' && load.events.length > 0 && (
        <ul className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3" data-testid="event-list">
          {load.events.map((e, i) => (
            <motion.li key={e.event_id} initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 * i, duration: 0.35 }}>
              <EventTile event={e} />
            </motion.li>
          ))}
        </ul>
      )}
    </main>
  )
}

function EventTile({ event: e }: { event: EventCard }) {
  const d = dateParts(e.starts_at)
  return (
    <Link
      to={`/events/${e.event_id}`}
      className="group flex h-full flex-col overflow-hidden rounded-2xl border bg-card shadow-sm ring-primary/40 transition-all duration-300 outline-none hover:-translate-y-1 hover:shadow-xl hover:shadow-primary/10 focus-visible:ring-3"
    >
      <EventArt id={e.event_id} className="h-36">
        <div className="absolute top-3 left-3 flex flex-col items-center rounded-xl bg-white/90 px-3 py-1.5 text-center text-neutral-900 shadow-sm backdrop-blur dark:bg-black/60 dark:text-white">
          <span className="text-[10px] font-semibold tracking-wider uppercase opacity-70">{d.month}</span>
          <span className="text-xl leading-none font-semibold tabular">{d.day}</span>
        </div>
        <span className="absolute right-3 bottom-3 rounded-full bg-black/45 px-2.5 py-1 text-xs font-medium text-white backdrop-blur">
          {e.city}
        </span>
      </EventArt>
      <div className="flex flex-1 flex-col gap-3 p-5">
        <div>
          <h3 className="text-base leading-snug font-semibold tracking-tight">{e.name}</h3>
          <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{e.tagline}</p>
        </div>
        <dl className="mt-auto grid gap-1.5 text-sm">
          <div className="flex items-center gap-2 text-muted-foreground">
            <dt className="sr-only">Venue</dt>
            <MapPin className="size-4 shrink-0" aria-hidden />
            <dd className="truncate">{e.venue}</dd>
          </div>
          <div className="flex items-center gap-2 text-muted-foreground">
            <dt className="sr-only">Date</dt>
            <CalendarDays className="size-4 shrink-0" aria-hidden />
            <dd>{formatDate(e.starts_at)}</dd>
          </div>
          <div className="flex items-center gap-2 text-muted-foreground">
            <dt className="sr-only">Time</dt>
            <Clock3 className="size-4 shrink-0" aria-hidden />
            <dd>{formatTime(e.starts_at)}</dd>
          </div>
        </dl>
        <div className="flex items-center justify-between border-t pt-4">
          <div>
            <div className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">Tickets</div>
            <div className="font-semibold tabular">{formatRange(e.from_price, e.to_price)}</div>
          </div>
          <span className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-3 py-1.5 text-sm font-medium text-primary transition-colors group-hover:bg-primary group-hover:text-primary-foreground">
            Choose seats <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" aria-hidden />
          </span>
        </div>
      </div>
    </Link>
  )
}

function EventGridSkeleton() {
  return (
    <ul className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3" aria-busy="true" aria-label="Loading events">
      {[0, 1, 2].map((i) => (
        <li key={i} className="overflow-hidden rounded-2xl border bg-card">
          <Skeleton className="h-36 rounded-none" />
          <div className="space-y-3 p-5">
            <Skeleton className="h-5 w-3/4" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-2/3" />
            <div className="space-y-2 pt-2">
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-4 w-2/5" />
            </div>
            <div className="flex justify-between border-t pt-4">
              <Skeleton className="h-8 w-24" />
              <Skeleton className="h-8 w-28 rounded-full" />
            </div>
          </div>
        </li>
      ))}
    </ul>
  )
}
