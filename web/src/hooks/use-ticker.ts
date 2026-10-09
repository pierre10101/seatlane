import { useEffect, useState } from 'react'

/** Re-renders every `ms` while `on`, returning performance.now(). It is a
 * stopwatch, not a clock: countdowns subtract the time elapsed since the
 * server's answer arrived from the remaining time the server reported. */
export function useTicker(on: boolean, ms = 250): number {
  const [t, setT] = useState(() => performance.now())
  useEffect(() => {
    if (!on) return
    const id = window.setInterval(() => setT(performance.now()), ms)
    return () => window.clearInterval(id)
  }, [on, ms])
  return t
}
