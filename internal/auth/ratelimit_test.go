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
