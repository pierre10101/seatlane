package auth

import "testing"

// Reserve is pure: the stored count and now in, the decision out.
func TestReserveBoundaries(t *testing.T) {
	const w0 = 1000
	for _, c := range []struct {
		prev    Attempts
		now     int64
		next    Attempts
		allowed bool
		retry   int64
	}{
		{Attempts{}, w0, Attempts{w0, 1}, true, 0},                    // no row: a new window
		{Attempts{w0, 4}, w0 + 899, Attempts{w0, 5}, true, 0},         // 5th attempt allowed
		{Attempts{w0, 5}, w0 + 1, Attempts{w0, 5}, false, 899},        // 6th refused
		{Attempts{w0, 5}, w0 + 899, Attempts{w0, 5}, false, 1},        // still refused one second before the end
		{Attempts{w0, 5}, w0 + 900, Attempts{w0 + 900, 1}, true, 0},   // window over exactly at +900
		{Attempts{w0, 2}, w0 + 5000, Attempts{w0 + 5000, 1}, true, 0}, // old window forgotten
	} {
		next, ok, retry := Reserve(c.prev, c.now)
		if next != c.next || ok != c.allowed || retry != c.retry {
			t.Errorf("Reserve(%+v, %d) = %+v %v %d, want %+v %v %d", c.prev, c.now, next, ok, retry, c.next, c.allowed, c.retry)
		}
	}
}

// The per-IP cap: 20 per window instead of 5, same window rule.
func TestReserveIPBoundaries(t *testing.T) {
	const w0 = 1000
	for _, c := range []struct {
		prev    Attempts
		now     int64
		next    Attempts
		allowed bool
		retry   int64
	}{
		{Attempts{}, w0, Attempts{w0, 1}, true, 0},
		{Attempts{w0, 5}, w0 + 1, Attempts{w0, 6}, true, 0},          // the pair cap's limit is not this one's
		{Attempts{w0, 19}, w0 + 899, Attempts{w0, 20}, true, 0},      // 20th allowed
		{Attempts{w0, 20}, w0 + 1, Attempts{w0, 20}, false, 899},     // 21st refused
		{Attempts{w0, 20}, w0 + 899, Attempts{w0, 20}, false, 1},     // still refused at +899
		{Attempts{w0, 20}, w0 + 900, Attempts{w0 + 900, 1}, true, 0}, // allowed again at +900
	} {
		next, ok, retry := ReserveIP(c.prev, c.now)
		if next != c.next || ok != c.allowed || retry != c.retry {
			t.Errorf("ReserveIP(%+v, %d) = %+v %v %d, want %+v %v %d", c.prev, c.now, next, ok, retry, c.next, c.allowed, c.retry)
		}
	}
}

// Both caps must allow; a refusal raises neither count; Retry-After covers
// every refusing cap.
func TestReserveBoth(t *testing.T) {
	const w0 = 1000
	for name, c := range map[string]struct {
		pair, ip, nextPair, nextIP Attempts
		now                        int64
		allowed                    bool
		retry                      int64
	}{
		"both allow: both counted": {Attempts{w0, 1}, Attempts{w0, 7}, Attempts{w0, 2}, Attempts{w0, 8}, w0 + 10, true, 0},
		"pair cap refuses":         {Attempts{w0, 5}, Attempts{w0, 7}, Attempts{w0, 5}, Attempts{w0, 7}, w0 + 10, false, 890},
		"ip cap refuses":           {Attempts{}, Attempts{w0, 20}, Attempts{}, Attempts{w0, 20}, w0 + 10, false, 890},
		"both refuse: the later":   {Attempts{w0 + 300, 5}, Attempts{w0, 20}, Attempts{w0 + 300, 5}, Attempts{w0, 20}, w0 + 400, false, 800},
		"ip window over at +900":   {Attempts{}, Attempts{w0, 20}, Attempts{w0 + 900, 1}, Attempts{w0 + 900, 1}, w0 + 900, true, 0},
	} {
		np, ni, ok, retry := ReserveBoth(c.pair, c.ip, c.now)
		if np != c.nextPair || ni != c.nextIP || ok != c.allowed || retry != c.retry {
			t.Errorf("%s: ReserveBoth = %+v %+v %v %d, want %+v %+v %v %d", name, np, ni, ok, retry, c.nextPair, c.nextIP, c.allowed, c.retry)
		}
	}
}

// A successful sign-in gives back only its own reservation, in its window.
func TestRefund(t *testing.T) {
	const w0 = 1000
	for _, c := range []struct{ cur, reserved, want Attempts }{
		{Attempts{w0, 8}, Attempts{w0, 3}, Attempts{w0, 7}},              // others failed since: only one given back
		{Attempts{w0, 1}, Attempts{w0, 1}, Attempts{w0, 0}},              //
		{Attempts{w0 + 900, 1}, Attempts{w0, 20}, Attempts{w0 + 900, 1}}, // a new window: nothing to give back
		{Attempts{}, Attempts{w0, 1}, Attempts{}},                        // no row
		{Attempts{w0, 0}, Attempts{w0, 1}, Attempts{w0, 0}},              // never below 0
	} {
		if got := Refund(c.cur, c.reserved); got != c.want {
			t.Errorf("Refund(%+v, %+v) = %+v, want %+v", c.cur, c.reserved, got, c.want)
		}
	}
}
