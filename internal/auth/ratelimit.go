package auth

// Sign-in rate limit (F18), two caps checked together: per client IP + email
// and per client IP across all emails. Pure: the stored counts and the
// current time go in, the decision and the counts to store come out. No
// clock is read here. There is deliberately no per-email cap across IPs (it
// would let anyone lock a victim out from anywhere).

// Limits of the sign-in rate limit.
const (
	MaxFailures      int64 = 5   // per IP + email: attempts that did not succeed, per window
	MaxFailuresPerIP int64 = 20  // per IP, across all emails, per window
	LimitWindow      int64 = 900 // seconds; each key's window starts at its first attempt
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
	return reserve(prev, now, MaxFailures)
}

// ReserveIP is Reserve for the per-IP count, with MaxFailuresPerIP.
func ReserveIP(prev Attempts, now int64) (next Attempts, allowed bool, retryAfter int64) {
	return reserve(prev, now, MaxFailuresPerIP)
}

func reserve(prev Attempts, now, max int64) (next Attempts, allowed bool, retryAfter int64) {
	cur := prev
	if cur.WindowStart+LimitWindow <= now {
		cur = Attempts{WindowStart: now, Failures: 0}
	}
	if cur.Failures >= max {
		return cur, false, cur.WindowStart + LimitWindow - now
	}
	return Attempts{WindowStart: cur.WindowStart, Failures: cur.Failures + 1}, true, 0
}

// ReserveBoth decides one sign-in attempt at now against both caps: pair is
// the IP + email count, ip the IP's count across all emails. The attempt is
// allowed only if both caps allow it, and then both counts go up by one (in
// advance). If either cap refuses, neither count is raised (the returned
// counts are not to be stored) and retryAfter is the seconds until every
// refusing cap's window is over, so a retry after it is not refused again by
// the same caps.
func ReserveBoth(pair, ip Attempts, now int64) (nextPair, nextIP Attempts, allowed bool, retryAfter int64) {
	nextPair, okPair, waitPair := Reserve(pair, now)
	nextIP, okIP, waitIP := ReserveIP(ip, now)
	if okPair && okIP {
		return nextPair, nextIP, true, 0
	}
	return pair, ip, false, max(waitPair, waitIP)
}

// Refund gives back the attempt reserved in the per-IP count when it turns
// out to be a successful sign-in, so only failures use up the IP's budget
// (clearing the whole IP count instead would let one known password reset
// the cap). cur is the stored count now, reserved the count saved by
// ReserveBoth. If the window has started again since, there is nothing to
// give back.
func Refund(cur, reserved Attempts) Attempts {
	if cur.WindowStart != reserved.WindowStart || cur.Failures <= 0 {
		return cur
	}
	return Attempts{WindowStart: cur.WindowStart, Failures: cur.Failures - 1}
}

// limitKey is the rate-limit key of an IP and a normalized email.
func limitKey(ip, email string) string { return ip + "|" + email }

// ipLimitKey is the rate-limit key of an IP across all emails. It has no
// "|", so it never equals a limitKey.
func ipLimitKey(ip string) string { return "ip:" + ip }
