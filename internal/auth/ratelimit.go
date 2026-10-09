package auth

// Sign-in rate limit (F18), per client IP + email. Pure: the stored count
// and the current time go in, the decision and the count to store come out.
// No clock is read here.

// Limits of the sign-in rate limit.
const (
	MaxFailures int64 = 5   // attempts that did not succeed, per window
	LimitWindow int64 = 900 // seconds; the window starts at its first attempt
)

// Attempts is one key's stored count.
type Attempts struct {
	WindowStart int64
	Failures    int64
}

// Reserve decides one sign-in attempt at now, given the key's stored count
// (zero if none). A window that started LimitWindow seconds or more before
// now is over: the count starts again at 0 with WindowStart = now. Then,
// with MaxFailures or more already counted, the attempt is refused
// (allowed = false, next = the window unchanged, retryAfter = the seconds
// until the window is over). Otherwise it is allowed and counted in advance
// (next.Failures + 1), so parallel guesses cannot pass the limit; a
// successful sign-in then clears the key.
func Reserve(prev Attempts, now int64) (next Attempts, allowed bool, retryAfter int64) {
	cur := prev
	if cur.WindowStart+LimitWindow <= now {
		cur = Attempts{WindowStart: now, Failures: 0}
	}
	if cur.Failures >= MaxFailures {
		return cur, false, cur.WindowStart + LimitWindow - now
	}
	return Attempts{WindowStart: cur.WindowStart, Failures: cur.Failures + 1}, true, 0
}

// limitKey is the rate-limit key of an IP and a normalized email.
func limitKey(ip, email string) string { return ip + "|" + email }
