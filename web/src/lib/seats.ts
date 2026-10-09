import type { Seat } from './api'

/** The five states, one per server flag. The UI never derives a state from
 * anything else: exactly one flag is true in every seat the server sends. */
export type SeatState = 'available' | 'mine' | 'other' | 'sold-mine' | 'sold-other'

export function seatState(s: Seat): SeatState {
  if (s.available) return 'available'
  if (s.held_by_me) return 'mine'
  if (s.held_by_other) return 'other'
  if (s.sold_to_me) return 'sold-mine'
  return 'sold-other'
}

export const STATE_TEXT: Record<SeatState, string> = {
  available: 'Available',
  mine: 'Held by you',
  other: 'Held by someone else',
  'sold-mine': 'Booked by you',
  'sold-other': 'Sold',
}

export type Section = { name: string; rank: number; rows: { label: string; seats: Seat[] }[] }

/** Groups seats for drawing: sections nearest the stage first, rows in
 * label order, seats by number. Pure layout; no seat rules. */
export function layout(seats: Seat[]): Section[] {
  const bySection = new Map<string, Section>()
  for (const s of seats) {
    const key = `${s.section_rank}\u0000${s.section}`
    let sec = bySection.get(key)
    if (!sec) bySection.set(key, (sec = { name: s.section, rank: s.section_rank, rows: [] }))
    let row = sec.rows.find((r) => r.label === s.row)
    if (!row) sec.rows.push((row = { label: s.row, seats: [] }))
    row.seats.push(s)
  }
  const sections = [...bySection.values()].sort((a, b) => a.rank - b.rank || a.name.localeCompare(b.name))
  for (const sec of sections) {
    sec.rows.sort((a, b) => a.label.localeCompare(b.label, 'en', { numeric: true }))
    for (const r of sec.rows) r.seats.sort((a, b) => a.number - b.number)
  }
  return sections
}
