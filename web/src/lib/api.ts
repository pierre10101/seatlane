// The API client. It only moves JSON: every seat, hold and sign-in rule
// lives on the server. The sign-in session is the HttpOnly cookie
// `seatlane_auth`, which the server sets; this code never reads or sends it.
// The one cookie it reads is `seatlane_csrf`, which it echoes in the
// X-CSRF-Token header on every state-changing request (CSRF protection).

export type Money = { cents: number; currency: string }

export type EventCard = {
  event_id: number
  name: string
  venue: string
  city: string
  starts_at: number
  tagline: string
  from_price: Money
  to_price: Money
}

export type Seat = {
  seat_id: number
  section: string
  section_rank: number
  row: string
  number: number
  price: Money
  available: boolean
  held_by_me: boolean
  held_by_other: boolean
  sold_to_me: boolean
  sold_to_other: boolean
}

export type MyHold = { seat_id: number; held_at: number; expires_at: number; active: boolean }

export type SeatMap = { event: EventCard; seats: Seat[]; now: number }
export type MyHolds = { holds: MyHold[]; now: number }
export type HoldAnswer = { seat_id: number; held_at: number; expires_at: number; now: number }
export type ConfirmAnswer = { seat_id: number; sold_at: number; now: number }
export type ConfirmAllAnswer = { confirmed: number; sold_at: number; now: number }
export type ReleaseAnswer = { seat_id: number; released_at: number; now: number }
export type Role = 'customer' | 'organizer' | 'admin'
export type Account = { user_id: number; email: string; role: Role }

/** An error answer. `id` is the server's `error.id` (F1..F19, bad_request,
 * unauthorized, forbidden, internal) or `network` when the server could not
 * be reached. The UI maps
 * the id to copy; it never reads the server's message. */
export class ApiError extends Error {
  readonly id: string
  readonly status: number
  constructor(id: string, status: number) {
    super(id)
    this.id = id
    this.status = status
  }
}

function csrfToken(): string {
  const m = document.cookie.match(/(?:^|;\s*)seatlane_csrf=([^;]*)/)
  return m ? decodeURIComponent(m[1]) : ''
}

async function request<T>(path: string, init: RequestInit = {}, retried = false): Promise<T> {
  let res: Response
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (init.body) headers['Content-Type'] = 'application/json'
  if (init.method && init.method !== 'GET') headers['X-CSRF-Token'] = csrfToken()
  try {
    res = await fetch(path, { credentials: 'same-origin', ...init, headers })
  } catch {
    throw new ApiError('network', 0)
  }
  if (res.ok) return (await res.json()) as T
  let id = 'internal'
  try {
    const body = (await res.json()) as { error?: { id?: unknown } }
    if (typeof body?.error?.id === 'string') id = body.error.id
  } catch {
    // not JSON: keep "internal"
  }
  // F19: the CSRF token was missing or stale (the page did not come from the
  // Go server, or the server restarted with a new key). The server set a
  // fresh token on this very answer, so one retry carries it.
  if (id === 'F19' && !retried) return request<T>(path, init, true)
  throw new ApiError(id, res.status)
}

const PAGE = 100 // page.MaxPageSize

export const api = {
  async listEvents(): Promise<EventCard[]> {
    const all: EventCard[] = []
    let after = 0
    do {
      const q = new URLSearchParams({ limit: String(PAGE) })
      if (after) q.set('after', String(after))
      const page = await request<{ events: EventCard[]; next_after: number }>(`/api/events?${q}`)
      all.push(...page.events)
      after = page.next_after
    } while (after)
    return all
  },

  /** Every page of the seat map. `now` is the server time of the last page. */
  async seatMap(eventId: number): Promise<SeatMap> {
    const seats: Seat[] = []
    let after = 0
    let last: { event: EventCard; seats: Seat[]; next_after: number; now: number }
    do {
      const q = new URLSearchParams({ limit: String(PAGE) })
      if (after) q.set('after', String(after))
      last = await request(`/api/events/${eventId}/seats?${q}`)
      seats.push(...last.seats)
      after = last.next_after
    } while (after)
    return { event: last.event, seats, now: last.now }
  },

  async myHolds(eventId: number): Promise<MyHolds> {
    const holds: MyHold[] = []
    let after = 0
    let now = 0
    do {
      const q = new URLSearchParams({ limit: String(PAGE) })
      if (after) q.set('after', String(after))
      const page = await request<{ holds: MyHold[]; next_after: number; now: number }>(`/api/events/${eventId}/holds?${q}`)
      holds.push(...page.holds)
      after = page.next_after
      now = page.now
    } while (after)
    return { holds, now }
  },

  /** Who is signed in; ApiError "unauthorized" when nobody is. */
  me: () => request<Account>('/api/me'),
  signUp: (email: string, password: string) => request<Account>('/api/sign-up', { method: 'POST', body: JSON.stringify({ email, password }) }),
  signIn: (email: string, password: string) => request<Account>('/api/sign-in', { method: 'POST', body: JSON.stringify({ email, password }) }),
  signOut: () => request<{ signed_out: boolean }>('/api/sign-out', { method: 'POST' }),

  hold: (seatId: number) => request<HoldAnswer>('/api/holds', { method: 'POST', body: JSON.stringify({ seat_id: seatId }) }),
  confirm: (seatId: number) => request<ConfirmAnswer>('/api/holds/confirm', { method: 'POST', body: JSON.stringify({ seat_id: seatId }) }),
  /** confirm_holds: every listed seat is booked together, or none is. */
  confirmAll: (seatIds: number[]) => request<ConfirmAllAnswer>('/api/holds/confirm-all', { method: 'POST', body: JSON.stringify({ seat_ids: seatIds }) }),
  release: (seatId: number) => request<ReleaseAnswer>('/api/holds/release', { method: 'POST', body: JSON.stringify({ seat_id: seatId }) }),
}
