import type { Money } from './api'

const TZ = 'Africa/Johannesburg' // the venues' time zone

const moneyFormats = new Map<string, Intl.NumberFormat>()

export function formatMoney(m: Money): string {
  const whole = m.cents % 100 === 0
  const key = `${m.currency}:${whole}`
  let f = moneyFormats.get(key)
  if (!f) {
    f = new Intl.NumberFormat('en-ZA', {
      style: 'currency',
      currency: m.currency,
      minimumFractionDigits: whole ? 0 : 2,
      maximumFractionDigits: whole ? 0 : 2,
    })
    moneyFormats.set(key, f)
  }
  return f.format(m.cents / 100)
}

export function formatRange(from: Money, to: Money): string {
  return from.cents === to.cents ? formatMoney(from) : `${formatMoney(from)} – ${formatMoney(to)}`
}

const dateLong = new Intl.DateTimeFormat('en-ZA', { timeZone: TZ, weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
const time = new Intl.DateTimeFormat('en-ZA', { timeZone: TZ, hour: '2-digit', minute: '2-digit', hour12: false })
const day = new Intl.DateTimeFormat('en-ZA', { timeZone: TZ, day: 'numeric' })
const month = new Intl.DateTimeFormat('en-ZA', { timeZone: TZ, month: 'short' })
const weekday = new Intl.DateTimeFormat('en-ZA', { timeZone: TZ, weekday: 'short' })

export const formatDate = (unix: number) => dateLong.format(unix * 1000)
export const formatTime = (unix: number) => time.format(unix * 1000)
export const dateParts = (unix: number) => ({
  day: day.format(unix * 1000),
  month: month.format(unix * 1000).replace('.', ''),
  weekday: weekday.format(unix * 1000),
})

/** mm:ss for a number of seconds (>= 0). */
export function clock(seconds: number): string {
  const s = Math.max(0, Math.ceil(seconds))
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
}

export const seatLabel = (s: { row: string; number: number }) => `${s.row}${s.number}`
