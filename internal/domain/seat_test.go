package domain

import "testing"

// Exactly one state is true for every combination, and expiry is exclusive:
// a hold whose expires_at equals now has expired.
func TestExactlyOneSeatState(t *testing.T) {
	const me, other, now = 11, 22, 1000
	for _, heldBy := range []int64{0, me, other} {
		for _, expires := range []int64{0, now - 1, now, now + 1} {
			for _, soldTo := range []int64{0, me, other} {
				got := []bool{
					IsAvailable(heldBy, expires, soldTo, now),
					IsHeldBy(heldBy, expires, soldTo, me, now),
					IsHeldByOther(heldBy, expires, soldTo, me, now),
					IsSoldTo(soldTo, me),
					IsSoldToOther(soldTo, me),
				}
				n := 0
				for _, b := range got {
					if b {
						n++
					}
				}
				if n != 1 {
					t.Errorf("held_by %d expires %d sold_to %d: %v", heldBy, expires, soldTo, got)
				}
			}
		}
	}
	if IsHeldBy(me, now, 0, me, now) || !IsAvailable(me, now, 0, now) {
		t.Fatal("a hold expiring exactly now has expired")
	}
}
