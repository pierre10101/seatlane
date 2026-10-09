import { ApiError } from './api'

export type Copy = { title: string; description: string }

// Friendly copy per failure ID (docs/failures.md). Keyed on error.id only:
// the server's message text is never read.
const COPY: Record<string, Copy> = {
  F1: { title: 'That seat is already held', description: 'Someone is holding it right now. Pick another seat, or check back in a few minutes.' },
  F2: { title: 'Your hold ran out', description: 'Holds last 10 minutes. If the seat is still free, you can hold it again.' },
  F3: { title: 'Nothing to confirm', description: 'This seat is not on hold any more. Hold it again to book it.' },
  F4: { title: 'That hold belongs to someone else', description: 'Only the person holding a seat can confirm it.' },
  F5: { title: 'Already yours', description: 'You have already booked this seat. It is in your tickets.' },
  F6: { title: 'Just sold', description: 'Someone else booked this seat a moment ago. Try one nearby.' },
  F7: { title: 'Seat not found', description: 'This seat is not part of the venue any more. Refresh to see the latest map.' },
  F8: { title: 'Your session has ended', description: 'Refresh the page to start a new one. Your browser needs to accept cookies.' },
  F9: { title: 'No hold to release', description: 'You are not holding this seat any more, so there is nothing to give back.' },
  F10: { title: 'Event not found', description: 'This event does not exist or is no longer on sale.' },
  F11: { title: 'Could not load that list', description: 'The page size was not accepted. Refresh and try again.' },
  F12: { title: 'Could not load that list', description: 'The list position was not accepted. Refresh and try again.' },
  F13: { title: 'A seat in your list is not yours any more', description: 'Someone else holds it, it was released or it is already booked. Release it and try again.' },
  bad_request: { title: 'That request did not go through', description: 'Something about it was not accepted. Refresh and try again.' },
  internal: { title: 'Something went wrong on our side', description: 'Nothing was changed. Please try again in a moment.' },
  network: { title: 'You seem to be offline', description: 'We could not reach Seatlane. Check your connection and try again.' },
}

export function errorId(e: unknown): string {
  return e instanceof ApiError ? e.id : 'internal'
}

export function errorCopy(e: unknown): Copy {
  return copyFor(errorId(e))
}

export function copyFor(id: string): Copy {
  return COPY[id] ?? COPY.internal
}

// "Confirm all N seats" (confirm_holds) books every seat together or none:
// its failures say that nothing was booked.
const GROUP_COPY: Record<string, Copy> = {
  F2: { title: 'A hold in your list ran out', description: 'Nothing was booked. Hold that seat again, or release it, then confirm all again.' },
  F13: { title: 'A seat in your list is not yours any more', description: 'Nothing was booked. Someone else holds it, it was released or it is already booked. Release it, then confirm all again.' },
}

export function groupCopyFor(id: string): Copy {
  const c = GROUP_COPY[id] ?? copyFor(id)
  // Offline: the answer never arrived, so the refresh after it shows what happened.
  if (id === 'network') return c
  return c.description.startsWith('Nothing was booked') ? c : { title: c.title, description: `Nothing was booked. ${c.description}` }
}
